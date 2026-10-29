package importacao

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ConfigREST descreve onde buscar os dados de uma API. As credenciais valem só para esta busca: ninguém as guarda.
type ConfigREST struct {
	URL     string
	Token   string // enviado como "Authorization: Bearer ..."
	Usuario string // alternativa ao token: HTTP Basic
	Senha   string
	Caminho string // onde está a lista no JSON, ex.: "data" ou "resultado.itens" (vazio = o JSON todo é a lista)
}

var redesProibidas = func() []*net.IPNet {
	var out []*net.IPNet
	for _, c := range []string{"100.64.0.0/10", "192.0.0.0/24", "198.18.0.0/15", "240.0.0.0/4", "0.0.0.0/8"} {
		_, n, _ := net.ParseCIDR(c)
		out = append(out, n)
	}
	return out
}()

// ipPublico diz se o endereço pode ser acessado pelo servidor: só endereços da internet pública. Bloqueia
// loopback, redes privadas, link-local (inclui os metadados de nuvem em 169.254.169.254), multicast e afins,
// para ninguém usar a importação para sondar a rede interna (SSRF).
func ipPublico(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	for _, n := range redesProibidas {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}

// ValidarURL confere o endereço antes de buscar: HTTPS, com nome de servidor, sem usuário e senha embutidos.
func ValidarURL(bruto string, permitirPrivado bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(bruto))
	if err != nil || u.Host == "" {
		return nil, errors.New("o endereço da API não é válido")
	}
	if u.User != nil {
		return nil, errors.New("não ponha usuário e senha no endereço: use os campos de credencial")
	}
	if u.Scheme != "https" && !(permitirPrivado && u.Scheme == "http") {
		return nil, errors.New("o endereço da API precisa ser HTTPS")
	}
	return u, nil
}

// clienteSeguro faz a conexão só com endereços públicos (a conferência é na hora de conectar, depois da resolução
// do nome, então não adianta um nome que aponte para a rede interna).
func clienteSeguro(permitirPrivado bool) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, rede, endereco string) (net.Conn, error) {
				host, porta, err := net.SplitHostPort(endereco)
				if err != nil {
					return nil, err
				}
				ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}
				for _, ip := range ips {
					if permitirPrivado || ipPublico(ip.IP) {
						return dialer.DialContext(ctx, rede, net.JoinHostPort(ip.IP.String(), porta))
					}
				}
				return nil, errors.New("o endereço da API aponta para a rede interna, que não é permitida")
			},
			TLSHandshakeTimeout: 10 * time.Second,
			DisableKeepAlives:   true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("redirecionamentos demais")
			}
			if _, err := ValidarURL(req.URL.String(), permitirPrivado); err != nil {
				return err
			}
			req.Header.Del("Authorization") // a credencial não acompanha o redirecionamento
			return nil
		},
	}
}

// BuscarJSON busca o JSON da API (no máximo MaxBytes) e devolve o corpo.
func BuscarJSON(ctx context.Context, cfg ConfigREST, permitirPrivado bool) ([]byte, error) {
	u, err := ValidarURL(cfg.URL, permitirPrivado)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.New("o endereço da API não é válido")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "UYSI-ARBO-Importacao/1.0")
	switch {
	case cfg.Token != "":
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	case cfg.Usuario != "":
		req.SetBasicAuth(cfg.Usuario, cfg.Senha)
	}
	resp, err := clienteSeguro(permitirPrivado).Do(req)
	if err != nil {
		// a mensagem de rede pode trazer o endereço; basta o motivo curto
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("não consegui acessar a API: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("a API respondeu com erro (HTTP %d); confira o endereço e as credenciais", resp.StatusCode)
	}
	dados, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return nil, errors.New("a resposta da API foi interrompida")
	}
	if len(dados) > MaxBytes {
		return nil, fmt.Errorf("a resposta da API é grande demais (máximo %d MB)", MaxBytes>>20)
	}
	return dados, nil
}

// PlanilhaDeJSON transforma uma lista de objetos JSON numa Planilha: o cabeçalho é a união das chaves (em ordem
// alfabética) e cada objeto vira uma linha (número 2 em diante, como numa planilha).
func PlanilhaDeJSON(dados []byte, caminho string) (Planilha, error) {
	var raiz any
	dec := json.NewDecoder(bytes.NewReader(dados))
	dec.UseNumber()
	if err := dec.Decode(&raiz); err != nil {
		return Planilha{}, errors.New("a resposta da API não é um JSON válido")
	}
	for _, passo := range strings.FieldsFunc(caminho, func(r rune) bool { return r == '.' || r == '/' }) {
		obj, ok := raiz.(map[string]any)
		if !ok {
			return Planilha{}, fmt.Errorf("o caminho %q não existe na resposta da API", caminho)
		}
		if raiz, ok = obj[passo]; !ok {
			return Planilha{}, fmt.Errorf("o caminho %q não existe na resposta da API", caminho)
		}
	}
	lista, ok := raiz.([]any)
	if !ok {
		return Planilha{}, errors.New("a resposta da API não é uma lista; informe o caminho onde está a lista (por exemplo, data)")
	}
	if len(lista) == 0 {
		return Planilha{}, errors.New("a API devolveu uma lista vazia")
	}
	if len(lista) > MaxLinhas {
		return Planilha{}, fmt.Errorf("a API devolveu itens demais (%d): o máximo é %d por importação", len(lista), MaxLinhas)
	}
	chaves := map[string]bool{}
	objetos := make([]map[string]any, len(lista))
	for i, item := range lista {
		obj, ok := item.(map[string]any)
		if !ok {
			return Planilha{}, fmt.Errorf("o item %d da lista não é um objeto", i+1)
		}
		objetos[i] = obj
		for k := range obj {
			chaves[k] = true
		}
	}
	var p Planilha
	for k := range chaves {
		p.Cabecalho = append(p.Cabecalho, k)
	}
	sort.Strings(p.Cabecalho)
	for i, obj := range objetos {
		cel := make([]string, len(p.Cabecalho))
		for j, k := range p.Cabecalho {
			cel[j] = textoJSON(obj[k])
		}
		p.Linhas = append(p.Linhas, Linha{N: i + 2, Celulas: cel})
	}
	return p, nil
}

// PlanilhaDeObjetos é o mesmo para objetos já lidos (usado pela importação via API do sistema).
func PlanilhaDeObjetos(objetos []map[string]any) (Planilha, error) {
	if len(objetos) == 0 {
		return Planilha{}, errors.New("envie ao menos uma linha")
	}
	if len(objetos) > MaxLinhas {
		return Planilha{}, fmt.Errorf("linhas demais (%d): o máximo é %d por importação", len(objetos), MaxLinhas)
	}
	lista := make([]any, len(objetos))
	for i, o := range objetos {
		lista[i] = o
	}
	dados, _ := json.Marshal(lista)
	return PlanilhaDeJSON(dados, "")
}

// textoJSON converte um valor JSON em texto de célula: números como vieram, booleanos como sim/não, nulos e
// estruturas aninhadas como vazio.
func textoJSON(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", x), "0"), ".")
	case bool:
		if x {
			return "sim"
		}
		return "não"
	}
	return ""
}

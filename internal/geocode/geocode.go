// Package geocode acha endereços pelo CEP (ViaCEP) e coordenadas pelo endereço (Nominatim, do
// OpenStreetMap). Os dois são gratuitos; os endereços dos clientes são mandados a eles, nunca dados
// de moradores. Os endereços dos serviços e o contato vêm da configuração, para trocar de provedor.
package geocode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	// ErrNaoEncontrado: o serviço respondeu, mas não conhece o CEP ou o endereço.
	ErrNaoEncontrado = errors.New("não encontrado")
	// ErrIndisponivel: o serviço não respondeu a tempo ou devolveu erro.
	ErrIndisponivel = errors.New("serviço de localização indisponível")
	// ErrDesligado: a localização automática está desligada neste servidor.
	ErrDesligado = errors.New("localização automática desligada")
)

// Endereco é o resultado de uma busca por CEP.
type Endereco struct {
	CEP        string `json:"cep"`
	Logradouro string `json:"logradouro"`
	Bairro     string `json:"bairro"`
	Cidade     string `json:"cidade"`
	UF         string `json:"uf"`
}

// Consulta é o que se sabe do endereço para achar a posição.
type Consulta struct {
	Logradouro, Numero, Bairro, Cidade, UF, CEP string
}

// Posicao é um ponto achado. Precisao diz o quão perto: "endereco" (com o número) ou "rua" (só a via).
type Posicao struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Precisao  string  `json:"precisao"`
}

// Geocoder é o que o servidor usa; os testes põem um falso no lugar.
type Geocoder interface {
	PorCEP(ctx context.Context, cep string) (Endereco, error)
	Localizar(ctx context.Context, c Consulta) (Posicao, error)
}

// Desligado recusa tudo: usado quando a configuração desliga a localização automática.
type Desligado struct{}

func (Desligado) PorCEP(context.Context, string) (Endereco, error) { return Endereco{}, ErrDesligado }
func (Desligado) Localizar(context.Context, Consulta) (Posicao, error) {
	return Posicao{}, ErrDesligado
}

// Gratuito usa ViaCEP e Nominatim.
type Gratuito struct {
	ViaCEPURL    string // padrão https://viacep.com.br
	NominatimURL string // padrão https://nominatim.openstreetmap.org
	Contato      string // e-mail ou site de quem opera o sistema: a política do Nominatim exige identificação
	Cliente      *http.Client

	mu     sync.Mutex
	ultima time.Time // última chamada ao Nominatim (no máximo 1 por segundo)
	cache  map[string]entrada
	espaco time.Duration
	agora  func() time.Time
	dormir func(ctx context.Context, d time.Duration) error
}

type entrada struct {
	pos   Posicao
	end   Endereco
	acha  bool
	valid time.Time
}

const (
	validadeDoCache = 24 * time.Hour
	maxCache        = 2000
)

func (g *Gratuito) iniciar() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cache != nil {
		return
	}
	g.cache = map[string]entrada{}
	if g.ViaCEPURL == "" {
		g.ViaCEPURL = "https://viacep.com.br"
	}
	if g.NominatimURL == "" {
		g.NominatimURL = "https://nominatim.openstreetmap.org"
	}
	if g.Cliente == nil {
		g.Cliente = &http.Client{Timeout: 8 * time.Second}
	}
	if g.espaco == 0 {
		g.espaco = 1100 * time.Millisecond
	}
	if g.agora == nil {
		g.agora = time.Now
	}
	if g.dormir == nil {
		g.dormir = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}
}

func (g *Gratuito) userAgent() string {
	if g.Contato != "" {
		return "UYSI-ARBO/1.0 (" + g.Contato + ")"
	}
	return "UYSI-ARBO/1.0"
}

func (g *Gratuito) get(ctx context.Context, u string, saida any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ErrIndisponivel
	}
	req.Header.Set("User-Agent", g.userAgent())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "pt-BR")
	resp, err := g.Cliente.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrIndisponivel, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
		return ErrNaoEncontrado
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP %d", ErrIndisponivel, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(saida); err != nil {
		return fmt.Errorf("%w: resposta ilegível", ErrIndisponivel)
	}
	return nil
}

var reNaoDigito = regexp.MustCompile(`\D`)

// SoDigitos devolve só os números de um CEP.
func SoDigitos(s string) string { return reNaoDigito.ReplaceAllString(s, "") }

func (g *Gratuito) PorCEP(ctx context.Context, cep string) (Endereco, error) {
	g.iniciar()
	d := SoDigitos(cep)
	if len(d) != 8 {
		return Endereco{}, ErrNaoEncontrado
	}
	chave := "cep:" + d
	if e, ok := g.doCache(chave); ok {
		if !e.acha {
			return Endereco{}, ErrNaoEncontrado
		}
		return e.end, nil
	}
	var r struct {
		CEP, Logradouro, Bairro, Localidade, UF string
		Erro                                    any `json:"erro"`
	}
	if err := g.get(ctx, strings.TrimRight(g.ViaCEPURL, "/")+"/ws/"+d+"/json/", &r); err != nil {
		if errors.Is(err, ErrNaoEncontrado) {
			g.guardar(chave, entrada{})
		}
		return Endereco{}, err
	}
	if r.Erro != nil && r.Erro != false && r.Erro != "false" {
		g.guardar(chave, entrada{})
		return Endereco{}, ErrNaoEncontrado
	}
	e := Endereco{CEP: r.CEP, Logradouro: r.Logradouro, Bairro: r.Bairro, Cidade: r.Localidade, UF: r.UF}
	g.guardar(chave, entrada{acha: true, end: e})
	return e, nil
}

func (g *Gratuito) doCache(chave string) (entrada, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.cache[chave]
	if !ok || g.agora().After(e.valid) {
		return entrada{}, false
	}
	return e, true
}

func (g *Gratuito) guardar(chave string, e entrada) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.cache) >= maxCache {
		g.cache = map[string]entrada{} // simples: esvazia quando enche
	}
	e.valid = g.agora().Add(validadeDoCache)
	g.cache[chave] = e
}

// esperarVez garante no máximo uma chamada por segundo ao Nominatim, para todos os clientes juntos.
func (g *Gratuito) esperarVez(ctx context.Context) error {
	g.mu.Lock()
	espera := g.espaco - g.agora().Sub(g.ultima)
	if espera < 0 {
		espera = 0
	}
	g.ultima = g.agora().Add(espera) // reserva a vez
	g.mu.Unlock()
	if espera > 0 {
		return g.dormir(ctx, espera)
	}
	return nil
}

func (g *Gratuito) buscar(ctx context.Context, q string) (Posicao, error) {
	chave := "q:" + strings.ToLower(q)
	if e, ok := g.doCache(chave); ok {
		if !e.acha {
			return Posicao{}, ErrNaoEncontrado
		}
		return e.pos, nil
	}
	if err := g.esperarVez(ctx); err != nil {
		return Posicao{}, fmt.Errorf("%w: %v", ErrIndisponivel, err)
	}
	var rs []struct {
		Lat, Lon string
	}
	u := strings.TrimRight(g.NominatimURL, "/") + "/search?format=jsonv2&limit=1&countrycodes=br&q=" + url.QueryEscape(q)
	if err := g.get(ctx, u, &rs); err != nil {
		return Posicao{}, err
	}
	if len(rs) == 0 {
		g.guardar(chave, entrada{})
		return Posicao{}, ErrNaoEncontrado
	}
	var p Posicao
	if _, err := fmt.Sscanf(rs[0].Lat, "%f", &p.Latitude); err != nil {
		return Posicao{}, fmt.Errorf("%w: resposta ilegível", ErrIndisponivel)
	}
	if _, err := fmt.Sscanf(rs[0].Lon, "%f", &p.Longitude); err != nil {
		return Posicao{}, fmt.Errorf("%w: resposta ilegível", ErrIndisponivel)
	}
	g.guardar(chave, entrada{acha: true, pos: p})
	return p, nil
}

// Localizar tenta com o número e, se não achar, só com a rua (marcando a precisão).
func (g *Gratuito) Localizar(ctx context.Context, c Consulta) (Posicao, error) {
	g.iniciar()
	if strings.TrimSpace(c.Logradouro) == "" || strings.TrimSpace(c.Cidade) == "" {
		return Posicao{}, ErrNaoEncontrado
	}
	montar := func(comNumero bool) string {
		var partes []string
		rua := strings.TrimSpace(c.Logradouro)
		if n := strings.TrimSpace(c.Numero); comNumero && n != "" && !strings.EqualFold(n, "s/n") && !strings.EqualFold(n, "sn") {
			rua += ", " + n
		}
		partes = append(partes, rua)
		if b := strings.TrimSpace(c.Bairro); b != "" {
			partes = append(partes, b)
		}
		cidade := strings.TrimSpace(c.Cidade)
		if uf := strings.TrimSpace(c.UF); uf != "" {
			cidade += " - " + uf
		}
		partes = append(partes, cidade, "Brasil")
		return strings.Join(partes, ", ")
	}
	numero := strings.TrimSpace(c.Numero)
	temNumero := numero != "" && !strings.EqualFold(numero, "s/n") && !strings.EqualFold(numero, "sn")
	if temNumero {
		p, err := g.buscar(ctx, montar(true))
		if err == nil {
			p.Precisao = "endereco"
			return p, nil
		}
		if !errors.Is(err, ErrNaoEncontrado) {
			return Posicao{}, err
		}
	}
	p, err := g.buscar(ctx, montar(false))
	if err != nil {
		return Posicao{}, err
	}
	p.Precisao = "rua"
	return p, nil
}

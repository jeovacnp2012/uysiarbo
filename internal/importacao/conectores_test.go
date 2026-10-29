package importacao

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMapearComPrioridades(t *testing.T) {
	locais, _ := TipoPorCodigo("locais")
	cab := []string{"ID", "Logradouro", "Nº", "Lado", "Tipo", "Bairro", "Quadra", "Cad_Situacao", "Sobra"}

	// Sem conector e sem mapa: só os nomes do próprio tipo (o que não bate fica faltando).
	m := locais.MapearComo(cab, nil, nil)
	if _, ok := m.Posicao["numero"]; ok || len(m.Faltando) == 0 {
		t.Errorf("sem preset: %+v", m)
	}
	// Com os candidatos do Geo Controle, tudo bate.
	gc, _ := ConectorPorCodigo("geocontrole")
	m = locais.MapearComo(cab, nil, gc.Candidatos["locais"])
	if len(m.Faltando) != 0 || m.Posicao["endereco"] != 1 || m.Posicao["numero"] != 2 || m.Posicao["quarteirao"] != 6 ||
		m.Posicao["ativo"] != 7 || m.Posicao["id_externo"] != 0 {
		t.Errorf("preset do Geo Controle: %+v", m)
	}
	if len(m.Ignoradas) != 1 || m.Ignoradas[0] != "Sobra" {
		t.Errorf("ignoradas: %v", m.Ignoradas)
	}
	// O mapa explícito vence os palpites, e "" significa não importar.
	m = locais.MapearComo(cab, map[string]string{"endereco": "Sobra", "ativo": ""}, gc.Candidatos["locais"])
	if m.Posicao["endereco"] != 8 {
		t.Errorf("explícito: %+v", m.Posicao)
	}
	if _, ok := m.Posicao["ativo"]; ok {
		t.Error("ativo marcado como não importar")
	}
	if m.MapaDe(cab)["endereco"] != "Sobra" || m.MapaDe(cab)["numero"] != "Nº" {
		t.Errorf("MapaDe: %v", m.MapaDe(cab))
	}
	// Uma coluna da origem não serve a duas colunas do sistema.
	m = locais.MapearComo([]string{"nome"}, map[string]string{"endereco": "nome", "numero": "nome"}, nil)
	if len(m.Posicao) != 1 {
		t.Errorf("a mesma coluna não pode ser usada duas vezes: %v", m.Posicao)
	}
	if _, ok := ConectorPorCodigo("planilha"); !ok {
		t.Error("conector planilha")
	}
	if _, ok := ConectorPorCodigo("xyz"); ok {
		t.Error("conector inexistente")
	}
}

func TestIPPublico(t *testing.T) {
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "200.17.0.1", "2606:4700:4700::1111"} {
		if !ipPublico(net.ParseIP(s)) {
			t.Errorf("%s deveria ser público", s)
		}
	}
	for _, s := range []string{"127.0.0.1", "10.0.0.5", "172.16.3.4", "192.168.1.1", "169.254.169.254", "0.0.0.0", "100.64.1.1", "224.0.0.1", "::1", "fd00::1", "fe80::1", "198.18.0.1"} {
		if ipPublico(net.ParseIP(s)) {
			t.Errorf("%s não pode ser acessado", s)
		}
	}
	if ipPublico(nil) {
		t.Error("nil")
	}
}

func TestValidarURL(t *testing.T) {
	for _, bom := range []string{"https://api.exemplo.com/locais", "https://api.exemplo.com:8443/v1/x?a=1"} {
		if _, err := ValidarURL(bom, false); err != nil {
			t.Errorf("%s: %v", bom, err)
		}
	}
	for _, ruim := range []string{"", "http://api.exemplo.com", "ftp://x.com", "https://", "https://user:senha@api.exemplo.com", "api.exemplo.com", "file:///etc/passwd"} {
		if _, err := ValidarURL(ruim, false); err == nil {
			t.Errorf("%q deveria ser recusado", ruim)
		}
	}
	if _, err := ValidarURL("http://127.0.0.1:9", true); err != nil {
		t.Errorf("http local é permitido quando a rede privada está liberada (testes): %v", err)
	}
}

func TestBuscarJSONBloqueiaRedeInterna(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) }))
	defer srv.Close()
	// O servidor de teste está em 127.0.0.1: sem liberar a rede privada, é recusado (e https é exigido antes).
	if _, err := BuscarJSON(context.Background(), ConfigREST{URL: srv.URL}, false); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("http sem liberar: %v", err)
	}
	for _, alvo := range []string{"https://localhost/x", "https://127.0.0.1/x", "https://169.254.169.254/latest/meta-data", "https://10.0.0.1/x", "https://[::1]/x"} {
		_, err := BuscarJSON(context.Background(), ConfigREST{URL: alvo}, false)
		if err == nil || !strings.Contains(err.Error(), "rede interna") {
			t.Errorf("%s deveria ser bloqueado como rede interna: %v", alvo, err)
		}
	}
}

func TestBuscarJSONComCredenciais(t *testing.T) {
	var auth, ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, ua = r.Header.Get("Authorization"), r.Header.Get("User-Agent")
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte(`{"data":[{"nome":"Centro","zona":"Norte"}]}`))
		case "/erro":
			http.Error(w, "segredo do servidor", 500)
		case "/grande":
			_, _ = w.Write([]byte(strings.Repeat("x", MaxBytes+10)))
		case "/volta":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/laco":
			http.Redirect(w, r, "/laco", http.StatusFound)
		}
	}))
	defer srv.Close()
	dados, err := BuscarJSON(context.Background(), ConfigREST{URL: srv.URL + "/ok", Token: "seg-token"}, true)
	if err != nil || !strings.Contains(string(dados), "Centro") || auth != "Bearer seg-token" || !strings.HasPrefix(ua, "UYSI-ARBO") {
		t.Fatalf("token: %v %q %q", err, auth, ua)
	}
	if _, err := BuscarJSON(context.Background(), ConfigREST{URL: srv.URL + "/ok", Usuario: "u", Senha: "s"}, true); err != nil || !strings.HasPrefix(auth, "Basic ") {
		t.Errorf("basic: %v %q", err, auth)
	}
	if _, err := BuscarJSON(context.Background(), ConfigREST{URL: srv.URL + "/erro"}, true); err == nil || strings.Contains(err.Error(), "segredo do servidor") || !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("erro da API não vaza o corpo: %v", err)
	}
	if _, err := BuscarJSON(context.Background(), ConfigREST{URL: srv.URL + "/grande"}, true); err == nil || !strings.Contains(err.Error(), "grande demais") {
		t.Errorf("limite de tamanho: %v", err)
	}
	if _, err := BuscarJSON(context.Background(), ConfigREST{URL: srv.URL + "/volta"}, true); err != nil {
		t.Errorf("redirecionamento simples: %v", err)
	}
	if _, err := BuscarJSON(context.Background(), ConfigREST{URL: srv.URL + "/laco"}, true); err == nil {
		t.Error("laço de redirecionamento deveria falhar")
	}
	// A mensagem de erro de rede não traz o endereço completo.
	_, err = BuscarJSON(context.Background(), ConfigREST{URL: "http://127.0.0.1:1/segredo?token=abc"}, true)
	if err == nil || strings.Contains(err.Error(), "token=abc") {
		t.Errorf("erro de conexão: %v", err)
	}
}

func TestPlanilhaDeJSON(t *testing.T) {
	p, err := PlanilhaDeJSON([]byte(`{"resultado":{"itens":[{"nome":"Centro","id":7,"mutirao":true,"x":null,"obj":{"a":1}},{"nome":"Jardim","zona":"Sul","lat":-13.5}]}}`), "resultado.itens")
	if err != nil || len(p.Linhas) != 2 {
		t.Fatalf("%+v %v", p, err)
	}
	pos := map[string]int{}
	for i, h := range p.Cabecalho {
		pos[h] = i
	}
	if p.Linhas[0].Celulas[pos["nome"]] != "Centro" || p.Linhas[0].Celulas[pos["id"]] != "7" || p.Linhas[0].Celulas[pos["mutirao"]] != "sim" ||
		p.Linhas[0].Celulas[pos["x"]] != "" || p.Linhas[0].Celulas[pos["obj"]] != "" || p.Linhas[1].Celulas[pos["lat"]] != "-13.5" || p.Linhas[1].Celulas[pos["id"]] != "" {
		t.Errorf("células: %+v", p)
	}
	if p.Linhas[0].N != 2 || p.Linhas[1].N != 3 {
		t.Errorf("numeração: %d %d", p.Linhas[0].N, p.Linhas[1].N)
	}
	// raiz como lista
	if p, err := PlanilhaDeJSON([]byte(`[{"nome":"A"}]`), ""); err != nil || len(p.Linhas) != 1 {
		t.Errorf("lista na raiz: %v", err)
	}
	for nome, c := range map[string][2]string{
		"json ruim":       {`{`, ""},
		"caminho errado":  {`{"data":[]}`, "outro"},
		"não é lista":     {`{"data":{"a":1}}`, "data"},
		"lista vazia":     {`{"data":[]}`, "data"},
		"item não objeto": {`{"data":[1,2]}`, "data"},
		"raiz objeto":     {`{"a":1}`, ""},
	} {
		if _, err := PlanilhaDeJSON([]byte(c[0]), c[1]); err == nil {
			t.Errorf("%s: deveria dar erro", nome)
		}
	}
	if _, err := PlanilhaDeObjetos(nil); err == nil {
		t.Error("sem linhas")
	}
	if p, err := PlanilhaDeObjetos([]map[string]any{{"nome": "A", "n": 3.0}}); err != nil || p.Linhas[0].Celulas[0] != "3" {
		t.Errorf("objetos: %+v %v", p, err)
	}
}

package server

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"uysiarbo/internal/apiauth"
)

var reSegredoNaTela = regexp.MustCompile(`id="s-segredo" type="text" value="(uas_[A-Za-z0-9_-]+)"`)
var reClientNaTela = regexp.MustCompile(`id="s-client" type="text" value="(uac_[A-Za-z0-9_-]+)"`)

func formApi(extras ...string) url.Values {
	v := url.Values{"nome": {"Sistema do SMS"}, "limite": {"120"}, "escopo": {"cidades:read", "cidades:write"}}
	for i := 0; i+1 < len(extras); i += 2 {
		if extras[i+1] == "-" {
			v.Del(extras[i])
		} else {
			v[extras[i]] = []string{extras[i+1]}
		}
	}
	return v
}

func TestGestaoDeCredenciaisDaAPI(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	carla := m.comCarla(t)

	for _, p := range []string{"/parametros/api", "/parametros/api/nova", "/parametros/api/1"} {
		quer(t, carla.get(p), http.StatusForbidden)
	}
	quer(t, carla.post("/parametros/api", formApi()), http.StatusForbidden)
	if strings.Contains(carla.get("/").Body.String(), `href="/parametros/api"`) {
		t.Error("o menu da Carla não deveria ter API e integrações")
	}
	quer(t, ana.get("/"), http.StatusOK, `href="/parametros/api"`)
	quer(t, ana.get("/parametros/api"), http.StatusOK, "Nenhuma credencial criada", "Nova credencial", "Contrato OpenAPI")
	quer(t, ana.get("/parametros/api/nova"), http.StatusOK, "Escopos", "Cidades", "Chamadas por minuto", "Válida até")

	// Validações.
	amanha := time.Now().Add(48 * time.Hour).Format("2006-01-02")
	for msg, f := range map[string]url.Values{
		"nome da credencial":        formApi("nome", "X"),
		"pelo menos um escopo":      formApi("escopo", "-"),
		"Escopo inválido":           {"nome": {"Sistema"}, "limite": {"10"}, "escopo": {"usuarios:read"}},
		"Data de validade inválida": formApi("valida_ate", "31/12/2030"),
		"de hoje em diante":         formApi("valida_ate", "2020-01-01"),
		"de 1 a 10000":              formApi("limite", "0"),
		"no máximo 300":             formApi("descricao", strings.Repeat("x", 301)),
	} {
		quer(t, ana.post("/parametros/api", f), http.StatusBadRequest, msg)
	}
	if cs, _ := m.cliQ.ListApiCredenciais(t.Context()); len(cs) != 0 {
		t.Fatalf("nenhuma credencial deveria existir: %d", len(cs))
	}

	// Criar: o segredo aparece uma vez, o banco guarda só o hash.
	rec := ana.post("/parametros/api", formApi("valida_ate", amanha, "limite", "60"))
	quer(t, rec, http.StatusOK, "Credencial criada", "Copie o segredo agora", "client_credentials", "/api/v1/token")
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a tela do segredo não pode ser guardada em cache")
	}
	segredo := reSegredoNaTela.FindStringSubmatch(rec.Body.String())
	cliente := reClientNaTela.FindStringSubmatch(rec.Body.String())
	if segredo == nil || cliente == nil {
		t.Fatalf("a tela deveria mostrar client_id e segredo: %s", rec.Body.String())
	}
	cred, err := m.cliQ.GetApiCredencialPorClientID(t.Context(), cliente[1])
	if err != nil || !apiauth.SegredoConfere(segredo[1], cred.SegredoHash) || strings.Contains(string(cred.SegredoHash), segredo[1]) {
		t.Fatalf("credencial no banco: %+v %v", cred, err)
	}
	if cred.LimitePorMinuto != 60 || !cred.ValidaAte.Valid || len(cred.Escopos) != 2 || cred.Nome != "Sistema do SMS" || !cred.CriadoPor.Valid {
		t.Errorf("campos gravados: %+v", cred)
	}
	quer(t, ana.post("/parametros/api", formApi()), http.StatusBadRequest, "Já existe uma credencial")
	// O segredo nunca vai para o histórico.
	ev := achar(m.eventos(t), "api.credencial_criar")
	if ev == nil || strings.Contains(string(ev.Depois)+string(ev.Detalhes)+ev.Rotulo, segredo[1]) || mapa(ev.Detalhes)["client_id"] != cliente[1] {
		t.Fatalf("api.credencial_criar: %+v", ev)
	}
	// E já funciona na API.
	tok := m.token(t, credTeste{ID: cred.ID, ClientID: cliente[1], Segredo: segredo[1]}, "")
	quemAPI(t, m.chamar(hostNM, http.MethodGet, "/api/v1/cidades", tok, "", nil), http.StatusOK, "")

	// Lista e ficha (com consumo).
	id := s64(cred.ID)
	quer(t, ana.get("/parametros/api"), http.StatusOK, "Sistema do SMS", cliente[1], "Ativa", "Chamadas (24 h)")
	quer(t, ana.get("/parametros/api/"+id), http.StatusOK, "Credencial de API", cliente[1], "Consumo", "Últimas 24 horas", "Últimos 30 dias", "GET /api/v1/cidades", "Gerar novo segredo", "Desativar credencial")
	if strings.Contains(ana.get("/parametros/api/"+id).Body.String(), segredo[1]) {
		t.Error("a ficha não pode mostrar o segredo")
	}
	quer(t, ana.get("/parametros/api/999"), http.StatusNotFound)
	quer(t, ana.get("/parametros/api/abc"), http.StatusNotFound)

	// Editar: tirar a gravação e o limite; antes e depois na auditoria; salvar igual não registra.
	quer(t, ana.post("/parametros/api/"+id, formApi("escopo", "cidades:read", "limite", "30", "valida_ate", amanha)), http.StatusSeeOther)
	ed := achar(m.eventos(t), "api.credencial_editar")
	if ed == nil || mapa(ed.Depois)["limite_por_minuto"] != float64(30) {
		t.Fatalf("api.credencial_editar: %+v", ed)
	}
	quer(t, ana.post("/parametros/api/"+id, formApi("escopo", "cidades:read", "limite", "30", "valida_ate", amanha)), http.StatusSeeOther)
	n := 0
	for _, e := range m.eventos(t) {
		if e.Acao == "api.credencial_editar" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("salvar sem mudança não deve registrar (%d eventos)", n)
	}
	quer(t, ana.post("/parametros/api/"+id, formApi("nome", "")), http.StatusBadRequest, "nome da credencial")
	// O token antigo ainda lê, mas o escopo de gravação saiu da credencial.
	quemAPI(t, m.chamar(hostNM, http.MethodPost, "/api/v1/cidades", tok, `{"nome":"Sinop","uf":"MT"}`, nil), http.StatusForbidden, "insufficient_scope")

	// Rotacionar: segredo novo aparece, o antigo e os tokens emitidos deixam de valer.
	rec = ana.post("/parametros/api/"+id+"/rotacionar", url.Values{})
	quer(t, rec, http.StatusOK, "Segredo novo gerado", "Copie o segredo agora")
	novo := reSegredoNaTela.FindStringSubmatch(rec.Body.String())
	if novo == nil || novo[1] == segredo[1] {
		t.Fatalf("segredo novo: %v", novo)
	}
	quemAPI(t, m.chamar(hostNM, http.MethodGet, "/api/v1/cidades", tok, "", nil), http.StatusUnauthorized, "invalid_token")
	velho := url.Values{"grant_type": {"client_credentials"}, "client_id": {cliente[1]}, "client_secret": {segredo[1]}}
	quemAPI(t, m.chamar(hostNM, http.MethodPost, "/api/v1/token", "", velho.Encode(), nil), http.StatusUnauthorized, "invalid_client")
	tok = m.token(t, credTeste{ID: cred.ID, ClientID: cliente[1], Segredo: novo[1]}, "")
	quemAPI(t, m.chamar(hostNM, http.MethodGet, "/api/v1/cidades", tok, "", nil), http.StatusOK, "")
	if e := achar(m.eventos(t), "api.credencial_rotacionar"); e == nil || strings.Contains(string(e.Detalhes), novo[1]) {
		t.Errorf("api.credencial_rotacionar: %+v", e)
	}

	// Desativar e reativar: vale na hora, com auditoria só quando muda.
	rec = ana.post("/parametros/api/"+id+"/ativo", url.Values{"ativo": {"0"}})
	if local(rec) != "/parametros/api/"+id+"?ok=desativado" {
		t.Fatalf("desativar: %d %q", rec.Code, local(rec))
	}
	quemAPI(t, m.chamar(hostNM, http.MethodGet, "/api/v1/cidades", tok, "", nil), http.StatusUnauthorized, "invalid_token")
	quer(t, ana.get("/parametros/api"), http.StatusOK, "Desativada")
	quer(t, ana.get("/parametros/api/"+id+"?ok=desativado"), http.StatusOK, "Credencial desativada", "Reativar credencial")
	quer(t, ana.post("/parametros/api/"+id+"/ativo", url.Values{"ativo": {"0"}}), http.StatusSeeOther)
	quer(t, ana.post("/parametros/api/"+id+"/ativo", url.Values{"ativo": {"1"}}), http.StatusSeeOther)
	quemAPI(t, m.chamar(hostNM, http.MethodGet, "/api/v1/cidades", tok, "", nil), http.StatusOK, "")
	des, rea := 0, 0
	for _, e := range m.eventos(t) {
		switch e.Acao {
		case "api.credencial_desativar":
			des++
		case "api.credencial_ativar":
			rea++
		}
	}
	if des != 1 || rea != 1 {
		t.Errorf("desativar=%d reativar=%d, queria 1 e 1", des, rea)
	}
	quer(t, ana.post("/parametros/api/999/ativo", url.Values{"ativo": {"0"}}), http.StatusNotFound)
	quer(t, ana.post("/parametros/api/999/rotacionar", url.Values{}), http.StatusNotFound)
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

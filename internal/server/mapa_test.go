package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"uysiarbo/internal/db/sqlccliente"
)

type colecaoTeste struct {
	Type     string `json:"type"`
	Features []struct {
		ID         int64          `json:"id"`
		Properties map[string]any `json:"properties"`
		Geometry   struct {
			Type string `json:"type"`
		} `json:"geometry"`
	} `json:"features"`
	Truncado bool `json:"truncado"`
	SemPonto int  `json:"sem_ponto"`
}

func lerColecao(t *testing.T, n *navegador, caminho string) colecaoTeste {
	t.Helper()
	rec := n.get(caminho)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/geo+json") {
		t.Fatalf("%s: %d %q", caminho, rec.Code, rec.Header().Get("Content-Type"))
	}
	var c colecaoTeste
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("%s: JSON inválido: %v\n%s", caminho, err, rec.Body.String())
	}
	if c.Type != "FeatureCollection" {
		t.Fatalf("%s: type = %q", caminho, c.Type)
	}
	return c
}

const caixaDoBairro = "-56.095,-13.835,-56.065,-13.805" // contém o polígono de teste
const caixaLonge = "-40.0,-5.0,-39.0,-4.0"

func TestPaginaDoMapa(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	quer(t, ana.get("/mapa"), http.StatusOK, `id="mapa"`, "https://tile.openstreetmap.org/{z}/{x}/{y}.png", "OpenStreetMap",
		"maplibre-gl.js", "/static/js/mapa.js", "Bairros", "Quarteirões", "Locais", "Tipo de local", "Centro", "Todos")
	quer(t, ana.get("/mapa?bairro="+s64(b)+"&tipo=comercio"), http.StatusOK, `data-bairro="`+s64(b)+`"`, `data-tipo="comercio"`)
	quer(t, ana.get("/"), http.StatusOK, `href="/mapa"`)
	quer(t, m.comCarla(t).get("/mapa"), http.StatusForbidden)

	// Os arquivos do MapLibre e do script são servidos.
	for _, p := range []string{"/static/vendor/maplibre-gl.js", "/static/vendor/maplibre-gl.css", "/static/js/mapa.js"} {
		if rec := ana.get(p); rec.Code != http.StatusOK || rec.Body.Len() < 500 {
			t.Errorf("%s: %d (%d bytes)", p, rec.Code, rec.Body.Len())
		}
	}
	// O painel inicial marca o módulo Mapa como disponível.
	quer(t, ana.get("/"), http.StatusOK, `href="/mapa"`)
}

func TestDadosDoMapa(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	a, b := m.criarBairro(t, cid, "Bairro A"), m.criarBairro(t, cid, "Bairro B")
	qa := m.criarQuarteirao(t, a, "1")
	qb := m.criarQuarteirao(t, b, "1")

	// Sem limites gravados: coleções vazias.
	if c := lerColecao(t, ana, "/mapa/bairros.geojson"); len(c.Features) != 0 {
		t.Fatalf("bairros sem limite: %d", len(c.Features))
	}

	// Limite do Bairro A e dois quarteirões do Bairro A (um fora da área consultada depois).
	quer(t, ana.post("/bairros/"+s64(a)+"/limite", url.Values{"acao": {"salvar"}, "geojson": {poligonoBairro}}), http.StatusSeeOther)
	quer(t, ana.post("/bairros/"+s64(a)+"/quarteiroes/limites", url.Values{"geojson": {colecaoDeQuarteiroes("1", "2")}}), http.StatusSeeOther)

	bs := lerColecao(t, ana, "/mapa/bairros.geojson")
	if len(bs.Features) != 1 || bs.Features[0].Properties["nome"] != "Bairro A" || !strings.HasSuffix(bs.Features[0].Geometry.Type, "Polygon") {
		t.Fatalf("bairros: %+v", bs)
	}
	if c := lerColecao(t, ana, "/mapa/bairros.geojson?bairro="+s64(b)); len(c.Features) != 0 {
		t.Errorf("filtro por outro bairro: %d", len(c.Features))
	}
	qs := lerColecao(t, ana, "/mapa/quarteiroes.geojson?bbox="+caixaDoBairro)
	if len(qs.Features) != 2 || qs.Features[0].Properties["bairro"] != "Bairro A" {
		t.Fatalf("quarteirões: %+v", qs)
	}
	if c := lerColecao(t, ana, "/mapa/quarteiroes.geojson?bbox="+caixaLonge); len(c.Features) != 0 {
		t.Errorf("quarteirões fora da caixa: %d", len(c.Features))
	}
	if c := lerColecao(t, ana, "/mapa/quarteiroes.geojson?bbox=lixo"); len(c.Features) != 2 { // caixa inválida = mundo todo
		t.Errorf("caixa inválida deveria valer o mundo: %d", len(c.Features))
	}
	// Quarteirão desativado some do mapa.
	if _, err := m.cliQ.SetQuarteiraoAtivo(t.Context(), sqlccliente.SetQuarteiraoAtivoParams{ID: qa, BairroID: a, Ativo: false}); err != nil {
		t.Fatal(err)
	}
	if c := lerColecao(t, ana, "/mapa/quarteiroes.geojson"); len(c.Features) != 1 {
		t.Errorf("quarteirões ativos = %d, queria 1", len(c.Features))
	}

	if _, err := m.cliQ.SetQuarteiraoAtivo(t.Context(), sqlccliente.SetQuarteiraoAtivoParams{ID: qa, BairroID: a, Ativo: true}); err != nil {
		t.Fatal(err)
	}

	// Locais: dois com coordenadas (um comércio), um sem, um em outro bairro com coordenadas longe.
	quer(t, ana.post("/locais", formLocal(a, qa, "numero", "1", "latitude", "-13.82", "longitude", "-56.08")), http.StatusSeeOther)
	quer(t, ana.post("/locais", formLocal(a, qa, "numero", "2", "tipo", "comercio", "latitude", "-13.821", "longitude", "-56.081")), http.StatusSeeOther)
	quer(t, ana.post("/locais", formLocal(a, qa, "numero", "3")), http.StatusSeeOther)
	quer(t, ana.post("/locais", formLocal(b, qb, "numero", "4", "latitude", "-4.5", "longitude", "-39.5")), http.StatusSeeOther)

	ls := lerColecao(t, ana, "/mapa/locais.geojson?bbox="+caixaDoBairro)
	if len(ls.Features) != 2 || ls.SemPonto != 1 || ls.Truncado {
		t.Fatalf("locais na caixa: %d features, sem_ponto=%d", len(ls.Features), ls.SemPonto)
	}
	if p := ls.Features[0].Properties; p["endereco"] != "Rua das Palmeiras, 1" || p["bairro"] != "Bairro A" || p["tipo"] != "residencial" || p["ativo"] != true {
		t.Errorf("propriedades: %+v", p)
	}
	if c := lerColecao(t, ana, "/mapa/locais.geojson?bbox="+caixaLonge); len(c.Features) != 1 {
		t.Errorf("local longe: %d", len(c.Features))
	}
	if c := lerColecao(t, ana, "/mapa/locais.geojson?bbox="+caixaDoBairro+"&tipo=comercio"); len(c.Features) != 1 || c.Features[0].Properties["tipo"] != "comercio" {
		t.Errorf("filtro de tipo: %+v", c)
	}
	if c := lerColecao(t, ana, "/mapa/locais.geojson?bbox="+caixaDoBairro+"&tipo=inexistente"); len(c.Features) != 2 {
		t.Errorf("tipo desconhecido deveria ser ignorado: %d", len(c.Features))
	}
	if c := lerColecao(t, ana, "/mapa/locais.geojson?bairro="+s64(b)); len(c.Features) != 1 {
		t.Errorf("filtro de bairro: %d", len(c.Features))
	}
	if c := lerColecao(t, ana, "/mapa/locais.geojson?local=2"); len(c.Features) != 1 || c.Features[0].ID != 2 {
		t.Errorf("foco no local: %+v", c)
	}

	// Grupo: só os locais do grupo.
	g := m.criarGrupoPelaTela(t, ana, "Rota")
	quer(t, ana.post("/locais/1/grupo", url.Values{"grupo_id": {g}}), http.StatusSeeOther)
	if c := lerColecao(t, ana, "/mapa/locais.geojson?grupo="+g); len(c.Features) != 1 || c.Features[0].ID != 1 {
		t.Errorf("filtro de grupo: %+v", c)
	}

	// Local desativado: some do mapa de quem não pode ver desativados, aparece para quem pode.
	quer(t, ana.post("/locais/2/ativo", url.Values{"ativo": {"0"}}), http.StatusSeeOther)
	c := lerColecao(t, ana, "/mapa/locais.geojson?local=2")
	if len(c.Features) != 1 || c.Features[0].Properties["ativo"] != false {
		t.Errorf("a Ana (vê desativados) deveria ver o local inativo: %+v", c)
	}
	pa, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Só locais"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pa.ID, "local_visualizar"))
	leo := m.novoUsuarioLogado(t, "Leo Dias", "leo@nm.gov.br", pa.ID)
	if c := lerColecao(t, leo, "/mapa/locais.geojson?local=2"); len(c.Features) != 0 {
		t.Errorf("o Leo não deveria ver o local desativado: %+v", c)
	}
	// Quem só tem permissão de locais abre o mapa, mas não os contornos de bairros.
	quer(t, leo.get("/mapa"), http.StatusOK, `data-locais`)
	quer(t, leo.get("/mapa/bairros.geojson"), http.StatusForbidden)
	quer(t, leo.get("/mapa/quarteiroes.geojson"), http.StatusForbidden)
	quer(t, m.comCarla(t).get("/mapa/locais.geojson"), http.StatusForbidden)
}

func TestMapaRespeitaEscopoDoPapel(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	a, b := m.criarBairro(t, cid, "Bairro A"), m.criarBairro(t, cid, "Bairro B")
	qa, qb := m.criarQuarteirao(t, a, "1"), m.criarQuarteirao(t, b, "1")
	for _, id := range []int64{a, b} {
		quer(t, ana.post("/bairros/"+s64(id)+"/limite", url.Values{"acao": {"salvar"}, "geojson": {poligonoBairro}}), http.StatusSeeOther)
	}
	quer(t, ana.post("/locais", formLocal(a, qa, "numero", "1", "latitude", "-13.82", "longitude", "-56.08")), http.StatusSeeOther)
	quer(t, ana.post("/locais", formLocal(b, qb, "numero", "2", "latitude", "-13.82", "longitude", "-56.08")), http.StatusSeeOther)
	quer(t, ana.post("/locais", formLocal(b, qb, "numero", "3", "tipo", "ponto_estrategico", "latitude", "-13.82", "longitude", "-56.08")), http.StatusSeeOther)
	if c := lerColecao(t, ana, "/mapa/bairros.geojson"); len(c.Features) != 2 {
		t.Fatalf("a Ana deveria ver os 2 bairros: %d", len(c.Features))
	}

	// Dora: só os bairros atribuídos (em bairros e em locais).
	pd, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Campo"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pd.ID, "bairro_visualizar", "bairro_meubairro", "local_visualizar", "local_meubairro"))
	dora := m.novoUsuarioLogado(t, "Dora Reis", "dora@nm.gov.br", pd.ID)
	did, _ := m.cliQ.GetUsuarioByEmail(t.Context(), "dora@nm.gov.br")
	_ = m.cliQ.AddResponsaveisAoBairro(t.Context(), addRespParams(a, did.ID))
	if c := lerColecao(t, dora, "/mapa/bairros.geojson"); len(c.Features) != 1 || c.Features[0].Properties["nome"] != "Bairro A" {
		t.Errorf("bairros da Dora: %+v", c)
	}
	if c := lerColecao(t, dora, "/mapa/locais.geojson"); len(c.Features) != 1 || c.Features[0].Properties["bairro"] != "Bairro A" {
		t.Errorf("locais da Dora: %+v", c)
	}
	if c := lerColecao(t, dora, "/mapa/locais.geojson?bairro="+s64(b)); len(c.Features) != 0 {
		t.Errorf("a Dora não pode ver o Bairro B nem pedindo por ele: %d", len(c.Features))
	}
	if c := lerColecao(t, dora, "/mapa/quarteiroes.geojson"); len(c.Features) != 0 { // nenhum quarteirão tem contorno
		t.Errorf("quarteirões: %d", len(c.Features))
	}
	if strings.Contains(dora.get("/mapa").Body.String(), "Bairro B") {
		t.Error("o filtro de bairros da Dora não deveria oferecer o Bairro B")
	}

	// Paulo: só pontos estratégicos.
	pp, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Pontos"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pp.ID, "local_visualizar", "local_pe"))
	paulo := m.novoUsuarioLogado(t, "Paulo Dias", "paulo@nm.gov.br", pp.ID)
	if c := lerColecao(t, paulo, "/mapa/locais.geojson"); len(c.Features) != 1 || c.Features[0].Properties["tipo"] != "ponto_estrategico" {
		t.Errorf("locais do Paulo: %+v", c)
	}
}

func TestCaixaDoPedido(t *testing.T) {
	req := func(q string) *http.Request {
		r, _ := http.NewRequest("GET", "/mapa/locais.geojson?"+q, nil)
		return r
	}
	mundo := caixaDoPedido(req(""))
	if mundo.O != -180 || mundo.N != 90 {
		t.Fatalf("sem caixa: %+v", mundo)
	}
	for _, ruim := range []string{"bbox=1,2,3", "bbox=a,b,c,d", "bbox=10,0,5,1", "bbox=0,10,1,5", "bbox=-200,0,1,1", "bbox=0,-95,1,1"} {
		if caixaDoPedido(req(ruim)) != mundo {
			t.Errorf("%s deveria virar o mundo todo", ruim)
		}
	}
	if c := caixaDoPedido(req("bbox=-56.1,-13.9,-56.0,-13.8")); c.O != -56.1 || c.N != -13.8 {
		t.Errorf("caixa válida: %+v", c)
	}
}

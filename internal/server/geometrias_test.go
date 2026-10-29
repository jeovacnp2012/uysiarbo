package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

const poligonoBairro = `{"type":"Polygon","coordinates":[[[-56.09,-13.83],[-56.07,-13.83],[-56.07,-13.81],[-56.09,-13.81],[-56.09,-13.83]]]}`

// laço (gravata): o PostGIS conserta em dois triângulos
const poligonoGravata = `{"type":"Polygon","coordinates":[[[-56.09,-13.83],[-56.07,-13.81],[-56.07,-13.83],[-56.09,-13.81],[-56.09,-13.83]]]}`

func colecaoDeQuarteiroes(nomes ...string) string {
	var itens []string
	for i, n := range nomes {
		dx := float64(i) * 0.002
		x0, x1 := -56.09+dx, -56.089+dx
		itens = append(itens, `{"type":"Feature","properties":{"nome":"`+n+`"},"geometry":{"type":"Polygon","coordinates":[[[`+
			f6(x0)+`,-13.83],[`+f6(x1)+`,-13.83],[`+f6(x1)+`,-13.829],[`+f6(x0)+`,-13.829],[`+f6(x0)+`,-13.83]]]}}`)
	}
	return `{"type":"FeatureCollection","features":[` + strings.Join(itens, ",") + `]}`
}

func f6(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }

func TestLimiteDoBairro(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	bid := m.criarBairro(t, cid, "Centro")
	url1 := "/bairros/" + s64(bid) + "/limite"

	quer(t, ana.get("/bairros/"+s64(bid)), http.StatusOK, "Limite no mapa", "ainda não tem limite", "Gravar limite")

	// Entradas ruins voltam para a ficha com a explicação, sem gravar nada.
	quer(t, ana.post(url1, url.Values{"acao": {"salvar"}, "geojson": {""}}), http.StatusBadRequest, "GeoJSON recusado", "cole o conteúdo")
	quer(t, ana.post(url1, url.Values{"acao": {"salvar"}, "geojson": {"{oi"}}), http.StatusBadRequest, "JSON mal formado", "{oi")
	quer(t, ana.post(url1, url.Values{"acao": {"salvar"}, "geojson": {`{"type":"Point","coordinates":[-56,-13]}`}}), http.StatusBadRequest, "não é aceito")
	quer(t, ana.post(url1, url.Values{"acao": {"salvar"}, "geojson": {colecaoDeQuarteiroes("1", "2")}}), http.StatusBadRequest, "único limite")
	quer(t, ana.get("/bairros/"+s64(bid)), http.StatusOK, "ainda não tem limite")

	// Gravar: aparece a área (cerca de 2,2 km x 2,2 km = ~480 ha) e a auditoria registra.
	rec := ana.post(url1, url.Values{"acao": {"salvar"}, "geojson": {poligonoBairro}})
	if rec.Code != http.StatusSeeOther || local(rec) != "/bairros/"+s64(bid)+"?limite=gravado#limite" {
		t.Fatalf("gravar: %d %q", rec.Code, local(rec))
	}
	corpo := ana.get(strings.Split(local(rec), "#")[0]).Body.String()
	for _, quer := range []string{"Limite do bairro gravado.", "Polígono gravado", " ha", "Substituir limite", "Remover limite"} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("a ficha deveria conter %q", quer)
		}
	}
	ev := achar(m.eventos(t), "bairro.limite")
	if ev == nil || mapa(ev.Depois)["area_ha"] == nil || ev.Antes != nil && len(mapa(ev.Antes)) != 0 {
		t.Fatalf("bairro.limite: %+v", ev)
	}

	// Substituir por um polígono em laço: o PostGIS conserta e a auditoria guarda antes e depois.
	quer(t, ana.post(url1, url.Values{"acao": {"salvar"}, "geojson": {poligonoGravata}}), http.StatusSeeOther)
	n := 0
	for _, e := range m.eventos(t) {
		if e.Acao == "bairro.limite" {
			n++
			if mapa(e.Antes)["area_ha"] != nil {
				if mapa(e.Antes)["area_ha"] == mapa(e.Depois)["area_ha"] {
					t.Error("a substituição deveria mudar a área")
				}
			}
		}
	}
	if n != 2 {
		t.Errorf("eram esperados 2 eventos bairro.limite, vieram %d", n)
	}

	// Remover (e remover de novo não registra nada).
	rec = ana.post(url1, url.Values{"acao": {"remover"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("remover: %d", rec.Code)
	}
	quer(t, ana.get("/bairros/"+s64(bid)), http.StatusOK, "ainda não tem limite")
	quer(t, ana.post(url1, url.Values{"acao": {"remover"}}), http.StatusSeeOther)
	rem := 0
	for _, e := range m.eventos(t) {
		if e.Acao == "bairro.limite_remover" {
			rem++
		}
	}
	if rem != 1 {
		t.Errorf("bairro.limite_remover = %d, queria 1", rem)
	}
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d: as ações novas devem registrar os próprios eventos", got)
	}

	quer(t, ana.post("/bairros/999999/limite", url.Values{"acao": {"salvar"}, "geojson": {poligonoBairro}}), http.StatusNotFound)
}

func TestImportarLimitesDosQuarteiroes(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	bid := m.criarBairro(t, cid, "Centro")
	quer(t, ana.post("/bairros/"+s64(bid)+"/quarteiroes", url.Values{"nomes": {"1-2"}}), http.StatusSeeOther)
	url1 := "/bairros/" + s64(bid) + "/quarteiroes/limites"

	// Recusas: sem nome, repetido, GeoJSON ruim; nada é criado (tudo ou nada).
	semNome := `{"type":"Feature","properties":{},"geometry":` + poligonoBairro + `}`
	quer(t, ana.post(url1, url.Values{"geojson": {semNome}}), http.StatusBadRequest, "não tem a propriedade")
	quer(t, ana.post(url1, url.Values{"geojson": {colecaoDeQuarteiroes("5", "5")}}), http.StatusBadRequest, "mais de uma vez")
	quer(t, ana.post(url1, url.Values{"geojson": {"x"}}), http.StatusBadRequest, "Importação recusada")
	if qs, _ := m.cliQ.ListQuarteiroesDoBairro(t.Context(), bid); len(qs) != 2 {
		t.Fatalf("quarteirões = %d, queria 2", len(qs))
	}

	// 1 e 2 existem; 9 é novo e será criado.
	rec := ana.post(url1, url.Values{"geojson": {colecaoDeQuarteiroes("1", "2", "9")}})
	if rec.Code != http.StatusSeeOther || !strings.Contains(local(rec), "limites=3") || !strings.Contains(local(rec), "novos=1") {
		t.Fatalf("importar: %d %q", rec.Code, local(rec))
	}
	corpo := ana.get(strings.Split(local(rec), "#")[0]).Body.String()
	for _, quer := range []string{"3 limite(s) de quarteirão importado(s), 1 quarteirão(ões) novo(s) criado(s).", "no mapa"} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("a ficha deveria conter %q", quer)
		}
	}
	if c := strings.Count(corpo, `<span class="selo">no mapa</span>`); c != 3 {
		t.Errorf("selos \"no mapa\" = %d, queria 3", c)
	}
	if qs, _ := m.cliQ.ListQuarteiroesDoBairro(t.Context(), bid); len(qs) != 3 {
		t.Errorf("quarteirões = %d, queria 3", len(qs))
	}
	ev := achar(m.eventos(t), "quarteirao.limites")
	if ev == nil || mapa(ev.Detalhes)["importados"] != float64(3) || mapa(ev.Detalhes)["criados"] != float64(1) {
		t.Errorf("quarteirao.limites: %+v", ev)
	}

	// Reimportar substitui sem duplicar.
	quer(t, ana.post(url1, url.Values{"geojson": {colecaoDeQuarteiroes("1")}}), http.StatusSeeOther)
	if qs, _ := m.cliQ.ListQuarteiroesDoBairro(t.Context(), bid); len(qs) != 3 {
		t.Errorf("reimportar não deveria criar quarteirões (%d)", len(qs))
	}
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

func TestLimitesRespeitamPermissaoEEscopo(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	bid := m.criarBairro(t, cid, "Centro")
	corpo := url.Values{"acao": {"salvar"}, "geojson": {poligonoBairro}}
	carla := m.comCarla(t)
	quer(t, carla.post("/bairros/"+s64(bid)+"/limite", corpo), http.StatusForbidden)
	quer(t, carla.post("/bairros/"+s64(bid)+"/quarteiroes/limites", url.Values{"geojson": {colecaoDeQuarteiroes("1")}}), http.StatusForbidden)
}

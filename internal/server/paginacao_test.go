package server

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestListagensPaginamComDezPorPadrao(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	for i := 1; i <= 25; i++ {
		m.criarCidade(t, "Cidade "+strconv.Itoa(100+i), "MT")
	}
	corpo := ana.get("/cidades").Body.String()
	if n := strings.Count(corpo, `class="paginacao"`); n != 2 {
		t.Errorf("a barra deve aparecer em cima e embaixo: %d", n)
	}
	for _, c := range []string{"1–10 de 25", "página 1 de 3", `<option value="10" selected>`, `<option value="30"`, `<option value="100"`} {
		if !strings.Contains(corpo, c) {
			t.Errorf("faltou %q", c)
		}
	}
	if !strings.Contains(corpo, "Cidade 101") || strings.Contains(corpo, "Cidade 111") {
		t.Error("a primeira página tem só os 10 primeiros")
	}
	quer(t, ana.get("/cidades?pagina=3"), http.StatusOK, "21–25 de 25", "Cidade 125")
	quer(t, ana.get("/cidades?pagina=99"), http.StatusOK, "21–25 de 25")
	quer(t, ana.get("/cidades?por_pagina=30"), http.StatusOK, "1–25 de 25", `<option value="30" selected>`)
	quer(t, ana.get("/cidades?por_pagina=7"), http.StatusOK, "1–10 de 25") // valor fora da lista volta ao padrão
	quer(t, ana.get("/cidades?por_pagina=20&pagina=2"), http.StatusOK, "21–25 de 25")
}

func TestTelasCarregamOScriptDeSelectPesquisavel(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.get("/cidades"), http.StatusOK, "/static/js/selectbusca.js")
	js := ana.get("/static/js/selectbusca.js")
	quer(t, js, http.StatusOK, "LIMITE = 5", "data-sem-busca")
}

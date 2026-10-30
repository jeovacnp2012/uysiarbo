package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOrdenarPorCampo(t *testing.T) {
	type linha struct {
		Nome   string
		Quando string
		N      int
		Ativo  bool
	}
	itens := []linha{
		{"b", "02/01/2026 10:00", 2, true},
		{"A", "31/12/2025 09:00", 10, false},
		{"c", "15/03/2026 08:00", 1, true},
	}
	ordem := func(query string) string {
		cp := append([]linha(nil), itens...)
		ordenarPorCampo(httptest.NewRequest("GET", "/x?"+query, nil), cp)
		s := ""
		for _, l := range cp {
			s += l.Nome
		}
		return s
	}
	for query, quer := range map[string]string{
		"": "bAc", "ordem=Nome": "Abc", "ordem=Nome&dir=desc": "cbA", "ordem=Quando": "Abc",
		"ordem=Quando&dir=desc": "cbA", "ordem=N": "cbA", "ordem=Ativo": "Abc", "ordem=Inexistente": "bAc",
	} {
		if got := ordem(query); got != quer {
			t.Errorf("%q: obtive %s, esperava %s", query, got, quer)
		}
	}
}

func TestExportarListaEmPDFEPlanilha(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	r := ana.get("/laboratorio?formato=pdf&colunas=ID||Status")
	if r.Code != 200 || !strings.HasPrefix(r.Body.String(), "%PDF") {
		t.Fatalf("pdf: status %d, início %q", r.Code, r.Body.String()[:min(20, r.Body.Len())])
	}
	r = ana.get("/laboratorio?formato=xlsx&colunas=ID||Status")
	if r.Code != 200 || !strings.Contains(r.Header().Get("Content-Type"), "spreadsheetml") || !strings.HasPrefix(r.Body.String(), "PK") {
		t.Fatalf("xlsx: status %d, tipo %q", r.Code, r.Header().Get("Content-Type"))
	}
	if ana.get("/relatorios?formato=pdf").Code == 404 {
		t.Fatal("relatórios têm o próprio formato e não passam pelo exportador de listas")
	}
}

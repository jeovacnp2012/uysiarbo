package server

import (
	"net/http/httptest"
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

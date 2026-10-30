package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestLvc(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	base := "/zoonose/lvc"
	lid := s64(c.local)
	hoje := time.Now().Format("2006-01-02")

	quer(t, ana.get(base), http.StatusOK)
	quer(t, ana.get(base+"/novo"), http.StatusOK)
	quer(t, ana.get(base+"/novo/"+lid), http.StatusOK, "Pedido de reteste")
	quer(t, ana.get(base+"/novo/999999"), http.StatusNotFound)

	form := func(extra map[string]string) url.Values {
		v := url.Values{"local": {lid}, "data": {hoje}, "nome": {"Rex"}, "sexo": {"macho"}, "proprietario": {"Joana"},
			"dpp": {"positivo"}, "pedido_reteste": {"1"}, "data_pedido": {hoje}, "resultado_reteste": {"aguardando"}, "situacao_cachorro": {"acompanhamento"}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.post(base, form(map[string]string{"data": "ontem"})), http.StatusBadRequest)
	quer(t, ana.post(base, form(map[string]string{"dpp": "talvez"})), http.StatusBadRequest)
	quer(t, ana.post(base, form(map[string]string{"proprietario_cpf": "123"})), http.StatusBadRequest)
	quer(t, ana.post(base, form(map[string]string{"data_morte": hoje})), http.StatusBadRequest)
	quer(t, ana.post(base, form(map[string]string{"local": "999999"})), http.StatusNotFound)

	rec := ana.post(base, form(nil))
	quer(t, rec, http.StatusSeeOther)
	caminho := strings.SplitN(local(rec), "?", 2)[0]
	if !strings.HasPrefix(caminho, base+"/") {
		t.Fatalf("redirecionou para %q", local(rec))
	}
	quer(t, ana.get(caminho), http.StatusOK, "Rex", "Joana")
	quer(t, ana.get(base+"?busca=rex"), http.StatusOK, "Nº de controle", "Positivo")
	quer(t, ana.get("/locais/"+lid), http.StatusOK, "Novo teste LVC", "Rex")

	quer(t, ana.post(caminho, form(map[string]string{"resultado_reteste": "negativo", "situacao_cachorro": "obito", "data_morte": hoje})), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusOK, "Negativo")
	quer(t, ana.post(caminho+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusNotFound)
}

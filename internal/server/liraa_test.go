package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestLiraaGruposEAgenda(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	loc, err := m.cliQ.GetLocalQualquer(t.Context(), c.local)
	if err != nil {
		t.Fatal(err)
	}
	bairro, quart := loc.BairroID, loc.QuarteiraoID

	// grupos
	quer(t, ana.get("/liraa/grupo"), http.StatusOK)
	quer(t, ana.get("/liraa/grupo/novo"), http.StatusOK)
	quer(t, ana.post("/liraa/grupo", url.Values{"nome": {""}}), http.StatusBadRequest)
	rec := ana.post("/liraa/grupo", url.Values{"nome": {"Estrato 01"}, "ativo": {"1"}})
	quer(t, rec, http.StatusSeeOther)
	g := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.post("/liraa/grupo", url.Values{"nome": {"estrato 01"}}), http.StatusBadRequest, "Já existe")
	quer(t, ana.get(g), http.StatusOK, "Estrato 01", "Bairros do grupo")
	quer(t, ana.post(g+"/bairros", url.Values{"bairro": {s64(bairro)}}), http.StatusSeeOther)
	quer(t, ana.get(g), http.StatusOK, "Rua das Palmeiras")
	quer(t, ana.post(g+"/bairros", url.Values{"bairro": {"999999"}}), http.StatusBadRequest)
	quer(t, ana.post(g+"/bairros/"+s64(bairro)+"/remover", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(g, url.Values{"nome": {"Estrato 1"}, "ativo": {"0"}}), http.StatusSeeOther)

	// agenda
	quer(t, ana.get("/liraa/agenda"), http.StatusOK)
	quer(t, ana.get("/liraa/agenda/novo"), http.StatusOK, "Quarteirão")
	quer(t, ana.get("/liraa/agenda/quarteiroes?bairro_id="+s64(bairro)), http.StatusOK)
	f := func(extra map[string]string) url.Values {
		v := url.Values{"ciclo": {s64(c.ciclo)}, "semana": {"12"}, "bairro": {s64(bairro)}, "quarteirao": {s64(quart)}, "agente": {m.idUsuario(t, emailAna)}, "situacao": {"aberto"}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.post("/liraa/agenda", f(map[string]string{"semana": "60"})), http.StatusBadRequest)
	quer(t, ana.post("/liraa/agenda", f(map[string]string{"quarteirao": "999999"})), http.StatusBadRequest)
	rec = ana.post("/liraa/agenda", f(nil))
	quer(t, rec, http.StatusSeeOther)
	a := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.get(a), http.StatusOK, `value="12"`)
	quer(t, ana.get("/liraa/agenda?semana=12"), http.StatusOK, "Aberto")
	quer(t, ana.get("/liraa/agenda?semana=13"), http.StatusOK, "Nenhum agendamento")
	quer(t, ana.post(a, f(map[string]string{"situacao": "encerrado"})), http.StatusSeeOther)
	quer(t, ana.get("/liraa/agenda?situacao=encerrado"), http.StatusOK, "Encerrado")
	quer(t, ana.post(a+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(a), http.StatusNotFound)
}

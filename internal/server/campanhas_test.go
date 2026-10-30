package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCampanhaAntirrabica(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agora := time.Now().UTC().Format(formatoHoraLocal)
	base := "/zoonose/campanha"
	lid := s64(c.local)

	quer(t, ana.get(base), http.StatusOK)
	quer(t, ana.get(base+"/novo"), http.StatusOK)
	quer(t, ana.get(base+"/novo/"+lid), http.StatusOK, "Animais vacinados")
	quer(t, ana.get(base+"/novo/999999"), http.StatusNotFound)

	form := func(extra map[string]string) url.Values {
		v := url.Values{"local": {lid}, "data": {agora}, "situacao": {"atendido"}, "vac_cachorros": {"2"}, "vac_gatos": {"1"}, "nao_cachorros": {"0"}, "nao_gatos": {"1"}, "descricao": {"quintal"}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.post(base, form(map[string]string{"data": "ontem"})), http.StatusBadRequest)
	quer(t, ana.post(base, form(map[string]string{"situacao": "inventada"})), http.StatusBadRequest)
	quer(t, ana.post(base, form(map[string]string{"vac_gatos": "-3"})), http.StatusBadRequest)
	quer(t, ana.post(base, form(map[string]string{"local": "999999"})), http.StatusNotFound)

	rec := ana.post(base, form(nil))
	quer(t, rec, http.StatusSeeOther)
	caminho := strings.SplitN(local(rec), "?", 2)[0]
	if !strings.HasPrefix(caminho, base+"/") {
		t.Fatalf("redirecionou para %q", local(rec))
	}
	quer(t, ana.get(caminho), http.StatusOK, "quintal")
	quer(t, ana.get(base+"?busca=quintal"), http.StatusOK, "Total vacinados")
	// a ficha do local lista o atendimento
	quer(t, ana.get("/locais/"+lid), http.StatusOK, "Nova Campanha", "Atendido")

	quer(t, ana.post(caminho, form(map[string]string{"nao_gatos": "4", "descricao": "no jardim"})), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusOK, "no jardim")
	quer(t, ana.post(caminho+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusNotFound)
}

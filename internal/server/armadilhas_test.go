package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestArmadilhasOvitrampaEEDL(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agora := time.Now().UTC().Format(formatoHoraLocal)

	for _, base := range []string{"/ovitrampas", "/edls"} {
		quer(t, ana.get(base), http.StatusOK)
		quer(t, ana.get(base+"/nova"), http.StatusOK)
		quer(t, ana.get(base+"/nova?local="+s64(c.local)), http.StatusOK)
		quer(t, ana.get(base+"/nova?local=999999"), http.StatusNotFound)

		form := func(extra map[string]string) url.Values {
			v := url.Values{"local": {s64(c.local)}, "data": {agora}, "latitude": {"-13,87"}, "longitude": {"-56.12"}, "anotacao": {"no quintal"}}
			for k, x := range extra {
				v.Set(k, x)
			}
			return v
		}
		quer(t, ana.post(base, form(map[string]string{"data": "ontem"})), http.StatusBadRequest)
		quer(t, ana.post(base, form(map[string]string{"latitude": "200"})), http.StatusBadRequest)
		quer(t, ana.post(base, form(map[string]string{"longitude": ""})), http.StatusBadRequest)
		quer(t, ana.post(base, form(map[string]string{"local": "999999"})), http.StatusNotFound)

		rec := ana.post(base, form(nil))
		quer(t, rec, http.StatusSeeOther)
		caminho := local(rec)
		if !strings.HasPrefix(caminho, base+"/") {
			t.Fatalf("redirecionou para %q", caminho)
		}
		quer(t, ana.get(caminho), http.StatusOK)

		// registro: ovitrampa conta ovos; EDL guarda o nível da água
		ruim := url.Values{"data": {agora}, "ovos": {"-1"}, "nivel": {"Transbordando"}}
		quer(t, ana.post(caminho+"/registros", ruim), http.StatusBadRequest)
		bom := url.Values{"data": {agora}, "ovos": {"37"}, "nivel": {"< 50%"}, "anotacao": {"troca de armadilha"}}
		quer(t, ana.post(caminho+"/registros", bom), http.StatusSeeOther)
		corpo := ana.get(caminho).Body.String()
		esperado := "&lt; 50%"
		if base == "/ovitrampas" {
			esperado = "37"
		}
		if !strings.Contains(corpo, "troca de armadilha") || !strings.Contains(corpo, esperado) {
			t.Fatalf("%s: a ficha deveria mostrar o registro (%s)", base, esperado)
		}
		quer(t, ana.get(base+"?busca=quintal"), http.StatusOK)

		// status: retirar exige a data; depois de retirada não recebe registros
		quer(t, ana.post(caminho+"/status", url.Values{"status": {"inventado"}}), http.StatusOK)
		quer(t, ana.post(caminho+"/status", url.Values{"status": {"retirado"}, "retirada": {""}}), http.StatusBadRequest)
		quer(t, ana.post(caminho+"/status", url.Values{"status": {"retirado"}, "retirada": {agora}}), http.StatusSeeOther)
		quer(t, ana.post(caminho+"/registros", bom), http.StatusConflict)

		// editar dados e excluir (lógica)
		quer(t, ana.post(caminho, form(map[string]string{"anotacao": "no jardim"})), http.StatusSeeOther)
		quer(t, ana.post(caminho+"/excluir", url.Values{}), http.StatusSeeOther)
		quer(t, ana.get(caminho), http.StatusNotFound)
	}
	// o tipo não vaza para a outra tela
	rec := ana.post("/ovitrampas", url.Values{"local": {s64(c.local)}, "data": {agora}})
	quer(t, rec, http.StatusSeeOther)
	quer(t, ana.get(strings.Replace(local(rec), "/ovitrampas/", "/edls/", 1)), http.StatusNotFound)
}

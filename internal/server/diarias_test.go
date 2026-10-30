package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDiariaDeTrabalho(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	base := "/zoonose/campanha/diaria"

	quer(t, ana.get(base), http.StatusOK)
	quer(t, ana.get(base+"/nova"), http.StatusOK)
	quer(t, ana.get(base+"/999999"), http.StatusNotFound)
	quer(t, ana.post(base, url.Values{"data": {"ontem"}}), http.StatusBadRequest)

	rec := ana.post(base, url.Values{"data": {time.Now().Format("2006-01-02")}, "anotacao": {"dia de teste"}})
	quer(t, rec, http.StatusSeeOther)
	caminho := strings.SplitN(local(rec), "?", 2)[0]
	if !strings.HasPrefix(caminho, base+"/") {
		t.Fatalf("redirecionou para %q", local(rec))
	}
	quer(t, ana.get(caminho), http.StatusOK, "dia de teste", "Aguardando", "Temperaturas")

	// equipe
	quer(t, ana.post(caminho+"/equipe", url.Values{"usuario": {"999999"}}), http.StatusBadRequest)
	quer(t, ana.post(caminho+"/equipe", url.Values{"usuario": {m.idUsuario(t, emailAna)}, "lider": {"1"}}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusOK, "líder")

	// registro: valores inválidos e válidos
	quer(t, ana.post(caminho+"/registro", url.Values{"ini_doses": {"-1"}}), http.StatusBadRequest)
	quer(t, ana.post(caminho+"/registro", url.Values{"estoque_08:00": {"999"}}), http.StatusBadRequest)
	quer(t, ana.post(caminho+"/registro", url.Values{"ini_doses": {"50"}, "estoque_08:00": {"4,5"}, "uso_08:00": {"5"}, "recebimento": {"07:49"}}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusOK, `value="50"`, `value="4.5"`, "07:49")

	// andamento: só inicia aguardando, só finaliza executando
	quer(t, ana.post(caminho+"/finalizar", url.Values{}), http.StatusBadRequest)
	quer(t, ana.post(caminho+"/iniciar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/iniciar", url.Values{}), http.StatusBadRequest)

	// atendimento ligado à diária aparece na ficha
	agora := time.Now().UTC().Format(formatoHoraLocal)
	id := strings.TrimPrefix(caminho, base+"/")
	quer(t, ana.post("/zoonose/campanha", url.Values{"local": {s64(c.local)}, "data": {agora}, "situacao": {"atendido"}, "vac_cachorros": {"3"}, "diaria": {id}}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusOK, "Vacinados: 3")
	// diária com atendimento não vai à lixeira
	quer(t, ana.post(caminho+"/finalizar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusOK, "Finalizado")
	quer(t, ana.post(caminho+"/reabrir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/cancelar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/reativar", url.Values{}), http.StatusSeeOther)
}

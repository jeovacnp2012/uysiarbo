package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDroneFluxoCompleto(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	hoje := time.Now().UTC().Format("2006-01-02")

	quer(t, ana.get("/drones"), http.StatusOK)
	quer(t, ana.get("/drones/nova"), http.StatusOK)
	quer(t, ana.get("/drones/nova?local="+s64(c.local)), http.StatusOK)
	quer(t, ana.get("/drones/nova?local=999999"), http.StatusNotFound)

	form := func(extra map[string]string) url.Values {
		v := url.Values{"local": {s64(c.local)}, "descricao": {"Casa fechada há vários ciclos"}, "data": {hoje}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.post("/drones", form(map[string]string{"descricao": " "})), http.StatusBadRequest)
	quer(t, ana.post("/drones", form(map[string]string{"data": ""})), http.StatusBadRequest)
	quer(t, ana.post("/drones", form(map[string]string{"local": "999999"})), http.StatusNotFound)

	rec := ana.post("/drones", form(nil))
	quer(t, rec, http.StatusSeeOther)
	caminho := local(rec)
	if !strings.HasPrefix(caminho, "/drones/") {
		t.Fatalf("redirecionou para %q", caminho)
	}
	corpo := ana.get(caminho)
	quer(t, corpo, http.StatusOK)
	if !strings.Contains(corpo.Body.String(), "Casa fechada") {
		t.Fatal("a ficha deveria mostrar a descrição")
	}

	// andamento: não encerra nem finaliza antes de atender
	for _, acao := range []string{"/encerrar", "/finalizar", "/reabrir"} {
		if rec = ana.post(caminho+acao, url.Values{}); !strings.Contains(local(rec), "caso_erro=") {
			t.Fatalf("%s antes de atender deveria voltar com erro: %s", acao, local(rec))
		}
	}
	quer(t, ana.post(caminho+"/atender", url.Values{}), http.StatusSeeOther)
	if rec = ana.post(caminho+"/atender", url.Values{}); !strings.Contains(local(rec), "caso_erro=") {
		t.Fatal("atender duas vezes deveria voltar com erro")
	}
	quer(t, ana.post(caminho+"/finalizar", url.Values{"texto": {"Sobrevoo feito"}}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/encerrar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/visita", url.Values{"visita": {""}}), http.StatusConflict)
	quer(t, ana.post(caminho+"/reabrir", url.Values{}), http.StatusSeeOther)

	// visita inexistente
	if rec = ana.post(caminho+"/visita", url.Values{"visita": {"999999"}}); !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("visita inexistente deveria voltar com erro: %s", local(rec))
	}

	quer(t, ana.get("/drones?busca=Casa&situacao=em_atendimento"), http.StatusOK)

	// local com solicitação ativa não vai para a lixeira; a solicitação vai (lógica) e some
	quer(t, ana.post(caminho+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusNotFound)
}

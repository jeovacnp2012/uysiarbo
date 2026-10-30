package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBaapFluxoCompleto(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	hoje := time.Now().UTC().Format("2006-01-02")

	quer(t, ana.get("/zoonose/baap"), http.StatusOK)
	quer(t, ana.get("/zoonose/baap/nova"), http.StatusOK)

	form := func(extra map[string]string) url.Values {
		v := url.Values{"paciente": {"João da Silva"}, "animal": {"Serpente"}, "local_picada": {"Perna"}, "data": {hoje}, "endereco": {"Sítio Boa Vista"}, "telefone": {"65999990000"}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.post("/zoonose/baap", form(map[string]string{"paciente": " "})), http.StatusBadRequest)
	quer(t, ana.post("/zoonose/baap", form(map[string]string{"animal": "Dragão"})), http.StatusBadRequest)
	quer(t, ana.post("/zoonose/baap", form(map[string]string{"animal": "Outro"})), http.StatusBadRequest)
	quer(t, ana.post("/zoonose/baap", form(map[string]string{"local_picada": "Cauda"})), http.StatusBadRequest)
	quer(t, ana.post("/zoonose/baap", form(map[string]string{"data": ""})), http.StatusBadRequest)

	rec := ana.post("/zoonose/baap", form(nil))
	quer(t, rec, http.StatusSeeOther)
	caminho := local(rec)
	if !strings.HasPrefix(caminho, "/zoonose/baap/") {
		t.Fatalf("redirecionou para %q", caminho)
	}
	corpo := ana.get(caminho)
	quer(t, corpo, http.StatusOK)
	if !strings.Contains(corpo.Body.String(), "João da Silva") {
		t.Fatal("a ficha deveria mostrar o paciente")
	}

	// andamento: só encerra depois de atender
	if rec = ana.post(caminho+"/encerrar", url.Values{}); !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("encerrar sem atender deveria voltar com erro: %s", local(rec))
	}
	quer(t, ana.post(caminho+"/atender", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho, form(map[string]string{"especie": "Cascavel"})), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/encerrar", url.Values{"texto": {"Paciente recuperado"}}), http.StatusSeeOther)
	quer(t, ana.post(caminho, form(nil)), http.StatusConflict)
	quer(t, ana.post(caminho+"/reabrir", url.Values{}), http.StatusSeeOther)

	// vínculos inexistentes
	quer(t, ana.post(caminho+"/local", url.Values{"local": {"999999"}}), http.StatusNotFound)
	if rec = ana.post(caminho+"/visita", url.Values{"visita": {"999999"}}); !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("visita inexistente deveria voltar com erro: %s", local(rec))
	}

	quer(t, ana.get("/zoonose/baap?busca=Silva&animal=Serpente"), http.StatusOK)
	quer(t, ana.post(caminho+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusNotFound)
}

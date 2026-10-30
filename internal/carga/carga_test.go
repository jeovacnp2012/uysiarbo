package carga

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPercentil(t *testing.T) {
	var l []time.Duration
	for i := 1; i <= 100; i++ {
		l = append(l, time.Duration(i)*time.Millisecond)
	}
	if got := Percentil(l, 50); got != 50*time.Millisecond {
		t.Errorf("p50 = %v", got)
	}
	if got := Percentil(l, 99); got != 99*time.Millisecond {
		t.Errorf("p99 = %v", got)
	}
	if got := Percentil(l, 100); got != 100*time.Millisecond {
		t.Errorf("máx = %v", got)
	}
	if Percentil(nil, 50) != 0 {
		t.Error("lista vazia deveria dar 0")
	}
}

func TestExecutarContaErros(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/falha" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res, err := Executar(t.Context(), Opcoes{URL: srv.URL + "/ok", Conexoes: 4, Duracao: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total == 0 || res.Erros != 0 || res.PorStatus[200] != res.Total {
		t.Fatalf("resultado inesperado: %+v", res)
	}
	if !strings.Contains(res.Resumo(), "Latência: p50") {
		t.Fatal("o resumo deveria trazer a latência")
	}

	res, err = Executar(t.Context(), Opcoes{URL: srv.URL + "/falha", Conexoes: 2, Duracao: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if res.Erros != res.Total || res.Total == 0 {
		t.Fatalf("HTTP 500 deveria contar como erro: %+v", res)
	}

	if _, err := Executar(t.Context(), Opcoes{URL: srv.URL, Conexoes: 0, Duracao: time.Second}); err == nil {
		t.Fatal("0 conexões deveria falhar")
	}
}

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"uysiarbo/internal/metrics"
)

func TestMetricasExigemToken(t *testing.T) {
	m := novoMundo(t)

	pedir := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "https://"+hostNM+"/metrics", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		m.h.ServeHTTP(rec, req)
		return rec
	}
	// os testes sobem o servidor sem METRICS_TOKEN: a rota não existe
	if rec := pedir("qualquer"); rec.Code != http.StatusNotFound {
		t.Fatalf("sem token configurado = %d, quer 404", rec.Code)
	}
}

func TestMetricasComTokenContamRequisicoes(t *testing.T) {
	h := &handlers{metricsToken: "segredo-de-teste-123"}
	h.metricas = novoRegistroDeTeste()
	h.metricas.Observar("GET", "/locais/{id}", 200, 1)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.metricasHandler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem cabeçalho = %d, quer 401", rec.Code)
	}
	req.Header.Set("Authorization", "Bearer errado")
	rec = httptest.NewRecorder()
	h.metricasHandler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("token errado = %d, quer 401", rec.Code)
	}
	req.Header.Set("Authorization", "Bearer segredo-de-teste-123")
	rec = httptest.NewRecorder()
	h.metricasHandler(rec, req)
	if rec.Code != http.StatusOK || !contem(rec.Body.String(), `uysiarbo_http_requests_total{metodo="GET",rota="/locais/{id}",status="200"} 1`) {
		t.Fatalf("token certo = %d:\n%s", rec.Code, rec.Body.String())
	}
}

func novoRegistroDeTeste() *metrics.Registro { return metrics.Novo("teste") }

func contem(s, parte string) bool { return strings.Contains(s, parte) }

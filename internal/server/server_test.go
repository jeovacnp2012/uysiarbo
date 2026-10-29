package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"uysiarbo/internal/httpx"
)

func newTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	New(newTestLogger()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quer 200", rec.Code)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Error("faltou o cabeçalho X-Request-Id")
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Errorf("corpo inesperado: %s", rec.Body.String())
	}
}

func TestNotFoundPadronizado(t *testing.T) {
	rec := httptest.NewRecorder()
	New(newTestLogger()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nao-existe", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quer 404", rec.Code)
	}
	var body httpx.ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code != "not_found" {
		t.Errorf("corpo inesperado: %s", rec.Body.String())
	}
	if body.Error.RequestID == "" {
		t.Error("erro sem request_id")
	}
}

func TestMetodoNaoPermitido(t *testing.T) {
	rec := httptest.NewRecorder()
	New(newTestLogger()).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, quer 405", rec.Code)
	}
}

func TestPanicViraErro500(t *testing.T) {
	r := chi.NewRouter()
	r.Use(requestID, recoverer(newTestLogger()))
	r.Get("/boom", func(http.ResponseWriter, *http.Request) { panic("falha de teste") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quer 500", rec.Code)
	}
	var body httpx.ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code != "internal_error" {
		t.Errorf("corpo inesperado: %s", rec.Body.String())
	}
}

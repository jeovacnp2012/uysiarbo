package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uysiarbo/internal/db/sqlc"

	"github.com/go-chi/chi/v5"

	"uysiarbo/internal/httpx"
)

func newTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(fakeDB{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

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
	newTestServer(fakeDB{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nao-existe", nil))

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
	newTestServer(fakeDB{}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))
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

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func TestReadyz(t *testing.T) {
	casos := []struct {
		nome   string
		err    error
		status int
	}{
		{"banco ok", nil, http.StatusOK},
		{"banco fora", errors.New("sem conexão"), http.StatusServiceUnavailable},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newTestServer(fakeDB{err: c.err}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if rec.Code != c.status {
				t.Fatalf("status = %d, quer %d", rec.Code, c.status)
			}
		})
	}
}

type fakeInfo struct{}

func (fakeInfo) GetAppInfo(context.Context, string) (sqlc.AppInfo, error) {
	return sqlc.AppInfo{Chave: "sistema", Valor: "UYSI ARBO"}, nil
}

func newTestServer(db Pinger) http.Handler {
	return New(Deps{Log: newTestLogger(), DB: db, Catalog: fakeInfo{}})
}

func get(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	newTestServer(fakeDB{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestHomeRenderizaLayout(t *testing.T) {
	rec := get(t, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quer 200", rec.Code)
	}
	body := rec.Body.String()
	for _, quer := range []string{"<!doctype html>", "UYSI ARBO", "menu-item", "Console da plataforma", `aria-current`} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(quer)) {
			t.Errorf("página sem %q", quer)
		}
	}
}

func TestComponentesTabelaFiltraEPagina(t *testing.T) {
	rec := get(t, "/componentes/tabela?q=exemplo+12")
	body := rec.Body.String()
	if !strings.Contains(body, "Bairro Exemplo 12") || strings.Contains(body, "Bairro Exemplo 13") {
		t.Errorf("filtro por nome não funcionou")
	}
	if strings.Contains(body, "<!doctype") {
		t.Error("o trecho HTMX não deve trazer a página inteira")
	}

	p2 := get(t, "/componentes/tabela?pagina=2").Body.String()
	if !strings.Contains(p2, "Bairro Exemplo 09") || strings.Contains(p2, "Bairro Exemplo 01<") {
		t.Errorf("paginação não funcionou")
	}
	if !strings.Contains(get(t, "/componentes/tabela?pagina=999").Body.String(), "página 6 de 6") {
		t.Errorf("página fora do limite deveria cair na última")
	}
}

func TestPaginaComponentesTemCampoDeSenhaComRevelar(t *testing.T) {
	body := get(t, "/componentes").Body.String()
	if !strings.Contains(body, "Revelar") || !strings.Contains(body, `type="password"`) {
		t.Error("campo de senha sem a opção de revelar")
	}
}

func TestEstaticos(t *testing.T) {
	for _, path := range []string{"/static/css/app.css", "/static/js/app.js", "/static/brand/uysi-symbol.png"} {
		if rec := get(t, path); rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", path, rec.Code)
		}
	}
}

func (fakeInfo) ListClientes(context.Context) ([]sqlc.Cliente, error) { return nil, nil }

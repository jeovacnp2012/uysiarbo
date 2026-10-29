package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/tenant"
	"uysiarbo/internal/testutil"
)

// novoServidorComClientes sobe o servidor com banco real e dois clientes provisionados.
func novoServidorComClientes(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()
	url := testutil.NewDatabase(t)
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &tenant.Service{Pool: pool, URL: url, Log: log}
	for _, c := range []tenant.Input{
		{Nome: "Prefeitura de Nova Mutum", Slug: "novamutum", Subdominio: "uysiarbonm"},
		{Nome: "Prefeitura de Sinop", Slug: "sinop", Subdominio: "uysiarbosn", Dominio: "arbo.sinop.mt.gov.br"},
	} {
		if _, err := svc.Provision(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	// Um cliente suspenso e um ainda provisionando.
	q := sqlc.New(pool)
	susp, _ := svc.Provision(ctx, tenant.Input{Nome: "Suspenso", Slug: "suspenso"})
	if _, err := q.UpdateClienteStatus(ctx, sqlc.UpdateClienteStatusParams{ID: susp.ID, Status: "suspenso"}); err != nil {
		t.Fatal(err)
	}

	pools := &tenant.Pools{URL: url}
	t.Cleanup(pools.Close)
	return New(Deps{
		Log:      log,
		DB:       pool,
		Catalog:  q,
		Resolver: &tenant.Resolver{Pool: pool, BaseDomain: "inovarsistemas.biz", AdminHost: "uysiarbo.inovarsistemas.biz"},
		Pools:    pools,
	})
}

func pedir(h http.Handler, host, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestEnderecoSelecionaOCliente(t *testing.T) {
	h := novoServidorComClientes(t)

	casos := []struct {
		nome, host, path string
		status           int
		contem           []string
		naoContem        []string
	}{
		{"subdomínio do cliente", "uysiarbonm.inovarsistemas.biz", "/", 200,
			[]string{"Prefeitura de Nova Mutum", "c_novamutum"}, []string{"Sinop"}},
		{"com porta e maiúsculas", "UYSIARBONM.inovarsistemas.biz:8443", "/", 200,
			[]string{"Prefeitura de Nova Mutum"}, nil},
		{"outro cliente, outro schema", "uysiarbosn.inovarsistemas.biz", "/", 200,
			[]string{"Prefeitura de Sinop", "c_sinop"}, []string{"Nova Mutum"}},
		{"domínio próprio", "arbo.sinop.mt.gov.br", "/", 200,
			[]string{"Prefeitura de Sinop"}, nil},
		{"console da plataforma", "uysiarbo.inovarsistemas.biz", "/", 200,
			[]string{"Console da plataforma", "Clientes cadastrados"}, []string{"c_novamutum"}},
		{"cliente suspenso", "suspenso.inovarsistemas.biz", "/", 403,
			[]string{"tenant_suspended"}, nil},
		{"cliente inexistente", "naoexiste.inovarsistemas.biz", "/", 404, []string{"not_found"}, nil},
		{"host de outro sistema", "labpront.inovarsistemas.biz", "/", 404, []string{"not_found"}, nil},
		{"dois níveis abaixo do base", "a.uysiarbonm.inovarsistemas.biz", "/", 404, nil, nil},
		{"healthz vale em qualquer host", "qualquer.coisa", "/healthz", 200, []string{`"ok"`}, nil},
		{"estáticos valem em qualquer host", "qualquer.coisa", "/static/css/app.css", 200, nil, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rec := pedir(h, c.host, c.path)
			body := rec.Body.String()
			if rec.Code != c.status {
				t.Fatalf("status = %d, quer %d (corpo: %.200s)", rec.Code, c.status, body)
			}
			for _, quer := range c.contem {
				if !strings.Contains(body, quer) {
					t.Errorf("faltou %q", quer)
				}
			}
			for _, nao := range c.naoContem {
				if strings.Contains(body, nao) {
					t.Errorf("não deveria conter %q", nao)
				}
			}
		})
	}
}

func TestMenuMostraNomeDoCliente(t *testing.T) {
	body := pedir(novoServidorComClientes(t), "uysiarbonm.inovarsistemas.biz", "/componentes").Body.String()
	if !strings.Contains(body, `class="menu-cliente"`) || !strings.Contains(body, "Prefeitura de Nova Mutum") {
		t.Error("o menu deveria mostrar o nome do cliente")
	}
}

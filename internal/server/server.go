// Package server monta o roteador HTTP da aplicação.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/httpx"
	"uysiarbo/internal/tenant"
	"uysiarbo/web"
)

// Pinger verifica se uma dependência (o banco) está respondendo.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps reúne o que o servidor precisa para atender.
type Deps struct {
	Log      *slog.Logger
	DB       Pinger
	Catalog  Catalog          // catálogo (schema public)
	Resolver *tenant.Resolver // nil = sem clientes: tudo é console (usado em testes simples)
	Pools    *tenant.Pools    // conexões por schema de cliente

	// Login. Sem Public, as páginas não exigem login (usado só em testes simples).
	Public        *sqlc.Queries
	Captcha       Captcha // nil = sem captcha
	RecaptchaSite string  // chave pública do reCAPTCHA (vai para a tela de entrar)
	Producao      bool    // cookies sempre Secure
}

type handlers struct {
	log     *slog.Logger
	cat     Catalog
	pools   *tenant.Pools
	pub     *sqlc.Queries
	captcha Captcha
	siteKey string
	prod    bool
}

func New(d Deps) http.Handler {
	h := &handlers{log: d.Log, cat: d.Catalog, pools: d.Pools, pub: d.Public, captcha: d.Captcha, siteKey: d.RecaptchaSite, prod: d.Producao}
	if h.captcha == nil {
		h.captcha = SemCaptcha{}
	}

	r := chi.NewRouter()
	r.Use(securityHeaders, requestID, middleware.RealIP, accessLog(d.Log), recoverer(d.Log))
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(tenancy(d.Log, d.Resolver), csrf, h.autenticacao)

	r.Get("/healthz", health)
	r.Get("/readyz", ready(d.Log, d.DB))

	r.Handle("/static/*", http.StripPrefix("/static/", staticHandler()))

	r.Get("/entrar", h.entrarForm)
	r.Post("/entrar", h.entrarPost)
	r.Post("/sair", h.sair)
	r.Get("/trocar-senha", h.trocarSenhaForm)
	r.Post("/trocar-senha", h.trocarSenhaPost)

	r.Get("/", h.home)
	r.Get("/componentes", h.components)
	r.Get("/componentes/tabela", h.componentsTable)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Página ou recurso não encontrado.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Método não permitido.")
	})
	return r
}

func staticHandler() http.Handler {
	files := http.FileServerFS(web.Static())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		files.ServeHTTP(w, r)
	})
}

// render escreve um componente templ como HTML (status 200).
func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	renderStatus(w, r, http.StatusOK, c)
}

// renderStatus escreve um componente templ como HTML com o status dado.
func renderStatus(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		slog.Error("renderizar página", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
	}
}

// health indica apenas que o processo está de pé.
func health(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready indica que a aplicação consegue atender: o banco precisa responder.
func ready(log *slog.Logger, db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			log.Error("banco indisponível", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
			httpx.Error(w, r, http.StatusServiceUnavailable, "db_unavailable", "Banco de dados indisponível.")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "ok"})
	}
}

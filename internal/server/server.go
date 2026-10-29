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

	"uysiarbo/internal/httpx"
	"uysiarbo/web"
)

// Pinger verifica se uma dependência (o banco) está respondendo.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps reúne o que o servidor precisa para atender.
type Deps struct {
	Log  *slog.Logger
	DB   Pinger
	Info AppInfoReader
}

type handlers struct {
	log  *slog.Logger
	info AppInfoReader
}

func New(d Deps) http.Handler {
	h := &handlers{log: d.Log, info: d.Info}

	r := chi.NewRouter()
	r.Use(requestID, middleware.RealIP, accessLog(d.Log), recoverer(d.Log))
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", health)
	r.Get("/readyz", ready(d.Log, d.DB))

	r.Handle("/static/*", http.StripPrefix("/static/", staticHandler()))

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

// render escreve um componente templ como HTML.
func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
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

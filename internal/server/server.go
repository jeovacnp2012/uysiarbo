// Package server monta o roteador HTTP da aplicação.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"uysiarbo/internal/httpx"
)

// Pinger verifica se uma dependência (o banco) está respondendo.
type Pinger interface {
	Ping(ctx context.Context) error
}

func New(log *slog.Logger, db Pinger) http.Handler {
	r := chi.NewRouter()
	r.Use(requestID, middleware.RealIP, accessLog(log), recoverer(log))
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", health)
	r.Get("/readyz", ready(log, db))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Página ou recurso não encontrado.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Método não permitido.")
	})
	return r
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

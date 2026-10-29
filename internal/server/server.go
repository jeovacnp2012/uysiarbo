// Package server monta o roteador HTTP da aplicação.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"uysiarbo/internal/httpx"
)

func New(log *slog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(requestID, middleware.RealIP, accessLog(log), recoverer(log))
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", health)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Página ou recurso não encontrado.")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Método não permitido.")
	})
	return r
}

func health(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"uysiarbo/internal/httpx"
	"uysiarbo/internal/metrics"
)

// requestID garante o cabeçalho X-Request-Id na requisição e na resposta.
func requestID(next http.Handler) http.Handler {
	return middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.GetReqID(r.Context())
		r.Header.Set("X-Request-Id", id)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r)
	}))
}

// requisicaoLenta é o tempo a partir do qual a requisição também gera um aviso no log.
const requisicaoLenta = time.Second

// rotaDe devolve o padrão da rota atendida (ex.: /locais/{id}), para as métricas não terem um valor por id.
func rotaDe(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return "(sem rota)"
}

// accessLog registra cada requisição de forma estruturada.
func accessLog(log *slog.Logger, m *metrics.Registro) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			m.Entrou()
			defer m.Saiu()
			next.ServeHTTP(ww, r)
			dur := time.Since(start)
			m.Observar(r.Method, rotaDe(r), ww.Status(), dur)
			if dur >= requisicaoLenta {
				log.Warn("requisição lenta",
					"method", r.Method, "path", r.URL.Path, "status", ww.Status(),
					"duration_ms", dur.Milliseconds(), "request_id", r.Header.Get("X-Request-Id"))
			}
			log.Info("requisição",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", r.Header.Get("X-Request-Id"),
			)
		})
	}
}

// recoverer transforma panics em erro 500 padronizado, com o stack no log.
func recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if rec == http.ErrAbortHandler {
						panic(rec)
					}
					log.Error("panic na requisição",
						"panic", rec,
						"stack", string(debug.Stack()),
						"request_id", r.Header.Get("X-Request-Id"),
					)
					httpx.Error(w, r, http.StatusInternalServerError, "internal_error", "Erro interno. Tente novamente.")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/metrics"
)

// metricasHandler expõe as métricas no formato do Prometheus. Só responde com o token certo
// (METRICS_TOKEN); sem token configurado a rota não existe.
func (h *handlers) metricasHandler(w http.ResponseWriter, r *http.Request) {
	if h.metricsToken == "" {
		http.NotFound(w, r)
		return
	}
	dado := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(dado), []byte(h.metricsToken)) != 1 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
		http.Error(w, "não autorizado", http.StatusUnauthorized)
		return
	}
	var pool *metrics.Pool
	if p, ok := h.db.(interface{ Stat() *pgxpool.Stat }); ok {
		s := p.Stat()
		pool = &metrics.Pool{Abertas: s.TotalConns(), Ociosas: s.IdleConns(), EmUso: s.AcquiredConns(), Maximo: s.MaxConns()}
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	h.metricas.Escrever(w, pool)
}

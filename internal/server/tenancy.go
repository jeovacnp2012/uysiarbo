package server

import (
	"log/slog"
	"net/http"
	"strings"

	"uysiarbo/internal/httpx"
	"uysiarbo/internal/tenant"
)

// semCliente são os caminhos que valem em qualquer endereço (monitoramento e arquivos estáticos).
func semCliente(path string) bool {
	return path == "/healthz" || path == "/readyz" || path == "/metrics" || strings.HasPrefix(path, "/static/")
}

// tenancy descobre o cliente pelo endereço da requisição e o guarda no contexto. Sem resolvedor
// (testes simples), tudo é tratado como console da plataforma.
func tenancy(log *slog.Logger, res *tenant.Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if res == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if semCliente(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			cliente, kind, err := res.Lookup(r.Context(), r.Host)
			if err != nil {
				log.Error("resolver cliente", "host", r.Host, "erro", err, "request_id", r.Header.Get("X-Request-Id"))
				httpx.Error(w, r, http.StatusServiceUnavailable, "tenant_unavailable", "Serviço temporariamente indisponível.")
				return
			}
			switch kind {
			case tenant.Admin:
				next.ServeHTTP(w, r)
			case tenant.Client:
				switch cliente.Status {
				case "ativo":
					next.ServeHTTP(w, r.WithContext(tenant.WithCliente(r.Context(), cliente)))
				case "suspenso":
					httpx.Error(w, r, http.StatusForbidden, "tenant_suspended", "O acesso deste cliente está suspenso. Fale com a administração do sistema.")
				default:
					httpx.Error(w, r, http.StatusNotFound, "not_found", "Endereço não reconhecido.")
				}
			default:
				httpx.Error(w, r, http.StatusNotFound, "not_found", "Endereço não reconhecido.")
			}
		})
	}
}

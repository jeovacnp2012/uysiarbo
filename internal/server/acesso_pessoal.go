package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"uysiarbo/internal/audit"
)

// registrandoAcessoPessoal registra na auditoria quem abriu a ficha de um registro com dados
// pessoais (LGPD: saber quem viu o quê). Só conta a abertura que deu certo (status 200).
func (h *handlers) registrandoAcessoPessoal(entidade string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next(ww, r)
		if ww.Status() != http.StatusOK {
			return
		}
		h.auditar(r, audit.Evento{
			Acao: "dados_pessoais.ver", Entidade: entidade, EntidadeID: chi.URLParam(r, "id"),
			Rotulo: "Ficha de " + entidade + " " + chi.URLParam(r, "id"),
		})
	}
}

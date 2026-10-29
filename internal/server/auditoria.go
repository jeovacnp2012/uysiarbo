package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/auth"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/tenant"
)

type auditKey struct{}

// auditEstado marca, durante uma requisição, que algum evento específico já foi registrado (para o
// registro genérico não duplicar).
type auditEstado struct{ feito bool }

func ipDe(r *http.Request) string {
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 && !strings.Contains(ip, "]") {
		ip = ip[:i]
	}
	return ip
}

// registrador devolve o histórico do escopo da requisição: o do cliente, ou o da plataforma no console.
func (h *handlers) registrador(r *http.Request) audit.Registrador {
	if h.pub == nil {
		return nil
	}
	if c, ok := tenant.FromContext(r.Context()); ok {
		pool, err := h.pools.Get(r.Context(), c.SchemaNome)
		if err != nil {
			h.log.Error("auditoria: pool do cliente", "erro", err)
			return nil
		}
		return audit.Cliente{Q: sqlccliente.New(pool)}
	}
	return audit.Plataforma{Q: h.pub}
}

// atorDaConta converte uma conta de login em ator da auditoria.
func atorDaConta(c auth.Conta) audit.Ator {
	tipo := audit.AtorUsuario
	if c.Tipo == auth.TipoPlataforma {
		tipo = audit.AtorPlataforma
	}
	return audit.Ator{Tipo: tipo, ID: c.ID, Nome: c.Nome, Email: c.Email}
}

// atorDaSessao é quem está logado na requisição (vazio se ninguém).
func atorDaSessao(r *http.Request) audit.Ator {
	if a, ok := authFrom(r.Context()); ok && a.sess != nil {
		return atorDaConta(a.sess.Conta)
	}
	return audit.Ator{}
}

// auditar registra um evento no histórico do escopo. Preenche ator (a sessão atual, se não vier no
// evento), IP, navegador e identificador da requisição. Falha ao gravar vai para o log: a ação do
// usuário não é desfeita, mas o erro aparece e deve ser tratado.
func (h *handlers) auditar(r *http.Request, e audit.Evento) {
	reg := h.registrador(r)
	if reg == nil {
		return
	}
	h.auditarCom(reg, r, e)
}

// auditarCom é como auditar, mas grava no registrador dado (por exemplo, dentro de uma transação,
// para a auditoria entrar junto com a mudança).
func (h *handlers) auditarCom(reg audit.Registrador, r *http.Request, e audit.Evento) {
	if st, ok := r.Context().Value(auditKey{}).(*auditEstado); ok {
		st.feito = true
	}
	if e.Ator.Tipo == "" {
		e.Ator = atorDaSessao(r)
	}
	if e.Ator.Tipo == "" {
		e.Ator = audit.Ator{Tipo: audit.AtorSistema, Nome: "Anônimo"}
	}
	e.IP, e.UserAgent, e.RequestID = ipDe(r), r.UserAgent(), r.Header.Get("X-Request-Id")
	if err := reg.Registrar(r.Context(), e); err != nil {
		h.log.Error("ERRO AO GRAVAR AUDITORIA", "acao", e.Acao, "erro", err, "request_id", e.RequestID)
	}
}

// naoAuditar marca a requisição como já tratada, sem registrar nada (ação sem efeito).
func naoAuditar(r *http.Request) {
	if st, ok := r.Context().Value(auditKey{}).(*auditEstado); ok {
		st.feito = true
	}
}

// auditoriaGenerica garante que toda gravação bem-sucedida deixe rastro. Se o handler registrou um
// evento específico, nada é acrescentado; senão, grava "requisicao.escrita" com o método, o caminho
// e o resultado. Assim, uma tela nova nunca passa em branco por esquecimento.
func (h *handlers) auditoriaGenerica(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.pub == nil || semCliente(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		st := &auditEstado{}
		r = r.WithContext(context.WithValue(r.Context(), auditKey{}, st))
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if st.feito || ww.Status() >= 400 {
			return
		}
		h.auditar(r, audit.Evento{
			Acao: "requisicao.escrita",
			Detalhes: map[string]any{
				"metodo": r.Method, "caminho": r.URL.Path, "status": ww.Status(),
			},
		})
	})
}

package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/auth"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

const sessaoCookie = "uysi_sessao"

type authKey struct{}

// authInfo é o que o middleware de login deixa no contexto: o serviço do escopo (console ou
// cliente) e, se a pessoa estiver logada, a sessão.
type authInfo struct {
	svc  *auth.Service
	sess *auth.Sessao
}

func authFrom(ctx context.Context) (authInfo, bool) {
	a, ok := ctx.Value(authKey{}).(authInfo)
	return a, ok
}

// authService monta o serviço de autenticação do escopo da requisição: no endereço de um cliente,
// os usuários e sessões ficam no schema dele (e o master da plataforma também pode entrar); no
// console, só contas da plataforma.
func (h *handlers) authService(ctx context.Context) (*auth.Service, error) {
	if c, ok := tenant.FromContext(ctx); ok {
		pool, err := h.pools.Get(ctx, c.SchemaNome)
		if err != nil {
			return nil, err
		}
		q := sqlccliente.New(pool)
		return &auth.Service{
			Contas:  []auth.ContaStore{auth.ClienteContas{Q: q}, auth.PlataformaContas{Q: h.pub, SoMaster: true}},
			Sessoes: auth.ClienteSessoes{Q: q},
		}, nil
	}
	return &auth.Service{
		Contas:  []auth.ContaStore{auth.PlataformaContas{Q: h.pub}},
		Sessoes: auth.PlataformaSessoes{Q: h.pub},
	}, nil
}

// autenticacao exige login em tudo, exceto monitoramento, arquivos estáticos e a tela de entrar.
// Sem consulta ao catálogo configurada (testes simples), não exige nada.
func (h *handlers) autenticacao(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.pub == nil || semCliente(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		svc, err := h.authService(r.Context())
		if err != nil {
			h.log.Error("serviço de autenticação", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
			http.Error(w, "Serviço temporariamente indisponível.", http.StatusServiceUnavailable)
			return
		}
		info := authInfo{svc: svc}
		if c, err := r.Cookie(sessaoCookie); err == nil {
			sess, err := svc.Autenticar(r.Context(), c.Value)
			switch {
			case err == nil:
				info.sess = sess
			case errors.Is(err, auth.ErrSessaoInvalida):
				h.limparSessao(w, r)
			default:
				h.log.Error("autenticar", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
				http.Error(w, "Serviço temporariamente indisponível.", http.StatusServiceUnavailable)
				return
			}
		}
		r = r.WithContext(context.WithValue(r.Context(), authKey{}, info))

		if r.URL.Path == "/entrar" {
			next.ServeHTTP(w, r)
			return
		}
		if info.sess == nil {
			destino := "/entrar"
			if r.Method == http.MethodGet && r.Header.Get("HX-Request") == "" {
				destino += "?proximo=" + url.QueryEscape(r.URL.RequestURI())
			}
			irPara(w, r, destino, http.StatusUnauthorized)
			return
		}
		if info.sess.Conta.DeveTrocarSenha && r.URL.Path != "/trocar-senha" && r.URL.Path != "/sair" {
			irPara(w, r, "/trocar-senha", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// irPara redireciona páginas comuns (303) e manda o HTMX redirecionar a página inteira.
func irPara(w http.ResponseWriter, r *http.Request, destino string, statusHTMX int) {
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", destino)
		w.WriteHeader(statusHTMX)
		return
	}
	http.Redirect(w, r, destino, http.StatusSeeOther)
}

func (h *handlers) gravarSessao(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessaoCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: h.prod || ehHTTPS(r), MaxAge: int((12 * time.Hour).Seconds()),
	})
}

func (h *handlers) limparSessao(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessaoCookie, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: h.prod || ehHTTPS(r), MaxAge: -1,
	})
}

// destinoSeguro só aceita caminhos do próprio site (evita redirecionar para um site de terceiros).
func destinoSeguro(s string) string {
	if s == "" || s[0] != '/' || strings.HasPrefix(s, "//") || strings.HasPrefix(s, "/\\") || strings.ContainsAny(s, "\r\n") {
		return "/"
	}
	return s
}

// nomeDoCliente é o nome mostrado nas telas de acesso (vazio no console).
func nomeDoCliente(r *http.Request) string {
	if c, ok := tenant.FromContext(r.Context()); ok {
		return c.Nome
	}
	return ""
}

var fusoLocal = func() *time.Location {
	if l, err := time.LoadLocation("America/Cuiaba"); err == nil {
		return l
	}
	return time.Local
}()

func (h *handlers) entrarForm(w http.ResponseWriter, r *http.Request) {
	if a, _ := authFrom(r.Context()); a.sess != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	render(w, r, views.Login(h.loginData(r, "", "", r.URL.Query().Get("proximo"))))
}

func (h *handlers) loginData(r *http.Request, email, erro, proximo string) views.LoginData {
	return views.LoginData{
		Cliente: nomeDoCliente(r), Email: email, Erro: erro, Proximo: destinoSeguro(proximo),
		CSRF: csrfToken(r.Context()), SiteKey: h.siteKey,
	}
}

func (h *handlers) entrarPost(w http.ResponseWriter, r *http.Request) {
	a, _ := authFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.PostFormValue("email"))
	proximo := r.PostFormValue("proximo")
	falha := func(status int, msg string) {
		renderStatus(w, r, status, views.Login(h.loginData(r, email, msg, proximo)))
	}

	ip := ipDe(r)
	ok, err := h.captcha.Verificar(r.Context(), r.PostFormValue("g-recaptcha-response"), ip)
	if err != nil {
		h.log.Error("verificar captcha", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
		falha(http.StatusServiceUnavailable, "Não foi possível verificar o desafio agora. Tente de novo em instantes.")
		return
	}
	if !ok {
		falha(http.StatusBadRequest, "Confirme que você não é um robô.")
		return
	}

	token, sess, err := a.svc.Entrar(r.Context(), email, r.PostFormValue("senha"), ip, r.UserAgent())
	var bloq auth.ErrBloqueada
	switch {
	case errors.As(err, &bloq):
		falha(http.StatusTooManyRequests, "Muitas tentativas erradas. Tente de novo depois das "+bloq.Ate.In(fusoLocal).Format("15:04")+".")
		return
	case errors.Is(err, auth.ErrCredenciais):
		// Só registra quando o e-mail existe (assim o histórico não enche de tentativas aleatórias).
		var cred auth.ErroCredenciais
		if errors.As(err, &cred) && cred.Conta != nil && cred.Conta.Ativo {
			ator := atorDaConta(*cred.Conta)
			det := map[string]any{"tentativa": cred.Conta.Tentativas + 1}
			h.auditar(r, audit.Evento{Ator: ator, Acao: "login.falha", Entidade: "conta", EntidadeID: cred.Conta.ID, Rotulo: cred.Conta.Nome, Detalhes: det})
			if cred.Bloqueou {
				h.auditar(r, audit.Evento{Ator: ator, Acao: "login.bloqueio", Entidade: "conta", EntidadeID: cred.Conta.ID, Rotulo: cred.Conta.Nome, Detalhes: det})
			}
		}
		falha(http.StatusUnauthorized, "E-mail ou senha incorretos.")
		return
	case err != nil:
		h.log.Error("entrar", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
		falha(http.StatusInternalServerError, "Não foi possível entrar agora. Tente de novo.")
		return
	}

	h.auditar(r, audit.Evento{
		Ator: atorDaConta(sess.Conta), Acao: "login.sucesso", Entidade: "conta", EntidadeID: sess.Conta.ID, Rotulo: sess.Conta.Nome,
		Detalhes: map[string]any{"troca_pendente": sess.Conta.DeveTrocarSenha},
	})
	h.gravarSessao(w, r, token)
	destino := destinoSeguro(proximo)
	if sess.Conta.DeveTrocarSenha {
		destino = "/trocar-senha"
	}
	http.Redirect(w, r, destino, http.StatusSeeOther)
}

func (h *handlers) sair(w http.ResponseWriter, r *http.Request) {
	if a, ok := authFrom(r.Context()); ok {
		if c, err := r.Cookie(sessaoCookie); err == nil {
			_ = a.svc.Sair(r.Context(), c.Value)
		}
		if a.sess != nil {
			h.auditar(r, audit.Evento{Acao: "logout", Entidade: "conta", EntidadeID: a.sess.Conta.ID, Rotulo: a.sess.Conta.Nome})
		}
	}
	h.limparSessao(w, r)
	http.Redirect(w, r, "/entrar", http.StatusSeeOther)
}

func (h *handlers) trocarSenhaForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.TrocarSenha(h.trocarData(r, "")))
}

func (h *handlers) trocarData(r *http.Request, erro string) views.TrocarSenhaData {
	a, _ := authFrom(r.Context())
	return views.TrocarSenhaData{
		Cliente: nomeDoCliente(r), Nome: a.sess.Conta.Nome, Obrigatoria: a.sess.Conta.DeveTrocarSenha,
		Erro: erro, CSRF: csrfToken(r.Context()),
	}
}

func (h *handlers) trocarSenhaPost(w http.ResponseWriter, r *http.Request) {
	a, _ := authFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	falha := func(msg string) {
		renderStatus(w, r, http.StatusBadRequest, views.TrocarSenha(h.trocarData(r, msg)))
	}
	nova := r.PostFormValue("nova")
	if nova != r.PostFormValue("confirmar") {
		falha("A confirmação não confere com a nova senha.")
		return
	}
	obrigatoria := a.sess.Conta.DeveTrocarSenha
	err := a.svc.TrocarSenha(r.Context(), a.sess, r.PostFormValue("atual"), nova)
	var fraca auth.SenhaFracaError
	switch {
	case errors.Is(err, auth.ErrSenhaAtual):
		falha("A senha atual não confere.")
		return
	case errors.As(err, &fraca):
		falha(fraca.Motivo)
		return
	case err != nil:
		h.log.Error("trocar senha", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
		renderStatus(w, r, http.StatusInternalServerError, views.TrocarSenha(h.trocarData(r, "Não foi possível trocar a senha agora. Tente de novo.")))
		return
	}
	h.auditar(r, audit.Evento{
		Acao: "senha.trocar", Entidade: "conta", EntidadeID: a.sess.Conta.ID, Rotulo: a.sess.Conta.Nome,
		Detalhes: map[string]any{"troca_obrigatoria": obrigatoria},
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

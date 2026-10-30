package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/auth"
	"uysiarbo/internal/comunicacao"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

const (
	validadeRecuperacao = time.Hour
	pedidosPorJanela    = 3 // pedidos por conta dentro da janela, para ninguém ser inundado de e-mails
	janelaRecuperacao   = 15 * time.Minute
)

func (h *handlers) esqueciSenhaForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.EsqueciSenha(h.esqueciData(r, "", "", false)))
}

func (h *handlers) esqueciData(r *http.Request, email, erro string, enviado bool) views.EsqueciSenhaData {
	return views.EsqueciSenhaData{Cliente: nomeDoCliente(r), Email: email, Erro: erro, Enviado: enviado, CSRF: csrfToken(r.Context()), SiteKey: h.siteKey}
}

// esqueciSenhaPost registra o pedido e manda o link por e-mail. A resposta é sempre a mesma, exista
// ou não a conta, para a tela não revelar quem tem cadastro. Só vale para usuários de um cliente; no
// console, a conta master é recuperada pela linha de comando.
func (h *handlers) esqueciSenhaPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.PostFormValue("email"))
	falha := func(status int, msg string) {
		renderStatus(w, r, status, views.EsqueciSenha(h.esqueciData(r, email, msg, false)))
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
	if email != "" {
		if err := h.enviarLinkDeSenha(r, email, ip); err != nil {
			h.log.Error("recuperar senha", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
		}
	}
	render(w, r, views.EsqueciSenha(h.esqueciData(r, email, "", true)))
}

func (h *handlers) enviarLinkDeSenha(r *http.Request, email, ip string) error {
	c, ok := tenant.FromContext(r.Context())
	if !ok {
		return nil
	}
	ctx := r.Context()
	pool, err := h.pools.Get(ctx, c.SchemaNome)
	if err != nil {
		return err
	}
	q := sqlccliente.New(pool)
	conta, err := auth.ClienteContas{Q: q}.PorEmail(ctx, email)
	if errors.Is(err, auth.ErrNaoEncontrada) {
		return nil
	}
	if err != nil {
		return err
	}
	if !conta.Ativo {
		return nil
	}
	uid, err := strconv.ParseInt(conta.ID, 10, 64)
	if err != nil {
		return err
	}
	recentes, err := q.CountRecuperacoesRecentes(ctx, sqlccliente.CountRecuperacoesRecentesParams{UsuarioID: uid, CriadoEm: pgtype.Timestamptz{Time: time.Now().Add(-janelaRecuperacao), Valid: true}})
	if err != nil {
		return err
	}
	if recentes >= pedidosPorJanela {
		return nil
	}
	codigo, hash, err := auth.NovoCodigo()
	if err != nil {
		return err
	}
	if err := q.CreateRecuperacaoSenha(ctx, sqlccliente.CreateRecuperacaoSenhaParams{
		TokenHash: hash, UsuarioID: uid, ExpiraEm: pgtype.Timestamptz{Time: time.Now().Add(validadeRecuperacao), Valid: true}, Ip: ip,
	}); err != nil {
		return err
	}
	h.auditar(r, audit.Evento{Ator: atorDaConta(conta), Acao: "senha.recuperar_pedido", Entidade: "conta", EntidadeID: conta.ID, Rotulo: conta.Nome})

	// O link não passa pela fila de Comunicações de propósito: ali o texto fica visível a quem
	// administra mensagens, e quem lê o link poderia trocar a senha de outra pessoa.
	if !h.provedorCom.Atende(comunicacao.Email) {
		h.log.Warn("recuperação de senha sem envio de e-mail configurado (SMTP_HOST)", "conta", conta.ID)
		return nil
	}
	esquema := "http"
	if h.prod || ehHTTPS(r) {
		esquema = "https"
	}
	link := esquema + "://" + r.Host + "/redefinir-senha?codigo=" + codigo
	corpo := "Olá, " + conta.Nome + ".\n\nRecebemos um pedido para criar uma nova senha no UYSI ARBO (" + c.Nome + ").\n" +
		"Abra o link abaixo para escolher a nova senha. Ele vale por 1 hora e só pode ser usado uma vez:\n\n" + link +
		"\n\nSe não foi você, ignore esta mensagem: a sua senha continua a mesma."
	envio, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	_, err = h.provedorCom.Enviar(envio, comunicacao.Mensagem{Canal: comunicacao.Email, Destinatario: conta.Email, Nome: conta.Nome, Assunto: "Nova senha - UYSI ARBO", Corpo: corpo})
	return err
}

func (h *handlers) redefinirData(r *http.Request, codigo, erro string) views.RedefinirSenhaData {
	return views.RedefinirSenhaData{Cliente: nomeDoCliente(r), Codigo: codigo, Erro: erro, CSRF: csrfToken(r.Context())}
}

// recuperacaoValida diz se o código existe, não foi usado e não venceu.
func recuperacaoValida(ctx context.Context, q *sqlccliente.Queries, codigo string) (sqlccliente.RecuperacoesSenha, bool) {
	hash, ok := auth.HashDoCodigo(codigo)
	if !ok {
		return sqlccliente.RecuperacoesSenha{}, false
	}
	rec, err := q.GetRecuperacaoSenhaParaUsar(ctx, hash)
	if err != nil || rec.UsadoEm.Valid || time.Now().After(rec.ExpiraEm.Time) {
		return sqlccliente.RecuperacoesSenha{}, false
	}
	return rec, true
}

func (h *handlers) redefinirSenhaForm(w http.ResponseWriter, r *http.Request) {
	codigo := r.URL.Query().Get("codigo")
	d := h.redefinirData(r, codigo, "")
	c, ok := tenant.FromContext(r.Context())
	valido := false
	if ok {
		if pool, err := h.pools.Get(r.Context(), c.SchemaNome); err == nil {
			_, valido = recuperacaoValida(r.Context(), sqlccliente.New(pool), codigo)
		}
	}
	d.Invalido = !valido
	render(w, r, views.RedefinirSenha(d))
}

func (h *handlers) redefinirSenhaPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	codigo := r.PostFormValue("codigo")
	d := h.redefinirData(r, codigo, "")
	invalido := func() {
		d.Invalido = true
		renderStatus(w, r, http.StatusBadRequest, views.RedefinirSenha(d))
	}
	indisponivel := func(err error) {
		h.log.Error("redefinir senha", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
		http.Error(w, "Serviço temporariamente indisponível.", http.StatusServiceUnavailable)
	}
	c, ok := tenant.FromContext(r.Context())
	if !ok {
		invalido()
		return
	}
	nova := r.PostFormValue("nova")
	if nova != r.PostFormValue("confirmar") {
		d.Erro = "A confirmação não confere com a nova senha."
		renderStatus(w, r, http.StatusBadRequest, views.RedefinirSenha(d))
		return
	}
	ctx := r.Context()
	pool, err := h.pools.Get(ctx, c.SchemaNome)
	if err != nil {
		indisponivel(err)
		return
	}
	svc, err := h.authService(ctx)
	if err != nil {
		indisponivel(err)
		return
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		indisponivel(err)
		return
	}
	defer tx.Rollback(ctx)
	q := sqlccliente.New(pool)
	qt := q.WithTx(tx)
	rec, valido := recuperacaoValida(ctx, qt, codigo)
	if !valido {
		invalido()
		return
	}
	contaID := strconv.FormatInt(rec.UsuarioID, 10)
	conta, _ := auth.ClienteContas{Q: q}.PorID(ctx, contaID)
	err = svc.RedefinirSenha(ctx, auth.TipoUsuario, contaID, nova)
	var fraca auth.SenhaFracaError
	switch {
	case errors.As(err, &fraca):
		d.Erro = fraca.Motivo
		renderStatus(w, r, http.StatusBadRequest, views.RedefinirSenha(d))
		return
	case errors.Is(err, auth.ErrNaoEncontrada):
		invalido()
		return
	case err != nil:
		h.log.Error("redefinir senha", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
		d.Erro = "Não foi possível trocar a senha agora. Tente de novo."
		renderStatus(w, r, http.StatusInternalServerError, views.RedefinirSenha(d))
		return
	}
	if err := qt.InvalidarRecuperacoesDoUsuario(ctx, rec.UsuarioID); err != nil {
		indisponivel(err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		indisponivel(err)
		return
	}
	h.auditar(r, audit.Evento{Ator: atorDaConta(conta), Acao: "senha.recuperar", Entidade: "conta", EntidadeID: conta.ID, Rotulo: conta.Nome})
	d.Pronto = true
	render(w, r, views.RedefinirSenha(d))
}

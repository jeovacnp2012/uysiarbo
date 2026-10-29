package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/auth"
	"uysiarbo/internal/authz"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

// exigir libera o handler só para quem tem a permissão. As telas de papéis e usuários existem no
// endereço de um cliente; no console da plataforma não há o que administrar aqui.
func (h *handlers) exigir(permissao string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, _ := authFrom(r.Context())
		if _, ok := tenant.FromContext(r.Context()); !ok || a.sess == nil {
			http.NotFound(w, r)
			return
		}
		if !a.sess.Principal.Pode(permissao) {
			renderStatus(w, r, http.StatusForbidden, views.SemPermissao(h.layout(r, "Sem permissão", "")))
			return
		}
		next(w, r)
	}
}

// pool devolve as consultas e o pool do schema do cliente da requisição.
func (h *handlers) cliente(r *http.Request) (*sqlccliente.Queries, *pgxpool.Pool, error) {
	c, _ := tenant.FromContext(r.Context())
	pool, err := h.pools.Get(r.Context(), c.SchemaNome)
	if err != nil {
		return nil, nil, err
	}
	return sqlccliente.New(pool), pool, nil
}

func (h *handlers) falhaInterna(w http.ResponseWriter, r *http.Request, onde string, err error) {
	h.log.Error(onde, "erro", err, "request_id", r.Header.Get("X-Request-Id"))
	http.Error(w, "Erro interno. Tente novamente.", http.StatusInternalServerError)
}

func idDaURL(r *http.Request) (int64, bool) {
	n, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return n, err == nil && n > 0
}

func violacaoUnica(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// ---------------------------------------------------------------- Papéis

func (h *handlers) papeisLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListPapeisResumo(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar papéis", err)
		return
	}
	linhas := make([]views.PapelLinha, len(rows))
	for i, p := range rows {
		linhas[i] = views.PapelLinha{ID: p.ID, Nome: p.Nome, Descricao: p.Descricao, Ativo: p.Ativo, Permissoes: int(p.TotalPermissoes), Usuarios: int(p.TotalUsuarios)}
	}
	render(w, r, views.PapeisLista(h.layout(r, "Papéis", "/papeis"), linhas))
}

// blocosParaEditor monta os blocos do catálogo para o editor, marcando o que o papel já tem e
// bloqueando o que quem edita não pode conceder.
func blocosParaEditor(marcadas map[string]bool, quem authz.Principal) []views.PermBloco {
	var out []views.PermBloco
	for _, b := range authz.Padrao.Blocos {
		blk := views.PermBloco{Nome: b.Nome}
		for _, g := range b.Grupos {
			grp := views.PermGrupo{Nome: g.Nome}
			for _, p := range g.Permissoes {
				it := views.PermItem{
					Codigo: p.Codigo, Rotulo: p.Rotulo, Restritiva: p.Restritiva,
					Marcada: marcadas[p.Codigo], Bloqueada: !quem.Pode(p.Codigo),
					Busca: views.BuscaTexto(b.Nome, g.Nome, p.Rotulo),
				}
				blk.Total++
				if it.Marcada {
					blk.Marcadas++
				}
				grp.Itens = append(grp.Itens, it)
			}
			blk.Grupos = append(blk.Grupos, grp)
		}
		out = append(out, blk)
	}
	return out
}

func (h *handlers) papelNovoForm(w http.ResponseWriter, r *http.Request) {
	a, _ := authFrom(r.Context())
	render(w, r, views.PapelForm(h.layout(r, "Novo papel", "/papeis"), views.PapelFormData{
		Ativo: true, Blocos: blocosParaEditor(nil, a.sess.Principal), CSRF: csrfToken(r.Context()),
	}))
}

func (h *handlers) papelEditarForm(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	papel, err := q.GetPapel(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar papel", err)
		return
	}
	perms, err := q.ListPermissoesDoPapel(r.Context(), id)
	if err != nil {
		h.falhaInterna(w, r, "permissões do papel", err)
		return
	}
	a, _ := authFrom(r.Context())
	render(w, r, views.PapelForm(h.layout(r, "Editar papel", "/papeis"), views.PapelFormData{
		ID: papel.ID, Nome: papel.Nome, Descricao: papel.Descricao, Ativo: papel.Ativo,
		Blocos: blocosParaEditor(conjunto(perms), a.sess.Principal), CSRF: csrfToken(r.Context()),
	}))
}

func conjunto(codigos []string) map[string]bool {
	m := make(map[string]bool, len(codigos))
	for _, c := range codigos {
		m[c] = true
	}
	return m
}

// permissoesFinais decide o que o papel passa a ter. Quem edita só concede e só retira o que ele
// mesmo tem: as permissões que ele não possui ficam exatamente como estavam. O master pode tudo.
func permissoesFinais(existentes, enviadas []string, quem authz.Principal) []string {
	final := conjunto(nil)
	for _, c := range existentes {
		if !quem.Pode(c) { // fora do alcance de quem edita: não mexe
			final[c] = true
		}
	}
	for _, c := range enviadas {
		if quem.Pode(c) {
			final[c] = true
		}
	}
	out := make([]string, 0, len(final))
	for c := range final {
		out = append(out, c)
	}
	slices.Sort(out)
	return out
}

func (h *handlers) papelSalvar(w http.ResponseWriter, r *http.Request) {
	a, _ := authFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	var id int64
	if chi.URLParam(r, "id") != "" {
		var ok bool
		if id, ok = idDaURL(r); !ok {
			http.NotFound(w, r)
			return
		}
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}

	nome := strings.TrimSpace(r.PostFormValue("nome"))
	descricao := strings.TrimSpace(r.PostFormValue("descricao"))
	ativo := r.PostFormValue("ativo") == "1"
	enviadas := r.PostForm["perm"]

	var existentes []string
	var papelAntes sqlccliente.Papei
	if id != 0 {
		var err error
		if papelAntes, err = q.GetPapel(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar papel", err)
			return
		}
		if existentes, err = q.ListPermissoesDoPapel(r.Context(), id); err != nil {
			h.falhaInterna(w, r, "permissões do papel", err)
			return
		}
	}

	falha := func(msg string) {
		marcadas := conjunto(permissoesFinais(existentes, enviadas, a.sess.Principal))
		renderStatus(w, r, http.StatusBadRequest, views.PapelForm(h.layout(r, "Papel", "/papeis"), views.PapelFormData{
			ID: id, Nome: nome, Descricao: descricao, Ativo: ativo, Erro: msg,
			Blocos: blocosParaEditor(marcadas, a.sess.Principal), CSRF: csrfToken(r.Context()),
		}))
	}
	switch {
	case nome == "" || len([]rune(nome)) > 80:
		falha("Informe o nome do papel (até 80 caracteres).")
		return
	case len([]rune(descricao)) > 300:
		falha("A descrição pode ter no máximo 300 caracteres.")
		return
	}
	if invalidas := authz.Padrao.Invalidos(enviadas); len(invalidas) > 0 {
		falha("Permissão desconhecida: " + invalidas[0] + ".")
		return
	}
	if id != 0 && !ativo {
		rows, err := q.ListPapeisResumo(r.Context())
		if err != nil {
			h.falhaInterna(w, r, "listar papéis", err)
			return
		}
		for _, p := range rows {
			if p.ID == id && p.TotalUsuarios > 0 {
				falha(fmt.Sprintf("Este papel tem %d usuário(s). Troque o papel deles antes de desativar.", p.TotalUsuarios))
				return
			}
		}
	}

	final := permissoesFinais(existentes, enviadas, a.sess.Principal)
	criando := id == 0
	tx, err := pool.Begin(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(r.Context())
	qt := sqlccliente.New(tx)

	if id == 0 {
		novo, err := qt.CreatePapel(r.Context(), sqlccliente.CreatePapelParams{Nome: nome, Descricao: descricao})
		if err == nil && !ativo {
			_, err = qt.UpdatePapel(r.Context(), sqlccliente.UpdatePapelParams{ID: novo.ID, Nome: nome, Descricao: descricao, Ativo: false})
		}
		if err != nil {
			h.erroDePapel(w, r, err, falha)
			return
		}
		id = novo.ID
	} else if _, err := qt.UpdatePapel(r.Context(), sqlccliente.UpdatePapelParams{ID: id, Nome: nome, Descricao: descricao, Ativo: ativo}); err != nil {
		h.erroDePapel(w, r, err, falha)
		return
	}
	if err := qt.ClearPermissoesDoPapel(r.Context(), id); err != nil {
		h.falhaInterna(w, r, "limpar permissões", err)
		return
	}
	if err := qt.AddPermissoesAoPapel(r.Context(), sqlccliente.AddPermissoesAoPapelParams{PapelID: id, Permissoes: final}); err != nil {
		h.falhaInterna(w, r, "gravar permissões", err)
		return
	}
	h.auditarPapel(r, qt, id, criando, papelAntes, nome, descricao, ativo, existentes, final)
	if err := tx.Commit(r.Context()); err != nil {
		h.falhaInterna(w, r, "confirmar transação", err)
		return
	}
	http.Redirect(w, r, "/papeis", http.StatusSeeOther)
}

func (h *handlers) erroDePapel(w http.ResponseWriter, r *http.Request, err error, falha func(string)) {
	if violacaoUnica(err) {
		falha("Já existe um papel com esse nome.")
		return
	}
	h.falhaInterna(w, r, "gravar papel", err)
}

// -------------------------------------------------------------- Usuários

// meuID devolve o ID do usuário logado quando ele é um usuário do cliente (o master não está na lista).
func meuID(r *http.Request) int64 {
	a, _ := authFrom(r.Context())
	if a.sess == nil || a.sess.Conta.Tipo != auth.TipoUsuario {
		return 0
	}
	n, _ := strconv.ParseInt(a.sess.Conta.ID, 10, 64)
	return n
}

func formatarData(loc *time.Location, t time.Time) string {
	if t.IsZero() {
		return "nunca"
	}
	return t.In(loc).Format("02/01/2006 15:04")
}

func (h *handlers) usuariosLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListUsuariosResumo(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar usuários", err)
		return
	}
	eu := meuID(r)
	linhas := make([]views.UsuarioLinha, len(rows))
	for i, u := range rows {
		linhas[i] = views.UsuarioLinha{
			ID: u.ID, Nome: u.Nome, Email: u.Email, Papel: u.PapelNome, Ativo: u.Ativo, LoginWeb: u.LoginWeb,
			LoginTablet: u.LoginTablet, DeveTrocar: u.DeveTrocarSenha, UltimoAcesso: formatarData(h.fuso(r), u.UltimoAcesso.Time), Eu: u.ID == eu,
		}
	}
	render(w, r, views.UsuariosLista(h.layout(r, "Usuários", "/usuarios"), linhas, true))
}

// opcoesDePapel lista os papéis ativos (mais o papel atual do usuário, mesmo inativo).
func (h *handlers) opcoesDePapel(ctx context.Context, q *sqlccliente.Queries, atual int64) ([]views.OpcaoPapel, error) {
	rows, err := q.ListPapeisResumo(ctx)
	if err != nil {
		return nil, err
	}
	var out []views.OpcaoPapel
	for _, p := range rows {
		if p.Ativo || p.ID == atual {
			out = append(out, views.OpcaoPapel{ID: p.ID, Nome: p.Nome, Selecionado: p.ID == atual})
		}
	}
	return out, nil
}

func (h *handlers) usuarioNovoForm(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	papeis, err := h.opcoesDePapel(r.Context(), q, 0)
	if err != nil {
		h.falhaInterna(w, r, "listar papéis", err)
		return
	}
	render(w, r, views.UsuarioForm(h.layout(r, "Novo usuário", "/usuarios"), views.UsuarioFormData{
		Papeis: papeis, LoginWeb: true, Ativo: true, CSRF: csrfToken(r.Context()),
	}))
}

func (h *handlers) usuarioEditarForm(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	u, err := q.GetUsuarioByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar usuário", err)
		return
	}
	papeis, err := h.opcoesDePapel(r.Context(), q, u.PapelID)
	if err != nil {
		h.falhaInterna(w, r, "listar papéis", err)
		return
	}
	render(w, r, views.UsuarioForm(h.layout(r, "Editar usuário", "/usuarios"), views.UsuarioFormData{
		ID: u.ID, Nome: u.Nome, Email: u.Email, Telefone: u.Telefone, Papeis: papeis, LoginWeb: u.LoginWeb,
		LoginTablet: u.LoginTablet, Ativo: u.Ativo, Eu: u.ID == meuID(r), CSRF: csrfToken(r.Context()),
	}))
}

type dadosUsuario struct {
	nome, email, telefone        string
	papelID                      int64
	loginWeb, loginTablet, ativo bool
}

func lerUsuario(r *http.Request) dadosUsuario {
	papel, _ := strconv.ParseInt(r.PostFormValue("papel_id"), 10, 64)
	return dadosUsuario{
		nome: strings.TrimSpace(r.PostFormValue("nome")), email: strings.ToLower(strings.TrimSpace(r.PostFormValue("email"))),
		telefone: strings.TrimSpace(r.PostFormValue("telefone")), papelID: papel,
		loginWeb: r.PostFormValue("login_web") == "1", loginTablet: r.PostFormValue("login_tablet") == "1",
		ativo: r.PostFormValue("ativo") == "1",
	}
}

func (d dadosUsuario) validar(novo bool) string {
	switch {
	case d.nome == "" || len([]rune(d.nome)) > 120:
		return "Informe o nome completo (até 120 caracteres)."
	case novo && (!strings.Contains(d.email, "@") || strings.ContainsAny(d.email, " ,;") || len(d.email) > 160):
		return "Informe um e-mail válido."
	case len([]rune(d.telefone)) > 30:
		return "O telefone pode ter no máximo 30 caracteres."
	case d.papelID == 0:
		return "Escolha o papel do usuário."
	case !d.loginWeb && !d.loginTablet:
		return "Marque pelo menos uma forma de acesso (web ou tablet)."
	}
	return ""
}

func (h *handlers) usuarioCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := lerUsuario(r)
	falha := func(msg string) {
		papeis, _ := h.opcoesDePapel(r.Context(), q, d.papelID)
		renderStatus(w, r, http.StatusBadRequest, views.UsuarioForm(h.layout(r, "Novo usuário", "/usuarios"), views.UsuarioFormData{
			Nome: d.nome, Email: d.email, Telefone: d.telefone, Papeis: papeis, LoginWeb: d.loginWeb,
			LoginTablet: d.loginTablet, Ativo: true, Erro: msg, CSRF: csrfToken(r.Context()),
		}))
	}
	if msg := d.validar(true); msg != "" {
		falha(msg)
		return
	}
	papel, err := q.GetPapel(r.Context(), d.papelID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !papel.Ativo) {
		falha("Escolha um papel ativo.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar papel", err)
		return
	}
	// O e-mail de um usuário da plataforma não pode ser reaproveitado aqui (o login o confundiria).
	if _, err := h.pub.GetPlataformaUsuarioByEmail(r.Context(), d.email); err == nil {
		falha("Este e-mail já pertence a um usuário da plataforma.")
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		h.falhaInterna(w, r, "buscar e-mail", err)
		return
	}

	senha, err := auth.Gerar(16)
	if err != nil {
		h.falhaInterna(w, r, "gerar senha", err)
		return
	}
	hash, err := auth.Hash(senha)
	if err != nil {
		h.falhaInterna(w, r, "hash da senha", err)
		return
	}
	u, err := q.CreateUsuario(r.Context(), sqlccliente.CreateUsuarioParams{
		Nome: d.nome, Email: d.email, Telefone: d.telefone, SenhaHash: hash, PapelID: d.papelID,
		LoginWeb: d.loginWeb, LoginTablet: d.loginTablet, DeveTrocarSenha: true,
	})
	if violacaoUnica(err) {
		falha("Já existe um usuário com esse e-mail.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "criar usuário", err)
		return
	}
	h.auditar(r, audit.Evento{
		Acao: "usuario.criar", Entidade: "usuario", EntidadeID: strconv.FormatInt(u.ID, 10), Rotulo: u.Nome,
		Depois: dadosParaAuditoria(u.Nome, u.Email, u.Telefone, papel.Nome, u.LoginWeb, u.LoginTablet, u.Ativo),
	})
	h.mostrarSenha(w, r, true, u.Nome, u.Email, senha)
}

func (h *handlers) usuarioSalvar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	atual, err := q.GetUsuarioByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar usuário", err)
		return
	}
	eu := id == meuID(r)
	d := lerUsuario(r)
	falha := func(msg string) {
		papeis, _ := h.opcoesDePapel(r.Context(), q, d.papelID)
		renderStatus(w, r, http.StatusBadRequest, views.UsuarioForm(h.layout(r, "Editar usuário", "/usuarios"), views.UsuarioFormData{
			ID: id, Nome: d.nome, Email: atual.Email, Telefone: d.telefone, Papeis: papeis, LoginWeb: d.loginWeb,
			LoginTablet: d.loginTablet, Ativo: d.ativo, Eu: eu, Erro: msg, CSRF: csrfToken(r.Context()),
		}))
	}
	if msg := d.validar(false); msg != "" {
		falha(msg)
		return
	}
	if eu && (!d.ativo || d.papelID != atual.PapelID) {
		falha("Você não pode desativar a si mesmo nem trocar o seu próprio papel.")
		return
	}
	if d.papelID != atual.PapelID {
		papel, err := q.GetPapel(r.Context(), d.papelID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !papel.Ativo) {
			falha("Escolha um papel ativo.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar papel", err)
			return
		}
	}
	if _, err := q.UpdateUsuario(r.Context(), sqlccliente.UpdateUsuarioParams{
		ID: id, Nome: d.nome, Telefone: d.telefone, PapelID: d.papelID, LoginWeb: d.loginWeb, LoginTablet: d.loginTablet, Ativo: d.ativo,
	}); err != nil {
		h.falhaInterna(w, r, "salvar usuário", err)
		return
	}
	h.auditarEdicaoUsuario(r, q, atual, d)
	// Desativar ou trocar o papel encerra as sessões abertas: a pessoa volta a entrar já com o novo acesso.
	if !d.ativo || d.papelID != atual.PapelID {
		a, _ := authFrom(r.Context())
		if err := a.svc.Sessoes.RemoverDaConta(r.Context(), auth.TipoUsuario, strconv.FormatInt(id, 10), nil); err != nil {
			h.falhaInterna(w, r, "encerrar sessões", err)
			return
		}
	}
	http.Redirect(w, r, "/usuarios", http.StatusSeeOther)
}

func (h *handlers) usuarioResetarSenha(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if id == meuID(r) {
		http.Error(w, "Para trocar a sua própria senha, use a opção de trocar senha.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	u, err := q.GetUsuarioByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar usuário", err)
		return
	}
	a, _ := authFrom(r.Context())
	senha, err := a.svc.ResetarSenha(r.Context(), auth.TipoUsuario, strconv.FormatInt(id, 10))
	if err != nil {
		h.falhaInterna(w, r, "resetar senha", err)
		return
	}
	h.log.Info("senha resetada", "usuario_id", id, "por", a.sess.Conta.Email, "request_id", r.Header.Get("X-Request-Id"))
	h.auditar(r, audit.Evento{
		Acao: "usuario.resetar_senha", Entidade: "usuario", EntidadeID: strconv.FormatInt(id, 10), Rotulo: u.Nome,
		Detalhes: map[string]any{"email": u.Email, "sessoes_encerradas": true, "troca_obrigatoria": true},
	})
	h.mostrarSenha(w, r, false, u.Nome, u.Email, senha)
}

// mostrarSenha exibe a senha temporária uma única vez. A resposta nunca vai para cache.
func (h *handlers) mostrarSenha(w http.ResponseWriter, r *http.Request, novo bool, nome, email, senha string) {
	w.Header().Set("Cache-Control", "no-store")
	titulo := "Senha resetada"
	if novo {
		titulo = "Usuário criado"
	}
	render(w, r, views.SenhaGerada(h.layout(r, titulo, "/usuarios"), views.SenhaGeradaData{Novo: novo, Nome: nome, Email: email, Senha: senha}))
}

// auditarPapel registra a criação ou edição de um papel dentro da mesma transação que a gravou.
func (h *handlers) auditarPapel(r *http.Request, qt *sqlccliente.Queries, id int64, criando bool, antes sqlccliente.Papei,
	nome, descricao string, ativo bool, permAntes, permDepois []string) {
	depois := map[string]any{"nome": nome, "descricao": descricao, "ativo": ativo}
	entrou, saiu := audit.DiferencaLista(permAntes, permDepois)
	det := map[string]any{"permissoes_adicionadas": entrou, "permissoes_removidas": saiu, "total_permissoes": len(permDepois)}
	ev := audit.Evento{Entidade: "papel", EntidadeID: strconv.FormatInt(id, 10), Rotulo: nome, Depois: depois, Detalhes: det}
	if criando {
		ev.Acao = "papel.criar"
	} else {
		a, d := audit.Diferenca(map[string]any{"nome": antes.Nome, "descricao": antes.Descricao, "ativo": antes.Ativo}, depois)
		if len(a)+len(d)+len(entrou)+len(saiu) == 0 {
			naoAuditar(r) // nada mudou
			return
		}
		ev.Acao, ev.Antes, ev.Depois = "papel.editar", a, d
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, ev)
}

func dadosParaAuditoria(nome, email, telefone, papel string, web, tablet, ativo bool) map[string]any {
	return map[string]any{
		"nome": nome, "email": email, "telefone": telefone, "papel": papel,
		"login_web": web, "login_tablet": tablet, "ativo": ativo,
	}
}

// auditarEdicaoUsuario registra o que mudou na edição de um usuário (só os campos alterados).
func (h *handlers) auditarEdicaoUsuario(r *http.Request, q *sqlccliente.Queries, antes sqlccliente.Usuario, d dadosUsuario) {
	nomePapel := func(id int64) string {
		if p, err := q.GetPapel(r.Context(), id); err == nil {
			return p.Nome
		}
		return strconv.FormatInt(id, 10)
	}
	a, dep := audit.Diferenca(
		dadosParaAuditoria(antes.Nome, antes.Email, antes.Telefone, nomePapel(antes.PapelID), antes.LoginWeb, antes.LoginTablet, antes.Ativo),
		dadosParaAuditoria(d.nome, antes.Email, d.telefone, nomePapel(d.papelID), d.loginWeb, d.loginTablet, d.ativo),
	)
	if len(a)+len(dep) == 0 {
		naoAuditar(r)
		return
	}
	h.auditar(r, audit.Evento{
		Acao: "usuario.editar", Entidade: "usuario", EntidadeID: strconv.FormatInt(antes.ID, 10), Rotulo: d.nome,
		Antes: a, Depois: dep,
	})
}

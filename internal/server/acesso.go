package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

const (
	acessoResponsavel = "responsavel_bairro"
	acessoParametro   = "parametro"
)

var rotulosDeAcesso = map[string]string{"bairro": "Bairro", "quarteirao": "Quarteirão", "local": "Local"}

func (h *handlers) metodoDeAcesso(r *http.Request, q *sqlccliente.Queries) string {
	if info, err := q.GetClienteInfo(r.Context(), "acesso_local"); err == nil && info.Valor == acessoParametro {
		return acessoParametro
	}
	return acessoResponsavel
}

func (h *handlers) montarAcesso(r *http.Request, q *sqlccliente.Queries, aviso, erro string) (views.AcessoData, error) {
	ctx := r.Context()
	d := views.AcessoData{Metodo: h.metodoDeAcesso(r, q), Aviso: aviso, Erro: erro, CSRF: csrfToken(ctx)}
	filtro := pgtype.Int8{}
	if n, err := strconv.ParseInt(r.URL.Query().Get("usuario"), 10, 64); err == nil && n > 0 {
		filtro = pgtype.Int8{Int64: n, Valid: true}
		d.FiltroUsuario = n
	}
	usuarios, err := q.ListUsuariosAtivosParaResponsavel(ctx)
	if err != nil {
		return d, err
	}
	for _, u := range usuarios {
		d.Usuarios = append(d.Usuarios, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome + " — " + u.Email, Selecionada: u.ID == filtro.Int64})
	}
	bairros, err := q.ListBairrosAtivosParaLocal(ctx, pgtype.Int8{})
	if err != nil {
		return d, err
	}
	for _, b := range bairros {
		d.Bairros = append(d.Bairros, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome})
	}
	rows, err := q.ListAcessosLocal(ctx, filtro)
	if err != nil {
		return d, err
	}
	for _, a := range rows {
		d.Linhas = append(d.Linhas, views.AcessoLinha{
			ID: a.ID, Usuario: a.UsuarioNome, Tipo: rotulosDeAcesso[a.Tipo], Referencia: a.ReferenciaNome, Descricao: a.Descricao,
			Desde: formatarData(h.fuso(r), a.CriadoEm.Time),
		})
	}
	return d, nil
}

func (h *handlers) acessoPagina(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	aviso := map[string]string{"metodo": "Método de controle de acesso salvo.", "criado": "Acesso concedido.", "removido": "Acesso retirado.", "ja": "Essa pessoa já tinha esse acesso."}[r.URL.Query().Get("ok")]
	d, err := h.montarAcesso(r, q, aviso, "")
	if err != nil {
		h.falhaInterna(w, r, "montar tela", err)
		return
	}
	render(w, r, h.telaAcesso(r, d))
}

func (h *handlers) acessoMetodo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	novo := r.PostFormValue("metodo")
	if novo != acessoResponsavel && novo != acessoParametro {
		d, e := h.montarAcesso(r, q, "", "Escolha um método válido.")
		if e != nil {
			h.falhaInterna(w, r, "montar tela", e)
			return
		}
		renderStatus(w, r, http.StatusBadRequest, h.telaAcesso(r, d))
		return
	}
	antes := h.metodoDeAcesso(r, q)
	if antes == novo {
		naoAuditar(r)
		http.Redirect(w, r, "/parametros/acesso?ok=metodo", http.StatusSeeOther)
		return
	}
	if err := q.SetClienteInfo(r.Context(), sqlccliente.SetClienteInfoParams{Chave: "acesso_local", Valor: novo}); err != nil {
		h.falhaInterna(w, r, "salvar método", err)
		return
	}
	h.auditar(r, audit.Evento{
		Acao: "acesso.metodo", Entidade: "parametro", EntidadeID: "acesso_local", Rotulo: "Controle de acesso aos locais",
		Antes: map[string]any{"metodo": rotuloDoMetodo(antes)}, Depois: map[string]any{"metodo": rotuloDoMetodo(novo)},
	})
	http.Redirect(w, r, "/parametros/acesso?ok=metodo", http.StatusSeeOther)
}

func rotuloDoMetodo(m string) string {
	if m == acessoParametro {
		return "Pela lista de acessos"
	}
	return "Pelo responsável do bairro"
}

// acessoCriar concede acesso a um bairro ou a um quarteirão (o acesso a um local sai da ficha do local).
func (h *handlers) acessoCriar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	falha := func(msg string) {
		d, e := h.montarAcesso(r, q, "", msg)
		if e != nil {
			h.falhaInterna(w, r, "montar tela", e)
			return
		}
		renderStatus(w, r, http.StatusBadRequest, h.telaAcesso(r, d))
	}
	uid, _ := strconv.ParseInt(r.PostFormValue("usuario_id"), 10, 64)
	bid, _ := strconv.ParseInt(r.PostFormValue("bairro_id"), 10, 64)
	qid, _ := strconv.ParseInt(r.PostFormValue("quarteirao_id"), 10, 64)
	desc := strings.TrimSpace(r.PostFormValue("descricao"))
	if len([]rune(desc)) > 200 {
		falha("A descrição pode ter no máximo 200 caracteres.")
		return
	}
	u, err := q.GetUsuarioByID(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !u.Ativo) {
		falha("Escolha uma pessoa ativa.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar usuário", err)
		return
	}
	b, err := q.GetBairro(ctx, bid)
	if errors.Is(err, pgx.ErrNoRows) {
		falha("Escolha o bairro.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar bairro", err)
		return
	}
	tipo, ref, alvo := "bairro", b.ID, "Bairro "+b.Nome
	if qid != 0 {
		qt, err := q.GetQuarteirao(ctx, qid)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && qt.BairroID != b.ID) {
			falha("Escolha um quarteirão do bairro selecionado.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar quarteirão", err)
			return
		}
		tipo, ref, alvo = "quarteirao", qt.ID, "Quarteirão "+qt.Nome+" ("+b.Nome+")"
	}
	n, err := q.CreateAcessoLocal(ctx, sqlccliente.CreateAcessoLocalParams{
		UsuarioID: u.ID, Tipo: tipo, ReferenciaID: ref, Descricao: desc, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
	})
	if err != nil {
		h.falhaInterna(w, r, "conceder acesso", err)
		return
	}
	if n == 0 {
		naoAuditar(r)
		http.Redirect(w, r, "/parametros/acesso?ok=ja", http.StatusSeeOther)
		return
	}
	h.auditar(r, audit.Evento{
		Acao: "acesso.criar", Entidade: "usuario", EntidadeID: strconv.FormatInt(u.ID, 10), Rotulo: u.Nome,
		Depois: map[string]any{"tipo": rotulosDeAcesso[tipo], "alvo": alvo, "descricao": desc},
	})
	http.Redirect(w, r, "/parametros/acesso?ok=criado", http.StatusSeeOther)
}

// acessoRemover retira uma atribuição. O campo "voltar" (só caminhos do próprio sistema) devolve a pessoa à tela de origem.
func (h *handlers) acessoRemover(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	a, err := q.GetAcessoLocal(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar acesso", err)
		return
	}
	u, _ := q.GetUsuarioByID(ctx, a.UsuarioID)
	if _, err := q.DeleteAcessoLocal(ctx, a.ID); err != nil {
		h.falhaInterna(w, r, "retirar acesso", err)
		return
	}
	h.auditar(r, audit.Evento{
		Acao: "acesso.remover", Entidade: "usuario", EntidadeID: strconv.FormatInt(a.UsuarioID, 10), Rotulo: u.Nome,
		Antes: map[string]any{"tipo": rotulosDeAcesso[a.Tipo], "referencia": a.ReferenciaID, "descricao": a.Descricao},
	})
	destino := "/parametros/acesso?ok=removido"
	if v := r.PostFormValue("voltar"); strings.HasPrefix(v, "/locais/") && !strings.ContainsAny(v, "\\\r\n") && !strings.HasPrefix(v, "//") {
		destino = v
	}
	http.Redirect(w, r, destino, http.StatusSeeOther)
}

// localAcessoCriar dá a uma pessoa acesso a um local específico (ficha do local).
func (h *handlers) localAcessoCriar(w http.ResponseWriter, r *http.Request) {
	l, q, ok := h.localDaURL(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	uid, _ := strconv.ParseInt(r.PostFormValue("usuario_id"), 10, 64)
	u, err := q.GetUsuarioByID(r.Context(), uid)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !u.Ativo) {
		http.Redirect(w, r, fmt.Sprintf("/locais/%d?ok=sempessoa", l.ID), http.StatusSeeOther)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar usuário", err)
		return
	}
	n, err := q.CreateAcessoLocal(r.Context(), sqlccliente.CreateAcessoLocalParams{
		UsuarioID: u.ID, Tipo: "local", ReferenciaID: l.ID, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
	})
	if err != nil {
		h.falhaInterna(w, r, "conceder acesso", err)
		return
	}
	if n > 0 {
		h.auditar(r, audit.Evento{
			Acao: "acesso.criar", Entidade: "usuario", EntidadeID: strconv.FormatInt(u.ID, 10), Rotulo: u.Nome,
			Depois: map[string]any{"tipo": "Local", "alvo": rotuloDoLocal(l)},
		})
	} else {
		naoAuditar(r)
	}
	http.Redirect(w, r, fmt.Sprintf("/locais/%d?ok=acesso", l.ID), http.StatusSeeOther)
}

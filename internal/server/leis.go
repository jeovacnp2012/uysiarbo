package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

func (h *handlers) leisLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListLeis(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar leis", err)
		return
	}
	linhas := make([]views.LeiLinha, len(rows))
	for i, l := range rows {
		linhas[i] = views.LeiLinha{ID: l.ID, Numero: l.Numero, Ano: int(l.Ano), Nome: l.Nome, Publicacao: dataBR(l.Publicacao), Ativo: l.Ativo, Trechos: int(l.TotalTrechos)}
	}
	lay := h.layout(r, "Leis municipais", "/leis")
	linhas = paginar(r, &lay, linhas)
	render(w, r, views.Leis(lay, linhas))
}

func (h *handlers) leiNovaForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.LeiForm(h.layout(r, "Nova lei", "/leis"), views.LeiFormData{Ativo: true, CSRF: csrfToken(r.Context())}))
}

// dadosDaLei monta a ficha da lei com os trechos.
func (h *handlers) dadosDaLei(ctx context.Context, r *http.Request, q *sqlccliente.Queries, l sqlccliente.LeisMunicipai) (views.LeiFormData, error) {
	ts, err := q.ListTrechosDaLei(ctx, l.ID)
	if err != nil {
		return views.LeiFormData{}, err
	}
	d := views.LeiFormData{
		ID: l.ID, Numero: l.Numero, Ano: strconv.Itoa(int(l.Ano)), Nome: l.Nome, Publicacao: dataISO(l.Publicacao),
		Ativo: l.Ativo, CSRF: csrfToken(r.Context()),
	}
	for _, t := range ts {
		d.Trechos = append(d.Trechos, views.LeiTrechoLinha{ID: t.ID, Artigo: t.Artigo, Texto: t.Texto})
	}
	return d, nil
}

func (h *handlers) leiEditarForm(w http.ResponseWriter, r *http.Request) {
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
	l, err := q.GetLei(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar lei", err)
		return
	}
	d, err := h.dadosDaLei(r.Context(), r, q, l)
	if err != nil {
		h.falhaInterna(w, r, "listar trechos", err)
		return
	}
	render(w, r, views.LeiForm(h.layout(r, "Lei municipal", "/leis"), d))
}

func (h *handlers) leiSalvar(w http.ResponseWriter, r *http.Request) {
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
	var antes sqlccliente.LeisMunicipai
	if id != 0 {
		if antes, err = q.GetLei(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar lei", err)
			return
		}
	}
	numero, anoTxt := strings.TrimSpace(r.PostFormValue("numero")), strings.TrimSpace(r.PostFormValue("ano"))
	nome, pubTxt := strings.TrimSpace(r.PostFormValue("nome")), strings.TrimSpace(r.PostFormValue("publicacao"))
	ativo := r.PostFormValue("ativo") == "1"
	if id == 0 {
		ativo = true
	}
	falha := func(msg string) {
		d := views.LeiFormData{ID: id, Numero: numero, Ano: anoTxt, Nome: nome, Publicacao: pubTxt, Ativo: ativo, Erro: msg, CSRF: csrfToken(r.Context())}
		if id != 0 {
			if base, err := h.dadosDaLei(r.Context(), r, q, antes); err == nil {
				d.Trechos = base.Trechos
			}
		}
		renderStatus(w, r, http.StatusBadRequest, views.LeiForm(h.layout(r, "Lei municipal", "/leis"), d))
	}
	ano, errAno := strconv.Atoi(anoTxt)
	var pub pgtype.Date
	if pubTxt != "" {
		d, err := time.Parse(formatoDataISO, pubTxt)
		if err != nil {
			falha("A data de publicação é inválida.")
			return
		}
		pub = pgtype.Date{Time: d, Valid: true}
	}
	switch {
	case numero == "" || len([]rune(numero)) > 15:
		falha("Informe o número da lei (até 15 caracteres).")
		return
	case errAno != nil || ano < 1900 || ano > 2100:
		falha("Informe o ano da lei (de 1900 a 2100).")
		return
	case len([]rune(nome)) < 2 || len([]rune(nome)) > 200:
		falha("Informe o nome da lei (de 2 a 200 caracteres).")
		return
	}

	tx, err := pool.Begin(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(r.Context())
	qt := sqlccliente.New(tx)
	rotulo := "Lei " + numero + "/" + strconv.Itoa(ano)
	ev := audit.Evento{Entidade: "lei", Rotulo: rotulo}
	novo := map[string]any{"numero": numero, "ano": ano, "nome": nome, "publicacao": pubTxt, "ativo": ativo}
	if id == 0 {
		l, err := qt.CreateLei(r.Context(), sqlccliente.CreateLeiParams{Numero: numero, Ano: int16(ano), Nome: nome, Publicacao: pub, Ativo: true})
		if violacaoUnica(err) {
			falha("Já existe uma lei com esse número e ano.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "criar lei", err)
			return
		}
		ev.Acao, ev.EntidadeID, ev.Depois = "lei.criar", strconv.FormatInt(l.ID, 10), novo
		if err := tx.Commit(r.Context()); err != nil {
			h.falhaInterna(w, r, "confirmar lei", err)
			return
		}
		h.auditar(r, ev)
		http.Redirect(w, r, fmt.Sprintf("/leis/%d", l.ID), http.StatusSeeOther)
		return
	}
	if _, err := qt.UpdateLei(r.Context(), sqlccliente.UpdateLeiParams{ID: id, Numero: numero, Ano: int16(ano), Nome: nome, Publicacao: pub, Ativo: ativo}); violacaoUnica(err) {
		falha("Já existe uma lei com esse número e ano.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "salvar lei", err)
		return
	}
	a, d := audit.Diferenca(map[string]any{
		"numero": antes.Numero, "ano": int(antes.Ano), "nome": antes.Nome, "publicacao": dataISO(antes.Publicacao), "ativo": antes.Ativo,
	}, novo)
	if len(a)+len(d) == 0 {
		naoAuditar(r)
	} else {
		ev.Acao, ev.EntidadeID, ev.Antes, ev.Depois = "lei.editar", strconv.FormatInt(id, 10), a, d
		h.auditarCom(audit.Cliente{Q: qt}, r, ev)
	}
	if err := tx.Commit(r.Context()); err != nil {
		h.falhaInterna(w, r, "confirmar lei", err)
		return
	}
	http.Redirect(w, r, "/leis", http.StatusSeeOther)
}

func (h *handlers) leiExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.excluirRegistro(w, r, "lei", id, "/leis?aviso=excluido", fmt.Sprintf("/leis/%d", id))
}

// ------------------------------------------------------------------- Trechos

func idDoTrecho(r *http.Request) (int64, bool) {
	n, err := strconv.ParseInt(chi.URLParam(r, "tid"), 10, 64)
	return n, err == nil && n > 0
}

// trechoSalvar cria (sem tid) ou altera (com tid) um trecho da lei.
func (h *handlers) trechoSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	leiID, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var tid int64
	if chi.URLParam(r, "tid") != "" {
		if tid, ok = idDoTrecho(r); !ok {
			http.NotFound(w, r)
			return
		}
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	lei, err := q.GetLei(r.Context(), leiID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar lei", err)
		return
	}
	var antes sqlccliente.LeisTrecho
	if tid != 0 {
		if antes, err = q.GetTrecho(r.Context(), sqlccliente.GetTrechoParams{ID: tid, LeiID: leiID}); errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar trecho", err)
			return
		}
	}
	artigo, texto := strings.TrimSpace(r.PostFormValue("artigo")), strings.TrimSpace(r.PostFormValue("texto"))
	var msg string
	switch {
	case len([]rune(artigo)) > 60:
		msg = "O rótulo do trecho tem no máximo 60 caracteres."
	case texto == "":
		msg = "Informe o texto do trecho."
	case len([]rune(texto)) > 4000:
		msg = "O texto do trecho tem no máximo 4000 caracteres."
	}
	if msg != "" {
		d, err := h.dadosDaLei(r.Context(), r, q, lei)
		if err != nil {
			h.falhaInterna(w, r, "listar trechos", err)
			return
		}
		d.ErroTrecho = msg
		if tid == 0 {
			d.NovoArtigo, d.NovoTexto = artigo, texto
		}
		renderStatus(w, r, http.StatusBadRequest, views.LeiForm(h.layout(r, "Lei municipal", "/leis"), d))
		return
	}

	tx, err := pool.Begin(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(r.Context())
	qt := sqlccliente.New(tx)
	rotulo := "Lei " + lei.Numero + "/" + strconv.Itoa(int(lei.Ano))
	ev := audit.Evento{Entidade: "lei_trecho", Rotulo: rotulo + " " + artigo}
	novo := map[string]any{"lei": rotulo, "artigo": artigo, "texto": texto}
	if tid == 0 {
		t, err := qt.CreateTrecho(r.Context(), sqlccliente.CreateTrechoParams{LeiID: leiID, Artigo: artigo, Texto: texto})
		if err != nil {
			h.falhaInterna(w, r, "criar trecho", err)
			return
		}
		ev.Acao, ev.EntidadeID, ev.Depois = "lei.trecho_criar", strconv.FormatInt(t.ID, 10), novo
	} else {
		if _, err := qt.UpdateTrecho(r.Context(), sqlccliente.UpdateTrechoParams{ID: tid, LeiID: leiID, Artigo: artigo, Texto: texto}); err != nil {
			h.falhaInterna(w, r, "salvar trecho", err)
			return
		}
		a, d := audit.Diferenca(map[string]any{"lei": rotulo, "artigo": antes.Artigo, "texto": antes.Texto}, novo)
		if len(a)+len(d) == 0 {
			naoAuditar(r)
		} else {
			ev.Acao, ev.EntidadeID, ev.Antes, ev.Depois = "lei.trecho_editar", strconv.FormatInt(tid, 10), a, d
		}
	}
	if ev.Acao != "" {
		h.auditarCom(audit.Cliente{Q: qt}, r, ev)
	}
	if err := tx.Commit(r.Context()); err != nil {
		h.falhaInterna(w, r, "confirmar trecho", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/leis/%d#trechos", leiID), http.StatusSeeOther)
}

func (h *handlers) trechoExcluir(w http.ResponseWriter, r *http.Request) {
	leiID, ok1 := idDaURL(r)
	tid, ok2 := idDoTrecho(r)
	if !ok1 || !ok2 {
		http.NotFound(w, r)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	// O trecho precisa ser desta lei.
	if _, err := q.GetTrecho(r.Context(), sqlccliente.GetTrechoParams{ID: tid, LeiID: leiID}); err != nil {
		http.NotFound(w, r)
		return
	}
	h.excluirRegistro(w, r, "lei_trecho", tid, fmt.Sprintf("/leis/%d#trechos", leiID), fmt.Sprintf("/leis/%d", leiID))
}

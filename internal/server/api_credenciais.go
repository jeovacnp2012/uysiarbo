package server

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/apiauth"
	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

func (h *handlers) apiCredDaURL(w http.ResponseWriter, r *http.Request) (sqlccliente.ApiCredenciai, *sqlccliente.Queries, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return sqlccliente.ApiCredenciai{}, nil, false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return sqlccliente.ApiCredenciai{}, nil, false
	}
	c, err := q.GetApiCredencial(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return c, nil, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar credencial", err)
		return c, nil, false
	}
	return c, q, true
}

func (h *handlers) escopoLinhas(marcados []string) []views.ApiEscopoLinha {
	out := make([]views.ApiEscopoLinha, len(apiauth.Recursos))
	for i, rec := range apiauth.Recursos {
		l, e := apiauth.Escopo(rec.Nome, false), apiauth.Escopo(rec.Nome, true)
		out[i] = views.ApiEscopoLinha{Rotulo: rec.Rotulo, Leitura: l, Escrita: e, LeituraM: slices.Contains(marcados, l), EscritaM: slices.Contains(marcados, e)}
	}
	return out
}

func (h *handlers) apiListaPagina(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListApiCredenciais(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar credenciais", err)
		return
	}
	loc := h.fuso(r)
	linhas := make([]views.ApiCredLinha, len(rows))
	for i, c := range rows {
		vencida := c.ValidaAte.Valid && !c.ValidaAte.Time.After(time.Now())
		validade := "sem validade"
		if c.ValidaAte.Valid {
			validade = c.ValidaAte.Time.In(loc).Format("02/01/2006")
		}
		uso := "nunca"
		if c.UltimoUsoEm.Valid {
			uso = formatarData(loc, c.UltimoUsoEm.Time)
		}
		linhas[i] = views.ApiCredLinha{ID: c.ID, Nome: c.Nome, ClientID: c.ClientID, Escopos: len(c.Escopos), Ativa: c.Ativo, Vencida: vencida, Validade: validade, UltimoUso: uso, Chamadas: int(c.Chamadas24h)}
	}
	aviso := map[string]string{"ativado": "Credencial reativada.", "desativado": "Credencial desativada: os tokens emitidos deixaram de valer.", "salvo": "Credencial salva."}[r.URL.Query().Get("ok")]
	render(w, r, views.ApiLista(h.layout(r, "API e integrações", "/parametros/api"), views.ApiListaData{Linhas: linhas, PodeEditar: true, Aviso: aviso}))
}

func (h *handlers) apiNovaForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.ApiForm(h.layout(r, "Nova credencial de API", "/parametros/api"), views.ApiFormData{
		Escopos: h.escopoLinhas([]string{"cidades:read"}), Limite: 120, Ativa: true, CSRF: csrfToken(r.Context()),
	}))
}

func (h *handlers) montarFichaApi(r *http.Request, q *sqlccliente.Queries, c sqlccliente.ApiCredenciai, aviso, erro string) (views.ApiFormData, error) {
	ctx := r.Context()
	loc := h.fuso(r)
	d := views.ApiFormData{
		ID: c.ID, Nome: c.Nome, Descricao: c.Descricao, Escopos: h.escopoLinhas(c.Escopos), Limite: int(c.LimitePorMinuto), Ativa: c.Ativo,
		ClientID: c.ClientID, Criada: formatarData(loc, c.CriadoEm.Time), UltimoUso: "nunca", Aviso: aviso, Erro: erro, CSRF: csrfToken(ctx),
	}
	if c.ValidaAte.Valid {
		d.ValidaAte = c.ValidaAte.Time.In(loc).Format("2006-01-02")
	}
	if c.RotacionadoEm.Valid {
		d.Rotacionada = formatarData(loc, c.RotacionadoEm.Time)
	}
	if c.UltimoUsoEm.Valid {
		d.UltimoUso = formatarData(loc, c.UltimoUsoEm.Time)
	}
	for _, j := range []struct {
		rotulo string
		dur    time.Duration
	}{{"Últimas 24 horas", 24 * time.Hour}, {"Últimos 7 dias", 7 * 24 * time.Hour}, {"Últimos 30 dias", 30 * 24 * time.Hour}} {
		res, err := q.ResumoApiChamadas(ctx, sqlccliente.ResumoApiChamadasParams{CredencialID: c.ID, Quando: pgtype.Timestamptz{Time: time.Now().Add(-j.dur), Valid: true}})
		if err != nil {
			return d, err
		}
		d.Resumos = append(d.Resumos, views.ApiResumo{Janela: j.rotulo, Total: int(res.Total), ComErro: int(res.ComErro), MediaMs: int(res.MediaMs)})
	}
	chs, err := q.ListApiChamadas(ctx, sqlccliente.ListApiChamadasParams{CredencialID: c.ID, Limit: 50})
	if err != nil {
		return d, err
	}
	for _, x := range chs {
		d.Chamadas = append(d.Chamadas, views.ApiChamadaLinha{Quando: formatarData(loc, x.Quando.Time), Metodo: x.Metodo, Caminho: x.Caminho, Status: int(x.Status), Duracao: int(x.DuracaoMs), IP: x.Ip})
	}
	return d, nil
}

func (h *handlers) apiFicha(w http.ResponseWriter, r *http.Request) {
	c, q, ok := h.apiCredDaURL(w, r)
	if !ok {
		return
	}
	aviso := map[string]string{"salvo": "Credencial salva.", "ativado": "Credencial reativada.", "desativado": "Credencial desativada: os tokens emitidos deixaram de valer."}[r.URL.Query().Get("ok")]
	d, err := h.montarFichaApi(r, q, c, aviso, "")
	if err != nil {
		h.falhaInterna(w, r, "montar ficha", err)
		return
	}
	render(w, r, views.ApiForm(h.layout(r, "Credencial de API", "/parametros/api"), d))
}

// lerFormApi valida o formulário (nome, escopos, validade, limite).
func (h *handlers) lerFormApi(r *http.Request) (nome, desc string, escopos []string, valida pgtype.Timestamptz, limite int32, msg string) {
	nome, desc = strings.TrimSpace(r.PostFormValue("nome")), strings.TrimSpace(r.PostFormValue("descricao"))
	switch {
	case len([]rune(nome)) < 2 || len([]rune(nome)) > 100:
		return nome, desc, nil, valida, 0, "Informe o nome da credencial (de 2 a 100 caracteres)."
	case len([]rune(desc)) > 300:
		return nome, desc, nil, valida, 0, "A descrição pode ter no máximo 300 caracteres."
	}
	escopos, err := apiauth.Normalizar(r.PostForm["escopo"])
	if err != nil {
		return nome, desc, nil, valida, 0, "Escopo inválido."
	}
	if len(escopos) == 0 {
		return nome, desc, nil, valida, 0, "Marque pelo menos um escopo."
	}
	if v := strings.TrimSpace(r.PostFormValue("valida_ate")); v != "" {
		dia, err := time.ParseInLocation("2006-01-02", v, h.fuso(r))
		if err != nil {
			return nome, desc, escopos, valida, 0, "Data de validade inválida."
		}
		fim := dia.Add(24*time.Hour - time.Second) // vale até o fim do dia escolhido
		if !fim.After(time.Now()) {
			return nome, desc, escopos, valida, 0, "A validade precisa ser uma data de hoje em diante."
		}
		valida = pgtype.Timestamptz{Time: fim, Valid: true}
	}
	n, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("limite")))
	if err != nil || n < 1 || n > 10000 {
		return nome, desc, escopos, valida, 0, "Chamadas por minuto: informe um número de 1 a 10000."
	}
	return nome, desc, escopos, valida, int32(n), ""
}

func dadosAuditoriaApi(nome, desc string, escopos []string, valida pgtype.Timestamptz, limite int32, loc *time.Location) map[string]any {
	v := "sem validade"
	if valida.Valid {
		v = valida.Time.In(loc).Format("02/01/2006")
	}
	return map[string]any{"nome": nome, "descricao": desc, "escopos": escopos, "validade": v, "limite_por_minuto": limite}
}

func (h *handlers) apiCriar(w http.ResponseWriter, r *http.Request) {
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
		d := views.ApiFormData{
			Nome: r.PostFormValue("nome"), Descricao: r.PostFormValue("descricao"), Escopos: h.escopoLinhas(r.PostForm["escopo"]),
			ValidaAte: r.PostFormValue("valida_ate"), Limite: 120, Ativa: true, Erro: msg, CSRF: csrfToken(ctx),
		}
		if n, err := strconv.Atoi(r.PostFormValue("limite")); err == nil {
			d.Limite = n
		}
		renderStatus(w, r, http.StatusBadRequest, views.ApiForm(h.layout(r, "Nova credencial de API", "/parametros/api"), d))
	}
	nome, desc, escopos, valida, limite, msg := h.lerFormApi(r)
	if msg != "" {
		falha(msg)
		return
	}
	clientID, segredo, hash, err := apiauth.NovaCredencial()
	if err != nil {
		h.falhaInterna(w, r, "gerar credencial", err)
		return
	}
	c, err := q.CreateApiCredencial(ctx, sqlccliente.CreateApiCredencialParams{
		Nome: nome, Descricao: desc, ClientID: clientID, SegredoHash: hash, Escopos: escopos, ValidaAte: valida, LimitePorMinuto: limite,
		CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
	})
	if violacaoUnica(err) {
		falha("Já existe uma credencial com esse nome.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "criar credencial", err)
		return
	}
	// o segredo nunca vai para a auditoria
	h.auditar(r, audit.Evento{
		Acao: "api.credencial_criar", Entidade: "api_credencial", EntidadeID: strconv.FormatInt(c.ID, 10), Rotulo: c.Nome,
		Depois: dadosAuditoriaApi(nome, desc, escopos, valida, limite, h.fuso(r)), Detalhes: map[string]any{"client_id": clientID},
	})
	h.mostrarSegredoApi(w, r, c, segredo, false)
}

func (h *handlers) mostrarSegredoApi(w http.ResponseWriter, r *http.Request, c sqlccliente.ApiCredenciai, segredo string, rotacao bool) {
	esquema := "https"
	if !ehHTTPS(r) && !h.prod {
		esquema = "http"
	}
	cl, _ := tenant.FromContext(r.Context())
	host := r.Host
	if cl.Slug == "" {
		host = ""
	}
	w.Header().Set("Cache-Control", "no-store")
	render(w, r, views.ApiSegredo(h.layout(r, "Credencial de API", "/parametros/api"), views.ApiSegredoData{
		ID: c.ID, Nome: c.Nome, ClientID: c.ClientID, Segredo: segredo, URLToken: fmt.Sprintf("%s://%s/api/v1/token", esquema, host), Rotacao: rotacao,
	}))
}

func (h *handlers) apiSalvar(w http.ResponseWriter, r *http.Request) {
	c, q, ok := h.apiCredDaURL(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	falha := func(msg string) {
		d, e := h.montarFichaApi(r, q, c, "", msg)
		if e != nil {
			h.falhaInterna(w, r, "montar ficha", e)
			return
		}
		d.Nome, d.Descricao, d.ValidaAte, d.Escopos = r.PostFormValue("nome"), r.PostFormValue("descricao"), r.PostFormValue("valida_ate"), h.escopoLinhas(r.PostForm["escopo"])
		if n, err := strconv.Atoi(r.PostFormValue("limite")); err == nil {
			d.Limite = n
		}
		renderStatus(w, r, http.StatusBadRequest, views.ApiForm(h.layout(r, "Credencial de API", "/parametros/api"), d))
	}
	nome, desc, escopos, valida, limite, msg := h.lerFormApi(r)
	if msg != "" {
		falha(msg)
		return
	}
	novo, err := q.UpdateApiCredencial(r.Context(), sqlccliente.UpdateApiCredencialParams{ID: c.ID, Nome: nome, Descricao: desc, Escopos: escopos, ValidaAte: valida, LimitePorMinuto: limite})
	if violacaoUnica(err) {
		falha("Já existe uma credencial com esse nome.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "salvar credencial", err)
		return
	}
	loc := h.fuso(r)
	a, dep := audit.Diferenca(dadosAuditoriaApi(c.Nome, c.Descricao, c.Escopos, c.ValidaAte, c.LimitePorMinuto, loc), dadosAuditoriaApi(novo.Nome, novo.Descricao, novo.Escopos, novo.ValidaAte, novo.LimitePorMinuto, loc))
	if len(a)+len(dep) == 0 {
		naoAuditar(r)
	} else {
		h.auditar(r, audit.Evento{Acao: "api.credencial_editar", Entidade: "api_credencial", EntidadeID: strconv.FormatInt(c.ID, 10), Rotulo: novo.Nome, Antes: a, Depois: dep})
	}
	http.Redirect(w, r, fmt.Sprintf("/parametros/api/%d?ok=salvo", c.ID), http.StatusSeeOther)
}

func (h *handlers) apiRotacionar(w http.ResponseWriter, r *http.Request) {
	c, q, ok := h.apiCredDaURL(w, r)
	if !ok {
		return
	}
	segredo, hash, err := apiauth.NovoSegredo()
	if err != nil {
		h.falhaInterna(w, r, "gerar segredo", err)
		return
	}
	novo, err := q.RotacionarApiCredencial(r.Context(), sqlccliente.RotacionarApiCredencialParams{ID: c.ID, SegredoHash: hash})
	if err != nil {
		h.falhaInterna(w, r, "rotacionar segredo", err)
		return
	}
	h.auditar(r, audit.Evento{
		Acao: "api.credencial_rotacionar", Entidade: "api_credencial", EntidadeID: strconv.FormatInt(c.ID, 10), Rotulo: c.Nome,
		Detalhes: map[string]any{"versao_do_segredo": novo.VersaoSegredo},
	})
	h.mostrarSegredoApi(w, r, novo, segredo, true)
}

func (h *handlers) apiAtivo(w http.ResponseWriter, r *http.Request) {
	c, q, ok := h.apiCredDaURL(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	ativo := r.PostFormValue("ativo") == "1"
	resultado := map[bool]string{true: "ativado", false: "desativado"}[ativo]
	if c.Ativo != ativo {
		if _, err := q.SetApiCredencialAtivo(r.Context(), sqlccliente.SetApiCredencialAtivoParams{ID: c.ID, Ativo: ativo}); err != nil {
			h.falhaInterna(w, r, "mudar situação", err)
			return
		}
		acao := "api.credencial_desativar"
		if ativo {
			acao = "api.credencial_ativar"
		}
		h.auditar(r, audit.Evento{Acao: acao, Entidade: "api_credencial", EntidadeID: strconv.FormatInt(c.ID, 10), Rotulo: c.Nome, Antes: map[string]any{"ativo": c.Ativo}, Depois: map[string]any{"ativo": ativo}})
	} else {
		naoAuditar(r)
	}
	http.Redirect(w, r, fmt.Sprintf("/parametros/api/%d?ok=%s", c.ID, resultado), http.StatusSeeOther)
}

package server

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// LIRAa (10.5): grupos (estratos de bairros) e agenda (ciclo, semana, quarteirão e agente).

var rotulosSituacaoAgenda = []views.CidadeOpcao{
	{Valor: "aberto", Rotulo: "Aberto"},
	{Valor: "encerrado", Rotulo: "Encerrado"},
	{Valor: "desativado", Rotulo: "Desativado"},
}

func liraaGrupoCaminho(id int64) string  { return "/liraa/grupo/" + strconv.FormatInt(id, 10) }
func liraaAgendaCaminho(id int64) string { return "/liraa/agenda/" + strconv.FormatInt(id, 10) }

// ---------------------------------------------------------------- grupos

func (h *handlers) liraaGrupos(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListLiraaGrupos(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar grupos do LIRAa", err)
		return
	}
	linhas := make([]views.LiraaGrupoLinha, len(rows))
	for i, x := range rows {
		linhas[i] = views.LiraaGrupoLinha{ID: x.ID, IDTexto: strconv.FormatInt(x.ID, 10), Nome: x.Nome, Ativo: x.Ativo, Bairros: x.Bairros}
	}
	lay := h.layout(r, "LIRAa — Grupos", "/liraa/grupo")
	linhas = paginar(r, &lay, linhas)
	render(w, r, views.LiraaGrupos(lay, views.LiraaGruposData{Linhas: linhas, PodeGerir: h.principal(r).Pode("liraa_grupo")}))
}

func (h *handlers) liraaGrupoNovo(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.LiraaGrupoFicha(h.layout(r, "Novo grupo do LIRAa", "/liraa/grupo"), views.LiraaGrupoData{Ativo: true, CSRF: csrfToken(r.Context()), PodeEditar: true}))
}

func (h *handlers) liraaGrupoCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := views.LiraaGrupoData{Nome: strings.TrimSpace(r.PostFormValue("nome")), Ativo: r.PostFormValue("ativo") != "0", CSRF: csrfToken(r.Context()), PodeEditar: true}
	if erro := validarNomeLiraa(d.Nome); erro != "" {
		d.Erro = erro
		renderStatus(w, r, http.StatusBadRequest, views.LiraaGrupoFicha(h.layout(r, "Novo grupo do LIRAa", "/liraa/grupo"), d))
		return
	}
	g, err := q.CreateLiraaGrupo(r.Context(), sqlccliente.CreateLiraaGrupoParams{Nome: d.Nome, Ativo: d.Ativo})
	if err != nil {
		if violacaoUnica(err) {
			d.Erro = "Já existe um grupo com esse nome."
			renderStatus(w, r, http.StatusBadRequest, views.LiraaGrupoFicha(h.layout(r, "Novo grupo do LIRAa", "/liraa/grupo"), d))
			return
		}
		h.falhaInterna(w, r, "criar grupo do LIRAa", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "liraa_grupo.criar", Entidade: "liraa_grupo", EntidadeID: strconv.FormatInt(g.ID, 10), Rotulo: g.Nome, Depois: map[string]any{"nome": g.Nome}})
	http.Redirect(w, r, liraaGrupoCaminho(g.ID)+"?aviso=criado", http.StatusSeeOther)
}

func validarNomeLiraa(nome string) string {
	if nome == "" || len([]rune(nome)) > 120 {
		return "Informe o nome do grupo (até 120 caracteres)."
	}
	return ""
}

func (h *handlers) carregarLiraaGrupo(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.LiraaGrupo, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.LiraaGrupo{}, false
	}
	g, err := q.GetLiraaGrupo(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return g, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar grupo do LIRAa", err)
		return g, false
	}
	return g, true
}

func (h *handlers) renderLiraaGrupo(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, g sqlccliente.LiraaGrupo, erro string) {
	d := views.LiraaGrupoData{ID: g.ID, Nome: g.Nome, Ativo: g.Ativo, CSRF: csrfToken(r.Context()), Erro: erro, PodeEditar: h.principal(r).Pode("liraa_grupo")}
	if n := strings.TrimSpace(r.PostFormValue("nome")); erro != "" && r.Method == http.MethodPost && r.URL.Path == liraaGrupoCaminho(g.ID) {
		d.Nome = n
	}
	switch r.URL.Query().Get("aviso") {
	case "criado":
		d.Aviso = "Grupo criado. Adicione os bairros do estrato."
	case "salvo":
		d.Aviso = "Alterações salvas."
	}
	bs, err := q.ListLiraaGrupoBairros(r.Context(), g.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar bairros do grupo", err)
		return
	}
	nos := map[int64]bool{}
	for _, b := range bs {
		nos[b.ID] = true
		d.Bairros = append(d.Bairros, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome})
	}
	todos, err := h.opcoesDeBairros(r, q, 0)
	if err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	for _, b := range todos {
		if !nos[b.ID] {
			d.Candidatos = append(d.Candidatos, b)
		}
	}
	status := http.StatusOK
	if erro != "" {
		status = http.StatusBadRequest
	}
	renderStatus(w, r, status, views.LiraaGrupoFicha(h.layout(r, g.Nome, "/liraa/grupo"), d))
}

func (h *handlers) liraaGrupoFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if g, ok := h.carregarLiraaGrupo(w, r, q); ok {
		h.renderLiraaGrupo(w, r, q, g, "")
	}
}

func (h *handlers) liraaGrupoSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	g, ok := h.carregarLiraaGrupo(w, r, q)
	if !ok {
		return
	}
	nome, ativo := strings.TrimSpace(r.PostFormValue("nome")), r.PostFormValue("ativo") != "0"
	if erro := validarNomeLiraa(nome); erro != "" {
		h.renderLiraaGrupo(w, r, q, g, erro)
		return
	}
	if _, err := q.UpdateLiraaGrupo(r.Context(), sqlccliente.UpdateLiraaGrupoParams{ID: g.ID, Nome: nome, Ativo: ativo}); err != nil {
		if violacaoUnica(err) {
			h.renderLiraaGrupo(w, r, q, g, "Já existe um grupo com esse nome.")
			return
		}
		h.falhaInterna(w, r, "salvar grupo do LIRAa", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "liraa_grupo.editar", Entidade: "liraa_grupo", EntidadeID: strconv.FormatInt(g.ID, 10), Rotulo: nome,
		Antes: map[string]any{"nome": g.Nome, "ativo": g.Ativo}, Depois: map[string]any{"nome": nome, "ativo": ativo}})
	http.Redirect(w, r, liraaGrupoCaminho(g.ID)+"?aviso=salvo", http.StatusSeeOther)
}

func (h *handlers) liraaGrupoBairro(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	g, ok := h.carregarLiraaGrupo(w, r, q)
	if !ok {
		return
	}
	bid, _ := strconv.ParseInt(r.PostFormValue("bairro"), 10, 64)
	b, err := q.GetBairro(r.Context(), bid)
	if err != nil {
		h.renderLiraaGrupo(w, r, q, g, "Escolha um bairro válido.")
		return
	}
	if err := q.AddLiraaGrupoBairro(r.Context(), sqlccliente.AddLiraaGrupoBairroParams{GrupoID: g.ID, BairroID: b.ID}); err != nil {
		h.falhaInterna(w, r, "adicionar bairro ao grupo", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "liraa_grupo.bairro", Entidade: "liraa_grupo", EntidadeID: strconv.FormatInt(g.ID, 10), Rotulo: g.Nome, Depois: map[string]any{"adicionou": b.Nome}})
	http.Redirect(w, r, liraaGrupoCaminho(g.ID)+"?aviso=salvo", http.StatusSeeOther)
}

func (h *handlers) liraaGrupoBairroRemover(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	g, ok := h.carregarLiraaGrupo(w, r, q)
	if !ok {
		return
	}
	bid, _ := strconv.ParseInt(r.PathValue("bid"), 10, 64)
	if n, err := q.RemoveLiraaGrupoBairro(r.Context(), sqlccliente.RemoveLiraaGrupoBairroParams{GrupoID: g.ID, BairroID: bid}); err != nil {
		h.falhaInterna(w, r, "remover bairro do grupo", err)
		return
	} else if n > 0 {
		h.auditar(r, audit.Evento{Acao: "liraa_grupo.bairro", Entidade: "liraa_grupo", EntidadeID: strconv.FormatInt(g.ID, 10), Rotulo: g.Nome, Depois: map[string]any{"removeu": bid}})
	}
	http.Redirect(w, r, liraaGrupoCaminho(g.ID)+"?aviso=salvo", http.StatusSeeOther)
}

func (h *handlers) liraaGrupoExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if g, ok := h.carregarLiraaGrupo(w, r, q); ok {
		h.excluirRegistro(w, r, "liraa_grupo", g.ID, "/liraa/grupo?aviso=excluido", liraaGrupoCaminho(g.ID))
	}
}

// ---------------------------------------------------------------- agenda

func ordemDeAgenda(ordem, dir string) (string, string) {
	switch ordem {
	case "id", "semana":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func (h *handlers) opcoesDeCiclos(r *http.Request, q *sqlccliente.Queries, escolhido int64) ([]views.CidadeOpcao, error) {
	cs, err := q.ListCiclos(r.Context())
	if err != nil {
		return nil, err
	}
	out := make([]views.CidadeOpcao, 0, len(cs))
	for _, c := range cs {
		out = append(out, views.CidadeOpcao{ID: c.ID, Rotulo: c.Nome, Selecionada: c.ID == escolhido})
	}
	return out, nil
}

func (h *handlers) liraaAgendaLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.LiraaAgendaFiltro{Ciclo: qs.Get("ciclo"), Semana: strings.TrimSpace(qs.Get("semana")), Bairro: qs.Get("bairro"), Agente: qs.Get("agente"), Situacao: qs.Get("situacao")}
	if !validoEm(rotulosSituacaoAgenda, f.Situacao) {
		f.Situacao = ""
	}
	semana := pgtype.Int4{}
	if n, err := strconv.Atoi(f.Semana); err == nil && n > 0 && n < 54 {
		semana = pgtype.Int4{Int32: int32(n), Valid: true}
	}
	ordem, dir := ordemDeAgenda(qs.Get("ordem"), qs.Get("dir"))
	arg := sqlccliente.ListLiraaAgendaParams{CicloID: idOpcional(f.Ciclo), Semana: semana, BairroID: idOpcional(f.Bairro), AgenteID: idOpcional(f.Agente), Situacao: f.Situacao, Ordem: ordem, Dir: dir}
	pag := paginaDe(r)
	total, err := q.CountLiraaAgenda(ctx, sqlccliente.CountLiraaAgendaParams{CicloID: arg.CicloID, Semana: arg.Semana, BairroID: arg.BairroID, AgenteID: arg.AgenteID, Situacao: arg.Situacao})
	if err != nil {
		h.falhaInterna(w, r, "contar agenda", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListLiraaAgenda(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar agenda", err)
		return
	}
	d := views.LiraaAgendaData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: h.principal(r).Pode("liraa_agenda_gerenciar")}
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.LiraaAgendaLinha{IDTexto: strconv.FormatInt(x.ID, 10), Ciclo: x.Ciclo, Semana: int(x.Semana), Bairro: x.Bairro, Quarteirao: x.Quarteirao, Agente: x.Agente, Situacao: rotuloDe(rotulosSituacaoAgenda, x.Situacao)})
	}
	if d.Ciclos, err = h.opcoesDeCiclos(r, q, arg.CicloID.Int64); err != nil {
		h.falhaInterna(w, r, "listar ciclos", err)
		return
	}
	if d.Bairros, err = h.opcoesDeBairros(r, q, arg.BairroID.Int64); err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	if d.Agentes, err = h.opcoesDeAgentes(r, q, arg.AgenteID.Int64); err != nil {
		h.falhaInterna(w, r, "listar agentes", err)
		return
	}
	d.Situacoes = marcarOpcao(rotulosSituacaoAgenda, f.Situacao)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"ciclo": f.Ciclo, "semana": f.Semana, "bairro": f.Bairro, "agente": f.Agente, "situacao": f.Situacao} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.LiraaAgenda(h.layout(r, "LIRAa — Agenda", "/liraa/agenda"), d))
}

// formAgenda monta o formulário (novo ou edição); a.ID 0 = novo.
func (h *handlers) formAgenda(r *http.Request, q *sqlccliente.Queries, a sqlccliente.LiraaAgenda) (views.LiraaAgendaForm, error) {
	d := views.LiraaAgendaForm{ID: a.ID, Semana: "", CSRF: csrfToken(r.Context()), PodeEditar: h.principal(r).Pode("liraa_agenda_gerenciar")}
	if a.ID != 0 {
		d.Semana = strconv.Itoa(int(a.Semana))
	}
	var err error
	if d.Ciclos, err = h.opcoesDeCiclos(r, q, a.CicloID); err != nil {
		return d, err
	}
	if d.Bairros, err = h.opcoesDeBairros(r, q, a.BairroID); err != nil {
		return d, err
	}
	if d.Agentes, err = h.opcoesDeAgentes(r, q, a.UsuarioID.Int64); err != nil {
		return d, err
	}
	sit := a.Situacao
	if sit == "" {
		sit = "aberto"
	}
	d.Situacoes = marcarOpcao(rotulosSituacaoAgenda, sit)
	d.Quarteiroes = nil
	if a.BairroID != 0 {
		qs, err := q.ListQuarteiroesAtivosDoBairro(r.Context(), sqlccliente.ListQuarteiroesAtivosDoBairroParams{BairroID: a.BairroID})
		if err != nil {
			return d, err
		}
		for _, x := range qs {
			d.Quarteiroes = append(d.Quarteiroes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: x.ID == a.QuarteiraoID})
		}
	}
	return d, nil
}

// lerAgenda valida o formulário; devolve os campos ou a mensagem de erro.
func (h *handlers) lerAgenda(r *http.Request, q *sqlccliente.Queries) (a sqlccliente.LiraaAgenda, erro string) {
	f := func(n string) string { return strings.TrimSpace(r.PostFormValue(n)) }
	a.CicloID, _ = strconv.ParseInt(f("ciclo"), 10, 64)
	a.BairroID, _ = strconv.ParseInt(f("bairro"), 10, 64)
	a.QuarteiraoID, _ = strconv.ParseInt(f("quarteirao"), 10, 64)
	a.UsuarioID = idOpcional(f("agente"))
	a.Situacao = f("situacao")
	n, err := strconv.Atoi(f("semana"))
	a.Semana = int32(n)
	ctx := r.Context()
	switch {
	case !validoEm(rotulosSituacaoAgenda, a.Situacao):
		erro = "Escolha a situação."
	case err != nil || n < 1 || n > 53:
		erro = "A semana precisa ser um número de 1 a 53."
	default:
		if _, e := q.GetCiclo(ctx, a.CicloID); e != nil {
			erro = "Escolha o ciclo."
		} else if qt, e := q.GetQuarteirao(ctx, a.QuarteiraoID); e != nil || qt.BairroID != a.BairroID {
			erro = "Escolha o bairro e um quarteirão desse bairro."
		} else if a.UsuarioID.Valid {
			if u, e := q.GetUsuarioByID(ctx, a.UsuarioID.Int64); e != nil || !u.Ativo {
				erro = "Escolha um agente ativo."
			}
		}
	}
	return
}

func (h *handlers) liraaAgendaNova(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d, err := h.formAgenda(r, q, sqlccliente.LiraaAgenda{})
	if err != nil {
		h.falhaInterna(w, r, "montar formulário da agenda", err)
		return
	}
	render(w, r, views.LiraaAgendaFicha(h.layout(r, "Novo agendamento", "/liraa/agenda"), d))
}

func (h *handlers) liraaAgendaCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	a, erro := h.lerAgenda(r, q)
	if erro != "" {
		d, e := h.formAgenda(r, q, a)
		if e != nil {
			h.falhaInterna(w, r, "montar formulário da agenda", e)
			return
		}
		d.Erro = erro
		renderStatus(w, r, http.StatusBadRequest, views.LiraaAgendaFicha(h.layout(r, "Novo agendamento", "/liraa/agenda"), d))
		return
	}
	n, err := q.CreateLiraaAgenda(r.Context(), sqlccliente.CreateLiraaAgendaParams{CicloID: a.CicloID, Semana: a.Semana, BairroID: a.BairroID, QuarteiraoID: a.QuarteiraoID,
		UsuarioID: a.UsuarioID, Situacao: a.Situacao, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}})
	if err != nil {
		h.falhaInterna(w, r, "criar agendamento", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "liraa_agenda.criar", Entidade: "liraa_agenda", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: "Agendamento " + strconv.FormatInt(n.ID, 10),
		Depois: map[string]any{"ciclo": n.CicloID, "semana": n.Semana, "quarteirao": n.QuarteiraoID}})
	http.Redirect(w, r, liraaAgendaCaminho(n.ID)+"?aviso=criado", http.StatusSeeOther)
}

func (h *handlers) carregarAgenda(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.LiraaAgenda, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.LiraaAgenda{}, false
	}
	a, err := q.GetLiraaAgenda(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return a, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar agendamento", err)
		return a, false
	}
	return a, true
}

func (h *handlers) liraaAgendaFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	a, ok := h.carregarAgenda(w, r, q)
	if !ok {
		return
	}
	d, err := h.formAgenda(r, q, a)
	if err != nil {
		h.falhaInterna(w, r, "montar formulário da agenda", err)
		return
	}
	if r.URL.Query().Get("aviso") != "" {
		d.Aviso = "Agendamento salvo."
	}
	render(w, r, views.LiraaAgendaFicha(h.layout(r, "Agendamento "+strconv.FormatInt(a.ID, 10), "/liraa/agenda"), d))
}

func (h *handlers) liraaAgendaSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	atual, ok := h.carregarAgenda(w, r, q)
	if !ok {
		return
	}
	a, erro := h.lerAgenda(r, q)
	a.ID = atual.ID
	if erro != "" {
		d, e := h.formAgenda(r, q, a)
		if e != nil {
			h.falhaInterna(w, r, "montar formulário da agenda", e)
			return
		}
		d.Erro = erro
		renderStatus(w, r, http.StatusBadRequest, views.LiraaAgendaFicha(h.layout(r, "Agendamento "+strconv.FormatInt(a.ID, 10), "/liraa/agenda"), d))
		return
	}
	if _, err := q.UpdateLiraaAgenda(r.Context(), sqlccliente.UpdateLiraaAgendaParams{ID: a.ID, CicloID: a.CicloID, Semana: a.Semana, BairroID: a.BairroID,
		QuarteiraoID: a.QuarteiraoID, UsuarioID: a.UsuarioID, Situacao: a.Situacao}); err != nil {
		h.falhaInterna(w, r, "salvar agendamento", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "liraa_agenda.editar", Entidade: "liraa_agenda", EntidadeID: strconv.FormatInt(a.ID, 10), Rotulo: "Agendamento " + strconv.FormatInt(a.ID, 10),
		Antes:  map[string]any{"semana": atual.Semana, "situacao": atual.Situacao, "agente": atual.UsuarioID.Int64},
		Depois: map[string]any{"semana": a.Semana, "situacao": a.Situacao, "agente": a.UsuarioID.Int64}})
	http.Redirect(w, r, liraaAgendaCaminho(a.ID)+"?aviso=salvo", http.StatusSeeOther)
}

func (h *handlers) liraaAgendaExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if a, ok := h.carregarAgenda(w, r, q); ok {
		h.excluirRegistro(w, r, "liraa_agenda", a.ID, "/liraa/agenda?aviso=excluido", liraaAgendaCaminho(a.ID))
	}
}

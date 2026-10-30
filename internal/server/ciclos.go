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
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

const (
	formatoData    = "02/01/2006"
	formatoDataISO = "2006-01-02"
)

func dataBR(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format(formatoData)
}

func dataISO(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format(formatoDataISO)
}

func (h *handlers) ciclosLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListCiclos(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar ciclos", err)
		return
	}
	tipo := h.tipoSemana(r, q)
	linhas := make([]views.CicloLinha, len(rows))
	for i, c := range rows {
		linhas[i] = views.CicloLinha{
			ID: c.ID, Nome: c.Nome, Inicio: dataBR(c.DataInicio), Final: dataBR(c.DataFinal),
			Semanas: textoDaSemana(c.DataInicio.Time, tipo) + " a " + textoDaSemana(c.DataFinal.Time, tipo),
			Fechado: c.Situacao == "fechado", Publico: c.PainelPublico, Atividades: c.Atividades,
		}
	}
	render(w, r, views.Ciclos(h.layout(r, "Ciclos", "/ciclos"), views.CiclosData{Linhas: linhas, PodeEditar: true, TipoSemana: tipo, RotuloSemana: rotuloDoTipoSemana(tipo), CSRF: csrfToken(r.Context())}))
}

func (h *handlers) cicloNovoForm(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := views.CicloFormData{PodeEditar: true, CSRF: csrfToken(r.Context())}
	if err := h.completarFormDeCiclo(r.Context(), q, &d, nil, nil, nil); err != nil {
		h.falhaInterna(w, r, "montar formulário do ciclo", err)
		return
	}
	render(w, r, views.CicloForm(h.layout(r, "Novo ciclo", "/ciclos"), d))
}

func (h *handlers) cicloEditarForm(w http.ResponseWriter, r *http.Request) {
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
	c, err := q.GetCiclo(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar ciclo", err)
		return
	}
	ativ, err1 := q.ListAtividadesDoCiclo(r.Context(), id)
	painel, err2 := q.ListAtividadesDoPainel(r.Context(), id)
	sinc, err3 := q.ListCiclosSincronizados(r.Context(), id)
	if err := errors.Join(err1, err2, err3); err != nil {
		h.falhaInterna(w, r, "ler dados do ciclo", err)
		return
	}
	d := views.CicloFormData{
		ID: c.ID, Nome: c.Nome, Inicio: dataISO(c.DataInicio), Final: dataISO(c.DataFinal),
		Fechado: c.Situacao == "fechado", PermiteForaPeriodo: c.PermiteForaPeriodo,
		PainelPublico: c.PainelPublico, PainelTexto: c.PainelPublicoTexto,
		PodeEditar: true, CSRF: csrfToken(r.Context()),
	}
	if err := h.completarFormDeCiclo(r.Context(), q, &d, ativ, painel, sinc); err != nil {
		h.falhaInterna(w, r, "montar formulário do ciclo", err)
		return
	}
	d.Aviso = h.avisoDeCruzamento(r.Context(), q, id, c.DataInicio, c.DataFinal)
	tipo := h.tipoSemana(r, q)
	d.Semanas = textoDaSemana(c.DataInicio.Time, tipo) + " a " + textoDaSemana(c.DataFinal.Time, tipo) + " (" + rotuloDoTipoSemana(tipo) + ")"
	render(w, r, views.CicloForm(h.layout(r, "Ciclo", "/ciclos"), d))
}

// completarFormDeCiclo preenche as listas de escolha (atividades e ciclos para sincronizar) com as marcas dadas.
func (h *handlers) completarFormDeCiclo(ctx context.Context, q *sqlccliente.Queries, d *views.CicloFormData, ativ, painel []int16, sinc []int64) error {
	cat, err := q.ListAtividades(ctx)
	if err != nil {
		return err
	}
	d.Atividades = make([]views.AtividadeOpcao, len(cat))
	for i, a := range cat {
		d.Atividades[i] = views.AtividadeOpcao{ID: a.ID, Sigla: a.Sigla, Nome: a.Nome, Marcada: slices.Contains(ativ, a.ID), NoPainel: slices.Contains(painel, a.ID)}
	}
	outros, err := q.ListCiclosParaSincronizar(ctx, d.ID)
	if err != nil {
		return err
	}
	d.Sincronizar = make([]views.CicloOpcao, len(outros))
	for i, o := range outros {
		d.Sincronizar[i] = views.CicloOpcao{ID: o.ID, Nome: o.Nome, Marcado: slices.Contains(sinc, o.ID)}
	}
	return nil
}

// avisoDeCruzamento avisa (sem impedir) quando o período cruza o de outro ciclo.
func (h *handlers) avisoDeCruzamento(ctx context.Context, q *sqlccliente.Queries, id int64, ini, fim pgtype.Date) string {
	outros, err := q.ListCiclosQueCruzam(ctx, sqlccliente.ListCiclosQueCruzamParams{Ignorar: id, Inicio: ini, Fim: fim})
	if err != nil || len(outros) == 0 {
		return ""
	}
	nomes := make([]string, len(outros))
	for i, o := range outros {
		nomes[i] = o.Nome
	}
	return "Atenção: o período deste ciclo cruza com o de " + strings.Join(nomes, ", ") + "."
}

func idsDoForm(valores []string, valido func(int64) bool) []int64 {
	var out []int64
	for _, v := range valores {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || !valido(n) || slices.Contains(out, n) {
			continue
		}
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

func (h *handlers) cicloSalvar(w http.ResponseWriter, r *http.Request) {
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
	var antes sqlccliente.Ciclo
	var antesAtiv, antesPainel []int16
	var antesSinc []int64
	if id != 0 {
		if antes, err = q.GetCiclo(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar ciclo", err)
			return
		}
		a1, e1 := q.ListAtividadesDoCiclo(r.Context(), id)
		a2, e2 := q.ListAtividadesDoPainel(r.Context(), id)
		a3, e3 := q.ListCiclosSincronizados(r.Context(), id)
		if err := errors.Join(e1, e2, e3); err != nil {
			h.falhaInterna(w, r, "ler dados do ciclo", err)
			return
		}
		antesAtiv, antesPainel, antesSinc = a1, a2, a3
	}

	cat, err := q.ListAtividades(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar atividades", err)
		return
	}
	ehAtividade := func(n int64) bool {
		return slices.ContainsFunc(cat, func(a sqlccliente.Atividade) bool { return int64(a.ID) == n })
	}
	paraInt16 := func(ns []int64) []int16 {
		out := make([]int16, len(ns))
		for i, n := range ns {
			out[i] = int16(n)
		}
		return out
	}
	outros, err := q.ListCiclosParaSincronizar(r.Context(), id)
	if err != nil {
		h.falhaInterna(w, r, "listar ciclos", err)
		return
	}
	ehOutro := func(n int64) bool {
		return slices.ContainsFunc(outros, func(o sqlccliente.ListCiclosParaSincronizarRow) bool { return o.ID == n })
	}
	ativ := paraInt16(idsDoForm(r.PostForm["atividade"], ehAtividade))
	painel := paraInt16(idsDoForm(r.PostForm["painel_atividade"], ehAtividade))
	sinc := idsDoForm(r.PostForm["sincronizar"], ehOutro)

	nome := strings.TrimSpace(r.PostFormValue("nome"))
	iniTxt, fimTxt := strings.TrimSpace(r.PostFormValue("data_inicio")), strings.TrimSpace(r.PostFormValue("data_final"))
	fechado, fora, publico := r.PostFormValue("fechado") == "1", r.PostFormValue("fora_periodo") == "1", r.PostFormValue("painel_publico") == "1"
	texto := strings.TrimSpace(r.PostFormValue("painel_texto"))

	falha := func(msg string) {
		d := views.CicloFormData{
			ID: id, Nome: nome, Inicio: iniTxt, Final: fimTxt, Fechado: fechado, PermiteForaPeriodo: fora,
			PainelPublico: publico, PainelTexto: texto, PodeEditar: true, Erro: msg, CSRF: csrfToken(r.Context()),
		}
		_ = h.completarFormDeCiclo(r.Context(), q, &d, ativ, painel, sinc)
		renderStatus(w, r, http.StatusBadRequest, views.CicloForm(h.layout(r, "Ciclo", "/ciclos"), d))
	}
	ini, errIni := time.Parse(formatoDataISO, iniTxt)
	fim, errFim := time.Parse(formatoDataISO, fimTxt)
	switch {
	case len([]rune(nome)) < 2 || len([]rune(nome)) > 60:
		falha("Informe o nome do ciclo (de 2 a 60 caracteres).")
		return
	case errIni != nil:
		falha("Informe a data de início do ciclo.")
		return
	case errFim != nil:
		falha("Informe a data final do ciclo.")
		return
	case fim.Before(ini):
		falha("A data final não pode ser anterior à data de início.")
		return
	case len([]rune(texto)) > 100:
		falha("O nome do painel público tem no máximo 100 caracteres.")
		return
	case publico && len(painel) == 0:
		falha("Para mostrar o ciclo no painel público, marque ao menos uma atividade do painel.")
		return
	}
	// O painel só mostra atividades que o ciclo tem.
	painel = slices.DeleteFunc(painel, func(a int16) bool { return !slices.Contains(ativ, a) })
	if publico && len(painel) == 0 {
		falha("As atividades do painel público precisam estar entre as atividades do ciclo.")
		return
	}
	situacao := "operando"
	if fechado {
		situacao = "fechado"
	}

	tx, err := pool.Begin(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(r.Context())
	qt := sqlccliente.New(tx)
	pIni, pFim := pgtype.Date{Time: ini, Valid: true}, pgtype.Date{Time: fim, Valid: true}
	ev := audit.Evento{Entidade: "ciclo", Rotulo: nome}
	siglas := func(ids []int16) []string {
		out := []string{}
		for _, i := range ids {
			for _, a := range cat {
				if a.ID == i {
					out = append(out, a.Sigla)
				}
			}
		}
		return out
	}
	novo := map[string]any{
		"nome": nome, "data_inicio": iniTxt, "data_final": fimTxt, "situacao": situacao, "permite_fora_periodo": fora,
		"painel_publico": publico, "painel_texto": texto, "atividades": strings.Join(siglas(ativ), ","),
		"painel_atividades": strings.Join(siglas(painel), ","), "sincronizar": len(sinc),
	}
	var cicloID int64
	if id == 0 {
		c, err := qt.CreateCiclo(r.Context(), sqlccliente.CreateCicloParams{
			Nome: nome, DataInicio: pIni, DataFinal: pFim, Situacao: situacao, PermiteForaPeriodo: fora,
			PainelPublico: publico, PainelPublicoTexto: texto,
		})
		if violacaoUnica(err) {
			falha("Já existe um ciclo com esse nome.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "criar ciclo", err)
			return
		}
		cicloID = c.ID
		ev.Acao, ev.EntidadeID, ev.Depois = "ciclo.criar", strconv.FormatInt(c.ID, 10), novo
	} else {
		cicloID = id
		if _, err := qt.UpdateCiclo(r.Context(), sqlccliente.UpdateCicloParams{
			ID: id, Nome: nome, DataInicio: pIni, DataFinal: pFim, Situacao: situacao, PermiteForaPeriodo: fora,
			PainelPublico: publico, PainelPublicoTexto: texto,
		}); violacaoUnica(err) {
			falha("Já existe um ciclo com esse nome.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "salvar ciclo", err)
			return
		}
		ev.EntidadeID = strconv.FormatInt(id, 10)
		a, d := audit.Diferenca(map[string]any{
			"nome": antes.Nome, "data_inicio": dataISO(antes.DataInicio), "data_final": dataISO(antes.DataFinal),
			"situacao": antes.Situacao, "permite_fora_periodo": antes.PermiteForaPeriodo,
			"painel_publico": antes.PainelPublico, "painel_texto": antes.PainelPublicoTexto,
			"atividades": strings.Join(siglas(antesAtiv), ","), "painel_atividades": strings.Join(siglas(antesPainel), ","),
			"sincronizar": len(antesSinc),
		}, novo)
		if len(a)+len(d) == 0 {
			naoAuditar(r)
		} else {
			ev.Acao, ev.Antes, ev.Depois = "ciclo.editar", a, d
		}
	}
	for _, e := range []error{
		qt.ClearAtividadesDoCiclo(r.Context(), cicloID),
		qt.AddAtividadesAoCiclo(r.Context(), sqlccliente.AddAtividadesAoCicloParams{Ciclo: cicloID, Atividades: ativ}),
		qt.ClearAtividadesDoPainel(r.Context(), cicloID),
		qt.AddAtividadesAoPainel(r.Context(), sqlccliente.AddAtividadesAoPainelParams{Ciclo: cicloID, Atividades: painel}),
		qt.ClearCiclosSincronizados(r.Context(), cicloID),
		qt.AddCiclosSincronizados(r.Context(), sqlccliente.AddCiclosSincronizadosParams{Ciclo: cicloID, Destinos: sinc}),
	} {
		if e != nil {
			h.falhaInterna(w, r, "gravar listas do ciclo", e)
			return
		}
	}
	if ev.Acao != "" {
		h.auditarCom(audit.Cliente{Q: qt}, r, ev)
	}
	if err := tx.Commit(r.Context()); err != nil {
		h.falhaInterna(w, r, "confirmar ciclo", err)
		return
	}
	// Se o período cruza o de outro ciclo, volta à ficha (onde o aviso aparece).
	if h.avisoDeCruzamento(r.Context(), q, cicloID, pIni, pFim) != "" {
		http.Redirect(w, r, fmt.Sprintf("/ciclos/%d", cicloID), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/ciclos", http.StatusSeeOther)
}

func (h *handlers) cicloExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.excluirRegistro(w, r, "ciclo", id, "/ciclos?aviso=excluido", fmt.Sprintf("/ciclos/%d", id))
}

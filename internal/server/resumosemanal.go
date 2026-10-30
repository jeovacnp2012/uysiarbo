package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/epi"
	"uysiarbo/web/views"
)

// Resumo Semanal: a ficha "Resumo Semanal do Serviço Antivetorial" do PNCD de uma semana do ciclo, de um
// bairro e de uma atividade, gravada com os números do momento em que foi gerada, para imprimir em PDF.

func (h *handlers) resumoEscopo(r *http.Request) pgtype.Int8 {
	if h.principal(r).Restrito("resumosemanal_meubairro") {
		id := meuID(r)
		if id == 0 {
			id = -1
		}
		return pgtype.Int8{Int64: id, Valid: true}
	}
	return pgtype.Int8{}
}

func (h *handlers) resumosLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.ResumoFiltro{Ciclo: qs.Get("ciclo"), Bairro: qs.Get("bairro"), Semana: strings.TrimSpace(qs.Get("semana")), De: qs.Get("de"), Ate: qs.Get("ate")}
	p := sqlccliente.ListResumosSemanaisParams{
		CicloID: idOpcional(f.Ciclo), BairroID: idOpcional(f.Bairro), SomenteUsuario: h.resumoEscopo(r),
		DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate),
	}
	if n, err := strconv.Atoi(f.Semana); err == nil && n > 0 {
		p.Semana = pgtype.Int4{Int32: int32(n), Valid: true}
	}
	rows, err := q.ListResumosSemanais(ctx, p)
	if err != nil {
		h.falhaInterna(w, r, "listar resumos", err)
		return
	}
	pode := h.principal(r).Pode("resumosemanal_adicionar")
	d := views.ResumosData{PodeGerar: pode, PodeExcluir: pode, Filtro: f, CSRF: csrfToken(ctx), Aviso: avisoDeResumo(qs.Get("aviso"))}
	loc := h.fuso(r)
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.ResumoLinha{ID: x.ID, Ciclo: x.Ciclo, Semana: int(x.Semana), Bairro: x.Bairro, Atividade: x.Atividade, Sigla: x.AtividadeSigla,
			Gerado: x.GeradoEm.Time.In(loc).Format("02/01/2006")})
	}
	d.Ciclos, d.Bairros, err = h.opcoesCicloBairro(ctx, q, r, idOpcional(f.Ciclo).Int64, idOpcional(f.Bairro).Int64)
	if err != nil {
		h.falhaInterna(w, r, "opções", err)
		return
	}
	lay := h.layout(r, "Resumo semanal", "/resumosemanal")
	d.Linhas = paginar(r, &lay, d.Linhas)
	render(w, r, views.ResumosSemanais(lay, d))
}

func avisoDeResumo(codigo string) string {
	switch codigo {
	case "excluido":
		return "Resumo semanal enviado para a lixeira."
	case "gerado":
		return "Resumo semanal gerado."
	}
	return ""
}

func (h *handlers) resumoNovo(w http.ResponseWriter, r *http.Request) {
	h.resumoFormulario(w, r, "", http.StatusOK)
}

func (h *handlers) resumoFormulario(w http.ResponseWriter, r *http.Request, erro string, status int) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := views.ResumoNovoData{Erro: erro, CSRF: csrfToken(ctx)}
	cs, err := q.ListCiclos(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar ciclos", err)
		return
	}
	for _, c := range cs {
		d.Ciclos = append(d.Ciclos, views.CidadeOpcao{ID: c.ID, Rotulo: c.Nome + " | " + strings.ToUpper(c.Situacao), Selecionada: s64txt(c.ID) == r.PostFormValue("ciclo_id")})
	}
	_, d.Bairros, err = h.opcoesCicloBairro(ctx, q, r, 0, idOpcional(r.PostFormValue("bairro_id")).Int64)
	if err != nil {
		h.falhaInterna(w, r, "opções", err)
		return
	}
	ativs, err := q.ListAtividades(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar atividades", err)
		return
	}
	for _, a := range ativs {
		d.Atividades = append(d.Atividades, views.CidadeOpcao{ID: int64(a.ID), Rotulo: a.Nome + " (" + a.Sigla + ")", Selecionada: strconv.Itoa(int(a.ID)) == r.PostFormValue("atividade_id")})
	}
	if c := idOpcional(r.PostFormValue("ciclo_id")); c.Valid {
		d.SemanasHTML = h.opcoesDeSemanas(r, q, c.Int64, r.PostFormValue("semana"))
	}
	renderStatus(w, r, status, views.ResumoNovo(h.layout(r, "Novo resumo semanal", "/resumosemanal"), d))
}

// resumoSemanas devolve as opções do select de semana: as semanas epidemiológicas do período do ciclo.
func (h *handlers) resumoSemanas(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	c := idOpcional(r.URL.Query().Get("ciclo_id"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if !c.Valid {
		_, _ = w.Write([]byte(`<option value=""></option>`))
		return
	}
	_, _ = w.Write([]byte(h.opcoesDeSemanas(r, q, c.Int64, "")))
}

func (h *handlers) opcoesDeSemanas(r *http.Request, q *sqlccliente.Queries, ciclo int64, escolhida string) string {
	c, err := q.GetCiclo(r.Context(), ciclo)
	if err != nil {
		return `<option value=""></option>`
	}
	tipo := h.tipoSemana(r, q)
	semanas := semanasDoCiclo(c, tipo)
	hoje := epi.Calcular(time.Now().In(h.fuso(r)), tipo)
	if escolhida == "" {
		// sem escolha, vale a semana de hoje, ou a última do ciclo
		for _, s := range semanas {
			if s.Ano == hoje.Ano && s.Numero == hoje.Numero {
				escolhida = strconv.Itoa(s.Numero)
			}
		}
		if escolhida == "" && len(semanas) > 0 {
			escolhida = strconv.Itoa(semanas[len(semanas)-1].Numero)
		}
	}
	var b strings.Builder
	for _, s := range semanas {
		rot := fmt.Sprintf("Ano %d - Semana %d", s.Ano, s.Numero)
		if s.Ano == hoje.Ano && s.Numero == hoje.Numero {
			rot += " (Atual)"
		}
		sel := ""
		if strconv.Itoa(s.Numero) == escolhida {
			sel = " selected"
		}
		fmt.Fprintf(&b, `<option value="%d"%s>%s</option>`, s.Numero, sel, html.EscapeString(rot))
	}
	return b.String()
}

// resumoEntrada é o que o formulário pede: ciclo, semana, bairro e atividade.
type resumoEntrada struct {
	q      *sqlccliente.Queries
	ciclo  sqlccliente.Ciclo
	semana int
	bairro int64
	ativ   sqlccliente.Atividade
}

func (h *handlers) lerResumoEntrada(r *http.Request) (resumoEntrada, string, bool) {
	var e resumoEntrada
	if err := r.ParseForm(); err != nil {
		return e, "Requisição inválida.", false
	}
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		return e, "Serviço temporariamente indisponível.", false
	}
	e.q = q
	cicloID, bairro, ativ := idOpcional(r.PostFormValue("ciclo_id")), idOpcional(r.PostFormValue("bairro_id")), idOpcional(r.PostFormValue("atividade_id"))
	semana, errSem := strconv.Atoi(r.PostFormValue("semana"))
	if !cicloID.Valid || errSem != nil || semana < 1 || semana > 53 || !bairro.Valid || !ativ.Valid {
		return e, "Escolha o ciclo, a semana, o bairro e a atividade.", false
	}
	if ok, err := h.bairroDoReconhecimentoVisivel(ctx, q, r, bairro.Int64); err != nil || !ok || (h.resumoEscopo(r).Valid && !h.bairroNoEscopoResumo(r, q, bairro.Int64)) {
		return e, "Escolha um bairro da lista.", false
	}
	if e.ciclo, err = q.GetCiclo(ctx, cicloID.Int64); err != nil {
		return e, "Escolha um ciclo da lista.", false
	}
	ativs, err := q.ListAtividades(ctx)
	if err != nil {
		return e, "Serviço temporariamente indisponível.", false
	}
	found := false
	for _, a := range ativs {
		if int64(a.ID) == ativ.Int64 {
			e.ativ, found = a, true
		}
	}
	if !found {
		return e, "Escolha uma atividade da lista.", false
	}
	e.semana, e.bairro = semana, bairro.Int64
	return e, "", true
}

// resumoGerar (Salvar) calcula a ficha e grava o retrato.
func (h *handlers) resumoGerar(w http.ResponseWriter, r *http.Request) {
	e, msg, ok := h.lerResumoEntrada(r)
	if !ok {
		h.resumoFormulario(w, r, msg, http.StatusBadRequest)
		return
	}
	dados, err := h.calcularPncd(r.Context(), e.q, r, e.ciclo, e.semana, e.bairro, e.ativ)
	if errors.Is(err, errSemanaForaDoCiclo) {
		h.resumoFormulario(w, r, "A semana escolhida não pertence ao ciclo.", http.StatusBadRequest)
		return
	} else if errors.Is(err, errResumoSemVisitas) {
		h.resumoFormulario(w, r, "Não há visitas nesta semana, ciclo, bairro e atividade.", http.StatusBadRequest)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "calcular resumo semanal", err)
		return
	}
	js, err := json.Marshal(dados)
	if err != nil {
		h.falhaInterna(w, r, "gravar resumo semanal", err)
		return
	}
	id, err := e.q.CreateResumoSemanal(r.Context(), sqlccliente.CreateResumoSemanalParams{
		CicloID: e.ciclo.ID, Semana: int16(e.semana), BairroID: pgtype.Int8{Int64: e.bairro, Valid: true},
		AtividadeID: pgtype.Int2{Int16: e.ativ.ID, Valid: true}, Dados: js, GeradoPor: porDoRequest(r),
	})
	if err != nil {
		h.falhaInterna(w, r, "gravar resumo semanal", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "resumosemanal.gerar", Entidade: "resumosemanal", EntidadeID: s64txt(id), Rotulo: "Semana " + strconv.Itoa(e.semana),
		Detalhes: map[string]any{"ciclo": e.ciclo.ID, "bairro": e.bairro, "atividade": e.ativ.Sigla, "imoveis": dados.ImTotal}})
	http.Redirect(w, r, "/resumosemanal?aviso=gerado", http.StatusSeeOther)
}

// resumoVisualizar mostra a ficha em PDF sem gravar nada.
func (h *handlers) resumoVisualizar(w http.ResponseWriter, r *http.Request) {
	e, msg, ok := h.lerResumoEntrada(r)
	if !ok {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	dados, err := h.calcularPncd(r.Context(), e.q, r, e.ciclo, e.semana, e.bairro, e.ativ)
	if errors.Is(err, errSemanaForaDoCiclo) {
		http.Error(w, "A semana escolhida não pertence ao ciclo.", http.StatusBadRequest)
		return
	} else if errors.Is(err, errResumoSemVisitas) {
		http.Error(w, "Não há visitas nesta semana, ciclo, bairro e atividade.", http.StatusBadRequest)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "calcular resumo semanal", err)
		return
	}
	h.enviarPncd(w, r, dados, "resumo_semanal_previa.pdf")
}

func (h *handlers) enviarPncd(w http.ResponseWriter, r *http.Request, dados pncdDados, nome string) {
	pdf, err := pncdPDF(dados)
	if err != nil {
		h.falhaInterna(w, r, "gerar pdf do resumo semanal", err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+nome+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

// bairroNoEscopoResumo confere se o bairro está entre os que a pessoa pode ver neste recurso.
func (h *handlers) bairroNoEscopoResumo(r *http.Request, q *sqlccliente.Queries, bairro int64) bool {
	bs, err := q.ListBairrosAtivosParaLocal(r.Context(), h.resumoEscopo(r))
	if err != nil {
		return false
	}
	for _, b := range bs {
		if b.ID == bairro {
			return true
		}
	}
	return false
}

// resumoVer entrega o PDF do resumo gravado.
func (h *handlers) resumoVer(w http.ResponseWriter, r *http.Request) {
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
	x, err := q.GetResumoSemanal(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar resumo semanal", err)
		return
	}
	if e := h.resumoEscopo(r); e.Valid && (!x.BairroID.Valid || !h.bairroNoEscopoResumo(r, q, x.BairroID.Int64)) {
		http.NotFound(w, r)
		return
	}
	var dados pncdDados
	if err := json.Unmarshal(x.Dados, &dados); err != nil || dados.Versao != versaoPncd {
		http.Error(w, "Este resumo foi gerado num formato antigo e não pode ser impresso. Gere de novo.", http.StatusConflict)
		return
	}
	h.enviarPncd(w, r, dados, "resumo_semanal_"+s64txt(id)+".pdf")
}

func (h *handlers) resumoExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.excluirRegistro(w, r, "resumosemanal", id, "/resumosemanal?aviso=excluido", "/resumosemanal")
}

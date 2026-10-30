package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Resumo Semanal: o retrato da produção de uma semana do ciclo (por bairro e atividade, ou de tudo), gravado
// com os números do momento em que foi gerado, para imprimir e arquivar.

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
	h.resumosTela(w, r, "", http.StatusOK)
}

func (h *handlers) resumosTela(w http.ResponseWriter, r *http.Request, erro string, status int) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	ciclo := idOpcional(qs.Get("ciclo"))
	rows, err := q.ListResumosSemanais(ctx, sqlccliente.ListResumosSemanaisParams{CicloID: ciclo, SomenteUsuario: h.resumoEscopo(r)})
	if err != nil {
		h.falhaInterna(w, r, "listar resumos", err)
		return
	}
	d := views.ResumosData{PodeGerar: h.principal(r).Pode("resumosemanal_adicionar"), Erro: erro, CSRF: csrfToken(ctx), Aviso: avisoDeResumo(qs.Get("aviso"))}
	loc := h.fuso(r)
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.ResumoLinha{ID: x.ID, Ciclo: x.Ciclo, Semana: int(x.Semana), Bairro: x.Bairro, Atividade: x.Atividade,
			Gerado: formatarData(loc, x.GeradoEm.Time)})
	}
	ciclos, bairros, err := h.opcoesCicloBairro(ctx, q, r, ciclo.Int64, 0)
	if err != nil {
		h.falhaInterna(w, r, "opções", err)
		return
	}
	d.Ciclos, d.Bairros = ciclos, bairros
	ativs, err := q.ListAtividades(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar atividades", err)
		return
	}
	for _, a := range ativs {
		d.Atividades = append(d.Atividades, views.CidadeOpcao{ID: int64(a.ID), Rotulo: a.Sigla + " — " + a.Nome})
	}
	lay := h.layout(r, "Resumo semanal", "/resumosemanal")
	d.Linhas = paginar(r, &lay, d.Linhas)
	renderStatus(w, r, status, views.ResumosSemanais(lay, d))
}

func avisoDeResumo(codigo string) string {
	switch codigo {
	case "excluido":
		return "Resumo semanal enviado para a lixeira."
	}
	return ""
}

// resumoGerar calcula os números da semana e grava o retrato.
func (h *handlers) resumoGerar(w http.ResponseWriter, r *http.Request) {
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
	ciclo, bairro, ativ := idOpcional(r.PostFormValue("ciclo_id")), idOpcional(r.PostFormValue("bairro_id")), idOpcional(r.PostFormValue("atividade_id"))
	semana, errSem := strconv.Atoi(r.PostFormValue("semana"))
	if !ciclo.Valid || errSem != nil || semana < 1 || semana > 53 {
		h.resumosTela(w, r, "Escolha o ciclo e informe a semana (de 1 a 53).", http.StatusBadRequest)
		return
	}
	if h.resumoEscopo(r).Valid && !bairro.Valid {
		h.resumosTela(w, r, "Escolha o bairro.", http.StatusBadRequest)
		return
	}
	if bairro.Valid {
		if ok, err := h.bairroDoReconhecimentoVisivel(ctx, q, r, bairro.Int64); err != nil || !ok || (h.resumoEscopo(r).Valid && !h.bairroNoEscopoResumo(r, q, bairro.Int64)) {
			h.resumosTela(w, r, "Escolha um bairro da lista.", http.StatusBadRequest)
			return
		}
	}
	if _, err := q.GetCiclo(ctx, ciclo.Int64); err != nil {
		h.resumosTela(w, r, "Escolha um ciclo da lista.", http.StatusBadRequest)
		return
	}
	dados, err := q.CalcularResumoSemanal(ctx, sqlccliente.CalcularResumoSemanalParams{
		CicloID: ciclo.Int64, Semana: pgtype.Int2{Int16: int16(semana), Valid: true}, BairroID: bairro,
		AtividadeID: ativ, SomenteAgente: h.escopoVisitas(r),
	})
	if err != nil {
		h.falhaInterna(w, r, "calcular resumo semanal", err)
		return
	}
	if dados.Visitas == 0 {
		h.resumosTela(w, r, "Não há visitas nesta semana, ciclo, bairro e atividade.", http.StatusBadRequest)
		return
	}
	js, _ := json.Marshal(dados)
	atividade := pgtype.Int2{}
	if ativ.Valid {
		atividade = pgtype.Int2{Int16: int16(ativ.Int64), Valid: true}
	}
	id, err := q.CreateResumoSemanal(ctx, sqlccliente.CreateResumoSemanalParams{
		CicloID: ciclo.Int64, Semana: int16(semana), BairroID: bairro, AtividadeID: atividade, Dados: js, GeradoPor: porDoRequest(r),
	})
	if err != nil {
		h.falhaInterna(w, r, "gravar resumo semanal", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "resumosemanal.gerar", Entidade: "resumosemanal", EntidadeID: s64txt(id), Rotulo: "Semana " + strconv.Itoa(semana),
		Detalhes: map[string]any{"ciclo": ciclo.Int64, "visitas": dados.Visitas}})
	http.Redirect(w, r, "/resumosemanal/"+s64txt(id), http.StatusSeeOther)
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
	var dados views.ResumoDados
	if err := json.Unmarshal(x.Dados, &dados); err != nil {
		h.falhaInterna(w, r, "ler retrato do resumo", err)
		return
	}
	d := views.ResumoVer{
		ID: x.ID, Ciclo: x.CicloNome, Semana: int(x.Semana), Bairro: x.BairroNome, Atividade: x.AtividadeNome,
		Gerado: formatarData(h.fuso(r), x.GeradoEm.Time), GeradoPor: x.GeradoPorNome, Dados: dados,
		PodeExcluir: h.principal(r).Pode("resumosemanal_adicionar"), CSRF: csrfToken(r.Context()),
	}
	render(w, r, views.ResumoSemanalVer(h.layout(r, "Resumo semanal", "/resumosemanal"), d))
}

func (h *handlers) resumoExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.excluirRegistro(w, r, "resumosemanal", id, "/resumosemanal?aviso=excluido", "/resumosemanal/"+s64txt(id))
}

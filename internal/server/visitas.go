package server

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/authz"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/epi"
	"uysiarbo/web/views"
)

const (
	maxFotoVisita      = 5 << 20 // 5 MB por foto
	maxFotosPorVisita  = 20
	maxFotosPorEnvio   = 10
	prefixoAssinatura  = "data:image/png;base64,"
	maxAssinaturaBytes = 400000
)

var (
	rotulosTipoVisita = []views.CidadeOpcao{
		{Valor: "normal", Rotulo: "Normal"},
		{Valor: "recuperacao", Rotulo: "Recuperação"},
	}
	rotulosPendencia = []views.CidadeOpcao{
		{Valor: "nenhuma", Rotulo: "Nenhuma"},
		{Valor: "fechado", Rotulo: "Fechado"},
		{Valor: "recusado", Rotulo: "Recusado"},
		{Valor: "tentativa", Rotulo: "Tentativa"},
	}
)

func rotuloDe(opcoes []views.CidadeOpcao, valor string) string {
	for _, o := range opcoes {
		if o.Valor == valor {
			return o.Rotulo
		}
	}
	return valor
}

func marcarOpcao(opcoes []views.CidadeOpcao, atual string) []views.CidadeOpcao {
	out := make([]views.CidadeOpcao, len(opcoes))
	for i, o := range opcoes {
		o.Selecionada = o.Valor == atual
		out[i] = o
	}
	return out
}

func tamanhoLegivel(n int) string {
	if n < 1024 {
		return strconv.Itoa(n) + " B"
	}
	if n < 1<<20 {
		return fmt.Sprintf("%.0f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}

// escopoVisitas devolve o filtro "apenas as minhas" do papel (agente a que a lista se limita).
func (h *handlers) escopoVisitas(r *http.Request) pgtype.Int8 {
	if h.principal(r).Restrito("visita_visualizar_minha") {
		if id := meuID(r); id != 0 {
			return pgtype.Int8{Int64: id, Valid: true}
		}
		return pgtype.Int8{Int64: -1, Valid: true}
	}
	return pgtype.Int8{}
}

// visitaVisivel diz se a visita está ao alcance de quem pede (desativada, "só as minhas" e acesso ao local).
func (h *handlers) visitaVisivel(ctx context.Context, r *http.Request, q *sqlccliente.Queries, v sqlccliente.Visita) (bool, error) {
	pr := h.principal(r)
	if !v.Ativo && !pr.Pode("visita_ver_desativados") {
		return false, nil
	}
	if so := h.escopoVisitas(r); so.Valid && so.Int64 != v.AgenteID {
		return false, nil
	}
	l, err := q.GetLocalQualquer(ctx, v.LocalID)
	if err != nil {
		return false, err
	}
	if somente, soPE := h.escopoLocais(r); somente.Valid || soPE {
		return h.localVisivel(ctx, r, q, l)
	}
	return true, nil
}

// ------------------------------------------------------------------- Formulário

func lerVisita(r *http.Request) views.VisitaCampos {
	f := func(n string) string { return strings.TrimSpace(r.PostFormValue(n)) }
	return views.VisitaCampos{
		Ciclo: f("ciclo"), Atividade: f("atividade"), Agente: f("agente"), Data: f("data"), Hora: f("hora"),
		Tipo: f("tipo"), Pendencia: f("pendencia"), Achou: f("achou") == "1",
		A1: f("a1"), A2: f("a2"), B: f("b"), C: f("c"), D1: f("d1"), D2: f("d2"), E: f("e"), TotalEliminado: f("total_eliminado"),
		Amostra: f("amostra") == "1", Tratamento: f("tratamento") == "1",
		L1Gramas: f("l1_gramas"), L1Tratados: f("l1_tratados"), L2Gramas: f("l2_gramas"), L2Tratados: f("l2_tratados"),
		AdulticidaTipo: f("adulticida_tipo"), AdulticidaCargas: f("adulticida_cargas"), TratEliminados: f("trat_eliminados"),
		Bri: f("bri_volume"), Anotacao: strings.TrimSpace(r.PostFormValue("anotacao")), Assinatura: f("assinatura"),
		Lat: f("lat"), Lon: f("lon"), Precisao: f("precisao"),
	}
}

func textoInt4(v pgtype.Int4) string {
	if !v.Valid {
		return ""
	}
	return strconv.Itoa(int(v.Int32))
}

func textoNumerico(n pgtype.Numeric) string {
	if !n.Valid {
		return ""
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return ""
	}
	return strconv.FormatFloat(v.Float64, 'f', -1, 64)
}

func camposDaVisita(v sqlccliente.Visita) views.VisitaCampos {
	hora := ""
	if v.HoraEntrada.Valid {
		hora = time.UnixMicro(v.HoraEntrada.Microseconds).UTC().Format("15:04")
	}
	return views.VisitaCampos{
		Ciclo: strconv.FormatInt(v.CicloID, 10), Atividade: strconv.Itoa(int(v.AtividadeID)), Agente: strconv.FormatInt(v.AgenteID, 10),
		Data: dataISO(v.Data), Hora: hora, Tipo: v.TipoVisita, Pendencia: v.Pendencia, Achou: v.AchouDeposito,
		A1: textoInt4(v.DepA1), A2: textoInt4(v.DepA2), B: textoInt4(v.DepB), C: textoInt4(v.DepC), D1: textoInt4(v.DepD1),
		D2: textoInt4(v.DepD2), E: textoInt4(v.DepE), TotalEliminado: textoInt4(v.TotalEliminado),
		Amostra: v.ColetouAmostra, Tratamento: v.FezTratamento,
		L1Gramas: textoNumerico(v.L1Gramas), L1Tratados: textoInt4(v.L1Tratados), L2Gramas: textoNumerico(v.L2Gramas),
		L2Tratados: textoInt4(v.L2Tratados), AdulticidaTipo: v.AdulticidaTipo, AdulticidaCargas: textoInt4(v.AdulticidaCargas),
		TratEliminados: textoInt4(v.TratamentoEliminados), Bri: textoNumerico(v.BriVolume), Anotacao: v.Anotacao, Assinatura: v.Assinatura,
		Lat: textoFloat(v.Latitude), Lon: textoFloat(v.Longitude), Precisao: textoFloat(v.PrecisaoGps),
	}
}

// montarFormVisita junta as listas de escolha, as fotos e as anotações à ficha.
func (h *handlers) montarFormVisita(ctx context.Context, r *http.Request, q *sqlccliente.Queries, id int64, local sqlccliente.Locai, c views.VisitaCampos, ativo bool, erro string) (views.VisitaFormData, error) {
	pr := h.principal(r)
	if !validoEm(rotulosTipoVisita, c.Tipo) {
		c.Tipo = "normal"
	}
	if !validoEm(rotulosPendencia, c.Pendencia) {
		c.Pendencia = "nenhuma"
	}
	d := views.VisitaFormData{
		ID: id, LocalID: local.ID, Local: enderecoCompleto(local.Endereco, local.Numero), Campos: c, Ativo: ativo, Erro: erro,
		Tipos: marcarOpcao(rotulosTipoVisita, c.Tipo), Pendencias: marcarOpcao(rotulosPendencia, c.Pendencia),
		PodeEditar: pr.Pode("visita_editar"), PodeEditarPendencia: pr.Pode("visita_editar_pendencia"),
		PodeAnotar: pr.Pode("visita_anotacao"), PodeExcluir: pr.Pode("visita_desativar"),
		CSRF: csrfToken(ctx), Fuso: h.fuso(r),
	}
	ciclos, err := q.ListCiclos(ctx)
	if err != nil {
		return d, err
	}
	for _, x := range ciclos {
		d.Ciclos = append(d.Ciclos, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: strconv.FormatInt(x.ID, 10) == c.Ciclo})
	}
	ats, err := q.ListAtividades(ctx)
	if err != nil {
		return d, err
	}
	for _, x := range ats {
		d.Atividade = append(d.Atividade, views.CidadeOpcao{ID: int64(x.ID), Rotulo: x.Sigla + " — " + x.Nome, Selecionada: strconv.Itoa(int(x.ID)) == c.Atividade})
	}
	us, err := q.ListUsuariosAtivosParaResponsavel(ctx)
	if err != nil {
		return d, err
	}
	for _, x := range us {
		d.Agentes = append(d.Agentes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: strconv.FormatInt(x.ID, 10) == c.Agente})
	}
	if data, err := time.Parse(formatoDataISO, c.Data); err == nil {
		d.Semana = textoDaSemana(data, h.tipoSemana(r, q))
	}
	if id != 0 {
		ams, err := q.ListAmostrasDaVisita(ctx, id)
		if err != nil {
			return d, err
		}
		d.Amostras = amostrasDaVisita(ams)
		d.DepositosNovos = rotulosDepositoAmostra
		d.ColetouAmostra = c.Amostra
		d.PodeAmostra = pr.Pode("visita_editar") && c.Amostra
		d.PodeResultado = pr.Pode("visita_laboratorio_adicionar")
		d.TubitoAutomatico = h.tubitoAutomatico(r, q)
		if d.MostrarHistorico = pr.Pode("auditoria_sistema"); d.MostrarHistorico {
			regs, err := q.ListAuditoriaDaVisita(ctx, sqlccliente.ListAuditoriaDaVisitaParams{Visita: strconv.FormatInt(id, 10), Limite: 30})
			if err != nil {
				return d, err
			}
			loc := h.fuso(r)
			for _, g := range regs {
				d.Historico = append(d.Historico, linhaAuditoria(registroAuditoria{
					g.ID, g.Quando.Time, g.AtorTipo, g.AtorNome, g.AtorEmail, g.Ip, g.UserAgent, g.Acao, g.Entidade, g.EntidadeID, g.Rotulo, g.Antes, g.Depois, g.Detalhes,
				}, loc))
			}
		}
		fotos, err := q.ListFotosDaVisita(ctx, id)
		if err != nil {
			return d, err
		}
		for _, f := range fotos {
			d.Fotos = append(d.Fotos, views.VisitaFotoLinha{ID: f.ID, Nome: f.Nome, Tamanho: tamanhoLegivel(int(f.Tamanho))})
		}
		ans, err := q.ListAnotacoesDaVisita(ctx, id)
		if err != nil {
			return d, err
		}
		for _, a := range ans {
			d.Anotacoes = append(d.Anotacoes, views.VisitaAnotacaoLinha{Autor: a.Autor, Quando: a.CriadoEm.Time, Texto: a.Texto})
		}
	}
	return d, nil
}

func validoEm(opcoes []views.CidadeOpcao, v string) bool {
	for _, o := range opcoes {
		if o.Valor == v {
			return true
		}
	}
	return false
}

func (h *handlers) visitaNovaForm(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	idLocal, _ := strconv.ParseInt(r.URL.Query().Get("local"), 10, 64)
	local, ok := h.localDaVisita(w, r, q, idLocal)
	if !ok {
		return
	}
	hoje := time.Now().In(h.fuso(r))
	c := views.VisitaCampos{Data: hoje.Format(formatoDataISO), Tipo: "normal", Pendencia: "nenhuma"}
	if id := meuID(r); id != 0 {
		c.Agente = strconv.FormatInt(id, 10)
	}
	if ciclos, err := q.ListCiclos(r.Context()); err == nil {
		for _, x := range ciclos {
			if ok, _ := cicloAceitaData(sqlccliente.Ciclo{DataInicio: x.DataInicio, DataFinal: x.DataFinal}, hoje); ok && x.Situacao == "operando" {
				c.Ciclo = strconv.FormatInt(x.ID, 10)
				break
			}
		}
	}
	d, err := h.montarFormVisita(r.Context(), r, q, 0, local, c, true, "")
	if err != nil {
		h.falhaInterna(w, r, "montar formulário da visita", err)
		return
	}
	render(w, r, views.VisitaForm(h.layout(r, "Registrar visita", "/visitas"), d))
}

// localDaVisita busca o local da visita e confere o acesso; responde 404 quando não pode.
func (h *handlers) localDaVisita(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, id int64) (sqlccliente.Locai, bool) {
	l, err := q.GetLocal(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || id == 0 {
		http.NotFound(w, r)
		return l, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar local", err)
		return l, false
	}
	if ok, err := h.localVisivel(r.Context(), r, q, l); err != nil {
		h.falhaInterna(w, r, "checar acesso ao local", err)
		return l, false
	} else if !ok {
		http.NotFound(w, r)
		return l, false
	}
	return l, true
}

// carregarVisita busca a visita e confere que quem pede pode vê-la.
func (h *handlers) carregarVisita(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.Visita, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.Visita{}, false
	}
	v, err := q.GetVisita(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return v, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar visita", err)
		return v, false
	}
	if ok, err := h.visitaVisivel(r.Context(), r, q, v); err != nil {
		h.falhaInterna(w, r, "checar acesso à visita", err)
		return v, false
	} else if !ok {
		http.NotFound(w, r)
		return v, false
	}
	return v, true
}

func (h *handlers) visitaFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	v, ok := h.carregarVisita(w, r, q)
	if !ok {
		return
	}
	local, err := q.GetLocalQualquer(r.Context(), v.LocalID)
	if err != nil {
		h.falhaInterna(w, r, "buscar local", err)
		return
	}
	d, err := h.montarFormVisita(r.Context(), r, q, v.ID, local, camposDaVisita(v), v.Ativo, "")
	if err != nil {
		h.falhaInterna(w, r, "montar ficha da visita", err)
		return
	}
	d.Aviso = avisoDaVisita(r.URL.Query().Get("aviso"))
	render(w, r, views.VisitaForm(h.layout(r, "Visita "+strconv.FormatInt(v.ID, 10), "/visitas"), d))
}

func avisoDaVisita(codigo string) string {
	switch codigo {
	case "salva":
		return "Visita salva."
	case "foto":
		return "Foto(s) adicionada(s)."
	case "anotacao":
		return "Anotação adicionada."
	case "amostra":
		return "Amostra salva."
	case "resultado":
		return "Resultado salvo."
	case "amostra-excluida":
		return "Tubito enviado para a lixeira."
	case "desativada":
		return "Visita desativada."
	case "reativada":
		return "Visita reativada."
	}
	return ""
}

// ------------------------------------------------------------------- Validação

func numericoOpcional(s, nome string, max float64) (pgtype.Numeric, string) {
	if s == "" {
		return pgtype.Numeric{}, ""
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil || v < 0 || v > max {
		return pgtype.Numeric{}, fmt.Sprintf("%s: informe um número de 0 a %g.", nome, max)
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(v, 'f', 2, 64)); err != nil {
		return pgtype.Numeric{}, nome + ": número inválido."
	}
	return n, ""
}

func floatOpcional(s, nome string, min, max float64) (pgtype.Float8, string) {
	if s == "" {
		return pgtype.Float8{}, ""
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil || v < min || v > max {
		return pgtype.Float8{}, fmt.Sprintf("%s: informe um número de %g a %g.", nome, min, max)
	}
	return pgtype.Float8{Float64: v, Valid: true}, ""
}

// assinaturaValida aceita só uma imagem PNG em base64, para a assinatura poder voltar à tela com segurança.
func assinaturaValida(s string) bool {
	if s == "" {
		return true
	}
	if len(s) > maxAssinaturaBytes || !strings.HasPrefix(s, prefixoAssinatura) {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, prefixoAssinatura))
	return err == nil
}

// visitaValidada é o resultado de validarVisita: os dados prontos para gravar.
type visitaValidada struct {
	create sqlccliente.CreateVisitaParams
	ciclo  sqlccliente.Ciclo
}

// validarVisita confere o formulário contra as regras do negócio. atual é nil numa visita nova.
func (h *handlers) validarVisita(ctx context.Context, r *http.Request, q *sqlccliente.Queries, pr authz.Principal, c views.VisitaCampos, atual *sqlccliente.Visita) (visitaValidada, string) {
	var out visitaValidada
	p := &out.create

	cicloID, err := strconv.ParseInt(c.Ciclo, 10, 64)
	if err != nil {
		return out, "Escolha o ciclo."
	}
	ciclo, err := q.GetCiclo(ctx, cicloID)
	if err != nil {
		return out, "O ciclo escolhido não existe."
	}
	out.ciclo = ciclo
	if atual != nil {
		if antigo, err := q.GetCiclo(ctx, atual.CicloID); err == nil {
			if ok, msg := cicloAceitaEdicao(pr, antigo); !ok {
				return out, msg
			}
		}
	}
	if ok, msg := cicloAceitaEdicao(pr, ciclo); !ok {
		return out, msg
	}
	data, err := time.Parse(formatoDataISO, c.Data)
	if err != nil {
		return out, "A data da visita é inválida."
	}
	if hoje := time.Now().In(h.fuso(r)); data.After(time.Date(hoje.Year(), hoje.Month(), hoje.Day(), 0, 0, 0, 0, time.UTC)) {
		return out, "A data da visita não pode estar no futuro."
	}
	if ok, msg := cicloAceitaData(ciclo, data); !ok {
		return out, msg
	}
	atv, err := strconv.ParseInt(c.Atividade, 10, 16)
	if err != nil {
		return out, "Escolha a atividade."
	}
	doCiclo, err := q.ListAtividadesDoCiclo(ctx, ciclo.ID)
	if err != nil {
		return out, "Não foi possível conferir as atividades do ciclo."
	}
	achou := false
	for _, a := range doCiclo {
		if int64(a) == atv {
			achou = true
		}
	}
	if !achou {
		return out, "Essa atividade não faz parte do ciclo " + ciclo.Nome + "."
	}
	agente, err := strconv.ParseInt(c.Agente, 10, 64)
	if err != nil {
		return out, "Escolha o agente."
	}
	if u, err := q.GetUsuarioByID(ctx, agente); err != nil || (!u.Ativo && !pr.Pode(permissaoAgenteInativo)) {
		return out, "O agente escolhido não existe ou está inativo."
	}
	if !validoEm(rotulosTipoVisita, c.Tipo) {
		return out, "Escolha o tipo da visita."
	}
	if !validoEm(rotulosPendencia, c.Pendencia) {
		return out, "Escolha a pendência."
	}
	if atual != nil && atual.Pendencia != c.Pendencia && !pr.Pode("visita_editar_pendencia") {
		return out, "Você não tem permissão para alterar a pendência da visita."
	}
	var hora pgtype.Time
	if c.Hora != "" {
		t, err := time.Parse("15:04", c.Hora)
		if err != nil {
			return out, "A hora de entrada é inválida."
		}
		hora = pgtype.Time{Microseconds: int64(t.Hour()*3600+t.Minute()*60) * 1e6, Valid: true}
	}

	p.CicloID, p.AtividadeID, p.AgenteID = ciclo.ID, int16(atv), agente
	p.Data = pgtype.Date{Time: data, Valid: true}
	p.HoraEntrada, p.TipoVisita, p.Pendencia = hora, c.Tipo, c.Pendencia
	sem := epi.Calcular(data, h.tipoSemana(r, q))
	p.Semana, p.SemanaAno = pgtype.Int2{Int16: int16(sem.Numero), Valid: true}, pgtype.Int2{Int16: int16(sem.Ano), Valid: true}

	if len([]rune(c.Anotacao)) > 4000 {
		return out, "A anotação passa de 4000 caracteres."
	}
	if !assinaturaValida(c.Assinatura) {
		return out, "A assinatura precisa ser uma imagem PNG em base64 (até 400 KB)."
	}
	p.Anotacao, p.Assinatura = c.Anotacao, c.Assinatura

	// Visita sem trabalho (imóvel fechado, recusa, tentativa) não tem depósitos, amostra nem tratamento.
	if c.Pendencia != "nenhuma" {
		if c.Achou || c.Amostra || c.Tratamento {
			return out, "Uma visita com pendência não pode ter depósitos, amostra nem tratamento."
		}
	} else {
		p.AchouDeposito, p.ColetouAmostra, p.FezTratamento = c.Achou, c.Amostra, c.Tratamento
	}
	var msg string
	ler := func(dest *pgtype.Int4, txt, nome string) {
		if msg != "" {
			return
		}
		*dest, msg = inteiroOpcional(txt, nome, 9999)
	}
	if p.AchouDeposito {
		ler(&p.DepA1, c.A1, "A1")
		ler(&p.DepA2, c.A2, "A2")
		ler(&p.DepB, c.B, "B")
		ler(&p.DepC, c.C, "C")
		ler(&p.DepD1, c.D1, "D1")
		ler(&p.DepD2, c.D2, "D2")
		ler(&p.DepE, c.E, "E")
		ler(&p.TotalEliminado, c.TotalEliminado, "Total eliminado")
		if msg == "" && p.TotalEliminado.Valid {
			soma := int32(0)
			for _, v := range []pgtype.Int4{p.DepA1, p.DepA2, p.DepB, p.DepC, p.DepD1, p.DepD2, p.DepE} {
				soma += v.Int32
			}
			if p.TotalEliminado.Int32 > soma {
				msg = "O total eliminado não pode passar da soma dos depósitos encontrados."
			}
		}
	}
	if p.FezTratamento && msg == "" {
		ler(&p.L1Tratados, c.L1Tratados, "L1: depósitos tratados")
		ler(&p.L2Tratados, c.L2Tratados, "L2: depósitos tratados")
		ler(&p.AdulticidaCargas, c.AdulticidaCargas, "Adulticida (cargas)")
		ler(&p.TratamentoEliminados, c.TratEliminados, "Depósitos eliminados no tratamento")
		if msg == "" {
			if p.L1Gramas, msg = numericoOpcional(c.L1Gramas, "Larvicida L1", 99999); msg == "" {
				if p.L2Gramas, msg = numericoOpcional(c.L2Gramas, "Larvicida L2", 99999); msg == "" {
					p.BriVolume, msg = numericoOpcional(c.Bri, "BRI", 99999)
				}
			}
		}
		if len([]rune(c.AdulticidaTipo)) > 40 {
			msg = "O tipo de adulticida passa de 40 caracteres."
		}
		p.AdulticidaTipo = c.AdulticidaTipo
	}
	if msg != "" {
		return out, msg
	}
	if p.Latitude, msg = floatOpcional(c.Lat, "Latitude", -90, 90); msg != "" {
		return out, msg
	}
	if p.Longitude, msg = floatOpcional(c.Lon, "Longitude", -180, 180); msg != "" {
		return out, msg
	}
	if p.Latitude.Valid != p.Longitude.Valid {
		return out, "Informe a latitude e a longitude juntas, ou nenhuma das duas."
	}
	if p.PrecisaoGps, msg = floatOpcional(c.Precisao, "Precisão do GPS", 0, 100000); msg != "" {
		return out, msg
	}
	return out, ""
}

func dadosAuditoriaVisita(p sqlccliente.CreateVisitaParams, ciclo string) map[string]any {
	n := func(v pgtype.Int4) any {
		if !v.Valid {
			return nil
		}
		return v.Int32
	}
	return map[string]any{
		"data": dataISO(p.Data), "ciclo": ciclo, "atividade": p.AtividadeID, "agente": p.AgenteID, "tipo": p.TipoVisita,
		"pendencia": p.Pendencia, "achou_deposito": p.AchouDeposito, "a1": n(p.DepA1), "a2": n(p.DepA2), "b": n(p.DepB), "c": n(p.DepC),
		"d1": n(p.DepD1), "d2": n(p.DepD2), "e": n(p.DepE), "total_eliminado": n(p.TotalEliminado), "coletou_amostra": p.ColetouAmostra,
		"fez_tratamento": p.FezTratamento, "l1_gramas": textoNumerico(p.L1Gramas), "l1_tratados": n(p.L1Tratados),
		"l2_gramas": textoNumerico(p.L2Gramas), "l2_tratados": n(p.L2Tratados), "adulticida_tipo": p.AdulticidaTipo,
		"adulticida_cargas": n(p.AdulticidaCargas), "tratamento_eliminados": n(p.TratamentoEliminados), "bri_volume": textoNumerico(p.BriVolume),
		"anotacao": p.Anotacao, "assinatura": semConteudoAssinatura(p.Assinatura),
		"latitude": textoFloat(p.Latitude), "longitude": textoFloat(p.Longitude),
	}
}

// semConteudoAssinatura evita levar a imagem inteira para a auditoria.
func semConteudoAssinatura(a string) string {
	if a == "" {
		return ""
	}
	return "assinada"
}

func paramsDaVisita(v sqlccliente.Visita) sqlccliente.CreateVisitaParams {
	return sqlccliente.CreateVisitaParams{
		LocalID: v.LocalID, CicloID: v.CicloID, AtividadeID: v.AtividadeID, AgenteID: v.AgenteID, Data: v.Data, HoraEntrada: v.HoraEntrada,
		TipoVisita: v.TipoVisita, Pendencia: v.Pendencia, Semana: v.Semana, SemanaAno: v.SemanaAno, Anotacao: v.Anotacao, Assinatura: v.Assinatura,
		AchouDeposito: v.AchouDeposito, DepA1: v.DepA1, DepA2: v.DepA2, DepB: v.DepB, DepC: v.DepC, DepD1: v.DepD1, DepD2: v.DepD2, DepE: v.DepE,
		TotalEliminado: v.TotalEliminado, ColetouAmostra: v.ColetouAmostra, FezTratamento: v.FezTratamento, L1Gramas: v.L1Gramas,
		L1Tratados: v.L1Tratados, L2Gramas: v.L2Gramas, L2Tratados: v.L2Tratados, AdulticidaTipo: v.AdulticidaTipo,
		AdulticidaCargas: v.AdulticidaCargas, TratamentoEliminados: v.TratamentoEliminados, BriVolume: v.BriVolume,
		Latitude: v.Latitude, Longitude: v.Longitude, PrecisaoGps: v.PrecisaoGps,
	}
}

// ------------------------------------------------------------------- Salvar

func (h *handlers) visitaSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	var atual *sqlccliente.Visita
	var local sqlccliente.Locai
	if chi.URLParam(r, "id") != "" {
		v, ok := h.carregarVisita(w, r, q)
		if !ok {
			return
		}
		atual = &v
		if local, err = q.GetLocalQualquer(r.Context(), v.LocalID); err != nil {
			h.falhaInterna(w, r, "buscar local", err)
			return
		}
	} else {
		idLocal, _ := strconv.ParseInt(r.PostFormValue("local"), 10, 64)
		var ok bool
		if local, ok = h.localDaVisita(w, r, q, idLocal); !ok {
			return
		}
	}
	campos := lerVisita(r)
	var id int64
	ativo := true
	if atual != nil {
		id, ativo = atual.ID, atual.Ativo
	}
	falha := func(msg string, status int) {
		d, err := h.montarFormVisita(r.Context(), r, q, id, local, campos, ativo, msg)
		if err != nil {
			h.falhaInterna(w, r, "montar formulário da visita", err)
			return
		}
		renderStatus(w, r, status, views.VisitaForm(h.layout(r, "Visita", "/visitas"), d))
	}
	// Quem não tem a permissão de pendência mantém a que a visita já tinha.
	if atual != nil && !h.principal(r).Pode("visita_editar_pendencia") {
		campos.Pendencia = atual.Pendencia
	}
	val, msg := h.validarVisita(r.Context(), r, q, h.principal(r), campos, atual)
	if msg != "" {
		falha(msg, http.StatusBadRequest)
		return
	}
	val.create.LocalID = local.ID

	tx, err := pool.Begin(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(r.Context())
	qt := sqlccliente.New(tx)
	ev := audit.Evento{Entidade: "visita", Rotulo: "Visita em " + enderecoCompleto(local.Endereco, local.Numero)}
	novo := dadosAuditoriaVisita(val.create, val.ciclo.Nome)

	if atual == nil {
		val.create.CriadoPor = pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}
		v, err := qt.CreateVisita(r.Context(), val.create)
		if err != nil {
			h.falhaInterna(w, r, "criar visita", err)
			return
		}
		ev.Acao, ev.EntidadeID, ev.Depois = "visita.criar", strconv.FormatInt(v.ID, 10), novo
		h.auditarCom(audit.Cliente{Q: qt}, r, ev)
		if err := tx.Commit(r.Context()); err != nil {
			h.falhaInterna(w, r, "confirmar visita", err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/visitas/%d?aviso=salva", v.ID), http.StatusSeeOther)
		return
	}
	p := val.create
	if _, err := qt.UpdateVisita(r.Context(), sqlccliente.UpdateVisitaParams{
		ID: atual.ID, CicloID: p.CicloID, AtividadeID: p.AtividadeID, AgenteID: p.AgenteID, Data: p.Data, HoraEntrada: p.HoraEntrada,
		TipoVisita: p.TipoVisita, Pendencia: p.Pendencia, Semana: p.Semana, SemanaAno: p.SemanaAno, Anotacao: p.Anotacao, Assinatura: p.Assinatura,
		AchouDeposito: p.AchouDeposito, DepA1: p.DepA1, DepA2: p.DepA2, DepB: p.DepB, DepC: p.DepC, DepD1: p.DepD1, DepD2: p.DepD2, DepE: p.DepE,
		TotalEliminado: p.TotalEliminado, ColetouAmostra: p.ColetouAmostra, FezTratamento: p.FezTratamento, L1Gramas: p.L1Gramas,
		L1Tratados: p.L1Tratados, L2Gramas: p.L2Gramas, L2Tratados: p.L2Tratados, AdulticidaTipo: p.AdulticidaTipo,
		AdulticidaCargas: p.AdulticidaCargas, TratamentoEliminados: p.TratamentoEliminados, BriVolume: p.BriVolume,
		Latitude: p.Latitude, Longitude: p.Longitude, PrecisaoGps: p.PrecisaoGps,
	}); err != nil {
		h.falhaInterna(w, r, "salvar visita", err)
		return
	}
	nomeAntigo := ""
	if c, err := q.GetCiclo(r.Context(), atual.CicloID); err == nil {
		nomeAntigo = c.Nome
	}
	a, d := audit.Diferenca(dadosAuditoriaVisita(paramsDaVisita(*atual), nomeAntigo), novo)
	if len(a)+len(d) == 0 {
		naoAuditar(r)
	} else {
		ev.Acao, ev.EntidadeID, ev.Antes, ev.Depois = "visita.editar", strconv.FormatInt(atual.ID, 10), a, d
		h.auditarCom(audit.Cliente{Q: qt}, r, ev)
	}
	if err := tx.Commit(r.Context()); err != nil {
		h.falhaInterna(w, r, "confirmar visita", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/visitas/%d?aviso=salva", atual.ID), http.StatusSeeOther)
}

// ------------------------------------------------------------------- Fotos

var tiposDeFoto = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

func (h *handlers) visitaFotoEnviar(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	v, ok := h.carregarVisita(w, r, q)
	if !ok {
		return
	}
	if ok, msg := h.visitaEditavel(r, q, v); !ok {
		http.Error(w, msg, http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFotosPorEnvio*maxFotoVisita+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		h.falhaFotos(w, r, q, v, "O envio é grande demais ou está incompleto (cada foto pode ter até 5 MB).")
		return
	}
	arquivos := r.MultipartForm.File["foto"]
	if len(arquivos) == 0 || len(arquivos) > maxFotosPorEnvio {
		h.falhaFotos(w, r, q, v, "Escolha de 1 a "+strconv.Itoa(maxFotosPorEnvio)+" fotos.")
		return
	}
	atuais, err := q.CountFotosDaVisita(r.Context(), v.ID)
	if err != nil {
		h.falhaInterna(w, r, "contar fotos", err)
		return
	}
	if int(atuais)+len(arquivos) > maxFotosPorVisita {
		h.falhaFotos(w, r, q, v, "Uma visita pode ter no máximo "+strconv.Itoa(maxFotosPorVisita)+" fotos.")
		return
	}
	type pronta struct {
		nome, tipo string
		dados      []byte
	}
	var prontas []pronta
	for _, fh := range arquivos {
		f, err := fh.Open()
		if err != nil {
			h.falhaFotos(w, r, q, v, "Não foi possível ler a foto "+fh.Filename+".")
			return
		}
		dados, err := io.ReadAll(io.LimitReader(f, maxFotoVisita+1))
		f.Close()
		if err != nil || len(dados) == 0 {
			h.falhaFotos(w, r, q, v, "Não foi possível ler a foto "+fh.Filename+".")
			return
		}
		if len(dados) > maxFotoVisita {
			h.falhaFotos(w, r, q, v, "A foto "+fh.Filename+" passa de 5 MB.")
			return
		}
		tipo := http.DetectContentType(dados)
		if !tiposDeFoto[tipo] {
			h.falhaFotos(w, r, q, v, "A foto "+fh.Filename+" não é JPEG, PNG nem WebP.")
			return
		}
		nome := fh.Filename
		if i := strings.LastIndexAny(nome, `/\`); i >= 0 {
			nome = nome[i+1:]
		}
		if rn := []rune(nome); len(rn) > 120 {
			nome = string(rn[:120])
		}
		prontas = append(prontas, pronta{nome, tipo, dados})
	}
	for _, p := range prontas {
		if _, err := q.CreateVisitaFoto(r.Context(), sqlccliente.CreateVisitaFotoParams{
			VisitaID: v.ID, Nome: p.nome, TipoMidia: p.tipo, Conteudo: p.dados, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
		}); err != nil {
			h.falhaInterna(w, r, "gravar foto", err)
			return
		}
	}
	h.auditar(r, audit.Evento{Acao: "visita.foto_adicionar", Entidade: "visita", EntidadeID: strconv.FormatInt(v.ID, 10), Rotulo: "Visita " + strconv.FormatInt(v.ID, 10),
		Depois: map[string]any{"fotos": len(prontas)}})
	http.Redirect(w, r, fmt.Sprintf("/visitas/%d?aviso=foto#fotos", v.ID), http.StatusSeeOther)
}

func (h *handlers) falhaFotos(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, v sqlccliente.Visita, msg string) {
	local, err := q.GetLocalQualquer(r.Context(), v.LocalID)
	if err != nil {
		h.falhaInterna(w, r, "buscar local", err)
		return
	}
	d, err := h.montarFormVisita(r.Context(), r, q, v.ID, local, camposDaVisita(v), v.Ativo, "")
	if err != nil {
		h.falhaInterna(w, r, "montar ficha da visita", err)
		return
	}
	d.ErroFoto = msg
	renderStatus(w, r, http.StatusBadRequest, views.VisitaForm(h.layout(r, "Visita "+strconv.FormatInt(v.ID, 10), "/visitas"), d))
}

// visitaEditavel confere se o ciclo da visita aceita alterações de quem pede.
func (h *handlers) visitaEditavel(r *http.Request, q *sqlccliente.Queries, v sqlccliente.Visita) (bool, string) {
	c, err := q.GetCiclo(r.Context(), v.CicloID)
	if err != nil {
		return true, ""
	}
	return cicloAceitaEdicao(h.principal(r), c)
}

func (h *handlers) visitaFotoVer(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	v, ok := h.carregarVisita(w, r, q)
	if !ok {
		return
	}
	fid, err := strconv.ParseInt(chi.URLParam(r, "fid"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := q.GetFotoDaVisita(r.Context(), sqlccliente.GetFotoDaVisitaParams{ID: fid, VisitaID: v.ID})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", f.TipoMidia)
	w.Header().Set("Content-Length", strconv.Itoa(len(f.Conteudo)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(f.Conteudo)
}

func (h *handlers) visitaFotoExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	v, ok := h.carregarVisita(w, r, q)
	if !ok {
		return
	}
	if ok, msg := h.visitaEditavel(r, q, v); !ok {
		http.Error(w, msg, http.StatusForbidden)
		return
	}
	fid, err := strconv.ParseInt(chi.URLParam(r, "fid"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	n, err := q.SoftDeleteVisitaFoto(r.Context(), sqlccliente.SoftDeleteVisitaFotoParams{ID: fid, Visita: v.ID, Por: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}})
	if err != nil {
		h.falhaInterna(w, r, "excluir foto", err)
		return
	}
	if n == 0 {
		http.NotFound(w, r)
		return
	}
	h.auditar(r, audit.Evento{Acao: "visita.foto_excluir", Entidade: "visita", EntidadeID: strconv.FormatInt(v.ID, 10), Rotulo: "Visita " + strconv.FormatInt(v.ID, 10),
		Antes: map[string]any{"foto": fid}})
	http.Redirect(w, r, fmt.Sprintf("/visitas/%d#fotos", v.ID), http.StatusSeeOther)
}

// ------------------------------------------------------------------- Anotações e exclusão

func (h *handlers) visitaAnotar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	v, ok := h.carregarVisita(w, r, q)
	if !ok {
		return
	}
	texto := strings.TrimSpace(r.PostFormValue("texto"))
	if n := len([]rune(texto)); n < 1 || n > 2000 {
		http.Error(w, "A anotação precisa ter de 1 a 2000 caracteres.", http.StatusBadRequest)
		return
	}
	if _, err := q.CreateVisitaAnotacao(r.Context(), sqlccliente.CreateVisitaAnotacaoParams{
		VisitaID: v.ID, UsuarioID: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, Texto: texto,
	}); err != nil {
		h.falhaInterna(w, r, "gravar anotação", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "visita.anotacao", Entidade: "visita", EntidadeID: strconv.FormatInt(v.ID, 10), Rotulo: "Visita " + strconv.FormatInt(v.ID, 10),
		Depois: map[string]any{"anotacao": texto}})
	http.Redirect(w, r, fmt.Sprintf("/visitas/%d?aviso=anotacao#anotacoes", v.ID), http.StatusSeeOther)
}

func (h *handlers) visitaExcluir(w http.ResponseWriter, r *http.Request) {
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
	v, ok := h.carregarVisita(w, r, q)
	if !ok {
		return
	}
	if ok, msg := h.visitaEditavel(r, q, v); !ok {
		http.Error(w, msg, http.StatusForbidden)
		return
	}
	h.excluirRegistro(w, r, "visita", id, "/visitas?aviso=excluido", fmt.Sprintf("/visitas/%d", id))
}

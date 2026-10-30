package server

import (
	"net/http"
	"strconv"
	"time"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/authz"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/epi"
)

const chaveSemana = "semana_epidemiologica"

// permissaoCicloFechado deixa mexer em registros de um ciclo fechado (a lista de visitas, na Etapa 6, usa a mesma).
const permissaoCicloFechado = "visita_editar_ciclo_fechado"

// permissaoAgenteInativo não existe no catálogo: só o importador a concede a si mesmo, para trazer visitas antigas
// de agentes que já foram desativados.
const permissaoAgenteInativo = "visita_agente_inativo"

// tipoSemana devolve o tipo de semana epidemiológica do cliente: "sinan" (padrão, domingo a sábado) ou "iso".
func (h *handlers) tipoSemana(r *http.Request, q *sqlccliente.Queries) string {
	if info, err := q.GetClienteInfo(r.Context(), chaveSemana); err == nil && epi.Valido(info.Valor) {
		return info.Valor
	}
	return epi.SINAN
}

func rotuloDoTipoSemana(t string) string {
	if t == epi.ISO {
		return "ISO 8601 (segunda a domingo)"
	}
	return "SINAN / Ministério da Saúde (domingo a sábado)"
}

// textoDaSemana escreve a semana epidemiológica da data, como "SE 44/2026".
func textoDaSemana(d time.Time, tipo string) string {
	s := epi.Calcular(d, tipo)
	return "SE " + strconv.Itoa(s.Numero) + "/" + strconv.Itoa(s.Ano)
}

// cicloAceitaEdicao diz se quem tem p pode alterar registros do ciclo: ciclo fechado só com a permissão própria.
func cicloAceitaEdicao(p authz.Principal, c sqlccliente.Ciclo) (bool, string) {
	if c.Situacao == "fechado" && !p.Pode(permissaoCicloFechado) {
		return false, "O ciclo " + c.Nome + " está fechado: só quem tem a permissão de editar em ciclo fechado pode alterar os registros dele."
	}
	return true, ""
}

// cicloAceitaData diz se a data cabe no período do ciclo (ou se o ciclo permite registros fora do período).
func cicloAceitaData(c sqlccliente.Ciclo, d time.Time) (bool, string) {
	dia := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	if c.PermiteForaPeriodo || (!dia.Before(c.DataInicio.Time) && !dia.After(c.DataFinal.Time)) {
		return true, ""
	}
	return false, "A data " + d.Format(formatoData) + " está fora do período do ciclo " + c.Nome + " (" + dataBR(c.DataInicio) + " a " + dataBR(c.DataFinal) + ")."
}

// ciclosSemanaSalvar salva o tipo de semana epidemiológica do cliente.
func (h *handlers) ciclosSemanaSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	novo := r.PostFormValue("tipo")
	if !epi.Valido(novo) {
		http.Error(w, "Tipo de semana inválido.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	antes := h.tipoSemana(r, q)
	if antes == novo {
		naoAuditar(r)
		http.Redirect(w, r, "/ciclos", http.StatusSeeOther)
		return
	}
	if err := q.SetClienteInfo(r.Context(), sqlccliente.SetClienteInfoParams{Chave: chaveSemana, Valor: novo}); err != nil {
		h.falhaInterna(w, r, "salvar tipo de semana", err)
		return
	}
	h.auditar(r, audit.Evento{
		Acao: "parametro.semana", Entidade: "parametro", EntidadeID: chaveSemana, Rotulo: "Semana epidemiológica",
		Antes: map[string]any{"tipo": rotuloDoTipoSemana(antes)}, Depois: map[string]any{"tipo": rotuloDoTipoSemana(novo)},
	})
	http.Redirect(w, r, "/ciclos", http.StatusSeeOther)
}

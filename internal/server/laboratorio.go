package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

const permissaoLabCicloFechado = "visita_laboratorio_ciclo_fechado"

var (
	rotulosDepositoAmostra = []views.CidadeOpcao{
		{Valor: "A1", Rotulo: "A1"}, {Valor: "A2", Rotulo: "A2"}, {Valor: "B", Rotulo: "B"}, {Valor: "C", Rotulo: "C"},
		{Valor: "D1", Rotulo: "D1"}, {Valor: "D2", Rotulo: "D2"}, {Valor: "E", Rotulo: "E"},
	}
	rotulosStatusAmostra = []views.CidadeOpcao{
		{Valor: "em_analise", Rotulo: "Em análise"},
		{Valor: "concluido", Rotulo: "Concluído"},
	}
)

// tubitoAutomatico diz se o número do tubito é gerado pelo sistema (parâmetro do cliente).
func (h *handlers) tubitoAutomatico(r *http.Request, q *sqlccliente.Queries) bool {
	for _, p := range parametros {
		if p.Chave == "visita_tubito_automatico" {
			return h.valorDoParametro(r, q, p) == "1"
		}
	}
	return true
}

// labCicloAceita confere se o ciclo aceita que quem pede altere resultados de laboratório.
func (h *handlers) labCicloAceita(r *http.Request, q *sqlccliente.Queries, v sqlccliente.Visita) (bool, string) {
	c, err := q.GetCiclo(r.Context(), v.CicloID)
	if err != nil {
		return true, ""
	}
	if c.Situacao == "fechado" && !h.principal(r).Pode(permissaoLabCicloFechado) {
		return false, "O ciclo " + c.Nome + " está fechado: só quem tem a permissão de alterar resultados em ciclo fechado pode mexer nas amostras dele."
	}
	return true, ""
}

func inteiroEntre(s, nome string, max int) (int, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ""
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > max {
		return 0, fmt.Sprintf("%s: informe um número inteiro de 0 a %d.", nome, max)
	}
	return n, ""
}

// amostrasDaVisita monta as linhas da seção "Amostras" da ficha da visita.
func amostrasDaVisita(lista []sqlccliente.Amostra) []views.AmostraLinha {
	out := make([]views.AmostraLinha, len(lista))
	for i, a := range lista {
		out[i] = views.AmostraLinha{
			ID: a.ID, Tubito: a.NumeroTubito, Deposito: a.Deposito, Larvas: int(a.Larvas), Pupas: int(a.Pupas), Observacao: a.Observacao,
			Status: a.Status, StatusRotulo: rotuloDe(rotulosStatusAmostra, a.Status), Aegypti: int(a.TotalAegypti),
			Albopictus: int(a.TotalAlbopictus), Negativo: int(a.TotalNegativo), Depositos: marcarOpcao(rotulosDepositoAmostra, a.Deposito),
			Situacoes: marcarOpcao(rotulosStatusAmostra, a.Status),
		}
	}
	return out
}

func (h *handlers) falhaAmostra(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, v sqlccliente.Visita, msg string) {
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
	d.ErroAmostra = msg
	renderStatus(w, r, http.StatusBadRequest, views.VisitaForm(h.layout(r, "Visita "+strconv.FormatInt(v.ID, 10), "/visitas"), d))
}

func (h *handlers) auditarAmostra(r *http.Request, acao string, a sqlccliente.Amostra, antes, depois map[string]any) {
	h.auditar(r, audit.Evento{
		Acao: acao, Entidade: "amostra", EntidadeID: strconv.FormatInt(a.ID, 10), Rotulo: "Tubito " + a.NumeroTubito,
		Detalhes: map[string]any{"visita": a.VisitaID}, Antes: antes, Depois: depois,
	})
}

// amostraDaURL carrega a amostra de /visitas/{id}/amostras/{aid} e confere que ela é da visita.
func (h *handlers) amostraDaURL(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, v sqlccliente.Visita) (sqlccliente.Amostra, bool) {
	aid, err := strconv.ParseInt(chi.URLParam(r, "aid"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return sqlccliente.Amostra{}, false
	}
	a, err := q.GetAmostra(r.Context(), aid)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && a.VisitaID != v.ID) {
		http.NotFound(w, r)
		return a, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar amostra", err)
		return a, false
	}
	return a, true
}

// coletaDoForm lê e valida os campos da coleta (depósito, larvas, pupas, observação).
func coletaDoForm(r *http.Request) (dep string, larvas, pupas int, obs, erro string) {
	f := func(n string) string { return strings.TrimSpace(r.PostFormValue(n)) }
	dep, obs = f("deposito"), f("observacao")
	if dep != "" && !validoEm(rotulosDepositoAmostra, dep) {
		return "", 0, 0, "", "Escolha um depósito válido."
	}
	larvas, erro = inteiroEntre(f("larvas"), "Larvas", 9999)
	if erro == "" {
		pupas, erro = inteiroEntre(f("pupas"), "Pupas", 9999)
	}
	if erro == "" && len([]rune(obs)) > 1000 {
		erro = "A observação passa de 1000 caracteres."
	}
	return
}

// amostraCriar registra um tubito na visita (número automático ou digitado, conforme o parâmetro).
func (h *handlers) amostraCriar(w http.ResponseWriter, r *http.Request) {
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
	if !v.ColetouAmostra {
		h.falhaAmostra(w, r, q, v, "Esta visita não está marcada como \"coletou amostra\": marque isso na visita antes de registrar tubitos.")
		return
	}
	dep, larvas, pupas, obs, msg := coletaDoForm(r)
	if msg != "" {
		h.falhaAmostra(w, r, q, v, msg)
		return
	}
	automatico := h.tubitoAutomatico(r, q)
	digitado := strings.TrimSpace(r.PostFormValue("tubito"))
	if !automatico && (digitado == "" || len([]rune(digitado)) > 30) {
		h.falhaAmostra(w, r, q, v, "Informe o número do tubito (até 30 caracteres).")
		return
	}
	var criada sqlccliente.Amostra
	// Numeração automática: tenta de novo se outra pessoa pegou o mesmo número ao mesmo tempo.
	for tentativa := 0; tentativa < 5 && criada.ID == 0; tentativa++ {
		numero := digitado
		if automatico {
			if numero, err = q.ProximoNumeroTubito(r.Context()); err != nil {
				h.falhaInterna(w, r, "próximo tubito", err)
				return
			}
		}
		a, err := q.CreateAmostra(r.Context(), sqlccliente.CreateAmostraParams{
			VisitaID: v.ID, NumeroTubito: numero, Deposito: dep, Larvas: int32(larvas), Pupas: int32(pupas), Observacao: obs, CriadoPor: porDoRequest(r),
		})
		switch {
		case err == nil:
			criada = a
		case violacaoUnica(err) && !automatico:
			h.falhaAmostra(w, r, q, v, "Já existe uma amostra com o tubito "+numero+".")
			return
		case !violacaoUnica(err):
			h.falhaInterna(w, r, "gravar amostra", err)
			return
		}
	}
	if criada.ID == 0 {
		h.falhaInterna(w, r, "gravar amostra", errors.New("numeração concorrente"))
		return
	}
	h.auditarAmostra(r, "amostra.criar", criada, nil, map[string]any{
		"tubito": criada.NumeroTubito, "deposito": criada.Deposito, "larvas": criada.Larvas, "pupas": criada.Pupas,
	})
	http.Redirect(w, r, fmt.Sprintf("/visitas/%d?aviso=amostra#amostras", v.ID), http.StatusSeeOther)
}

// amostraColeta corrige os dados da coleta (depósito, larvas, pupas, observação).
func (h *handlers) amostraColeta(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	v, ok := h.carregarVisita(w, r, q)
	if !ok {
		return
	}
	a, ok := h.amostraDaURL(w, r, q, v)
	if !ok {
		return
	}
	if ok, msg := h.visitaEditavel(r, q, v); !ok {
		http.Error(w, msg, http.StatusForbidden)
		return
	}
	dep, larvas, pupas, obs, msg := coletaDoForm(r)
	if msg != "" {
		h.falhaAmostra(w, r, q, v, msg)
		return
	}
	nova, err := q.UpdateAmostraColeta(r.Context(), sqlccliente.UpdateAmostraColetaParams{
		ID: a.ID, Deposito: dep, Larvas: int32(larvas), Pupas: int32(pupas), Observacao: obs,
	})
	if err != nil {
		h.falhaInterna(w, r, "gravar amostra", err)
		return
	}
	h.auditarAmostra(r, "amostra.editar", nova,
		map[string]any{"deposito": a.Deposito, "larvas": a.Larvas, "pupas": a.Pupas},
		map[string]any{"deposito": nova.Deposito, "larvas": nova.Larvas, "pupas": nova.Pupas})
	http.Redirect(w, r, fmt.Sprintf("/visitas/%d?aviso=amostra#amostras", v.ID), http.StatusSeeOther)
}

// amostraResultado grava o resultado da análise (status e totais); permissão própria do laboratório.
func (h *handlers) amostraResultado(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	v, ok := h.carregarVisita(w, r, q)
	if !ok {
		return
	}
	a, ok := h.amostraDaURL(w, r, q, v)
	if !ok {
		return
	}
	if ok, msg := h.labCicloAceita(r, q, v); !ok {
		http.Error(w, msg, http.StatusForbidden)
		return
	}
	f := func(n string) string { return strings.TrimSpace(r.PostFormValue(n)) }
	status := f("status")
	if !validoEm(rotulosStatusAmostra, status) {
		h.falhaAmostra(w, r, q, v, "Escolha o status da amostra.")
		return
	}
	aeg, m1 := inteiroEntre(f("aegypti"), "Total Aedes aegypti", 99999)
	alb, m2 := inteiroEntre(f("albopictus"), "Total Aedes albopictus", 99999)
	neg, m3 := inteiroEntre(f("negativo"), "Total negativo", 99999)
	for _, m := range []string{m1, m2, m3} {
		if m != "" {
			h.falhaAmostra(w, r, q, v, m)
			return
		}
	}
	nova, err := q.UpdateAmostraResultado(r.Context(), sqlccliente.UpdateAmostraResultadoParams{
		ID: a.ID, Status: status, TotalAegypti: int32(aeg), TotalAlbopictus: int32(alb), TotalNegativo: int32(neg),
		ResultadoPor: porDoRequest(r),
	})
	if err != nil {
		h.falhaInterna(w, r, "gravar resultado", err)
		return
	}
	h.auditarAmostra(r, "amostra.resultado", nova,
		map[string]any{"status": a.Status, "aegypti": a.TotalAegypti, "albopictus": a.TotalAlbopictus, "negativo": a.TotalNegativo},
		map[string]any{"status": nova.Status, "aegypti": nova.TotalAegypti, "albopictus": nova.TotalAlbopictus, "negativo": nova.TotalNegativo})
	http.Redirect(w, r, fmt.Sprintf("/visitas/%d?aviso=resultado#amostras", v.ID), http.StatusSeeOther)
}

// amostraExcluir envia o tubito para a Lixeira.
func (h *handlers) amostraExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	v, ok := h.carregarVisita(w, r, q)
	if !ok {
		return
	}
	a, ok := h.amostraDaURL(w, r, q, v)
	if !ok {
		return
	}
	if ok, msg := h.visitaEditavel(r, q, v); !ok {
		http.Error(w, msg, http.StatusForbidden)
		return
	}
	h.excluirRegistro(w, r, "amostra", a.ID, fmt.Sprintf("/visitas/%d?aviso=amostra-excluida#amostras", v.ID), fmt.Sprintf("/visitas/%d#amostras", v.ID))
}

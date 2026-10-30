package server

import (
	"net/http"
	"strconv"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// dashboard mostra o resumo do ciclo: visitas, depósitos, pendências, amostras, produção por semana e por agente.
// Quem só vê as próprias visitas (visita_visualizar_minha) enxerga só os números dela.
func (h *handlers) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	ciclos, err := q.ListCiclos(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar ciclos", err)
		return
	}
	// Ciclo escolhido na URL; sem escolha, o ciclo em andamento (ou o mais recente). "todos" soma tudo.
	escolha := r.URL.Query().Get("ciclo")
	ciclo := idOpcional(escolha)
	if escolha == "" {
		if atual, err := q.DashboardCicloAtual(ctx); err == nil {
			ciclo = idOpcional(strconv.FormatInt(atual.ID, 10))
		}
	}
	d := views.DashboardData{Todos: !ciclo.Valid}
	for _, c := range ciclos {
		d.Ciclos = append(d.Ciclos, views.CidadeOpcao{ID: c.ID, Rotulo: c.Nome, Selecionada: ciclo.Valid && c.ID == ciclo.Int64})
	}
	somente := h.escopoVisitas(r)

	res, err := q.DashboardResumo(ctx, sqlccliente.DashboardResumoParams{CicloID: ciclo, SomenteAgente: somente})
	if err != nil {
		h.falhaInterna(w, r, "resumo do dashboard", err)
		return
	}
	d.Visitas, d.Locais, d.ComDeposito, d.Fechadas, d.Recusadas = int(res.Visitas), int(res.Locais), int(res.ComDeposito), int(res.Fechadas), int(res.Recusadas)
	d.Tentativas, d.ComAmostra, d.ComTratamento, d.Agentes = int(res.Tentativas), int(res.ComAmostra), int(res.ComTratamento), int(res.Agentes)

	semanas, err := q.DashboardPorSemana(ctx, sqlccliente.DashboardPorSemanaParams{CicloID: ciclo, SomenteAgente: somente})
	if err != nil {
		h.falhaInterna(w, r, "semanas do dashboard", err)
		return
	}
	for _, s := range semanas {
		d.Semanas = append(d.Semanas, views.BarraDash{Rotulo: "S" + strconv.Itoa(int(s.Semana)), Valor: int(s.Visitas), Destaque: int(s.ComDeposito)})
	}
	agentes, err := q.DashboardPorAgente(ctx, sqlccliente.DashboardPorAgenteParams{CicloID: ciclo, SomenteAgente: somente})
	if err != nil {
		h.falhaInterna(w, r, "agentes do dashboard", err)
		return
	}
	for _, a := range agentes {
		d.PorAgente = append(d.PorAgente, views.BarraDash{Rotulo: a.Agente, Valor: int(a.Visitas), Destaque: int(a.ComDeposito)})
	}
	amostras, err := q.DashboardAmostrasPorStatus(ctx, sqlccliente.DashboardAmostrasPorStatusParams{CicloID: ciclo, SomenteAgente: somente})
	if err != nil {
		h.falhaInterna(w, r, "amostras do dashboard", err)
		return
	}
	for _, a := range amostras {
		d.Amostras = append(d.Amostras, views.BarraDash{Rotulo: rotuloDe(rotulosStatusAmostra, a.Status), Valor: int(a.Total)})
	}
	render(w, r, views.Dashboard(h.layout(r, "Dashboard", "/dashboard"), d))
}

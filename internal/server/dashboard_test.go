package server

import (
	"net/http"
	"testing"
)

func TestDashboardResumoDoCiclo(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)

	// Sem visitas: a tela abre com zeros e avisa que não há dados.
	quer(t, ana.get("/dashboard"), http.StatusOK, "Dashboard", "Sem dados no período", "01 Ciclo")

	quer(t, ana.post("/visitas", c.form(agente, "achou", "1", "a1", "2", "total_eliminado", "2")), http.StatusSeeOther)
	quer(t, ana.post("/visitas", c.form(agente, "pendencia", "fechado")), http.StatusSeeOther)
	quer(t, ana.post("/visitas", c.form(agente, "amostra", "1")), http.StatusSeeOther)

	corpo := ana.get("/dashboard?ciclo=" + s64(c.ciclo))
	quer(t, corpo, http.StatusOK, "Visitas por semana", "Visitas por agente", "1 fechadas", "1 locais visitados")
	quer(t, ana.get("/dashboard?ciclo=todos"), http.StatusOK, "Todos os ciclos")

	// Quem não vê visitas não abre o dashboard.
	sem := m.usuarioCom(t, "Sem Visitas", "sem@nm.gov.br", "bairro_visualizar")
	quer(t, sem.get("/dashboard"), http.StatusForbidden)
}

package server

import (
	"net/http"
	"testing"
)

func TestRelatorioVisitas(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)

	quer(t, ana.get("/relatorios"), http.StatusOK, "Visitas")
	quer(t, ana.get("/relatorios/visitas"), http.StatusOK, "Gerar relatório", "Baixar PDF")
	quer(t, ana.get("/relatorios/inexistente"), http.StatusNotFound)

	quer(t, ana.post("/visitas", c.form(agente, "achou", "1", "a1", "2", "total_eliminado", "2")), http.StatusSeeOther)
	quer(t, ana.post("/visitas", c.form(agente, "pendencia", "fechado")), http.StatusSeeOther)

	quer(t, ana.get("/relatorios/visitas?gerar=1&ciclo="+s64(c.ciclo)), http.StatusOK, "Total: 2 visitas, 1 com depósito")
	quer(t, ana.get("/relatorios/visitas?gerar=1&pendencia=fechado"), http.StatusOK, "Total: 1 visitas")
	quer(t, ana.get("/relatorios/visitas?gerar=1&de=2999-01-01"), http.StatusOK, "Nenhum registro")

	pdf := ana.get("/relatorios/visitas?formato=pdf")
	quer(t, pdf, http.StatusOK)
	if got := pdf.Header().Get("Content-Type"); got != "application/pdf" || pdf.Body.Len() < 500 || pdf.Body.String()[:4] != "%PDF" {
		t.Fatalf("PDF inválido: %s", got)
	}
	csv := ana.get("/relatorios/visitas?formato=csv")
	quer(t, csv, http.StatusOK, "Endereço;Agente")
	x := ana.get("/relatorios/visitas?formato=xlsx")
	quer(t, x, http.StatusOK)
	quer(t, ana.get("/relatorios/visitas?formato=doc"), http.StatusBadRequest)

	sem := m.usuarioCom(t, "Sem Rel", "semrel@nm.gov.br", "visita_visualizar")
	quer(t, sem.get("/relatorios/visitas"), http.StatusNotFound)
	quer(t, sem.get("/relatorios"), http.StatusOK, "nenhum relatório")
}

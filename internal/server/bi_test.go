package server

import (
	"net/http"
	"net/url"
	"testing"
)

func TestBiIndicesPorBairro(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)

	quer(t, ana.get("/bi"), http.StatusOK, "Nenhuma visita sem pendência")

	// Uma visita com tubito positivo, outra negativa e uma fechada (não conta como pesquisada).
	quer(t, ana.post("/visitas", c.form(agente, "amostra", "1")), http.StatusSeeOther)
	quer(t, ana.post("/visitas", c.form(agente, "pendencia", "fechado")), http.StatusSeeOther)
	lista, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	var id string
	for _, v := range lista {
		if v.ColetouAmostra {
			id = s64(v.ID)
		}
	}
	quer(t, ana.post("/visitas/"+id+"/amostras", url.Values{"deposito": {"A2"}, "larvas": {"4"}}), http.StatusSeeOther)
	as, _ := m.cliQ.ListAmostras(t.Context(), listarAmostras())
	quer(t, ana.post("/visitas/"+id+"/amostras/"+s64(as[0].ID)+"/resultado",
		url.Values{"status": {"concluido"}, "aegypti": {"3"}}), http.StatusSeeOther)

	quer(t, ana.get("/bi?ciclo="+s64(c.ciclo)), http.StatusOK, "IIP", "Breteau", "100.0%", `id="bi-dados"`, "bi-depositos", "Risco de surto")
	quer(t, ana.get("/bi?ciclo=todos&atividade=1"), http.StatusOK, "Cobertura")

	sem := m.usuarioCom(t, "Sem BI", "sembi@nm.gov.br", "visita_visualizar")
	quer(t, sem.get("/bi"), http.StatusForbidden)
}

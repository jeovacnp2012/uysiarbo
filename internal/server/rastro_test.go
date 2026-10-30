package server

import (
	"net/http"
	"testing"
)

func TestRastroGPS(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)

	quer(t, ana.get("/mapa/rastro"), http.StatusOK, "Rastro GPS", "01 Ciclo")
	quer(t, ana.get("/mapa/rastro.geojson"), http.StatusOK, `"pontos":0`)

	quer(t, ana.post("/visitas", c.form(agente, "lat", "-13.83", "lon", "-56.08", "precisao", "8")), http.StatusSeeOther)
	quer(t, ana.post("/visitas", c.form(agente, "lat", "-13.84", "lon", "-56.09")), http.StatusSeeOther)
	quer(t, ana.post("/visitas", c.form(agente)), http.StatusSeeOther)
	quer(t, ana.get("/mapa/rastro.geojson?ciclo="+s64(c.ciclo)), http.StatusOK, `"pontos":2`, `"LineString"`, `"Point"`)
	quer(t, ana.get("/mapa/rastro.geojson?de=2999-01-01"), http.StatusOK, `"pontos":0`)

	sem := m.usuarioCom(t, "Sem GPS", "semgps@nm.gov.br", "visita_visualizar")
	quer(t, sem.get("/mapa/rastro"), http.StatusForbidden)
}

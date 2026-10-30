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
	if _, err := m.pool.Exec(t.Context(), "UPDATE c_novamutum.locais SET latitude = -13.8, longitude = -56.1"); err != nil {
		t.Fatal(err)
	}
	// Sem GPS, só aparece a posição do local quando se pede.
	quer(t, ana.get("/mapa/rastro.geojson?ciclo="+s64(c.ciclo)+"&origem=local"), http.StatusOK, `"pontos":3`, `"do_local":true`)
	quer(t, ana.get("/mapa/rastro.geojson?de=2999-01-01"), http.StatusOK, `"pontos":0`)

	sem := m.usuarioCom(t, "Sem GPS", "semgps@nm.gov.br", "visita_visualizar")
	quer(t, sem.get("/mapa/rastro"), http.StatusForbidden)
}

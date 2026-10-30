package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

const limiteRastro = 5000

// rastroPagina mostra no mapa o caminho percorrido pelos agentes: os pontos de GPS gravados nas visitas feitas
// pelo aplicativo, ligados em ordem de data e hora, uma linha por agente e dia.
func (h *handlers) rastroPagina(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	d := views.RastroData{TilesURL: h.mapaTiles, Credito: h.mapaCredito, De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate")), Origem: qs.Get("origem")}
	ciclo, agente := idOpcional(qs.Get("ciclo")), idOpcional(qs.Get("agente"))
	ciclos, err := q.ListCiclos(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar ciclos", err)
		return
	}
	for _, c := range ciclos {
		d.Ciclos = append(d.Ciclos, views.CidadeOpcao{ID: c.ID, Rotulo: c.Nome, Selecionada: c.ID == ciclo.Int64})
	}
	us, err := q.ListUsuarios(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar agentes", err)
		return
	}
	for _, u := range us {
		d.Agentes = append(d.Agentes, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome, Selecionada: u.ID == agente.Int64})
	}
	render(w, r, views.Rastro(h.layout(r, "Rastro GPS", "/mapa/rastro"), d))
}

// rastroGeoJSON devolve os pontos (visitas com GPS) e as linhas por agente e dia.
func (h *handlers) rastroGeoJSON(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	rows, err := q.ListRastroGPS(r.Context(), sqlccliente.ListRastroGPSParams{
		CicloID: idOpcional(qs.Get("ciclo")), AgenteID: idOpcional(qs.Get("agente")),
		DataDe: dataOpcional(strings.TrimSpace(qs.Get("de"))), DataAte: dataOpcional(strings.TrimSpace(qs.Get("ate"))),
		SomenteAgente: h.escopoVisitas(r), UsarLocal: qs.Get("origem") == "local",
	})
	if err != nil {
		h.falhaInterna(w, r, "rastro GPS", err)
		return
	}
	truncado := len(rows) > limiteRastro
	if truncado {
		rows = rows[:limiteRastro]
	}
	features := []map[string]any{}
	linhas := map[string][][2]float64{}
	var ordem []string
	for _, v := range rows {
		hora := ""
		if v.HoraEntrada.Valid {
			min := v.HoraEntrada.Microseconds / 60_000_000
			hora = fmt.Sprintf("%02d:%02d", min/60, min%60)
		}
		data := dataBR(v.Data)
		pt := [2]float64{v.Longitude, v.Latitude}
		features = append(features, map[string]any{
			"type": "Feature", "geometry": map[string]any{"type": "Point", "coordinates": pt},
			"properties": map[string]any{
				"id": v.ID, "agente": v.Agente, "data": data, "hora": hora, "precisao": v.Precisao,
				"deposito": v.AchouDeposito, "do_local": v.DoLocal, "pendencia": v.Pendencia, "endereco": enderecoCompleto(v.Endereco, v.Numero),
			},
		})
		chave := fmt.Sprintf("%d|%s|%s", v.AgenteID, data, v.Agente)
		if _, ok := linhas[chave]; !ok {
			ordem = append(ordem, chave)
		}
		linhas[chave] = append(linhas[chave], pt)
	}
	for _, chave := range ordem {
		if pts := linhas[chave]; len(pts) > 1 {
			partes := strings.SplitN(chave, "|", 3)
			features = append(features, map[string]any{
				"type": "Feature", "geometry": map[string]any{"type": "LineString", "coordinates": pts},
				"properties": map[string]any{"linha": true, "agente": partes[2], "data": partes[1]},
			})
		}
	}
	w.Header().Set("Content-Type", "application/geo+json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "FeatureCollection", "features": features, "truncado": truncado, "pontos": len(rows)})
}

package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"uysiarbo/internal/geo"
	"uysiarbo/web/views"
)

// escopoDoMapa devolve as restrições do papel para bairros/quarteirões e para os locais.
func (h *handlers) escopoDoMapa(r *http.Request) (bairros, locais geo.Escopo) {
	if s := h.somenteUsuario(r); s.Valid {
		v := s.Int64
		bairros.Somente = &v
	}
	somente, soPE := h.escopoLocais(r)
	if somente.Valid {
		v := somente.Int64
		locais.Somente = &v
	}
	locais.SoPE = soPE
	locais.VerDesativados = h.principal(r).Pode("local_ver_desativados")
	return bairros, locais
}

// caixaDoPedido lê ?bbox=oeste,sul,leste,norte; sem ele (ou inválido) vale o mundo todo.
func caixaDoPedido(r *http.Request) geo.Caixa {
	mundo := geo.Caixa{O: -180, S: -90, L: 180, N: 90}
	partes := strings.Split(r.URL.Query().Get("bbox"), ",")
	if len(partes) != 4 {
		return mundo
	}
	var v [4]float64
	for i, p := range partes {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return mundo
		}
		v[i] = f
	}
	c := geo.Caixa{O: v[0], S: v[1], L: v[2], N: v[3]}
	if c.O < -180 || c.L > 180 || c.S < -90 || c.N > 90 || c.O >= c.L || c.S >= c.N {
		return mundo
	}
	return c
}

func idDoPedido(r *http.Request, nome string) int64 {
	n, _ := strconv.ParseInt(r.URL.Query().Get(nome), 10, 64)
	if n < 0 {
		return 0
	}
	return n
}

func (h *handlers) mapaPagina(w http.ResponseWriter, r *http.Request) {
	pr := h.principal(r)
	camadaBairros, camadaLocais := pr.Pode("bairro_visualizar"), pr.Pode("local_visualizar")
	if !camadaBairros && !camadaLocais {
		renderStatus(w, r, http.StatusForbidden, views.SemPermissao(h.layout(r, "Sem permissão", "")))
		return
	}
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := views.MapaData{
		TilesURL: h.mapaTiles, Credito: h.mapaCredito, CamadaBairros: camadaBairros, CamadaLocais: camadaLocais,
		Bairro: idDoPedido(r, "bairro"), Grupo: idDoPedido(r, "grupo"), Local: idDoPedido(r, "local"), Tipo: r.URL.Query().Get("tipo"),
	}
	somente, _ := h.escopoLocais(r)
	bairros, err := q.ListBairrosAtivosParaLocal(ctx, somente)
	if err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	for _, b := range bairros {
		d.Bairros = append(d.Bairros, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome, Selecionada: b.ID == d.Bairro})
	}
	if pr.Pode("local_grupo_visualizar") {
		gs, err := q.ListGruposParaEscolher(ctx)
		if err != nil {
			h.falhaInterna(w, r, "listar grupos", err)
			return
		}
		for _, g := range gs {
			d.Grupos = append(d.Grupos, views.CidadeOpcao{ID: g.ID, Rotulo: g.Nome, Selecionada: g.ID == d.Grupo})
		}
	} else {
		d.Grupo = 0
	}
	d.Tipos = opcoesTipo(d.Tipo)
	render(w, r, views.Mapa(h.layout(r, "Mapa", "/mapa"), d))
}

func (h *handlers) escreverGeoJSON(w http.ResponseWriter, r *http.Request, col geo.Colecao) {
	w.Header().Set("Content-Type", "application/geo+json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	if err := json.NewEncoder(w).Encode(col); err != nil {
		h.log.Error("escrever geojson", "erro", err)
	}
}

func (h *handlers) mapaBairros(w http.ResponseWriter, r *http.Request) {
	_, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	esc, _ := h.escopoDoMapa(r)
	col, err := geo.BairrosGeoJSON(r.Context(), pool, esc, idDoPedido(r, "bairro"))
	if err != nil {
		h.falhaInterna(w, r, "bairros do mapa", err)
		return
	}
	h.escreverGeoJSON(w, r, col)
}

func (h *handlers) mapaQuarteiroes(w http.ResponseWriter, r *http.Request) {
	_, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	esc, _ := h.escopoDoMapa(r)
	col, err := geo.QuarteiroesGeoJSON(r.Context(), pool, esc, caixaDoPedido(r), idDoPedido(r, "bairro"))
	if err != nil {
		h.falhaInterna(w, r, "quarteirões do mapa", err)
		return
	}
	h.escreverGeoJSON(w, r, col)
}

func (h *handlers) mapaLocais(w http.ResponseWriter, r *http.Request) {
	_, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	_, esc := h.escopoDoMapa(r)
	tipo := r.URL.Query().Get("tipo")
	if tipo != "" && rotuloTipoLocal(tipo) == tipo {
		tipo = "" // tipo desconhecido: ignora o filtro
	}
	grupo := idDoPedido(r, "grupo")
	if !h.principal(r).Pode("local_grupo_visualizar") {
		grupo = 0
	}
	col, err := geo.LocaisGeoJSON(r.Context(), pool, esc, caixaDoPedido(r), geo.FiltroDeLocais{
		BairroID: idDoPedido(r, "bairro"), GrupoID: grupo, Tipo: tipo, LocalID: idDoPedido(r, "local"),
	})
	if err != nil {
		h.falhaInterna(w, r, "locais do mapa", err)
		return
	}
	h.escreverGeoJSON(w, r, col)
}

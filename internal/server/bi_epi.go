package server

import (
	"net/http"
	"strconv"
	"time"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/epi"
	"uysiarbo/web/views"
)

// biEpiPagina é o B.I. Epidemiológico: as investigações do período por doença, bairro e semana
// epidemiológica (pela data de início). Confirmado = resultado positivo; descartado = negativo;
// taxa de confirmação = confirmados ÷ casos com resultado definido (positivo, negativo ou inconclusivo).
// O período padrão é o ano corrente até hoje. Investigações sem data de início ficam de fora do período.
// O bairro só existe quando a investigação está ligada a um local; as demais aparecem como "Sem bairro".
func (h *handlers) biEpiPagina(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	hoje := time.Now().In(h.fuso(r))
	de, ate := qs.Get("de"), qs.Get("ate")
	if !qs.Has("de") && !qs.Has("ate") {
		de, ate = time.Date(hoje.Year(), 1, 1, 0, 0, 0, 0, hoje.Location()).Format("2006-01-02"), hoje.Format("2006-01-02")
	}
	doenca := idOpcional(qs.Get("doenca"))
	escopo := h.escopoInvestigacoes(r)
	tipoSemana := h.tipoSemana(r, q)

	porDoenca, err := q.EpiPorDoenca(ctx, sqlccliente.EpiPorDoencaParams{DataDe: dataOpcional(de), DataAte: dataOpcional(ate), DoencaID: doenca, SomenteUsuario: escopo})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico por doença", err)
		return
	}
	porBairro, err := q.EpiPorBairro(ctx, sqlccliente.EpiPorBairroParams{DataDe: dataOpcional(de), DataAte: dataOpcional(ate), DoencaID: doenca, SomenteUsuario: escopo})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico por bairro", err)
		return
	}
	porDia, err := q.EpiPorDia(ctx, sqlccliente.EpiPorDiaParams{DataDe: dataOpcional(de), DataAte: dataOpcional(ate), DoencaID: doenca, SomenteUsuario: escopo})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico por dia", err)
		return
	}

	d := views.BiEpiData{De: de, Ate: ate}
	for _, x := range porDoenca {
		d.Doencas = append(d.Doencas, views.BiEpiDoenca{
			Nome: x.Doenca, Total: int(x.Total), Confirmados: int(x.Confirmados), Descartados: int(x.Descartados),
			Inconclusivos: int(x.Inconclusivos), EmInvestigacao: int(x.EmInvestigacao),
		})
		d.Total += int(x.Total)
		d.Confirmados += int(x.Confirmados)
		d.Descartados += int(x.Descartados)
		d.Inconclusivos += int(x.Inconclusivos)
		d.EmInvestigacao += int(x.EmInvestigacao)
	}
	for _, x := range porBairro {
		nome := x.Bairro
		if nome == "" {
			nome = "Sem bairro (endereço só em texto)"
		}
		d.Bairros = append(d.Bairros, views.BarraDash{Rotulo: nome, Valor: int(x.Total), Destaque: int(x.Confirmados)})
	}
	if len(d.Bairros) > 12 {
		d.Bairros = d.Bairros[:12]
	}

	// semanas epidemiológicas, em ordem de data (porDia já vem ordenado)
	type semana struct {
		rotulo      string
		total, conf int
	}
	var semanas []*semana
	atual := map[string]*semana{}
	for _, x := range porDia {
		if !x.Dia.Valid {
			continue
		}
		s := epi.Calcular(x.Dia.Time, tipoSemana)
		rotulo := "SE " + strconv.Itoa(s.Numero) + "/" + strconv.Itoa(s.Ano)
		if atual[rotulo] == nil {
			atual[rotulo] = &semana{rotulo: rotulo}
			semanas = append(semanas, atual[rotulo])
		}
		atual[rotulo].total += int(x.Total)
		atual[rotulo].conf += int(x.Confirmados)
	}
	if len(semanas) > 30 { // só as 30 semanas mais recentes cabem no gráfico
		semanas = semanas[len(semanas)-30:]
	}
	for _, s := range semanas {
		d.Semanas = append(d.Semanas, views.BarraDash{Rotulo: s.rotulo, Valor: s.total, Destaque: s.conf})
	}

	doencas, err := q.ListDoencas(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar doenças", err)
		return
	}
	for _, x := range doencas {
		d.Opcoes = append(d.Opcoes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: doenca.Valid && x.ID == doenca.Int64})
	}
	render(w, r, views.BiEpi(h.layout(r, "B.I. Epidemiológico", "/bi-epidemiologico"), d))
}

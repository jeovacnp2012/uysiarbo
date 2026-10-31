package server

import (
	"fmt"
	"html"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/epi"
	"uysiarbo/web/views"
)

// biEpiPagina é o B.I. Epidemiológico, no formato do modelo "BI Dengue" (docs/): resumo, epidemiologia, histórico
// de casos, entomologia, histórico de campo, cruzamento por bairro e as notas de fonte.
//
// Casos = investigações pela data de início. Confirmado = resultado positivo; descartado = negativo; taxa de
// confirmação = confirmados ÷ casos com resultado definido (positivo, negativo ou inconclusivo). O bairro só existe
// quando a investigação está ligada a um local; as demais ficam em "Sem bairro". Campo = visitas dos ciclos que
// começam no ano: fechado sem recuperação é o imóvel que teve visita "fechado" e nenhuma visita trabalhada no mesmo
// ciclo; recuperado, o fechado que depois foi trabalhado. Sexo, faixa etária, laboratório e bloqueios não existem
// nas investigações daqui e por isso não aparecem.
func (h *handlers) biEpiPagina(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	loc := h.fuso(r)
	hoje := time.Now().In(loc)
	ano := hoje.Year()
	if a, err := strconv.Atoi(qs.Get("ano")); err == nil && a >= 2000 && a <= hoje.Year()+1 {
		ano = a
	}
	doenca := idOpcional(qs.Get("doenca"))
	escopo := h.escopoInvestigacoes(r)
	tipoSemana := h.tipoSemana(r, q)

	dia := func(a int, m time.Month, d int) pgtype.Date {
		return pgtype.Date{Time: time.Date(a, m, d, 0, 0, 0, 0, time.UTC), Valid: true}
	}
	de := dia(ano, 1, 1)
	ate := dia(ano, 12, 31)
	if ano == hoje.Year() {
		ate = dia(ano, hoje.Month(), hoje.Day())
	}

	porDoenca, err := q.EpiPorDoenca(ctx, sqlccliente.EpiPorDoencaParams{DataDe: de, DataAte: ate, DoencaID: doenca, SomenteUsuario: escopo})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico por doença", err)
		return
	}
	porBairro, err := q.EpiPorBairro(ctx, sqlccliente.EpiPorBairroParams{DataDe: de, DataAte: ate, DoencaID: doenca, SomenteUsuario: escopo})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico por bairro", err)
		return
	}
	porDia, err := q.EpiPorDia(ctx, sqlccliente.EpiPorDiaParams{DataDe: de, DataAte: ate, DoencaID: doenca, SomenteUsuario: escopo})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico por dia", err)
		return
	}
	oport, err := q.EpiOportunidade(ctx, sqlccliente.EpiOportunidadeParams{DataDe: de, DataAte: ate, DoencaID: doenca, SomenteUsuario: escopo})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico oportunidade", err)
		return
	}
	desde := dia(ano-3, 1, 1)
	porMes, err := q.EpiPorMes(ctx, sqlccliente.EpiPorMesParams{Desde: desde, Ate: ate, DoencaID: doenca, SomenteUsuario: escopo})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico por mês", err)
		return
	}
	mesmoPeriodo, err := q.EpiPorAnoMesmoPeriodo(ctx, sqlccliente.EpiPorAnoMesmoPeriodoParams{Desde: desde, Ate: ate,
		MesDia: int32(int(ate.Time.Month())*100 + ate.Time.Day()), DoencaID: doenca, SomenteUsuario: escopo})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico mesmo período", err)
		return
	}
	somenteAgente := h.escopoVisitas(r)
	var somenteBairros pgtype.Int8
	if h.principal(r).Restrito("bairro_meubairro") {
		id := meuID(r)
		if id == 0 {
			id = -1
		}
		somenteBairros = pgtype.Int8{Int64: id, Valid: true}
	}
	ciclos, err := q.CampoPorCiclo(ctx, sqlccliente.CampoPorCicloParams{AnoDe: int32(ano - 3), AnoAte: int32(ano), SomenteAgente: somenteAgente, SomenteUsuario: somenteBairros})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico campo por ciclo", err)
		return
	}
	campoBairro, err := q.CampoFechadosPorBairro(ctx, sqlccliente.CampoFechadosPorBairroParams{Ano: int32(ano), SomenteAgente: somenteAgente, SomenteUsuario: somenteBairros})
	if err != nil {
		h.falhaInterna(w, r, "B.I. epidemiológico campo por bairro", err)
		return
	}
	doencas, err := q.ListDoencas(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar doenças", err)
		return
	}

	d := views.BiEpiData{Ano: ano, Hoje: hoje.Format("02/01/2006"), Doenca: doenca.Int64}
	for a := hoje.Year(); a >= hoje.Year()-6; a-- {
		d.Anos = append(d.Anos, views.CidadeOpcao{ID: int64(a), Rotulo: strconv.Itoa(a), Selecionada: a == ano})
	}
	for _, x := range doencas {
		d.Opcoes = append(d.Opcoes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: doenca.Valid && x.ID == doenca.Int64})
		if doenca.Valid && x.ID == doenca.Int64 {
			d.NomeDoenca = x.Nome
		}
	}
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

	g := &d.Graficos
	// semanas epidemiológicas do ano, em ordem de data (porDia já vem ordenado)
	type semana struct {
		rotulo                 string
		total, conf, desc, inc int
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
		atual[rotulo].desc += int(x.Descartados)
		atual[rotulo].inc += int(x.Inconclusivos)
	}
	pico, picoRotulo := 0, ""
	for _, s := range semanas {
		g.Semanas = append(g.Semanas, s.rotulo)
		g.SemConf = append(g.SemConf, s.conf)
		g.SemDesc = append(g.SemDesc, s.desc)
		g.SemOutros = append(g.SemOutros, s.total-s.conf-s.desc)
		g.SemTotal = append(g.SemTotal, s.total)
		if s.total > pico {
			pico, picoRotulo = s.total, s.rotulo
		}
	}

	// bairros: os 12 com mais casos
	campoDe := map[string]sqlccliente.CampoFechadosPorBairroRow{}
	for _, c := range campoBairro {
		campoDe[c.Bairro] = c
	}
	for i, x := range porBairro {
		nome := x.Bairro
		if nome == "" {
			nome = "Sem bairro (endereço só em texto)"
		}
		if i < 12 {
			g.Bairros = append(g.Bairros, nome)
			g.BairroTotal = append(g.BairroTotal, int(x.Total))
			g.BairroConf = append(g.BairroConf, int(x.Confirmados))
		}
		if x.Bairro == "" {
			continue
		}
		linha := views.BiEpiBairro{Bairro: x.Bairro, Casos: int(x.Total), Confirmados: int(x.Confirmados), PctSemRec: -1}
		if c, ok := campoDe[x.Bairro]; ok && c.Imoveis > 0 {
			linha.Imoveis = int(c.Imoveis)
			linha.PctSemRec = pct1(int(c.FechadosSemRec), int(c.Imoveis))
			g.Pontos = append(g.Pontos, views.BiEpiPonto{Nome: x.Bairro, X: float64(x.Total), Y: linha.PctSemRec})
		}
		d.BairrosAtencao = append(d.BairrosAtencao, linha)
	}

	// histórico: curva mensal por ano e total no mesmo período
	g.MesesAnos = map[string]views.BiEpiMeses{}
	for a := ano - 3; a <= ano; a++ {
		g.MesesAnos[strconv.Itoa(a)] = views.BiEpiMeses{Total: make([]int, 12), Conf: make([]int, 12)}
	}
	for _, m := range porMes {
		if mm, ok := g.MesesAnos[strconv.Itoa(int(m.Ano))]; ok && m.Mes >= 1 && m.Mes <= 12 {
			mm.Total[m.Mes-1], mm.Conf[m.Mes-1] = int(m.Total), int(m.Confirmados)
		}
	}
	periodoDe := map[int]sqlccliente.EpiPorAnoMesmoPeriodoRow{}
	for _, x := range mesmoPeriodo {
		periodoDe[int(x.Ano)] = x
	}
	campoAno := map[int]*views.BiEpiAno{}
	for a := ano - 3; a <= ano; a++ {
		p := periodoDe[a]
		linha := views.BiEpiAno{Ano: a, Notificados: int(p.Total), Confirmados: int(p.Confirmados), Descartados: int(p.Descartados)}
		d.Anos4 = append(d.Anos4, linha)
		g.AnosRotulos = append(g.AnosRotulos, strconv.Itoa(a))
		g.AnosTotal = append(g.AnosTotal, int(p.Total))
		g.AnosConf = append(g.AnosConf, int(p.Confirmados))
	}
	for i := range d.Anos4 {
		campoAno[d.Anos4[i].Ano] = &d.Anos4[i]
	}

	// campo por ciclo
	var ultimo *views.BiEpiCiclo
	for _, c := range ciclos {
		linha := views.BiEpiCiclo{
			Nome: c.Ciclo, Ano: int(c.Ano), Visitas: int(c.Visitas), Imoveis: int(c.Imoveis), Fechados: int(c.Fechados),
			Recuperados: int(c.Recuperados), SemRec: int(c.FechadosSemRec), Positivos: int(c.Positivos),
		}
		if a := campoAno[int(c.Ano)]; a != nil {
			a.Visitas += linha.Visitas
			a.Imoveis += linha.Imoveis
			a.Fechados += linha.Fechados
			a.SemRec += linha.SemRec
			a.Recuperados += linha.Recuperados
			a.Positivos += linha.Positivos
		}
		if int(c.Ano) != ano {
			continue
		}
		d.Ciclos = append(d.Ciclos, linha)
		g.CicloRotulos = append(g.CicloRotulos, c.Ciclo)
		g.CicloSemRec = append(g.CicloSemRec, pct1(linha.SemRec, linha.Imoveis))
		g.CicloRecup = append(g.CicloRecup, pct1(linha.Recuperados, linha.Fechados))
		g.CicloImoveis = append(g.CicloImoveis, linha.Imoveis)
	}
	if n := len(d.Ciclos); n > 0 {
		ultimo = &d.Ciclos[n-1]
	}
	for _, c := range campoBairro {
		if c.FechadosSemRec > 0 && c.Imoveis > 0 {
			g.PendBairros = append(g.PendBairros, c.Bairro)
			g.PendValores = append(g.PendValores, pct1(int(c.FechadosSemRec), int(c.Imoveis)))
		}
	}
	// os 15 bairros com maior percentual de fechados sem recuperação
	idx := make([]int, len(g.PendBairros))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return g.PendValores[idx[a]] > g.PendValores[idx[b]] })
	if len(idx) > 15 {
		idx = idx[:15]
	}
	nomes, vals := make([]string, 0, len(idx)), make([]float64, 0, len(idx))
	for _, i := range idx {
		nomes, vals = append(nomes, g.PendBairros[i]), append(vals, g.PendValores[i])
	}
	g.PendBairros, g.PendValores = nomes, vals

	// oportunidade e situação
	d.Oport = views.BiEpiOport{
		Total: int(oport.Total), Encerradas: int(oport.Encerradas), EmAtendimento: int(oport.EmAtendimento), Aguardando: int(oport.Aguardando),
		Mediana: strconv.FormatFloat(oport.MedianaDias, 'f', 0, 64), Ate7: int(oport.Ate7), Ate30: int(oport.Ate30), ComTempo: int(oport.ComTempo),
	}

	// faixa de destaque, cartões e leitura rápida
	d.Titulo = fmt.Sprintf("%d investigações em %d, %d confirmadas", d.Total, ano, d.Confirmados)
	d.Subtitulo = "Taxa de confirmação de " + d.TaxaConfirmacao() + " entre os casos com resultado definido."
	if d.NomeDoenca != "" {
		d.Titulo = fmt.Sprintf("%s: %d investigações em %d, %d confirmadas", d.NomeDoenca, d.Total, ano, d.Confirmados)
	}
	if len(d.Doencas) > 0 && d.NomeDoenca == "" {
		d.Insights = append(d.Insights, fmt.Sprintf("A doença mais investigada em %d foi <b>%s</b>, com <b>%d</b> casos (%s do total).", ano, html.EscapeString(d.Doencas[0].Nome), d.Doencas[0].Total, razaoPct(d.Doencas[0].Total, d.Total)))
	}
	if picoRotulo != "" {
		d.Insights = append(d.Insights, fmt.Sprintf("O pico foi na <b>%s</b>, com <b>%d</b> investigações iniciadas na semana.", picoRotulo, pico))
	}
	if ant, ok := periodoDe[ano-1]; ok && ant.Total > 0 {
		cur := periodoDe[ano]
		var_ := (float64(cur.Total) - float64(ant.Total)) * 100 / float64(ant.Total)
		d.Insights = append(d.Insights, fmt.Sprintf("No mesmo período de %d houve <b>%d</b> investigações; em %d, <b>%d</b> (%+.0f%%).", ano-1, ant.Total, ano, cur.Total, var_))
	}
	if ultimo != nil && ultimo.Imoveis > 0 {
		d.Insights = append(d.Insights, fmt.Sprintf("No <b>%s</b>, <b>%s</b> dos %d imóveis visitados ficaram fechados sem recuperação e %s dos fechados foram recuperados.",
			html.EscapeString(ultimo.Nome), razaoPct(ultimo.SemRec, ultimo.Imoveis), ultimo.Imoveis, razaoPct(ultimo.Recuperados, ultimo.Fechados)))
	}
	d.KPIsEpi = []views.BiEpiKpi{
		{Rotulo: "Investigações", Valor: strconv.Itoa(d.Total), Detalhe: "com início em " + strconv.Itoa(ano)},
		{Rotulo: "Confirmados", Valor: strconv.Itoa(d.Confirmados), Detalhe: "taxa " + d.TaxaConfirmacao()},
		{Rotulo: "Descartados", Valor: strconv.Itoa(d.Descartados), Detalhe: strconv.Itoa(d.Inconclusivos) + " inconclusivos"},
		{Rotulo: "Em investigação", Valor: strconv.Itoa(d.EmInvestigacao), Detalhe: "resultado aguardando"},
		{Rotulo: "Encerradas", Valor: strconv.Itoa(d.Oport.Encerradas), Detalhe: "mediana " + d.Oport.Mediana + " dias até encerrar"},
	}
	if ultimo != nil {
		d.GrupoCampo = "Entomologia · " + ultimo.Nome
		d.KPIsEnt = []views.BiEpiKpi{
			{Rotulo: "Imóveis visitados", Valor: strconv.Itoa(ultimo.Imoveis), Detalhe: strconv.Itoa(ultimo.Visitas) + " visitas"},
			{Rotulo: "Encontrados fechados", Valor: strconv.Itoa(ultimo.Fechados), Detalhe: razaoPct(ultimo.Fechados, ultimo.Imoveis) + " dos visitados"},
			{Rotulo: "Recuperados", Valor: strconv.Itoa(ultimo.Recuperados), Detalhe: razaoPct(ultimo.Recuperados, ultimo.Fechados) + " dos fechados"},
			{Rotulo: "Fechados sem recuperação", Valor: strconv.Itoa(ultimo.SemRec), Detalhe: razaoPct(ultimo.SemRec, ultimo.Imoveis) + " dos visitados"},
			{Rotulo: "Imóveis positivos", Valor: strconv.Itoa(ultimo.Positivos), Detalhe: razaoPct(ultimo.Positivos, ultimo.Imoveis) + " dos visitados"},
		}
	}
	for i := range d.Anos4 {
		a := &d.Anos4[i]
		a.PctSemRec = pct1(a.SemRec, a.Imoveis)
		a.PctRecup = pct1(a.Recuperados, a.Fechados)
		g.HCampoImoveis = append(g.HCampoImoveis, a.Imoveis)
		g.HCampoSemRec = append(g.HCampoSemRec, a.PctSemRec)
		g.HCampoRecup = append(g.HCampoRecup, a.PctRecup)
	}
	render(w, r, views.BiEpi(h.layout(r, "B.I. Epidemiológico", "/bi-epidemiologico"), d))
}

func pct1(parte, total int) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(parte)*1000/float64(total)) / 10
}

func razaoPct(parte, total int) string {
	if total <= 0 {
		return "—"
	}
	return strconv.FormatFloat(float64(parte)*100/float64(total), 'f', 1, 64) + "%"
}

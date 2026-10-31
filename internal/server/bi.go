package server

import (
	"fmt"
	"math"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// biPagina é o B.I. de infestação por bairro: IIP, Breteau e cobertura da atividade no período.
//
//	IIP      = imóveis positivos / imóveis pesquisados x 100
//	Breteau  = recipientes positivos / imóveis pesquisados x 100
//	Cobertura = imóveis pesquisados / locais ativos do bairro x 100
//
// Imóvel pesquisado é o local com visita sem pendência; positivo, o que teve tubito concluído com Aedes;
// recipiente positivo é o tubito positivo. As fórmulas seguem o padrão do PNCD e precisam ser validadas
// com a área técnica (ver plano 7.2).
func (h *handlers) biPagina(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	arg := sqlccliente.BiPorBairroParams{
		CicloID: idOpcional(qs.Get("ciclo")), AgenteID: idOpcional(qs.Get("agente")), AtividadeID: idOpcional(qs.Get("atividade")),
		BairroID: idOpcional(qs.Get("bairro")), SomenteAgente: h.escopoVisitas(r),
	}
	if h.principal(r).Restrito("bairro_meubairro") {
		id := meuID(r)
		if id == 0 {
			id = -1
		}
		arg.SomenteUsuario = pgtype.Int8{Int64: id, Valid: true}
	}
	if qs.Get("ciclo") == "" {
		if atual, err := q.DashboardCicloAtual(ctx); err == nil {
			arg.CicloID = pgtype.Int8{Int64: atual.ID, Valid: true}
		}
	}
	rows, err := q.BiPorBairro(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "B.I. por bairro", err)
		return
	}
	d := views.BiData{Todos: !arg.CicloID.Valid}
	var pesq, pos, rec, locais int
	for _, b := range rows {
		d.Linhas = append(d.Linhas, views.BiLinha{
			Bairro: b.Bairro, Locais: int(b.Locais), Pesquisados: int(b.Pesquisados), Positivos: int(b.Positivos), Recipientes: int(b.Recipientes),
		})
		pesq, pos, rec, locais = pesq+int(b.Pesquisados), pos+int(b.Positivos), rec+int(b.Recipientes), locais+int(b.Locais)
	}
	d.Total = views.BiLinha{Bairro: "Total", Locais: locais, Pesquisados: pesq, Positivos: pos, Recipientes: rec}
	if err := h.biGraficos(r, q, arg, &d); err != nil {
		h.falhaInterna(w, r, "B.I. gráficos", err)
		return
	}

	ciclos, err := q.ListCiclos(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar ciclos", err)
		return
	}
	for _, c := range ciclos {
		d.Ciclos = append(d.Ciclos, views.CidadeOpcao{ID: c.ID, Rotulo: c.Nome, Selecionada: arg.CicloID.Valid && c.ID == arg.CicloID.Int64})
	}
	ativs, err := q.ListAtividades(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar atividades", err)
		return
	}
	for _, a := range ativs {
		d.Atividades = append(d.Atividades, views.CidadeOpcao{ID: int64(a.ID), Rotulo: a.Sigla + " — " + a.Nome, Selecionada: int64(a.ID) == arg.AtividadeID.Int64})
	}
	bairros, err := q.ListBairrosAtivosParaLocal(ctx, arg.SomenteUsuario)
	if err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	for _, b := range bairros {
		d.Bairros = append(d.Bairros, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome, Selecionada: b.ID == arg.BairroID.Int64})
	}
	us, err := q.ListUsuarios(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar agentes", err)
		return
	}
	for _, u := range us {
		d.Agentes = append(d.Agentes, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome, Selecionada: u.ID == arg.AgenteID.Int64})
	}
	render(w, r, views.Bi(h.layout(r, "B.I.", "/bi"), d))
}

// biGraficos reúne os dados dos gráficos (visitas, depósitos, positivos, cobertura e amostras) do mesmo filtro da tabela.
func (h *handlers) biGraficos(r *http.Request, q *sqlccliente.Queries, arg sqlccliente.BiPorBairroParams, d *views.BiData) error {
	ctx := r.Context()
	visitas, err := q.BiVisitasPorBairro(ctx, sqlccliente.BiVisitasPorBairroParams{CicloID: arg.CicloID, AgenteID: arg.AgenteID, AtividadeID: arg.AtividadeID, BairroID: arg.BairroID, SomenteAgente: arg.SomenteAgente, SomenteUsuario: arg.SomenteUsuario})
	if err != nil {
		return err
	}
	tot, err := q.BiTotais(ctx, sqlccliente.BiTotaisParams{CicloID: arg.CicloID, AgenteID: arg.AgenteID, AtividadeID: arg.AtividadeID, BairroID: arg.BairroID, SomenteAgente: arg.SomenteAgente, SomenteUsuario: arg.SomenteUsuario})
	if err != nil {
		return err
	}
	tipos, err := q.BiPositivosPorTipo(ctx, sqlccliente.BiPositivosPorTipoParams{CicloID: arg.CicloID, AgenteID: arg.AgenteID, AtividadeID: arg.AtividadeID, BairroID: arg.BairroID, SomenteAgente: arg.SomenteAgente, SomenteUsuario: arg.SomenteUsuario})
	if err != nil {
		return err
	}
	g := &d.Graficos
	var dep [7]int
	for _, v := range visitas {
		g.VisitasBairros = append(g.VisitasBairros, v.Bairro)
		g.Visitas = append(g.Visitas, int(v.Visitas))
		for i, n := range []int32{v.A1, v.A2, v.B, v.C, v.D1, v.D2, v.E} {
			dep[i] += int(n)
		}
	}
	g.Depositos = dep[:]
	// positivos por bairro: os 10 maiores e "Outros"
	pos := append([]views.BiLinha(nil), d.Linhas...)
	sort.SliceStable(pos, func(i, j int) bool { return pos[i].Positivos > pos[j].Positivos })
	outros, nOutros := 0, 0
	for i, b := range pos {
		if b.Positivos == 0 {
			break
		}
		if i < 10 {
			g.PosBairros = append(g.PosBairros, b.Bairro)
			g.PosBairro = append(g.PosBairro, b.Positivos)
		} else {
			outros += b.Positivos
			nOutros++
		}
	}
	if nOutros > 0 {
		g.PosBairros = append(g.PosBairros, fmt.Sprintf("Outros (%d)", nOutros))
		g.PosBairro = append(g.PosBairro, outros)
	}
	// cobertura por bairro, da maior para a menor
	cob := append([]views.BiLinha(nil), d.Linhas...)
	sort.SliceStable(cob, func(i, j int) bool { return cob[i].CoberturaNum() > cob[j].CoberturaNum() })
	for _, b := range cob {
		g.CobBairros = append(g.CobBairros, b.Bairro)
		g.Cobertura = append(g.Cobertura, b.CoberturaNum())
	}
	g.AmostrasPositivas, g.AmostrasNegativas = int(tot.AmostrasPositivas), int(tot.AmostrasNegativas)
	for _, t := range tipos {
		g.TipoLocais = append(g.TipoLocais, rotuloDe(opcoesTipo(""), t.Tipo))
		g.TipoValores = append(g.TipoValores, int(t.Locais))
	}
	d.Quarteiroes, d.QuarteiroesVisitados = int(tot.Quarteiroes), int(tot.QuarteiroesVisitados)
	d.LocaisAtivos, d.LocaisVisitados, d.Fechados = int(tot.Locais), int(tot.LocaisVisitados), int(tot.Fechados)
	if tot.Locais > 0 {
		g.CoberturaCiclo = math.Round(float64(tot.LocaisVisitados)*1000/float64(tot.Locais)) / 10
	}
	return nil
}

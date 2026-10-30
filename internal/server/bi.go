package server

import (
	"net/http"

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
		SomenteAgente: h.escopoVisitas(r),
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

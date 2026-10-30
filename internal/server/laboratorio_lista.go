package server

import (
	"net/http"
	"net/url"
	"strings"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// laboratorioLista é a listagem dos tubitos com filtros ao vivo (tubito, status, ciclo, agente, período e endereço).
func (h *handlers) laboratorioLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	txt := func(k string) string { return strings.TrimSpace(qs.Get(k)) }
	f := views.LaboratorioFiltro{
		Tubito: txt("tubito"), Status: qs.Get("status"), Ciclo: qs.Get("ciclo"), Agente: qs.Get("agente"),
		De: txt("de"), Ate: txt("ate"), Endereco: txt("endereco"),
	}
	if !validoEm(rotulosStatusAmostra, f.Status) {
		f.Status = ""
	}
	count := sqlccliente.CountAmostrasParams{
		SomenteAgente: h.escopoVisitas(r), Tubito: f.Tubito, Status: f.Status, CicloID: idOpcional(f.Ciclo), AgenteID: idOpcional(f.Agente),
		DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate), Endereco: f.Endereco,
	}
	pag := paginaDe(r)
	total, err := q.CountAmostras(ctx, count)
	if err != nil {
		h.falhaInterna(w, r, "contar amostras", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListAmostras(ctx, sqlccliente.ListAmostrasParams{
		SomenteAgente: count.SomenteAgente, Tubito: count.Tubito, Status: count.Status, CicloID: count.CicloID, AgenteID: count.AgenteID,
		DataDe: count.DataDe, DataAte: count.DataAte, Endereco: count.Endereco,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar amostras", err)
		return
	}
	d := views.LaboratorioData{Filtro: f, Pag: pag, Situacoes: marcarOpcao(rotulosStatusAmostra, f.Status)}
	for _, a := range rows {
		d.Linhas = append(d.Linhas, views.AmostraListaLinha{
			ID: a.ID, VisitaID: a.VisitaID, Tubito: a.NumeroTubito, Deposito: a.Deposito, Endereco: enderecoCompleto(a.Endereco, a.Numero),
			Data: dataBR(a.Data), Ciclo: a.CicloNome, Agente: a.AgenteNome, Status: a.Status, StatusRotulo: rotuloDe(rotulosStatusAmostra, a.Status),
			Larvas: int(a.Larvas), Pupas: int(a.Pupas), Aegypti: int(a.TotalAegypti), Albopictus: int(a.TotalAlbopictus), Negativo: int(a.TotalNegativo),
		})
	}
	ciclos, err := q.ListCiclos(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar ciclos", err)
		return
	}
	for _, c := range ciclos {
		d.Ciclos = append(d.Ciclos, views.CidadeOpcao{ID: c.ID, Rotulo: c.Nome, Selecionada: c.ID == count.CicloID.Int64})
	}
	us, err := q.ListUsuarios(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar agentes", err)
		return
	}
	for _, u := range us {
		d.Agentes = append(d.Agentes, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome, Selecionada: u.ID == count.AgenteID.Int64})
	}
	d.Query = url.Values{}
	for k, v := range map[string]string{"tubito": f.Tubito, "status": f.Status, "ciclo": f.Ciclo, "agente": f.Agente, "de": f.De, "ate": f.Ate, "endereco": f.Endereco} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Laboratorio(h.layout(r, "Laboratório", "/laboratorio"), d))
}

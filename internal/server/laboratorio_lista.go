package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

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
		Numero: txt("numero"), Bairro: qs.Get("bairro"), Quarteirao: qs.Get("quarteirao"), Lado: txt("lado"),
		Visita: txt("visita"), Atividade: qs.Get("atividade"), Semana: txt("semana"),
	}
	ordem, dir := ordemDoLaboratorio(qs.Get("ordem"), qs.Get("dir"))
	semana := pgtype.Int4{}
	if n, err := strconv.Atoi(f.Semana); err == nil && n >= 1 && n <= 53 {
		semana = pgtype.Int4{Int32: int32(n), Valid: true}
	}
	if !validoEm(rotulosStatusAmostra, f.Status) {
		f.Status = ""
	}
	count := sqlccliente.CountLaboratorioParams{
		SomenteAgente: h.escopoVisitas(r), Tubito: f.Tubito, Status: f.Status, CicloID: idOpcional(f.Ciclo), AgenteID: idOpcional(f.Agente),
		DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate), Endereco: f.Endereco,
		Numero: f.Numero, BairroID: idOpcional(f.Bairro), QuarteiraoID: idOpcional(f.Quarteirao), Lado: f.Lado,
		IDTexto: f.Visita, AtividadeID: idOpcional(f.Atividade), Semana: semana,
	}
	pag := paginaDe(r)
	total, err := q.CountLaboratorio(ctx, count)
	if err != nil {
		h.falhaInterna(w, r, "contar laboratório", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListLaboratorio(ctx, sqlccliente.ListLaboratorioParams{
		SomenteAgente: count.SomenteAgente, Tubito: count.Tubito, Status: count.Status, CicloID: count.CicloID, AgenteID: count.AgenteID,
		DataDe: count.DataDe, DataAte: count.DataAte, Endereco: count.Endereco,
		Numero: count.Numero, BairroID: count.BairroID, QuarteiraoID: count.QuarteiraoID, Lado: count.Lado,
		IDTexto: count.IDTexto, AtividadeID: count.AtividadeID, Semana: count.Semana, Ordem: ordem, Dir: dir,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar laboratório", err)
		return
	}
	d := views.LaboratorioData{Filtro: f, Pag: pag, Situacoes: marcarOpcao(rotulosStatusAmostra, f.Status), Ordem: ordem, Dir: dir}
	for _, a := range rows {
		d.Linhas = append(d.Linhas, linhaDoLaboratorio(a))
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
	atividades, err := q.ListAtividades(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar atividades", err)
		return
	}
	for _, a := range atividades {
		d.Atividades = append(d.Atividades, views.CidadeOpcao{ID: int64(a.ID), Rotulo: a.Sigla + " — " + a.Nome, Selecionada: int64(a.ID) == count.AtividadeID.Int64})
	}
	somente, _ := h.escopoLocais(r)
	bairros, err := q.ListBairrosAtivosParaLocal(ctx, somente)
	if err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	for _, b := range bairros {
		d.Bairros = append(d.Bairros, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome, Selecionada: b.ID == count.BairroID.Int64})
	}
	if count.BairroID.Valid {
		qts, err := q.ListQuarteiroesAtivosDoBairro(ctx, sqlccliente.ListQuarteiroesAtivosDoBairroParams{BairroID: count.BairroID.Int64, SomenteUsuario: somente})
		if err != nil {
			h.falhaInterna(w, r, "listar quarteirões", err)
			return
		}
		for _, x := range qts {
			d.Quarteiroes = append(d.Quarteiroes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: x.ID == count.QuarteiraoID.Int64})
		}
	}
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{
		"tubito": f.Tubito, "status": f.Status, "ciclo": f.Ciclo, "agente": f.Agente, "de": f.De, "ate": f.Ate, "endereco": f.Endereco,
		"numero": f.Numero, "bairro": f.Bairro, "quarteirao": f.Quarteirao, "lado": f.Lado, "visita": f.Visita, "atividade": f.Atividade, "semana": f.Semana,
	} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Laboratorio(h.layout(r, "Laboratório", "/laboratorio"), d))
}

// linhaDoLaboratorio monta a linha no formato do Geo Controle: ID, endereço com QT e Lado, status da visita,
// data e hora com o agente, atividade com a sigla e "Nº de tubetes" como "2 (124, 125)".
func linhaDoLaboratorio(a sqlccliente.ListLaboratorioRow) views.LaboratorioLinha {
	id := a.IDExterno
	if id == "" {
		id = strconv.FormatInt(a.ID, 10)
	}
	endereco := enderecoCompleto(a.Endereco, a.Numero)
	if a.Quarteirao != "" {
		endereco += " QT: " + a.Quarteirao
	}
	if a.Lado != "" {
		endereco += " Lado: " + a.Lado
	}
	if a.Bairro != "" {
		endereco += " " + a.Bairro
	}
	quando := dataBR(a.Data)
	if a.HoraEntrada.Valid {
		min := a.HoraEntrada.Microseconds / 60_000_000
		quando += fmt.Sprintf(" %02d:%02d", min/60, min%60)
	}
	semana := ""
	if a.Semana.Valid {
		semana = strconv.Itoa(int(a.Semana.Int16))
	}
	status, rotulo := "concluido", "Concluído"
	if a.Pendente {
		status, rotulo = "em_analise", "Em análise"
	}
	return views.LaboratorioLinha{
		VisitaID: a.ID, ID: id, Endereco: endereco, Status: status, StatusRotulo: rotulo, EmAnalise: a.Pendente,
		DataAgente: quando + " " + a.AgenteNome, Ciclo: a.CicloNome, Semana: semana, Atividade: a.AtividadeNome + " " + a.AtividadeSigla,
		Tubetes: strconv.Itoa(int(a.Tubetes)) + " (" + a.Numeros + ")",
		Aegypti: int(a.TotalAegypti), Albopictus: int(a.TotalAlbopictus), Negativo: int(a.TotalNegativo),
	}
}

// colunasOrdenaveisLaboratorio são as colunas do laboratório que fazem sentido ordenar (as chaves do SQL).
var colunasOrdenaveisLaboratorio = map[string]bool{
	"id": true, "endereco": true, "status": true, "data": true, "ciclo": true, "semana": true, "atividade": true,
	"tubetes": true, "aegypti": true, "albopictus": true, "negativo": true,
}

// ordemDoLaboratorio valida a coluna e o sentido pedidos; sem pedido (ou inválido), a ordem padrão (mais recentes primeiro).
func ordemDoLaboratorio(ordem, dir string) (string, string) {
	if !colunasOrdenaveisLaboratorio[ordem] {
		return "", ""
	}
	if dir != "desc" {
		dir = "asc"
	}
	return ordem, dir
}

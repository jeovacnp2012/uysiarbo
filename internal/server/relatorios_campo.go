package server

import (
	"context"
	"fmt"
	"net/http"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/relatorio"
)

// Relatórios de campo e de vigilância (Campanha, LIRAa, Locais por criticidade e folha de pesquisa em papel).
// Registrados em relatorios_def.go; as colunas são inferidas do Geo Controle e devem ser conferidas com a área técnica.

func (h *handlers) relatorioCampanha(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelCampanha(ctx, sqlccliente.RelCampanhaParams{DataDe: f.data("de"), DataAte: f.data("ate"), Fuso: h.fuso(r).String(),
		BairroID: f.id("bairro"), SomenteUsuario: h.usuarioDoEscopo(r)})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Campanha antirrábica por bairro", Colunas: []relatorio.Coluna{
		col("Bairro", 4, false), col("Atendimentos", 1.5, true), col("Cães vacinados", 1.6, true), col("Gatos vacinados", 1.6, true),
		col("Cães não vacinados", 1.8, true), col("Gatos não vacinados", 1.8, true), col("Recusas", 1.1, true), col("Fechadas", 1.1, true)}}
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Bairro, num(v.Atendimentos), num(v.VacCachorros), num(v.VacGatos),
			num(v.NaoCachorros), num(v.NaoGatos), num(v.Recusas), num(v.Fechadas)})
	}
	totalizar(&doc, "Total", 1, 2, 3, 4, 5, 6, 7)
	doc.Observar = "Fechadas = situação fechada ou responsável ausente. O período vale para a data e hora do atendimento, no fuso da empresa."
	return doc, nil
}

// relatorioLiraa: imóvel pesquisado = local com visita da atividade LIRAa sem pendência; positivo = local com tubito
// concluído com Aedes; IIP = positivos ÷ pesquisados e Breteau = recipientes positivos ÷ pesquisados, ambos x100.
func (h *handlers) relatorioLiraa(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelLiraa(ctx, sqlccliente.RelLiraaParams{CicloID: f.id("ciclo"), BairroID: f.id("bairro"),
		SomenteAgente: h.escopoVisitas(r), SomenteUsuario: h.usuarioDoEscopo(r)})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "LIRAa por bairro", Colunas: []relatorio.Coluna{
		col("Bairro", 4, false), col("Quart. programados", 1.7, true), col("Quart. encerrados", 1.7, true), col("Pesquisados", 1.4, true),
		col("Pendências", 1.3, true), col("Depósitos", 1.2, true), col("Positivos", 1.2, true), col("Recipientes +", 1.4, true),
		col("IIP", 1, true), col("Breteau", 1.1, true)}}
	var pesq, pos, rec int32
	for _, v := range rows {
		pesq, pos, rec = pesq+v.Pesquisados, pos+v.Positivos, rec+v.Recipientes
		doc.Linhas = append(doc.Linhas, []string{v.Bairro, num(v.Programados), num(v.Encerrados), num(v.Pesquisados), num(v.Pendencias),
			num(v.Depositos), num(v.Positivos), num(v.Recipientes), indiceLiraa(v.Positivos, v.Pesquisados), indiceLiraa(v.Recipientes, v.Pesquisados)})
	}
	totalizar(&doc, "Total", 1, 2, 3, 4, 5, 6, 7)
	doc.Totais[8], doc.Totais[9] = indiceLiraa(pos, pesq), indiceLiraa(rec, pesq)
	doc.Observar = "IIP = imóveis positivos ÷ imóveis pesquisados x 100; Breteau = recipientes positivos ÷ imóveis pesquisados x 100. Fórmulas a validar com a área técnica."
	return doc, nil
}

func indiceLiraa(parte, base int32) string {
	if base == 0 {
		return "-"
	}
	return decimal(float64(parte) * 100 / float64(base))
}

func (h *handlers) relatorioCriticidade(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelCriticidade(ctx, sqlccliente.RelCriticidadeParams{BairroID: f.id("bairro"), SomenteUsuario: h.usuarioDoEscopo(r),
		SomenteAgente: h.escopoVisitas(r), Minimo: reincidenciaMinima})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Locais por criticidade", Colunas: []relatorio.Coluna{
		col("Bairro", 3, false), col("Quarteirão", 1.3, false), col("Endereço", 4, false), col("Tipo", 1.6, false),
		col("Visitas com depósito", 1.8, true), col("Tubitos positivos", 1.7, true), col("Última", 1.3, false)}}
	rows = cortarLinhas(rows, &doc)
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Bairro, v.Quarteirao, enderecoCompleto(v.Endereco, v.Numero), rotuloDe(opcoesTipo(""), v.Tipo),
			num(v.ComDeposito), num(v.Positivos), dataBR(v.Ultima)})
	}
	doc.Totais = make([]string, len(doc.Colunas))
	doc.Totais[0] = fmt.Sprintf("Total: %d locais", len(rows))
	doc.Observar = "Considera as visitas dos últimos 12 meses (mesmo critério da reincidência na ficha do local). Ordem: mais tubitos positivos, depois mais visitas com depósito."
	return doc, nil
}

func (h *handlers) relatorioPesquisaPapel(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelLocaisParaPapel(ctx, sqlccliente.RelLocaisParaPapelParams{BairroID: f.id("bairro"), Tipo: f["tipo"], SomenteUsuario: h.usuarioDoEscopo(r)})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Pesquisa entomológica / tratamento (folha de campo)", Colunas: []relatorio.Coluna{
		col("Bairro", 2.5, false), col("Quart.", 1, false), col("Endereço", 4, false), col("Tipo", 1.4, false),
		col("Pend.", 1, false), col("A1", 0.8, false), col("A2", 0.8, false), col("B", 0.8, false), col("C", 0.8, false),
		col("D1", 0.8, false), col("D2", 0.8, false), col("E", 0.8, false), col("Tubito", 1.4, false), col("Larvicida", 1.4, false)}}
	rows = cortarLinhas(rows, &doc)
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Bairro, v.Quarteirao, enderecoCompleto(v.Endereco, v.Numero), rotuloDe(opcoesTipo(""), v.Tipo),
			"", "", "", "", "", "", "", "", "", ""})
	}
	doc.Totais = make([]string, len(doc.Colunas))
	doc.Totais[0] = fmt.Sprintf("Total: %d locais", len(rows))
	doc.Observar = "Folha para preencher a mão: depósitos A1 a E, número do tubito e larvicida. Os dados devem ser lançados depois em Visitas."
	return doc, nil
}

package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/relatorio"
)

// Relatórios do sistema além do de Visitas (8.2). Todos seguem o mesmo molde: consulta agregada em
// queries_cliente/relatorios.sql, linhas em texto e uma linha de totais. As colunas seguem o que o Geo Controle
// mostra; cada um deve ser conferido contra um exemplo real antes de ser dado como fechado.
//
// Campanha, LIRAa, criticidade e a folha de pesquisa em papel ficam em relatorios_campo.go.

func init() {
	periodo := []campoRelatorio{{"de", "De", "data"}, {"ate", "Até", "data"}}
	relatoriosDoSistema = append(relatoriosDoSistema,
		definicaoRelatorio{
			Slug: "local-visita", Titulo: "Local x visita", Permissao: "relatorio_local_visita",
			Resumo: "Os locais ativos com quantas visitas cada um recebeu no ciclo; filtre pelos que foram ou não visitados.",
			Campos: []campoRelatorio{{"ciclo", "Ciclo", "ciclo"}, {"atividade", "Atividade", "atividade"}, {"bairro", "Bairro", "bairro"},
				{"pendencia", "Pendência", "pendencia"}, {"situacao", "Foi visitado", "simnao"}},
			Gerar: (*handlers).relatorioLocalVisita,
		},
		definicaoRelatorio{
			Slug: "producao", Titulo: "Acompanhamento de produção", Permissao: "relatorio_producao",
			Resumo: "Por agente: visitas, locais, dias trabalhados, pendências, depósitos, amostras e tratamentos.",
			Campos: append([]campoRelatorio{{"ciclo", "Ciclo", "ciclo"}, {"atividade", "Atividade", "atividade"}, {"agente", "Agente", "agente"}}, periodo...),
			Gerar:  (*handlers).relatorioProducao,
		},
		definicaoRelatorio{
			Slug: "antivetorial", Titulo: "Serviço antivetorial", Permissao: "relatorio_antivetorial",
			Resumo: "Por atividade: visitas, depósitos encontrados e eliminados, larvicida (L1 e L2) e adulticida.",
			Campos: append([]campoRelatorio{{"atividade", "Atividade", "atividade"}}, periodo...),
			Gerar:  (*handlers).relatorioAntivetorial,
		},
		definicaoRelatorio{
			Slug: "dia-trabalhado", Titulo: "Dia trabalhado", Permissao: "relatorio_dia_trabalhado",
			Resumo: "Por agente e dia: visitas, locais e horário da primeira e da última visita.",
			Campos: append([]campoRelatorio{{"agente", "Agente", "agente"}}, periodo...),
			Gerar:  (*handlers).relatorioDiaTrabalhado,
		},
		definicaoRelatorio{
			Slug: "laboratorio-total", Titulo: "Laboratório: totais", Permissao: "relatorio_laboratorio_total",
			Resumo: "Por bairro: tubitos, em análise, concluídos, positivos e totais de Aedes aegypti e albopictus.",
			Campos: periodo,
			Gerar:  (*handlers).relatorioLaboratorioTotal,
		},
		definicaoRelatorio{
			Slug: "investigacao-total", Titulo: "Investigação: totais", Permissao: "relatorio_investigacao_total",
			Resumo: "Por doença: investigações, situação e resultado (positivo, negativo, inconclusivo).",
			Campos: periodo,
			Gerar:  (*handlers).relatorioInvestigacaoTotal,
		},
		definicaoRelatorio{
			Slug: "investigacao-sinan", Titulo: "Investigações: layout SINAN / e-SUS", Permissao: "relatorio_investigacao_total",
			Resumo: "As investigações em colunas (agravo, paciente, endereço, classificação final, encerramento) para digitar ou importar no SINAN ou e-SUS.",
			Campos: periodo,
			Gerar:  (*handlers).relatorioInvestigacaoSinan,
		},
		definicaoRelatorio{
			Slug: "animais", Titulo: "Animais: totais", Permissao: "relatorio_animais",
			Resumo: "Por bairro: locais, moradores, cães, gatos e roedores cadastrados.",
			Campos: []campoRelatorio{{"bairro", "Bairro", "bairro"}},
			Gerar:  (*handlers).relatorioAnimais,
		},
		definicaoRelatorio{
			Slug: "bri", Titulo: "Borrifação residual (BRI)", Permissao: "relatorio_bri",
			Resumo: "Visitas com borrifação residual intradomiciliar e o volume usado.",
			Campos: append([]campoRelatorio{{"ciclo", "Ciclo", "ciclo"}, {"bairro", "Bairro", "bairro"}}, periodo...),
			Gerar:  (*handlers).relatorioBRI,
		},
		definicaoRelatorio{
			Slug: "local-incompleto", Titulo: "Locais com cadastro incompleto", Permissao: "relatorio_local_incompleto",
			Resumo: "Locais sem coordenada, sem CEP e, nos residenciais, sem proprietário ou sem número de moradores.",
			Campos: []campoRelatorio{{"bairro", "Bairro", "bairro"}, {"tipo", "Tipo de local", "tipolocal"}},
			Gerar:  (*handlers).relatorioLocalIncompleto,
		},
		definicaoRelatorio{
			Slug: "campanha", Titulo: "Campanha de vacinação", Permissao: "relatorio_campanha",
			Resumo: "Por bairro: atendimentos da campanha antirrábica, cães e gatos vacinados e não vacinados, recusas e casas fechadas.",
			Campos: append([]campoRelatorio{{"bairro", "Bairro", "bairro"}}, periodo...),
			Gerar:  (*handlers).relatorioCampanha,
		},
		definicaoRelatorio{
			Slug: "liraa", Titulo: "LIRAa", Permissao: "relatorio_liraa",
			Resumo: "Por bairro: quarteirões programados e encerrados, imóveis pesquisados, depósitos, positivos, IIP e Breteau.",
			Campos: []campoRelatorio{{"ciclo", "Ciclo", "ciclo"}, {"bairro", "Bairro", "bairro"}},
			Gerar:  (*handlers).relatorioLiraa,
		},
		definicaoRelatorio{
			Slug: "criticidade", Titulo: "Locais por criticidade", Permissao: "relatorio_local_visita",
			Resumo: "Locais com depósito encontrado em 2 ou mais visitas nos últimos 12 meses, os com tubitos positivos primeiro (apoio ao sobrevoo e ao bloqueio).",
			Campos: []campoRelatorio{{"bairro", "Bairro", "bairro"}},
			Gerar:  (*handlers).relatorioCriticidade,
		},
		definicaoRelatorio{
			Slug: "pesquisa-papel", Titulo: "Pesquisa entomológica / tratamento em papel", Permissao: "relatorio_visita",
			Resumo: "Lista de locais por quarteirão com colunas em branco para preencher a mão em campo quando o tablet não estiver disponível.",
			Campos: []campoRelatorio{{"bairro", "Bairro", "bairro"}, {"tipo", "Tipo de local", "tipolocal"}},
			Gerar:  (*handlers).relatorioPesquisaPapel,
		},
	)
}

func col(t string, peso float64, direita bool) relatorio.Coluna {
	return relatorio.Coluna{Titulo: t, Peso: peso, Direita: direita}
}

func num(n int32) string { return strconv.Itoa(int(n)) }

func decimal(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) }

func horaDe(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	min := t.Microseconds / 60_000_000
	return fmt.Sprintf("%02d:%02d", min/60, min%60)
}

// totalizar monta a linha de totais somando as colunas indicadas (as demais ficam vazias; rotulo vai na primeira).
func totalizar(doc *relatorio.Doc, rotulo string, somar ...int) {
	tot := make([]string, len(doc.Colunas))
	tot[0] = rotulo
	for _, c := range somar {
		soma := 0
		for _, l := range doc.Linhas {
			n, _ := strconv.Atoi(l[c])
			soma += n
		}
		tot[c] = strconv.Itoa(soma)
	}
	doc.Totais = tot
}

// cortarLinhas aplica o teto de linhas do motor às consultas que pedem MaxLinhas+1.
func cortarLinhas[T any](rows []T, doc *relatorio.Doc) []T {
	if len(rows) > relatorio.MaxLinhas {
		doc.Cortado = true
		return rows[:relatorio.MaxLinhas]
	}
	return rows
}

func (h *handlers) usuarioDoEscopo(r *http.Request) pgtype.Int8 {
	somente, _ := h.escopoLocais(r)
	return somente
}

func (h *handlers) relatorioLocalVisita(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelLocalVisita(ctx, sqlccliente.RelLocalVisitaParams{
		CicloID: f.id("ciclo"), AtividadeID: f.id("atividade"), Pendencia: f["pendencia"], SomenteAgente: h.escopoVisitas(r),
		BairroID: f.id("bairro"), SomenteUsuario: h.usuarioDoEscopo(r), Situacao: f["situacao"],
	})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Relatório local x visita", Colunas: []relatorio.Coluna{
		col("Bairro", 3, false), col("Quarteirão", 1.4, false), col("Endereço", 4, false), col("Lado", 0.8, false),
		col("Visitas", 1, true), col("Última", 1.4, false), col("Com depósito", 1.4, true)}}
	rows = cortarLinhas(rows, &doc)
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Bairro, v.Quarteirao, enderecoCompleto(v.Endereco, v.Numero), v.Lado,
			num(v.Visitas), dataBR(v.Ultima), num(v.ComDeposito)})
	}
	totalizar(&doc, fmt.Sprintf("Total: %d locais", len(rows)), 4, 6)
	return doc, nil
}

func (h *handlers) relatorioProducao(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelProducao(ctx, sqlccliente.RelProducaoParams{
		CicloID: f.id("ciclo"), AgenteID: f.id("agente"), AtividadeID: f.id("atividade"),
		DataDe: f.data("de"), DataAte: f.data("ate"), SomenteAgente: h.escopoVisitas(r),
	})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Acompanhamento de produção", Colunas: []relatorio.Coluna{
		col("Agente", 4, false), col("Dias", 0.8, true), col("Visitas", 1, true), col("Locais", 1, true), col("Fechadas", 1.1, true),
		col("Recusadas", 1.2, true), col("Tentativas", 1.2, true), col("Com depósito", 1.4, true), col("Com amostra", 1.4, true), col("Com tratamento", 1.6, true)}}
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Agente, num(v.Dias), num(v.Visitas), num(v.Locais), num(v.Fechadas),
			num(v.Recusadas), num(v.Tentativas), num(v.ComDeposito), num(v.ComAmostra), num(v.ComTratamento)})
	}
	totalizar(&doc, "Total", 2, 4, 5, 6, 7, 8, 9)
	doc.Observar = "Dias e locais são contados por agente; a soma dos locais pode repetir um local visitado por mais de um agente."
	return doc, nil
}

func (h *handlers) relatorioAntivetorial(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelAntivetorial(ctx, sqlccliente.RelAntivetorialParams{
		AtividadeID: f.id("atividade"), DataDe: f.data("de"), DataAte: f.data("ate"), SomenteAgente: h.escopoVisitas(r),
	})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Serviço antivetorial", Colunas: []relatorio.Coluna{
		col("Atividade", 3, false), col("Visitas", 1, true), col("Locais", 1, true), col("Depósitos", 1.2, true), col("Eliminados", 1.3, true),
		col("L1 (g)", 1, true), col("L1 trat.", 1.1, true), col("L2 (g)", 1, true), col("L2 trat.", 1.1, true), col("Cargas adult.", 1.5, true)}}
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Sigla + " — " + v.Atividade, num(v.Visitas), num(v.Locais), num(v.Depositos), num(v.Eliminados),
			decimal(v.L1Gramas), num(v.L1Tratados), decimal(v.L2Gramas), num(v.L2Tratados), num(v.Cargas)})
	}
	totalizar(&doc, "Total", 1, 3, 4, 6, 8, 9)
	doc.Observar = "Depósitos = soma dos depósitos encontrados (A1 a E). L1 e L2 são as quantidades de larvicida em gramas e depósitos tratados."
	return doc, nil
}

func (h *handlers) relatorioDiaTrabalhado(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelDiaTrabalhado(ctx, sqlccliente.RelDiaTrabalhadoParams{
		AgenteID: f.id("agente"), DataDe: f.data("de"), DataAte: f.data("ate"), SomenteAgente: h.escopoVisitas(r),
	})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Visitas por dia trabalhado", Colunas: []relatorio.Coluna{
		col("Agente", 4, false), col("Data", 1.3, false), col("Visitas", 1, true), col("Locais", 1, true), col("Primeira", 1.1, false), col("Última", 1.1, false)}}
	rows = cortarLinhas(rows, &doc)
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Agente, dataBR(v.Data), num(v.Visitas), num(v.Locais), horaDe(v.Primeira), horaDe(v.Ultima)})
	}
	totalizar(&doc, fmt.Sprintf("Total: %d dias trabalhados", len(rows)), 2)
	return doc, nil
}

func (h *handlers) relatorioLaboratorioTotal(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelLaboratorioTotal(ctx, sqlccliente.RelLaboratorioTotalParams{DataDe: f.data("de"), DataAte: f.data("ate"), SomenteAgente: h.escopoVisitas(r)})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Laboratório: totais por bairro", Colunas: []relatorio.Coluna{
		col("Bairro", 4, false), col("Tubitos", 1, true), col("Em análise", 1.2, true), col("Concluídos", 1.2, true), col("Positivos", 1.2, true),
		col("Aegypti", 1.1, true), col("Albopictus", 1.3, true), col("Negativo", 1.1, true)}}
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Bairro, num(v.Tubitos), num(v.EmAnalise), num(v.Concluidos), num(v.Positivos),
			num(v.Aegypti), num(v.Albopictus), num(v.Negativo)})
	}
	totalizar(&doc, "Total", 1, 2, 3, 4, 5, 6, 7)
	doc.Observar = "Positivo é o tubito concluído com Aedes aegypti ou albopictus. O período vale para a data da visita."
	return doc, nil
}

func (h *handlers) relatorioAnimais(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelAnimais(ctx, sqlccliente.RelAnimaisParams{BairroID: f.id("bairro"), SomenteUsuario: h.usuarioDoEscopo(r)})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Total de animais por bairro", Colunas: []relatorio.Coluna{
		col("Bairro", 4, false), col("Locais", 1, true), col("Moradores", 1.3, true), col("Cães", 1, true), col("Gatos", 1, true), col("Roedores", 1.2, true)}}
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Bairro, num(v.Locais), num(v.Humanos), num(v.Cachorros), num(v.Gatos), num(v.Roedores)})
	}
	totalizar(&doc, "Total", 1, 2, 3, 4, 5)
	return doc, nil
}

func (h *handlers) relatorioBRI(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelBRI(ctx, sqlccliente.RelBRIParams{CicloID: f.id("ciclo"), BairroID: f.id("bairro"), DataDe: f.data("de"), DataAte: f.data("ate"), SomenteAgente: h.escopoVisitas(r)})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Borrifação residual intradomiciliar (BRI)", Colunas: []relatorio.Coluna{
		col("ID", 1, true), col("Data", 1.3, false), col("Endereço", 4, false), col("Bairro", 3, false), col("Quarteirão", 1.3, false),
		col("Agente", 3, false), col("Ciclo", 1.3, false), col("Volume", 1, true)}}
	rows = cortarLinhas(rows, &doc)
	var total float64
	for _, v := range rows {
		total += v.Volume
		doc.Linhas = append(doc.Linhas, []string{strconv.FormatInt(v.ID, 10), dataBR(v.Data), enderecoCompleto(v.Endereco, v.Numero), v.Bairro,
			v.Quarteirao, v.Agente, v.Ciclo, decimal(v.Volume)})
	}
	doc.Totais = make([]string, len(doc.Colunas))
	doc.Totais[2], doc.Totais[7] = fmt.Sprintf("Total: %d visitas", len(rows)), decimal(total)
	return doc, nil
}

func (h *handlers) relatorioLocalIncompleto(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelLocalIncompleto(ctx, sqlccliente.RelLocalIncompletoParams{BairroID: f.id("bairro"), Tipo: f["tipo"], SomenteUsuario: h.usuarioDoEscopo(r)})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Locais com cadastro incompleto", Colunas: []relatorio.Coluna{
		col("Bairro", 3, false), col("Quarteirão", 1.3, false), col("ID", 0.9, true), col("Endereço", 4, false), col("Tipo", 1.6, false), col("O que falta", 4, false)}}
	rows = cortarLinhas(rows, &doc)
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Bairro, v.Quarteirao, strconv.FormatInt(v.ID, 10), enderecoCompleto(v.Endereco, v.Numero),
			rotuloDe(opcoesTipo(""), v.Tipo), v.Faltas})
	}
	doc.Totais = make([]string, len(doc.Colunas))
	doc.Totais[0] = fmt.Sprintf("Total: %d locais", len(rows))
	return doc, nil
}

func (h *handlers) relatorioInvestigacaoTotal(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelInvestigacaoTotal(ctx, sqlccliente.RelInvestigacaoTotalParams{DataDe: f.data("de"), DataAte: f.data("ate"), SomenteUsuario: h.escopoInvestigacoes(r)})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Investigação: totais por doença", Colunas: []relatorio.Coluna{
		col("Doença", 4, false), col("Total", 1, true), col("Aguardando", 1.3, true), col("Em atendimento", 1.6, true), col("Encerradas", 1.3, true),
		col("Positivos", 1.2, true), col("Negativos", 1.2, true), col("Inconclusivos", 1.5, true), col("Sem resultado", 1.5, true)}}
	for _, v := range rows {
		doc.Linhas = append(doc.Linhas, []string{v.Doenca, num(v.Total), num(v.Aguardando), num(v.EmAtendimento), num(v.Encerradas),
			num(v.Positivos), num(v.Negativos), num(v.Inconclusivos), num(v.SemResultado)})
	}
	totalizar(&doc, "Total", 1, 2, 3, 4, 5, 6, 7, 8)
	doc.Observar = "O período vale para a data de início da investigação (na falta dela, a data do registro)."
	return doc, nil
}

// relatorioInvestigacaoSinan lista as investigações no formato de planilha para digitar ou importar no SINAN / e-SUS.
// Não é o arquivo oficial (DBF/XML): é a ficha em colunas, com a classificação final no vocabulário da vigilância.
func (h *handlers) relatorioInvestigacaoSinan(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.RelInvestigacaoSinan(ctx, sqlccliente.RelInvestigacaoSinanParams{DataDe: f.data("de"), DataAte: f.data("ate"), SomenteUsuario: h.escopoInvestigacoes(r)})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{Titulo: "Investigações no layout SINAN / e-SUS", Colunas: []relatorio.Coluna{
		col("Nº", 0.8, true), col("Nº de origem", 1.2, false), col("Agravo", 2, false), col("Data da notificação", 1.5, false), col("Paciente", 3, false),
		col("Telefone", 1.5, false), col("Logradouro", 3, false), col("Bairro", 2, false), col("Classificação final", 1.7, false),
		col("Situação da investigação", 1.8, false), col("Data de encerramento", 1.6, false), col("Investigador", 2, false), col("Observação", 3, false)}}
	rows = cortarLinhas(rows, &doc)
	loc := h.fuso(r)
	classif := map[string]string{"positivo": "Confirmado", "negativo": "Descartado", "inconclusivo": "Inconclusivo", "aguardando": "Em investigação"}
	for _, v := range rows {
		logradouro, bairro := v.LogradouroLocal, v.BairroLocal
		if logradouro == "" {
			logradouro = v.EnderecoTexto
		}
		encerrada := ""
		if v.EncerradaEm.Valid {
			encerrada = v.EncerradaEm.Time.In(loc).Format("02/01/2006")
		}
		doc.Linhas = append(doc.Linhas, []string{strconv.FormatInt(v.ID, 10), v.IDOrigem, v.Agravo, dataBR(v.DataInicio), v.Paciente, v.Telefone, logradouro, bairro,
			classif[v.Resultado], rotuloDe(rotulosSituacaoInvestigacao, v.Situacao), encerrada, v.Responsavel, v.Observacao})
	}
	doc.Observar = "Planilha para digitação ou importação no SINAN / e-SUS Notifica, não é o arquivo oficial. Classificação final: positivo = Confirmado, negativo = Descartado. " +
		"Sexo, data de nascimento e CID não são guardados nas investigações e não saem aqui. O período vale para a data de início (na falta dela, a do registro)."
	return doc, nil
}

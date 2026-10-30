package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/relatorio"
	"uysiarbo/web/views"
)

// definicaoRelatorio descreve um relatório do sistema. Para acrescentar um: uma entrada em relatoriosDoSistema
// (slug, título, permissão do catálogo, campos de filtro) e a função Gerar que devolve o relatorio.Doc; telas,
// PDF, planilha e CSV vêm do motor.
type definicaoRelatorio struct {
	Slug, Titulo, Resumo, Permissao string
	Campos                          []campoRelatorio
	Gerar                           func(h *handlers, ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error)
}

// campoRelatorio é um filtro do formulário: Tipo = ciclo, atividade, agente, bairro, pendencia, simnao, data.
type campoRelatorio struct{ Nome, Rotulo, Tipo string }

type filtrosRelatorio map[string]string

func (f filtrosRelatorio) data(nome string) pgtype.Date { return dataOpcional(f[nome]) }
func (f filtrosRelatorio) id(nome string) pgtype.Int8   { return idOpcional(f[nome]) }

var relatoriosDoSistema = []definicaoRelatorio{
	{
		Slug: "visitas", Titulo: "Visitas", Permissao: "relatorio_visita",
		Resumo: "Todas as visitas do período, com agente, ciclo, atividade, pendência e o que foi feito em cada uma.",
		Campos: []campoRelatorio{
			{"ciclo", "Ciclo", "ciclo"}, {"atividade", "Atividade", "atividade"}, {"agente", "Agente", "agente"},
			{"bairro", "Bairro", "bairro"}, {"de", "De", "data"}, {"ate", "Até", "data"},
			{"pendencia", "Pendência", "pendencia"}, {"deposito", "Achou depósito", "simnao"},
		},
		Gerar: (*handlers).relatorioVisitas,
	},
}

func definicaoPorSlug(slug string) (definicaoRelatorio, bool) {
	for _, d := range relatoriosDoSistema {
		if d.Slug == slug {
			return d, true
		}
	}
	return definicaoRelatorio{}, false
}

func simNaoTexto(b bool) string {
	if b {
		return "Sim"
	}
	return "Não"
}

func (h *handlers) relatorioVisitas(ctx context.Context, r *http.Request, q *sqlccliente.Queries, f filtrosRelatorio) (relatorio.Doc, error) {
	rows, err := q.ListVisitas(ctx, sqlccliente.ListVisitasParams{
		VerDesativadas: h.principal(r).Pode("visita_ver_desativados"), SomenteAgente: h.escopoVisitas(r),
		BairroID: f.id("bairro"), CicloID: f.id("ciclo"), AgenteID: f.id("agente"), AtividadeID: f.id("atividade"),
		DataDe: f.data("de"), DataAte: f.data("ate"), Pendencia: f["pendencia"], Deposito: f["deposito"],
		Limite: relatorio.MaxLinhas + 1,
	})
	if err != nil {
		return relatorio.Doc{}, err
	}
	doc := relatorio.Doc{
		Titulo: "Relatório de visitas",
		Colunas: []relatorio.Coluna{
			{Titulo: "ID", Peso: 1, Direita: true}, {Titulo: "Data", Peso: 1.4}, {Titulo: "Endereço", Peso: 4}, {Titulo: "Agente", Peso: 3},
			{Titulo: "Ciclo", Peso: 1.3}, {Titulo: "Sem.", Peso: 0.8, Direita: true}, {Titulo: "Ativ.", Peso: 1}, {Titulo: "Pendência", Peso: 1.6},
			{Titulo: "Depósito", Peso: 1.3}, {Titulo: "Amostra", Peso: 1.3}, {Titulo: "Tratamento", Peso: 1.5},
		},
	}
	if len(rows) > relatorio.MaxLinhas {
		rows, doc.Cortado = rows[:relatorio.MaxLinhas], true
	}
	var comDeposito int
	for _, v := range rows {
		semana := ""
		if v.Semana.Valid {
			semana = strconv.Itoa(int(v.Semana.Int16))
		}
		if v.AchouDeposito {
			comDeposito++
		}
		doc.Linhas = append(doc.Linhas, []string{
			strconv.FormatInt(v.ID, 10), dataBR(v.Data), enderecoCompleto(v.Endereco, v.Numero), v.AgenteNome, v.CicloNome, semana,
			v.AtividadeSigla, rotuloDe(rotulosPendencia, v.Pendencia), simNaoTexto(v.AchouDeposito), simNaoTexto(v.ColetouAmostra), simNaoTexto(v.FezTratamento),
		})
	}
	doc.Totais = make([]string, len(doc.Colunas))
	doc.Totais[2] = fmt.Sprintf("Total: %d visitas, %d com depósito", len(rows), comDeposito)
	return doc, nil
}

// relatoriosIndice lista os relatórios que a pessoa pode gerar.
func (h *handlers) relatoriosIndice(w http.ResponseWriter, r *http.Request) {
	pr := h.principal(r)
	d := views.RelatoriosIndice{}
	for _, def := range relatoriosDoSistema {
		if pr.Pode(def.Permissao) {
			d.Itens = append(d.Itens, views.RelatorioItem{Slug: def.Slug, Titulo: def.Titulo, Resumo: def.Resumo})
		}
	}
	render(w, r, views.Relatorios(h.layout(r, "Relatórios", "/relatorios"), d))
}

const linhasNaTela = 200

// relatorioGerar mostra o formulário de filtros e, com "gerar", o resultado em tela; com formato=pdf|xlsx|csv, baixa.
func (h *handlers) relatorioGerar(w http.ResponseWriter, r *http.Request) {
	def, ok := definicaoPorSlug(chi.URLParam(r, "slug"))
	if !ok || !h.principal(r).Pode(def.Permissao) {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := filtrosRelatorio{}
	for _, c := range def.Campos {
		v := strings.TrimSpace(qs.Get(c.Nome))
		switch c.Tipo {
		case "pendencia":
			if !validoEm(rotulosPendencia, v) {
				v = ""
			}
		case "simnao":
			v = simNaoValido(v)
		case "tipolocal":
			if !validoEm(opcoesTipo(""), v) {
				v = ""
			}
		case "data":
			if _, err := time.Parse(formatoDataISO, v); err != nil {
				v = ""
			}
		}
		f[c.Nome] = v
	}

	lay := h.layout(r, def.Titulo, "/relatorios")
	d := views.RelatorioTela{Slug: def.Slug, Titulo: def.Titulo, Resumo: def.Resumo}
	if d.Campos, err = h.camposDoRelatorio(ctx, q, def, f, r); err != nil {
		h.falhaInterna(w, r, "opções do relatório", err)
		return
	}
	formato := qs.Get("formato")
	if qs.Get("gerar") == "" && formato == "" {
		render(w, r, views.RelatorioPagina(lay, d))
		return
	}
	doc, err := def.Gerar(h, ctx, r, q, f)
	if err != nil {
		h.falhaInterna(w, r, "gerar relatório "+def.Slug, err)
		return
	}
	doc.Cliente = lay.Cliente
	doc.Gerado = formatarData(h.fuso(r), time.Now())
	for _, c := range d.Campos {
		if c.Valor == "" {
			continue
		}
		valor := c.Valor
		for _, o := range c.Opcoes {
			if o.Selecionada {
				valor = o.Rotulo
			}
		}
		doc.Filtros = append(doc.Filtros, relatorio.Par{Nome: c.Rotulo, Valor: valor})
	}

	if formato != "" {
		h.baixarRelatorio(w, r, def, doc, formato)
		return
	}
	d.Gerado, d.Cortado, d.Total = true, doc.Cortado, len(doc.Linhas)
	for _, c := range doc.Colunas {
		d.Colunas = append(d.Colunas, c.Titulo)
	}
	d.Linhas = doc.Linhas
	if len(d.Linhas) > linhasNaTela {
		d.Linhas = d.Linhas[:linhasNaTela]
	}
	d.Totais = doc.Totais
	d.Mostradas = len(d.Linhas)
	d.Query = qs.Encode()
	render(w, r, views.RelatorioPagina(lay, d))
}

func (h *handlers) baixarRelatorio(w http.ResponseWriter, r *http.Request, def definicaoRelatorio, doc relatorio.Doc, formato string) {
	nome := def.Slug + "-" + time.Now().In(h.fuso(r)).Format("20060102-1504")
	var err error
	switch formato {
	case "pdf":
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`.pdf"`)
		err = doc.PDF(w)
	case "xlsx":
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`.xlsx"`)
		err = doc.XLSX(w)
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`.csv"`)
		err = doc.CSV(w)
	default:
		http.Error(w, "formato inválido", http.StatusBadRequest)
		return
	}
	if err != nil {
		h.log.Error("baixar relatório", "relatorio", def.Slug, "formato", formato, "erro", err)
		return
	}
	// dados saindo do sistema ficam registrados
	h.auditar(r, audit.Evento{Acao: "relatorio.baixar", Entidade: "relatorio", EntidadeID: def.Slug, Rotulo: def.Titulo,
		Detalhes: map[string]any{"formato": formato, "linhas": len(doc.Linhas)}})
}

// camposDoRelatorio monta as opções de cada filtro do formulário.
func (h *handlers) camposDoRelatorio(ctx context.Context, q *sqlccliente.Queries, def definicaoRelatorio, f filtrosRelatorio, r *http.Request) ([]views.RelatorioCampo, error) {
	var out []views.RelatorioCampo
	for _, c := range def.Campos {
		campo := views.RelatorioCampo{Nome: c.Nome, Rotulo: c.Rotulo, Tipo: c.Tipo, Valor: f[c.Nome]}
		sel := func(id int64) bool { return strconv.FormatInt(id, 10) == f[c.Nome] }
		switch c.Tipo {
		case "ciclo":
			cs, err := q.ListCiclos(ctx)
			if err != nil {
				return nil, err
			}
			for _, x := range cs {
				campo.Opcoes = append(campo.Opcoes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: sel(x.ID)})
			}
		case "atividade":
			as, err := q.ListAtividades(ctx)
			if err != nil {
				return nil, err
			}
			for _, x := range as {
				campo.Opcoes = append(campo.Opcoes, views.CidadeOpcao{ID: int64(x.ID), Rotulo: x.Sigla + " — " + x.Nome, Selecionada: sel(int64(x.ID))})
			}
		case "agente":
			us, err := q.ListUsuarios(ctx)
			if err != nil {
				return nil, err
			}
			for _, x := range us {
				campo.Opcoes = append(campo.Opcoes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: sel(x.ID)})
			}
		case "bairro":
			somente, _ := h.escopoLocais(r)
			bs, err := q.ListBairrosAtivosParaLocal(ctx, somente)
			if err != nil {
				return nil, err
			}
			for _, x := range bs {
				campo.Opcoes = append(campo.Opcoes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: sel(x.ID)})
			}
		case "pendencia":
			for _, o := range rotulosPendencia {
				campo.Opcoes = append(campo.Opcoes, views.CidadeOpcao{Valor: o.Valor, Rotulo: o.Rotulo, Selecionada: o.Valor == f[c.Nome]})
			}
		case "tipolocal":
			campo.Opcoes = opcoesTipo(f[c.Nome])
		case "simnao":
			campo.Opcoes = []views.CidadeOpcao{{Valor: "sim", Rotulo: "Sim", Selecionada: f[c.Nome] == "sim"}, {Valor: "nao", Rotulo: "Não", Selecionada: f[c.Nome] == "nao"}}
		}
		out = append(out, campo)
	}
	return out, nil
}

// relatoriosPermitidos diz se a pessoa pode gerar ao menos um relatório (o item do menu só aparece então).
func relatoriosPermitidos(pode func(string) bool) bool {
	for _, d := range relatoriosDoSistema {
		if pode(d.Permissao) {
			return true
		}
	}
	return false
}

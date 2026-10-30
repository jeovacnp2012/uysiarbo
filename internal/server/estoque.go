package server

import (
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Estoque (11.1): unidades, produtos (inclusive fracionáveis), lotes e a movimentação do saldo.

var (
	rotulosTipoProduto = []views.CidadeOpcao{
		{Valor: "vacina", Rotulo: "Vacina"},
		{Valor: "equipamento", Rotulo: "Equipamento"},
		{Valor: "insumo", Rotulo: "Insumo"},
		{Valor: "outros", Rotulo: "Outros"},
	}
	rotulosSaidaProduto = []views.CidadeOpcao{
		{Valor: "campo_permanente", Rotulo: "Campo e permanente"},
		{Valor: "campo", Rotulo: "Apenas campo"},
		{Valor: "permanente", Rotulo: "Apenas permanente"},
	}
	rotulosFracao = []views.CidadeOpcao{
		{Valor: "dose", Rotulo: "Dose"},
		{Valor: "ml", Rotulo: "Mililitro (ml)"},
		{Valor: "mg", Rotulo: "Miligrama (mg)"},
		{Valor: "ui", Rotulo: "Unidade Internacional (UI)"},
	}
	rotulosCategoriaDiaria = []views.CidadeOpcao{
		{Valor: "vacina", Rotulo: "Vacina"},
		{Valor: "seringa", Rotulo: "Seringa"},
	}
	rotulosSituacaoProduto = []views.CidadeOpcao{
		{Valor: "ativo", Rotulo: "Ativo"},
		{Valor: "inativo", Rotulo: "Inativo"},
	}
	rotulosSituacaoLote = []views.CidadeOpcao{
		{Valor: "com_saldo", Rotulo: "Com saldo"},
		{Valor: "zerado", Rotulo: "Sem saldo"},
		{Valor: "a_vencer", Rotulo: "Vence em 30 dias"},
		{Valor: "vencido", Rotulo: "Vencido"},
	}
	// Movimentações que a tela permite lançar à mão; as demais (retirada, devolução, uso) vêm da 11.2.
	rotulosMovimentoManual = []views.CidadeOpcao{
		{Valor: "entrada", Rotulo: "Entrada adicional (soma ao saldo)"},
		{Valor: "perda", Rotulo: "Perda (subtrai do saldo)"},
		{Valor: "baixa", Rotulo: "Baixa (subtrai do saldo)"},
		{Valor: "ajuste", Rotulo: "Ajuste (define o novo saldo)"},
	}
	rotulosMovimento = []views.CidadeOpcao{
		{Valor: "entrada", Rotulo: "Entrada"},
		{Valor: "retirada", Rotulo: "Retirada"},
		{Valor: "devolucao", Rotulo: "Devolução"},
		{Valor: "uso", Rotulo: "Uso"},
		{Valor: "perda", Rotulo: "Perda"},
		{Valor: "baixa", Rotulo: "Baixa"},
		{Valor: "ajuste", Rotulo: "Ajuste"},
	}
)

// ------------------------------------------------------------------ números

// qtdTexto mostra a quantidade com vírgula decimal e sem zeros à direita.
func qtdTexto(n pgtype.Numeric) string {
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return "0"
	}
	return strings.ReplaceAll(strconv.FormatFloat(v.Float64, 'f', -1, 64), ".", ",")
}

func qtdFloat(n pgtype.Numeric) float64 {
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return 0
	}
	return v.Float64
}

func numericoDe(v float64, casas int) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(v, 'f', casas, 64))
	return n
}

// lerQuantidade lê um número com vírgula ou ponto, de 0 (ou acima de 0) até 999.999.999,999.
func lerQuantidade(s, nome string, permiteZero bool) (float64, string) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 999999999 || (!permiteZero && v <= 0) {
		if permiteZero {
			return 0, nome + ": informe um número de 0 a 999.999.999."
		}
		return 0, nome + ": informe um número maior que zero."
	}
	return math.Round(v*1000) / 1000, ""
}

func ordemDeEstoque(ordem, dir string, validas ...string) (string, string) {
	for _, v := range validas {
		if ordem == v {
			if dir != "desc" {
				dir = "asc"
			}
			return ordem, dir
		}
	}
	return "", ""
}

// ------------------------------------------------------------------ unidades

func (h *handlers) estoqueUnidades(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListEstoqueUnidades(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar unidades de estoque", err)
		return
	}
	linhas := make([]views.EstoqueUnidadeLinha, len(rows))
	for i, x := range rows {
		linhas[i] = views.EstoqueUnidadeLinha{IDTexto: strconv.FormatInt(x.ID, 10), Nome: x.Nome, Descricao: x.Descricao, Ativo: x.Ativo, Produtos: x.Produtos}
	}
	lay := h.layout(r, "Unidades de Estoque", "/estoque/unidade")
	linhas = paginar(r, &lay, linhas)
	render(w, r, views.EstoqueUnidades(lay, views.EstoqueUnidadesData{Linhas: linhas, PodeGerir: h.principal(r).Pode("estoque_unidade_novo")}))
}

func (h *handlers) estoqueUnidadeNova(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.EstoqueUnidadeFicha(h.layout(r, "Nova unidade de estoque", "/estoque/unidade"), views.EstoqueUnidadeForm{Ativo: true, CSRF: csrfToken(r.Context()), PodeEditar: true}))
}

func lerUnidade(r *http.Request) (views.EstoqueUnidadeForm, string) {
	d := views.EstoqueUnidadeForm{
		Nome: strings.TrimSpace(r.PostFormValue("nome")), Descricao: strings.TrimSpace(r.PostFormValue("descricao")),
		Ativo: r.PostFormValue("ativo") != "0", CSRF: csrfToken(r.Context()), PodeEditar: true,
	}
	switch {
	case d.Nome == "" || len([]rune(d.Nome)) > 100:
		return d, "Informe o nome da unidade (até 100 caracteres)."
	case len([]rune(d.Descricao)) > 500:
		return d, "A descrição aceita até 500 caracteres."
	}
	return d, ""
}

func (h *handlers) estoqueUnidadeCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d, erro := lerUnidade(r)
	if erro == "" {
		var u sqlccliente.EstoqueUnidade
		if u, err = q.CreateEstoqueUnidade(r.Context(), sqlccliente.CreateEstoqueUnidadeParams{Nome: d.Nome, Descricao: d.Descricao, Ativo: d.Ativo}); err == nil {
			h.auditar(r, audit.Evento{Acao: "estoque_unidade.criar", Entidade: "estoque_unidade", EntidadeID: strconv.FormatInt(u.ID, 10), Rotulo: u.Nome, Depois: map[string]any{"nome": u.Nome}})
			http.Redirect(w, r, "/estoque/unidade/"+strconv.FormatInt(u.ID, 10)+"?aviso=criado", http.StatusSeeOther)
			return
		}
		if !violacaoUnica(err) {
			h.falhaInterna(w, r, "criar unidade de estoque", err)
			return
		}
		erro = "Já existe uma unidade com esse nome."
	}
	d.Erro = erro
	renderStatus(w, r, http.StatusBadRequest, views.EstoqueUnidadeFicha(h.layout(r, "Nova unidade de estoque", "/estoque/unidade"), d))
}

func (h *handlers) carregarEstoqueUnidade(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.EstoqueUnidade, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.EstoqueUnidade{}, false
	}
	u, err := q.GetEstoqueUnidade(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return u, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar unidade de estoque", err)
		return u, false
	}
	return u, true
}

func (h *handlers) renderEstoqueUnidade(w http.ResponseWriter, r *http.Request, u sqlccliente.EstoqueUnidade, d views.EstoqueUnidadeForm, erro string) {
	d.ID, d.CSRF, d.Erro, d.PodeEditar = u.ID, csrfToken(r.Context()), erro, h.principal(r).Pode("estoque_unidade_editar")
	if erro == "" {
		d.Nome, d.Descricao, d.Ativo = u.Nome, u.Descricao, u.Ativo
	}
	switch r.URL.Query().Get("aviso") {
	case "criado":
		d.Aviso = "Unidade criada."
	case "salvo":
		d.Aviso = "Alterações salvas."
	}
	status := http.StatusOK
	if erro != "" {
		status = http.StatusBadRequest
	}
	renderStatus(w, r, status, views.EstoqueUnidadeFicha(h.layout(r, u.Nome, "/estoque/unidade"), d))
}

func (h *handlers) estoqueUnidadeFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if u, ok := h.carregarEstoqueUnidade(w, r, q); ok {
		h.renderEstoqueUnidade(w, r, u, views.EstoqueUnidadeForm{}, "")
	}
}

func (h *handlers) estoqueUnidadeSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	u, ok := h.carregarEstoqueUnidade(w, r, q)
	if !ok {
		return
	}
	d, erro := lerUnidade(r)
	if erro == "" {
		if _, err = q.UpdateEstoqueUnidade(r.Context(), sqlccliente.UpdateEstoqueUnidadeParams{ID: u.ID, Nome: d.Nome, Descricao: d.Descricao, Ativo: d.Ativo}); err == nil {
			h.auditar(r, audit.Evento{Acao: "estoque_unidade.editar", Entidade: "estoque_unidade", EntidadeID: strconv.FormatInt(u.ID, 10), Rotulo: d.Nome,
				Antes:  map[string]any{"nome": u.Nome, "descricao": u.Descricao, "ativo": u.Ativo},
				Depois: map[string]any{"nome": d.Nome, "descricao": d.Descricao, "ativo": d.Ativo}})
			http.Redirect(w, r, "/estoque/unidade/"+strconv.FormatInt(u.ID, 10)+"?aviso=salvo", http.StatusSeeOther)
			return
		}
		if !violacaoUnica(err) {
			h.falhaInterna(w, r, "salvar unidade de estoque", err)
			return
		}
		erro = "Já existe uma unidade com esse nome."
	}
	h.renderEstoqueUnidade(w, r, u, d, erro)
}

func (h *handlers) estoqueUnidadeExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if u, ok := h.carregarEstoqueUnidade(w, r, q); ok {
		h.excluirRegistro(w, r, "estoque_unidade", u.ID, "/estoque/unidade?aviso=excluido", "/estoque/unidade/"+strconv.FormatInt(u.ID, 10))
	}
}

// ------------------------------------------------------------------ produtos

func (h *handlers) opcoesDeUnidadesEstoque(r *http.Request, q *sqlccliente.Queries, escolhida int64) ([]views.CidadeOpcao, error) {
	rows, err := q.ListEstoqueUnidadesAtivas(r.Context())
	if err != nil {
		return nil, err
	}
	out := make([]views.CidadeOpcao, len(rows))
	for i, x := range rows {
		out[i] = views.CidadeOpcao{ID: x.ID, Valor: strconv.FormatInt(x.ID, 10), Rotulo: x.Nome, Selecionada: x.ID == escolhida}
	}
	return out, nil
}

func (h *handlers) opcoesDeProdutosEstoque(r *http.Request, q *sqlccliente.Queries, escolhido int64) ([]views.CidadeOpcao, error) {
	rows, err := q.ListEstoqueProdutosAtivos(r.Context())
	if err != nil {
		return nil, err
	}
	out := make([]views.CidadeOpcao, len(rows))
	for i, x := range rows {
		out[i] = views.CidadeOpcao{ID: x.ID, Valor: strconv.FormatInt(x.ID, 10), Rotulo: x.Nome, Selecionada: x.ID == escolhido}
	}
	return out, nil
}

func (h *handlers) estoqueProdutos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.EstoqueProdutoFiltro{Busca: strings.TrimSpace(qs.Get("busca")), Tipo: qs.Get("tipo"), Unidade: qs.Get("unidade"), Situacao: qs.Get("situacao")}
	if !validoEm(rotulosTipoProduto, f.Tipo) {
		f.Tipo = ""
	}
	if !validoEm(rotulosSituacaoProduto, f.Situacao) {
		f.Situacao = ""
	}
	ordem, dir := ordemDeEstoque(qs.Get("ordem"), qs.Get("dir"), "id", "nome")
	pag := paginaDe(r)
	total, err := q.CountEstoqueProdutos(ctx, sqlccliente.CountEstoqueProdutosParams{Busca: f.Busca, Tipo: f.Tipo, UnidadeID: idOpcional(f.Unidade), Situacao: f.Situacao})
	if err != nil {
		h.falhaInterna(w, r, "contar produtos de estoque", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListEstoqueProdutos(ctx, sqlccliente.ListEstoqueProdutosParams{
		Busca: f.Busca, Tipo: f.Tipo, UnidadeID: idOpcional(f.Unidade), Situacao: f.Situacao, Ordem: ordem, Dir: dir,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar produtos de estoque", err)
		return
	}
	d := views.EstoqueProdutosData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNovo: h.principal(r).Pode("estoque_produto_novo")}
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.EstoqueProdutoLinha{
			IDTexto: strconv.FormatInt(x.ID, 10), Nome: x.Nome, Tipo: rotuloDe(rotulosTipoProduto, x.Tipo), Unidade: x.Unidade,
			Medida: x.UnidadeMedida, Saldo: qtdTexto(x.Saldo), Fracionavel: x.Fracionavel, Ativo: x.Ativo,
		})
	}
	if d.Unidades, err = h.opcoesDeUnidadesEstoque(r, q, idOpcional(f.Unidade).Int64); err != nil {
		h.falhaInterna(w, r, "listar unidades de estoque", err)
		return
	}
	d.Tipos, d.Situacoes = marcarOpcao(rotulosTipoProduto, f.Tipo), marcarOpcao(rotulosSituacaoProduto, f.Situacao)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "tipo": f.Tipo, "unidade": f.Unidade, "situacao": f.Situacao} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.EstoqueProdutos(h.layout(r, "Produtos", "/estoque/produto"), d))
}

func (h *handlers) formularioProduto(r *http.Request, q *sqlccliente.Queries, d views.EstoqueProdutoForm, unidade int64, tipo, saida, fracao, categoria string) (views.EstoqueProdutoForm, error) {
	var err error
	if d.Unidades, err = h.opcoesDeUnidadesEstoque(r, q, unidade); err != nil {
		return d, err
	}
	if tipo == "" {
		tipo = "insumo"
	}
	if saida == "" {
		saida = "campo_permanente"
	}
	d.Tipos, d.Saidas = marcarOpcao(rotulosTipoProduto, tipo), marcarOpcao(rotulosSaidaProduto, saida)
	d.Fracoes, d.Categorias = marcarOpcao(rotulosFracao, fracao), marcarOpcao(rotulosCategoriaDiaria, categoria)
	d.CSRF = csrfToken(r.Context())
	return d, nil
}

func (h *handlers) estoqueProdutoNovo(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d, err := h.formularioProduto(r, q, views.EstoqueProdutoForm{Ativo: true, PodeEditar: true}, 0, "", "", "", "")
	if err != nil {
		h.falhaInterna(w, r, "listar unidades de estoque", err)
		return
	}
	render(w, r, views.EstoqueProdutoFicha(h.layout(r, "Novo produto", "/estoque/produto"), d))
}

type produtoLido struct {
	arg  sqlccliente.CreateEstoqueProdutoParams
	form views.EstoqueProdutoForm
}

// lerProduto lê e valida o formulário do produto.
func lerProduto(r *http.Request) (produtoLido, string) {
	p := produtoLido{}
	unidade := idOpcional(r.PostFormValue("unidade"))
	p.arg = sqlccliente.CreateEstoqueProdutoParams{
		UnidadeID: unidade.Int64, Nome: strings.TrimSpace(r.PostFormValue("nome")), Tipo: r.PostFormValue("tipo"),
		UnidadeMedida: strings.TrimSpace(r.PostFormValue("medida")), SaidaPermitida: r.PostFormValue("saida"),
		Fracionavel: r.PostFormValue("fracionavel") == "1", Descricao: strings.TrimSpace(r.PostFormValue("descricao")),
		SelecionavelDiaria: r.PostFormValue("selecionavel") == "1", Ativo: r.PostFormValue("ativo") != "0",
	}
	p.form = views.EstoqueProdutoForm{
		Nome: p.arg.Nome, Medida: p.arg.UnidadeMedida, Doses: strings.TrimSpace(r.PostFormValue("doses")), Descricao: p.arg.Descricao,
		Fracionavel: p.arg.Fracionavel, SelecionavelDiaria: p.arg.SelecionavelDiaria, Ativo: p.arg.Ativo, PodeEditar: true,
	}
	fracao, categoria := r.PostFormValue("fracao"), r.PostFormValue("categoria")
	switch {
	case !unidade.Valid:
		return p, "Escolha a unidade de estoque."
	case p.arg.Nome == "" || len([]rune(p.arg.Nome)) > 150:
		return p, "Informe o nome do produto (até 150 caracteres)."
	case !validoEm(rotulosTipoProduto, p.arg.Tipo):
		return p, "Escolha o tipo do produto."
	case p.arg.UnidadeMedida == "" || len([]rune(p.arg.UnidadeMedida)) > 20:
		return p, "Informe a unidade de medida (até 20 caracteres)."
	case !validoEm(rotulosSaidaProduto, p.arg.SaidaPermitida):
		return p, "Escolha a saída permitida."
	case len([]rune(p.arg.Descricao)) > 1000:
		return p, "A descrição aceita até 1000 caracteres."
	}
	if p.arg.Fracionavel {
		doses, erro := lerQuantidade(p.form.Doses, "Frações por unidade", false)
		if erro != "" {
			return p, erro
		}
		if !validoEm(rotulosFracao, fracao) {
			return p, "Escolha a unidade da fração."
		}
		p.arg.DosesPorUnidade = numericoDe(doses, 3)
		p.arg.UnidadeFracao = pgtype.Text{String: fracao, Valid: true}
	}
	if p.arg.SelecionavelDiaria {
		if !validoEm(rotulosCategoriaDiaria, categoria) {
			return p, "Escolha a categoria do produto na diária."
		}
		p.arg.CategoriaDiaria = pgtype.Text{String: categoria, Valid: true}
	}
	return p, ""
}

func dadosDoProduto(a sqlccliente.CreateEstoqueProdutoParams) map[string]any {
	return map[string]any{
		"nome": a.Nome, "tipo": a.Tipo, "unidade_medida": a.UnidadeMedida, "saida_permitida": a.SaidaPermitida, "fracionavel": a.Fracionavel,
		"doses_por_unidade": qtdTexto(a.DosesPorUnidade), "unidade_fracao": a.UnidadeFracao.String, "selecionavel_diaria": a.SelecionavelDiaria,
		"categoria_diaria": a.CategoriaDiaria.String, "ativo": a.Ativo,
	}
}

func (h *handlers) estoqueProdutoCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	p, erro := lerProduto(r)
	if erro == "" {
		if u, e := q.GetEstoqueUnidade(r.Context(), p.arg.UnidadeID); e != nil || !u.Ativo {
			erro = "Escolha uma unidade de estoque ativa."
		}
	}
	if erro == "" {
		var x sqlccliente.EstoqueProduto
		if x, err = q.CreateEstoqueProduto(r.Context(), p.arg); err == nil {
			h.auditar(r, audit.Evento{Acao: "estoque_produto.criar", Entidade: "estoque_produto", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: x.Nome, Depois: dadosDoProduto(p.arg)})
			http.Redirect(w, r, estoqueProdutoCaminho(x.ID)+"?aviso=criado", http.StatusSeeOther)
			return
		}
		if !violacaoUnica(err) {
			h.falhaInterna(w, r, "criar produto de estoque", err)
			return
		}
		erro = "Já existe um produto com esse nome nessa unidade."
	}
	d, err := h.formularioProduto(r, q, p.form, p.arg.UnidadeID, p.arg.Tipo, p.arg.SaidaPermitida, r.PostFormValue("fracao"), r.PostFormValue("categoria"))
	if err != nil {
		h.falhaInterna(w, r, "listar unidades de estoque", err)
		return
	}
	d.Erro = erro
	renderStatus(w, r, http.StatusBadRequest, views.EstoqueProdutoFicha(h.layout(r, "Novo produto", "/estoque/produto"), d))
}

func estoqueProdutoCaminho(id int64) string { return "/estoque/produto/" + strconv.FormatInt(id, 10) }

func (h *handlers) carregarEstoqueProduto(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.EstoqueProduto, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.EstoqueProduto{}, false
	}
	p, err := q.GetEstoqueProduto(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return p, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar produto de estoque", err)
		return p, false
	}
	return p, true
}

func (h *handlers) renderEstoqueProduto(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, x sqlccliente.EstoqueProduto, enviado *produtoLido, erro string) {
	d := views.EstoqueProdutoForm{
		ID: x.ID, Nome: x.Nome, Medida: x.UnidadeMedida, Doses: qtdTexto(x.DosesPorUnidade), Descricao: x.Descricao,
		Fracionavel: x.Fracionavel, SelecionavelDiaria: x.SelecionavelDiaria, Ativo: x.Ativo,
	}
	if !x.DosesPorUnidade.Valid {
		d.Doses = ""
	}
	unidade, tipo, saida, fracao, categoria := x.UnidadeID, x.Tipo, x.SaidaPermitida, x.UnidadeFracao.String, x.CategoriaDiaria.String
	if enviado != nil {
		d = enviado.form
		d.ID = x.ID
		unidade, tipo, saida = enviado.arg.UnidadeID, enviado.arg.Tipo, enviado.arg.SaidaPermitida
		fracao, categoria = r.PostFormValue("fracao"), r.PostFormValue("categoria")
	}
	d.PodeEditar, d.Erro = h.principal(r).Pode("estoque_produto_editar"), erro
	switch r.URL.Query().Get("aviso") {
	case "criado":
		d.Aviso = "Produto criado. Registre as entradas em Entradas / Lotes."
	case "salvo":
		d.Aviso = "Alterações salvas."
	}
	if s, err := q.SumEstoqueSaldoDoProduto(r.Context(), x.ID); err == nil {
		d.Saldo = qtdTexto(s)
	}
	d, err := h.formularioProduto(r, q, d, unidade, tipo, saida, fracao, categoria)
	if err != nil {
		h.falhaInterna(w, r, "listar unidades de estoque", err)
		return
	}
	// A unidade atual continua escolhível mesmo se estiver inativa.
	status := http.StatusOK
	if erro != "" {
		status = http.StatusBadRequest
	}
	renderStatus(w, r, status, views.EstoqueProdutoFicha(h.layout(r, x.Nome, "/estoque/produto"), d))
}

func (h *handlers) estoqueProdutoFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if x, ok := h.carregarEstoqueProduto(w, r, q); ok {
		h.renderEstoqueProduto(w, r, q, x, nil, "")
	}
}

func (h *handlers) estoqueProdutoSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	x, ok := h.carregarEstoqueProduto(w, r, q)
	if !ok {
		return
	}
	p, erro := lerProduto(r)
	if erro == "" {
		if u, e := q.GetEstoqueUnidadeQualquer(r.Context(), p.arg.UnidadeID); e != nil || u.DeletedAt.Valid || (!u.Ativo && u.ID != x.UnidadeID) {
			erro = "Escolha uma unidade de estoque ativa."
		}
	}
	if erro == "" {
		_, err = q.UpdateEstoqueProduto(r.Context(), sqlccliente.UpdateEstoqueProdutoParams{
			ID: x.ID, UnidadeID: p.arg.UnidadeID, Nome: p.arg.Nome, Tipo: p.arg.Tipo, UnidadeMedida: p.arg.UnidadeMedida, SaidaPermitida: p.arg.SaidaPermitida,
			Fracionavel: p.arg.Fracionavel, DosesPorUnidade: p.arg.DosesPorUnidade, UnidadeFracao: p.arg.UnidadeFracao, Descricao: p.arg.Descricao,
			SelecionavelDiaria: p.arg.SelecionavelDiaria, CategoriaDiaria: p.arg.CategoriaDiaria, Ativo: p.arg.Ativo,
		})
		if err == nil {
			antes := dadosDoProduto(sqlccliente.CreateEstoqueProdutoParams{
				Nome: x.Nome, Tipo: x.Tipo, UnidadeMedida: x.UnidadeMedida, SaidaPermitida: x.SaidaPermitida, Fracionavel: x.Fracionavel,
				DosesPorUnidade: x.DosesPorUnidade, UnidadeFracao: x.UnidadeFracao, SelecionavelDiaria: x.SelecionavelDiaria, CategoriaDiaria: x.CategoriaDiaria, Ativo: x.Ativo,
			})
			a, dp := audit.Diferenca(antes, dadosDoProduto(p.arg))
			h.auditar(r, audit.Evento{Acao: "estoque_produto.editar", Entidade: "estoque_produto", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: p.arg.Nome, Antes: a, Depois: dp})
			http.Redirect(w, r, estoqueProdutoCaminho(x.ID)+"?aviso=salvo", http.StatusSeeOther)
			return
		}
		if !violacaoUnica(err) {
			h.falhaInterna(w, r, "salvar produto de estoque", err)
			return
		}
		erro = "Já existe um produto com esse nome nessa unidade."
	}
	h.renderEstoqueProduto(w, r, q, x, &p, erro)
}

func (h *handlers) estoqueProdutoExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if x, ok := h.carregarEstoqueProduto(w, r, q); ok {
		h.excluirRegistro(w, r, "estoque_produto", x.ID, "/estoque/produto?aviso=excluido", estoqueProdutoCaminho(x.ID))
	}
}

// ------------------------------------------------------------------ lotes

func estoqueLoteCaminho(id int64) string { return "/estoque/lote/" + strconv.FormatInt(id, 10) }

func (h *handlers) estoqueLotes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.EstoqueLoteFiltro{Busca: strings.TrimSpace(qs.Get("busca")), Produto: qs.Get("produto"), Situacao: qs.Get("situacao")}
	if !validoEm(rotulosSituacaoLote, f.Situacao) {
		f.Situacao = ""
	}
	ordem, dir := ordemDeEstoque(qs.Get("ordem"), qs.Get("dir"), "id", "validade")
	hoje, _ := time.Parse(formatoDataISO, h.hojeISO(r))
	hojeD := pgtype.Date{Time: hoje, Valid: true}
	pag := paginaDe(r)
	total, err := q.CountEstoqueLotes(ctx, sqlccliente.CountEstoqueLotesParams{Busca: f.Busca, ProdutoID: idOpcional(f.Produto), Situacao: f.Situacao, Hoje: hojeD})
	if err != nil {
		h.falhaInterna(w, r, "contar lotes de estoque", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListEstoqueLotes(ctx, sqlccliente.ListEstoqueLotesParams{
		Busca: f.Busca, ProdutoID: idOpcional(f.Produto), Situacao: f.Situacao, Hoje: hojeD, Ordem: ordem, Dir: dir,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar lotes de estoque", err)
		return
	}
	d := views.EstoqueLotesData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNovo: h.principal(r).Pode("estoque_entrada_novo")}
	for _, x := range rows {
		alerta := ""
		if x.DataValidade.Valid {
			switch {
			case x.DataValidade.Time.Before(hoje):
				alerta = "(vencido)"
			case !x.DataValidade.Time.After(hoje.AddDate(0, 0, 30)):
				alerta = "(vence em breve)"
			}
		}
		d.Linhas = append(d.Linhas, views.EstoqueLoteLinha{
			IDTexto: strconv.FormatInt(x.ID, 10), Produto: x.Produto, Numero: x.NumeroLote, Validade: dataBR(x.DataValidade), Alerta: alerta,
			Quantidade: qtdTexto(x.Quantidade), Saldo: qtdTexto(x.Saldo), Medida: x.UnidadeMedida, Fornecedor: x.Fornecedor,
		})
	}
	if d.Produtos, err = h.opcoesDeProdutosEstoque(r, q, idOpcional(f.Produto).Int64); err != nil {
		h.falhaInterna(w, r, "listar produtos de estoque", err)
		return
	}
	d.Situacoes = marcarOpcao(rotulosSituacaoLote, f.Situacao)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "produto": f.Produto, "situacao": f.Situacao} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.EstoqueLotes(h.layout(r, "Entradas / Lotes", "/estoque/lote"), d))
}

func (h *handlers) estoqueLoteNovo(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	produtos, err := h.opcoesDeProdutosEstoque(r, q, idOpcional(r.URL.Query().Get("produto")).Int64)
	if err != nil {
		h.falhaInterna(w, r, "listar produtos de estoque", err)
		return
	}
	render(w, r, views.EstoqueLoteFicha(h.layout(r, "Nova entrada de estoque", "/estoque/lote"), views.EstoqueLoteForm{Produtos: produtos, CSRF: csrfToken(r.Context()), PodeEditar: true}))
}

// loteLido guarda os campos comuns da entrada e da edição do lote.
type loteLido struct {
	form                                     views.EstoqueLoteForm
	fabricacao, validade                     pgtype.Date
	custo                                    pgtype.Numeric
	fabricante, nota, fornecedor, observacao string
}

func lerLote(r *http.Request) (loteLido, string) {
	l := loteLido{}
	f := &l.form
	f.Fabricante, f.Fabricacao, f.Validade = strings.TrimSpace(r.PostFormValue("fabricante")), r.PostFormValue("fabricacao"), r.PostFormValue("validade")
	f.Custo, f.NotaFiscal, f.Fornecedor, f.Observacao = strings.TrimSpace(r.PostFormValue("custo")), strings.TrimSpace(r.PostFormValue("nota")), strings.TrimSpace(r.PostFormValue("fornecedor")), strings.TrimSpace(r.PostFormValue("observacao"))
	f.PodeEditar = true
	l.fabricante, l.nota, l.fornecedor, l.observacao = f.Fabricante, f.NotaFiscal, f.Fornecedor, f.Observacao
	var erro string
	if l.fabricacao, erro = dataOpcionalTexto(f.Fabricacao, "Fabricação"); erro != "" {
		return l, erro
	}
	if l.validade, erro = dataOpcionalTexto(f.Validade, "Validade"); erro != "" {
		return l, erro
	}
	if l.fabricacao.Valid && l.validade.Valid && l.validade.Time.Before(l.fabricacao.Time) {
		return l, "A validade não pode ser anterior à fabricação."
	}
	if f.Custo != "" {
		v, erro := lerQuantidade(f.Custo, "Custo unitário", true)
		if erro != "" {
			return l, erro
		}
		l.custo = numericoDe(v, 2)
	}
	switch {
	case len([]rune(l.fabricante)) > 100:
		return l, "O fabricante aceita até 100 caracteres."
	case len([]rune(l.nota)) > 30:
		return l, "A nota fiscal aceita até 30 caracteres."
	case len([]rune(l.fornecedor)) > 120:
		return l, "O fornecedor aceita até 120 caracteres."
	case len([]rune(l.observacao)) > 1000:
		return l, "A observação aceita até 1000 caracteres."
	}
	return l, ""
}

func (h *handlers) estoqueLoteCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	l, erro := lerLote(r)
	l.form.Numero, l.form.Quantidade = strings.TrimSpace(r.PostFormValue("numero")), strings.TrimSpace(r.PostFormValue("quantidade"))
	produto := idOpcional(r.PostFormValue("produto"))
	var qtd float64
	if erro == "" {
		switch {
		case !produto.Valid:
			erro = "Escolha o produto."
		case l.form.Numero == "" || len([]rune(l.form.Numero)) > 40:
			erro = "Informe o número do lote (até 40 caracteres)."
		default:
			qtd, erro = lerQuantidade(l.form.Quantidade, "Quantidade recebida", false)
		}
	}
	if erro == "" {
		if p, e := q.GetEstoqueProduto(ctx, produto.Int64); e != nil || !p.Ativo {
			erro = "Escolha um produto ativo."
		}
	}
	if erro == "" {
		tx, e := pool.Begin(ctx)
		if e != nil {
			h.falhaInterna(w, r, "iniciar entrada de estoque", e)
			return
		}
		defer tx.Rollback(ctx)
		qt := q.WithTx(tx)
		var lote sqlccliente.EstoqueLote
		lote, err = qt.CreateEstoqueLote(ctx, sqlccliente.CreateEstoqueLoteParams{
			ProdutoID: produto.Int64, NumeroLote: l.form.Numero, Fabricante: l.fabricante, DataFabricacao: l.fabricacao, DataValidade: l.validade,
			Quantidade: numericoDe(qtd, 3), CustoUnitario: l.custo, NotaFiscal: l.nota, Fornecedor: l.fornecedor, Observacao: l.observacao,
		})
		if err == nil {
			err = qt.CreateEstoqueMovimentacao(ctx, sqlccliente.CreateEstoqueMovimentacaoParams{
				LoteID: lote.ID, Tipo: "entrada", Quantidade: numericoDe(qtd, 3), SaldoApos: numericoDe(qtd, 3),
				UsuarioID: idOuNulo(meuID(r)), Descricao: "Entrada do lote", Origem: "entrada",
			})
		}
		if err == nil {
			h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "estoque_lote.criar", Entidade: "estoque_lote", EntidadeID: strconv.FormatInt(lote.ID, 10),
				Rotulo: "Lote " + lote.NumeroLote, Depois: map[string]any{"produto_id": produto.Int64, "numero": lote.NumeroLote, "quantidade": qtdTexto(lote.Quantidade), "validade": dataBR(lote.DataValidade)}})
			if err = tx.Commit(ctx); err == nil {
				http.Redirect(w, r, estoqueLoteCaminho(lote.ID)+"?aviso=criado", http.StatusSeeOther)
				return
			}
		}
		if !violacaoUnica(err) {
			h.falhaInterna(w, r, "registrar entrada de estoque", err)
			return
		}
		erro = "Já existe um lote com esse número para o produto."
	}
	var e error
	if l.form.Produtos, e = h.opcoesDeProdutosEstoque(r, q, produto.Int64); e != nil {
		h.falhaInterna(w, r, "listar produtos de estoque", e)
		return
	}
	l.form.CSRF, l.form.Erro = csrfToken(ctx), erro
	renderStatus(w, r, http.StatusBadRequest, views.EstoqueLoteFicha(h.layout(r, "Nova entrada de estoque", "/estoque/lote"), l.form))
}

func idOuNulo(id int64) pgtype.Int8 {
	if id == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: id, Valid: true}
}

func (h *handlers) carregarEstoqueLote(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.EstoqueLote, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.EstoqueLote{}, false
	}
	l, err := q.GetEstoqueLote(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return l, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar lote de estoque", err)
		return l, false
	}
	return l, true
}

func (h *handlers) renderEstoqueLote(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, x sqlccliente.EstoqueLote, enviado *loteLido, erro string) {
	p, err := q.GetEstoqueProdutoQualquer(r.Context(), x.ProdutoID)
	if err != nil {
		h.falhaInterna(w, r, "buscar produto do lote", err)
		return
	}
	d := views.EstoqueLoteForm{
		Fabricante: x.Fabricante, Fabricacao: dataISO(x.DataFabricacao), Validade: dataISO(x.DataValidade),
		NotaFiscal: x.NotaFiscal, Fornecedor: x.Fornecedor, Observacao: x.Observacao,
	}
	if x.CustoUnitario.Valid {
		d.Custo = qtdTexto(x.CustoUnitario)
	}
	if enviado != nil {
		d = enviado.form
	}
	d.ID, d.ProdutoNome, d.Medida, d.Numero = x.ID, p.Nome, p.UnidadeMedida, x.NumeroLote
	d.Quantidade, d.Saldo = qtdTexto(x.Quantidade), qtdTexto(x.Saldo)
	d.TiposMovimento = marcarOpcao(rotulosMovimentoManual, r.PostFormValue("tipo"))
	d.CSRF, d.Erro, d.PodeEditar = csrfToken(r.Context()), erro, h.principal(r).Pode("estoque_entrada_novo")
	switch r.URL.Query().Get("aviso") {
	case "criado":
		d.Aviso = "Entrada registrada."
	case "salvo":
		d.Aviso = "Alterações salvas."
	case "movimentado":
		d.Aviso = "Movimentação registrada."
	}
	movs, err := q.ListEstoqueMovimentacoes(r.Context(), x.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar movimentações do lote", err)
		return
	}
	for _, m := range movs {
		d.Movimentos = append(d.Movimentos, views.EstoqueMovimento{
			Quando: m.CriadoEm.Time.In(h.fuso(r)).Format("02/01/2006 15:04"), Tipo: rotuloDe(rotulosMovimento, m.Tipo), Quantidade: qtdTexto(m.Quantidade),
			SaldoApos: qtdTexto(m.SaldoApos), Descricao: m.Descricao, Origem: m.Origem, Por: m.Por,
		})
	}
	status := http.StatusOK
	if erro != "" {
		status = http.StatusBadRequest
	}
	renderStatus(w, r, status, views.EstoqueLoteFicha(h.layout(r, "Lote "+x.NumeroLote, "/estoque/lote"), d))
}

func (h *handlers) estoqueLoteFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if x, ok := h.carregarEstoqueLote(w, r, q); ok {
		h.renderEstoqueLote(w, r, q, x, nil, "")
	}
}

func (h *handlers) estoqueLoteSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	x, ok := h.carregarEstoqueLote(w, r, q)
	if !ok {
		return
	}
	l, erro := lerLote(r)
	if erro != "" {
		h.renderEstoqueLote(w, r, q, x, &l, erro)
		return
	}
	if _, err = q.UpdateEstoqueLote(r.Context(), sqlccliente.UpdateEstoqueLoteParams{
		ID: x.ID, Fabricante: l.fabricante, DataFabricacao: l.fabricacao, DataValidade: l.validade, CustoUnitario: l.custo,
		NotaFiscal: l.nota, Fornecedor: l.fornecedor, Observacao: l.observacao,
	}); err != nil {
		h.falhaInterna(w, r, "salvar lote de estoque", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "estoque_lote.editar", Entidade: "estoque_lote", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: "Lote " + x.NumeroLote,
		Antes:  map[string]any{"fabricante": x.Fabricante, "validade": dataBR(x.DataValidade), "nota_fiscal": x.NotaFiscal, "fornecedor": x.Fornecedor},
		Depois: map[string]any{"fabricante": l.fabricante, "validade": dataBR(l.validade), "nota_fiscal": l.nota, "fornecedor": l.fornecedor}})
	http.Redirect(w, r, estoqueLoteCaminho(x.ID)+"?aviso=salvo", http.StatusSeeOther)
}

// estoqueLoteMovimentar lança entrada adicional, perda, baixa ou ajuste do saldo, com o histórico, numa só transação.
func (h *handlers) estoqueLoteMovimentar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	x, ok := h.carregarEstoqueLote(w, r, q)
	if !ok {
		return
	}
	tipo, desc := r.PostFormValue("tipo"), strings.TrimSpace(r.PostFormValue("descricao"))
	valor, erro := lerQuantidade(r.PostFormValue("quantidade"), "Quantidade", tipo == "ajuste")
	switch {
	case !validoEm(rotulosMovimentoManual, tipo):
		erro = "Escolha o tipo da movimentação."
	case erro == "" && (desc == "" || len([]rune(desc)) > 255):
		erro = "Informe o motivo (até 255 caracteres)."
	}
	if erro != "" {
		h.renderEstoqueLote(w, r, q, x, nil, erro)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar movimentação de estoque", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := q.WithTx(tx)
	atual, err := qt.GetEstoqueLoteParaAjuste(ctx, x.ID) // trava a linha: dois lançamentos ao mesmo tempo não se atropelam
	if err != nil {
		h.falhaInterna(w, r, "travar lote de estoque", err)
		return
	}
	saldo, delta := qtdFloat(atual.Saldo), valor
	switch tipo {
	case "entrada":
		saldo += valor
	case "perda", "baixa":
		saldo, delta = saldo-valor, -valor
	case "ajuste":
		delta, saldo = valor-saldo, valor
	}
	saldo = math.Round(saldo*1000) / 1000
	delta = math.Round(delta*1000) / 1000
	if saldo < 0 {
		h.renderEstoqueLote(w, r, q, atual, nil, "O saldo do lote ("+qtdTexto(atual.Saldo)+") não cobre essa saída.")
		return
	}
	if saldo > 999999999 {
		h.renderEstoqueLote(w, r, q, atual, nil, "O saldo ficaria grande demais.")
		return
	}
	if err = qt.SetEstoqueLoteSaldo(ctx, sqlccliente.SetEstoqueLoteSaldoParams{ID: x.ID, Saldo: numericoDe(saldo, 3)}); err == nil {
		err = qt.CreateEstoqueMovimentacao(ctx, sqlccliente.CreateEstoqueMovimentacaoParams{
			LoteID: x.ID, Tipo: tipo, Quantidade: numericoDe(delta, 3), SaldoApos: numericoDe(saldo, 3), UsuarioID: idOuNulo(meuID(r)), Descricao: desc, Origem: "manual",
		})
	}
	if err != nil {
		h.falhaInterna(w, r, "registrar movimentação de estoque", err)
		return
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "estoque_lote.movimentar", Entidade: "estoque_lote", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: "Lote " + x.NumeroLote,
		Antes: map[string]any{"saldo": qtdTexto(atual.Saldo)}, Depois: map[string]any{"saldo": qtdTexto(numericoDe(saldo, 3)), "tipo": tipo, "motivo": desc}})
	if err = tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "gravar movimentação de estoque", err)
		return
	}
	http.Redirect(w, r, estoqueLoteCaminho(x.ID)+"?aviso=movimentado", http.StatusSeeOther)
}

func (h *handlers) estoqueLoteExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if x, ok := h.carregarEstoqueLote(w, r, q); ok {
		h.excluirRegistro(w, r, "estoque_lote", x.ID, "/estoque/lote?aviso=excluido", estoqueLoteCaminho(x.ID))
	}
}

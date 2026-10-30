package views

import "net/url"

// Estoque (11.1): unidades, produtos e lotes.

type EstoqueUnidadeLinha struct {
	IDTexto, Nome, Descricao string
	Ativo                    bool
	Produtos                 int32
}

type EstoqueUnidadesData struct {
	Linhas    []EstoqueUnidadeLinha
	PodeGerir bool
}

type EstoqueUnidadeForm struct {
	ID                int64
	Nome, Descricao   string
	Ativo             bool
	PodeEditar        bool
	Erro, Aviso, CSRF string
}

type EstoqueProdutoLinha struct {
	IDTexto, Nome, Tipo, Unidade, Medida, Saldo string
	Fracionavel, Ativo                          bool
}

type EstoqueProdutoFiltro struct{ Busca, Tipo, Unidade, Situacao string }

type EstoqueProdutosData struct {
	Linhas     []EstoqueProdutoLinha
	Filtro     EstoqueProdutoFiltro
	Tipos      []CidadeOpcao
	Unidades   []CidadeOpcao
	Situacoes  []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNovo   bool
}

type EstoqueProdutoForm struct {
	ID                                           int64
	Nome, Medida, Doses, Descricao               string
	Fracionavel, SelecionavelDiaria, Ativo       bool
	Unidades, Tipos, Saidas, Fracoes, Categorias []CidadeOpcao
	Saldo                                        string
	PodeEditar                                   bool
	Erro, Aviso, CSRF                            string
}

func (d EstoqueProdutoForm) Novo() bool { return d.ID == 0 }

type EstoqueLoteLinha struct {
	IDTexto, Produto, Numero, Validade, Quantidade, Saldo, Medida, Fornecedor, Alerta string
}

type EstoqueLoteFiltro struct{ Busca, Produto, Situacao string }

type EstoqueLotesData struct {
	Linhas     []EstoqueLoteLinha
	Filtro     EstoqueLoteFiltro
	Produtos   []CidadeOpcao
	Situacoes  []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNovo   bool
}

type EstoqueMovimento struct {
	Quando, Tipo, Quantidade, SaldoApos, Descricao, Origem, Por string
}

type EstoqueLoteForm struct {
	ID                                                   int64
	Produtos                                             []CidadeOpcao
	ProdutoNome, Medida                                  string
	Numero, Fabricante, Fabricacao, Validade, Quantidade string
	Saldo, Custo, NotaFiscal, Fornecedor, Observacao     string
	Movimentos                                           []EstoqueMovimento
	TiposMovimento                                       []CidadeOpcao
	PodeEditar                                           bool
	Erro, Aviso, CSRF                                    string
}

func (d EstoqueLoteForm) Novo() bool { return d.ID == 0 }

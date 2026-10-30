package views

import "net/url"

// Estoque (11.2): retiradas e balanço físico.

type EstoqueRetiradaLinha struct {
	IDTexto, Tipo, Responsavel, Saida, Previsto, Retorno, Status string
	Itens                                                        int
}

type EstoqueRetiradaFiltro struct{ Status, Tipo, Responsavel string }

type EstoqueRetiradasData struct {
	Linhas        []EstoqueRetiradaLinha
	Filtro        EstoqueRetiradaFiltro
	Status, Tipos []CidadeOpcao
	Responsaveis  []CidadeOpcao
	Pag           Pagination
	Query         url.Values
	Ordem, Dir    string
	PodeNova      bool
}

type EstoqueRetiradaItem struct {
	ID, Produto, Lote, Medida, Retirado, Usado, Devolvido, Perda string
}

type EstoqueRetiradaForm struct {
	ID                                      int64
	Tipo, TipoRotulo, Status, StatusRotulo  string
	Tipos, Responsaveis, Lotes              []CidadeOpcao
	Saida, Previsto, Retorno, Observacao    string
	Itens                                   []EstoqueRetiradaItem
	PodeEditar, PodeFinalizar, PodeCancelar bool
	Erro, Aviso, CSRF                       string
}

func (d EstoqueRetiradaForm) Novo() bool   { return d.ID == 0 }
func (d EstoqueRetiradaForm) Aberta() bool { return d.Status == "aberta" }
func (d EstoqueRetiradaForm) Campo() bool  { return d.Tipo == "campo" }

type EstoqueBalancoLinha struct {
	Unidade, Produto, Tipo, Medida, EmEstoque, EmRetirada, Total, Vencido, ProximoVencimento string
}

type EstoqueBalancoData struct {
	Linhas   []EstoqueBalancoLinha
	Unidade  string
	Tipo     string
	Unidades []CidadeOpcao
	Tipos    []CidadeOpcao
	Pag      Pagination
	Query    url.Values
}

package views

import "net/url"

// Patrimônio (11.3).

type PatrimonioLinha struct {
	IDTexto, Numero, Serie, Nome, Responsavel, Entrada, Status string
}

type PatrimonioFiltro struct{ Busca, Status, Responsavel string }

type PatrimoniosData struct {
	Linhas       []PatrimonioLinha
	Filtro       PatrimonioFiltro
	Status       []CidadeOpcao
	Responsaveis []CidadeOpcao
	Pag          Pagination
	Query        url.Values
	Ordem, Dir   string
	PodeNovo     bool
}

type PatrimonioEvento struct{ Quando, Tipo, Descricao, Por string }

type PatrimonioForm struct {
	ID                                       int64
	Numero, Serie, Nome, Entrada, Observacao string
	Status, StatusRotulo, Responsavel        string
	Lotes, Responsaveis                      []CidadeOpcao
	Eventos                                  []PatrimonioEvento
	PodeEditar, PodeAtribuir, PodeManutencao bool
	PodeAnotar, PodeBaixa, PodeReativar      bool
	PodeExcluir                              bool
	Erro, Aviso, CSRF                        string
}

func (d PatrimonioForm) Novo() bool { return d.ID == 0 }

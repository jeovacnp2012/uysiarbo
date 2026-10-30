package views

import "net/url"

type LvcLinha struct {
	ID                                    int64
	IDTexto, Data, Endereco, Proprietario string
	Cachorro, Dpp, Reteste, Situacao, Por string
}

type LvcFiltro struct{ Busca, Dpp, Situacao, Bairro, De, Ate string }

type LvcData struct {
	Linhas     []LvcLinha
	Filtro     LvcFiltro
	Dpps       []CidadeOpcao
	Situacoes  []CidadeOpcao
	Bairros    []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNovo   bool
}

type LvcFormData struct {
	ID                                  int64
	LocalID                             int64
	LocalEndereco                       string
	Data, Nome, Raca, Pelagem, Idade    string
	Proprietario, Telefone, Cpf         string
	Observacao                          string
	Sexos, Dpps, Retestes, Situacoes    []CidadeOpcao
	Reteste                             bool
	DataPedido, DataRecebido, DataMorte string
	PodeEditar                          bool
	Erro, Aviso, CSRF                   string
}

func (d LvcFormData) Novo() bool { return d.ID == 0 }

// LvcSecaoFicha é a seção de LVC da ficha do local.
type LvcSecaoFicha struct {
	Pode, PodeNovo bool
	Linhas         []LvcFichaLinha
}

type LvcFichaLinha struct {
	ID                                 int64
	Data, Nome, Dpp, Reteste, Situacao string
}

package views

import "net/url"

type CampanhaLinha struct {
	ID                              int64
	IDTexto, Data, Endereco, Bairro string
	Situacao, Aplicador             string
	Vacinados, NaoVacinados         int
	Estoque                         bool
}

type CampanhasFiltro struct{ Busca, Situacao, Bairro, De, Ate string }

type CampanhasData struct {
	Linhas     []CampanhaLinha
	Filtro     CampanhasFiltro
	Situacoes  []CidadeOpcao
	Bairros    []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNova   bool
}

// CampanhaEscolherLocal é o primeiro passo da nova campanha quando não se vem da ficha do local.
type CampanhaEscolherLocal struct {
	Busca  string
	Buscou bool
	Locais []LocalVinculo
}

// CampanhaFormData serve à nova campanha e à edição (ID 0 = nova).
type CampanhaFormData struct {
	ID            int64
	LocalID       int64
	LocalEndereco string
	Data          string // aaaa-mm-ddThh:mm
	Situacoes     []CidadeOpcao
	Aplicadores   []CidadeOpcao
	Aplicador     string
	Diarias       []CidadeOpcao
	Diaria        string
	Vac, Nao      [2]string // cachorros, gatos
	Descricao     string
	Estoque       bool
	PodeEditar    bool
	Erro, Aviso   string
	CSRF          string
}

func (d CampanhaFormData) Novo() bool { return d.ID == 0 }

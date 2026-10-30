package views

import "net/url"

type DenunciaLinha struct {
	ID             int64
	IDTexto        string
	Titulo         string
	Tipo           string
	Cidadao        string
	Data           string
	Endereco       string
	Bairro         string
	Origem         string
	Situacao       string
	SituacaoRotulo string
	Responsavel    string
}

type DenunciasFiltro struct{ Busca, Tipo, Situacao, Origem, Anonima, Bairro, De, Ate string }

type DenunciasData struct {
	Linhas     []DenunciaLinha
	Filtro     DenunciasFiltro
	Tipos      []CidadeOpcao
	Situacoes  []CidadeOpcao
	Origens    []CidadeOpcao
	Anonimas   []CidadeOpcao
	Bairros    []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNova   bool
}

// DenunciaFormData são os dados editáveis da denúncia (nova ou existente).
type DenunciaFormData struct {
	ID         int64 // 0 = nova
	Anonima    bool
	Nome       string
	Telefone   string
	CPF        string
	Tipos      []CidadeOpcao
	Origens    []CidadeOpcao
	Bairros    []CidadeOpcao
	Tipo       string
	Origem     string
	Titulo     string
	Descricao  string
	Data       string // aaaa-mm-dd
	Endereco   string
	Anteriores []DenunciaAnterior
	Erro       string
	CSRF       string
}

func (d DenunciaFormData) Novo() bool { return d.ID == 0 }

// DenunciaAnterior é uma denúncia antiga do mesmo telefone.
type DenunciaAnterior struct {
	ID       int64
	Titulo   string
	Data     string
	Situacao string
}

type DenunciaFichaData struct {
	ID             int64
	IDTexto        string
	Titulo         string
	Tipo           string
	Origem         string
	Data           string
	Descricao      string
	Anonima        bool
	Cidadao        string
	Telefone       string
	Endereco       string
	Bairro         string
	LocalID        int64
	LocalEndereco  string
	VisitaID       int64
	Situacao       string
	SituacaoRotulo string
	Responsavel    string
	Fechada        bool
	Dados          DenunciaFormData
	PodeEditar     bool
	PodeLocal      bool
	PodeVisita     bool
	Busca          string
	Buscou         bool
	Locais         []LocalVinculo
	Aviso          string
	CSRF           string
	Caso           FichaCasoData
}

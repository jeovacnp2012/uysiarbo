package views

import "net/url"

type BaapLinha struct {
	ID             int64
	IDTexto        string
	Sinam          string
	Paciente       string
	Animal         string
	Data           string
	Endereco       string
	Bairro         string
	Situacao       string
	SituacaoRotulo string
	Responsavel    string
}

type BaapFiltro struct{ Busca, Situacao, Animal, Bairro, De, Ate string }

type BaapData struct {
	Linhas     []BaapLinha
	Filtro     BaapFiltro
	Situacoes  []CidadeOpcao
	Animais    []CidadeOpcao
	Bairros    []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNova   bool
}

// BaapFormData são os dados editáveis da notificação (nova ou existente).
type BaapFormData struct {
	ID               int64 // 0 = nova
	Sinam            string
	Data             string // aaaa-mm-dd
	Paciente         string
	NomeMae          string
	Nascimento       string // aaaa-mm-dd
	Telefone         string
	Telefone2        string
	Animais          []CidadeOpcao
	LocaisPicada     []CidadeOpcao
	Animal           string
	AnimalOutro      string
	Especie          string
	LocalPicada      string
	SituacaoPaciente string
	AtuacaoVGA       string
	Observacao       string
	Endereco         string
	Numero           string
	Referencia       string
	Bairros          []CidadeOpcao
	Erro             string
	CSRF             string
}

func (d BaapFormData) Novo() bool { return d.ID == 0 }

type BaapFichaData struct {
	ID             int64
	IDTexto        string
	Sinam          string
	Paciente       string
	Animal         string
	Especie        string
	LocalPicada    string
	Data           string
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
	Dados          BaapFormData
	PodeEditar     bool
	PodeAtender    bool
	PodeEncerrar   bool
	PodeLocal      bool
	PodeVisita     bool
	Busca          string
	Buscou         bool
	Locais         []LocalVinculo
	Aviso          string
	CSRF           string
	Caso           FichaCasoData
}

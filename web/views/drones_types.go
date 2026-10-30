package views

import "net/url"

type DroneLinha struct {
	ID             int64
	IDTexto        string
	Descricao      string
	Data           string
	Endereco       string
	Bairro         string
	AbertoPor      string
	Situacao       string
	SituacaoRotulo string
	Responsavel    string
}

type DronesFiltro struct{ Busca, Situacao, Bairro, De, Ate string }

type DronesData struct {
	Linhas     []DroneLinha
	Filtro     DronesFiltro
	Situacoes  []CidadeOpcao
	Bairros    []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNova   bool
}

// DroneEscolherLocal é o primeiro passo da nova solicitação: achar o local.
type DroneEscolherLocal struct {
	Busca  string
	Buscou bool
	Locais []LocalVinculo
}

// DroneFormData é a nova solicitação (o local já está escolhido).
type DroneFormData struct {
	LocalID       int64
	LocalEndereco string
	Descricao     string
	Data          string // aaaa-mm-dd
	Erro          string
	CSRF          string
}

type DroneFichaData struct {
	ID             int64
	IDTexto        string
	Descricao      string
	Data           string
	AbertoPor      string
	Bairro         string
	LocalID        int64
	LocalEndereco  string
	VisitaID       int64
	Situacao       string
	SituacaoRotulo string
	Responsavel    string
	Fechada        bool
	PodeVisita     bool
	PodeAtender    bool
	PodeEncerrar   bool
	Aviso          string
	CSRF           string
	Caso           FichaCasoData
}

package views

import "net/url"

// ---- Notificações (9.4)

type NotificacaoLinha struct {
	ID             int64
	Numero         string
	Motivo         string
	Data           string
	Prazo          string
	Vencida        bool
	Endereco       string
	Bairro         string
	Situacao       string
	SituacaoRotulo string
	Auto           string
}

type NotificacoesFiltro struct{ Busca, Situacao, Vencidas, Bairro, De, Ate string }

type NotificacoesData struct {
	Linhas     []NotificacaoLinha
	Filtro     NotificacoesFiltro
	Situacoes  []CidadeOpcao
	Vencidas   []CidadeOpcao
	Bairros    []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNova   bool
}

// NotificacaoEscolherLocal é o primeiro passo da nova notificação: achar o local.
type NotificacaoEscolherLocal struct {
	Busca  string
	Buscou bool
	Locais []LocalVinculo
}

type NotificacaoFormData struct {
	ID            int64 // 0 = nova
	LocalID       int64
	LocalEndereco string
	Motivo        string
	Descricao     string
	Data          string // aaaa-mm-dd
	Prazo         string
	Trecho        string
	Visita        string
	Trechos       []CidadeOpcao
	Erro          string
	CSRF          string
}

func (d NotificacaoFormData) Novo() bool { return d.ID == 0 }

type AutoResumo struct {
	ID       int64
	Numero   string
	Rotulo   string
	Situacao string
	Valor    string
	Data     string
}

type NotificacaoFichaData struct {
	ID             int64
	Numero         string
	Motivo         string
	Descricao      string
	Data           string
	Prazo          string
	Vencida        bool
	Situacao       string
	SituacaoRotulo string
	Fechada        bool
	LocalID        int64
	LocalEndereco  string
	Bairro         string
	Trecho         string
	VisitaID       int64
	Autos          []AutoResumo
	PodeEditar     bool
	PodeAuto       bool
	Dados          NotificacaoFormData
	AutoForm       AutoFormData
	Aviso          string
	CSRF           string
	Caso           FichaCasoData
}

// ---- Autos de infração

type AutoLinha struct {
	ID                int64
	Numero            string
	NotificacaoID     int64
	NotificacaoNumero string
	Endereco          string
	Bairro            string
	Valor             string
	Data              string
	Vencimento        string
	Vencido           bool
	Situacao          string
	SituacaoRotulo    string
}

type AutosFiltro struct{ Busca, Situacao, Vencidos, Bairro, De, Ate string }

type AutosData struct {
	Linhas     []AutoLinha
	Filtro     AutosFiltro
	Situacoes  []CidadeOpcao
	Vencidos   []CidadeOpcao
	Bairros    []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
}

type AutoFormData struct {
	ID            int64 // 0 = novo
	NotificacaoID int64
	Descricao     string
	Valor         string // 1.234,56
	Data          string
	Vencimento    string
	Trecho        string
	Trechos       []CidadeOpcao
	Erro          string
	CSRF          string
}

func (d AutoFormData) Novo() bool { return d.ID == 0 }

type AutoFichaData struct {
	ID                int64
	Numero            string
	NotificacaoID     int64
	NotificacaoNumero string
	LocalID           int64
	LocalEndereco     string
	Descricao         string
	Trecho            string
	Valor             string
	Data              string
	Vencimento        string
	Vencido           bool
	PagoEm            string
	Situacao          string
	SituacaoRotulo    string
	PodeEditar        bool
	Dados             AutoFormData
	Aviso             string
	CSRF              string
	Caso              FichaCasoData
}

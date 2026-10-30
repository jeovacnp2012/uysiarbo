package views

// CasoEvento é uma linha da linha do tempo de um caso.
type CasoEvento struct {
	Evento  string // criacao, comentario, acao, encaminhamento, anexo
	Texto   string
	Autor   string
	Destino string // quem recebeu o caso (encaminhamento)
	De      string // situação anterior (ação)
	Para    string // situação nova (ação)
	Quando  string
}

type CasoAnexo struct {
	ID      int64
	Nome    string
	Tamanho string
	Quando  string
	Autor   string
	Imagem  bool
}

// CasoAcao é um botão de ação do caso; quem o monta (cada tipo de caso) define o endereço que recebe o POST.
type CasoAcao struct {
	Rotulo    string
	URL       string
	Icone     string
	Confirmar string
	Perigo    bool
}

// FichaCasoData alimenta o componente FichaCaso, o mesmo para todo tipo de caso.
type FichaCasoData struct {
	Tipo           string
	ID             int64
	CSRF           string
	Eventos        []CasoEvento
	Anexos         []CasoAnexo
	Responsavel    string
	Usuarios       []CidadeOpcao
	Acoes          []CasoAcao
	PodeEditar     bool
	PodeEncaminhar bool
	Erro           string
}

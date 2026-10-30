package views

import "net/url"

// Comunicações (12.1).

type ComunicacaoLinha struct {
	IDTexto, Canal, Destino, Resumo, Status, Tentativas, Criada string
}

type ComunicacaoFiltro struct{ Busca, Canal, Status string }

type ComunicacoesData struct {
	Linhas     []ComunicacaoLinha
	Filtro     ComunicacaoFiltro
	Canais     []CidadeOpcao
	Statuses   []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNova   bool
	EmFila     int
	Aviso      string
	CSRF       string
}

type ComunicacaoNova struct {
	Canal, Destinatario, Nome, Assunto, Corpo string
	Canais                                    []CidadeOpcao
	Erro, CSRF                                string
}

type ComunicacaoEvento struct{ Quando, Tipo, Descricao, Por string }

type ComunicacaoFicha struct {
	ID                                                         int64
	Canal, Destinatario, Nome, Assunto, Corpo                  string
	Status, StatusRotulo                                       string
	Tentativas, MaxTentativas                                  int
	UltimoErro, Provedor, IDProvedor, Origem                   string
	Criada, CriadaPor, Enviada, Entregue, Visualizada, Proxima string
	Eventos                                                    []ComunicacaoEvento
	PodeEnviar, PodeCancelar, PodeReenviar                     bool
	PodeStatus, PodeVisualizar                                 bool
	Erro, Aviso, CSRF                                          string
}

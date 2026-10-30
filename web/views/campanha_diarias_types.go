package views

import "net/url"

type DiariaLinha struct {
	ID                                  int64
	IDTexto, Status, StatusRotulo, Data string
	Por, Equipe                         string
}

type DiariasFiltro struct{ Status, Agente, De, Ate string }

type DiariasData struct {
	Linhas     []DiariaLinha
	Filtro     DiariasFiltro
	Status     []CidadeOpcao
	Agentes    []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNova   bool
}

type DiariaNovaData struct {
	Data, Anotacao, Erro, CSRF string
}

type DiariaCampo struct{ Nome, Rotulo, Valor string }

type DiariaGrupo struct {
	Titulo string
	Campos []DiariaCampo
}

type DiariaTemperatura struct{ Horario, Estoque, Uso string }

type DiariaMembro struct {
	ID    int64
	Nome  string
	Lider bool
}

type DiariaAtendimento struct {
	ID                      int64
	Hora, Endereco          string
	Vacinados, NaoVacinados int
}

type DiariaFichaData struct {
	ID                                int64
	IDTexto, Data, DataISO, Anotacao  string
	Status, StatusRotulo, CriadoPor   string
	Aberta                            bool
	PodeEditar, PodeRegistrar         bool
	PodeFinalizar, PodeDesativar      bool
	Grupos                            []DiariaGrupo
	Recebimento                       string
	Temperaturas                      []DiariaTemperatura
	Equipe                            []DiariaMembro
	Candidatos                        []CidadeOpcao
	Atendimentos                      []DiariaAtendimento
	VacinaLotes, SeringaLotes         []CidadeOpcao
	Baixas                            []DiariaBaixa
	EstoqueObrigatorio                bool
	TotalVacinados, TotalNaoVacinados int
	Erro, Aviso, CSRF                 string
}

// DiariaBaixa é o consumo de estoque lançado ao finalizar a diária (11.4).
type DiariaBaixa struct{ Categoria, Produto, Lote, Quantidade string }

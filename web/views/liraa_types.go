package views

import "net/url"

type LiraaGrupoLinha struct {
	ID      int64
	IDTexto string
	Nome    string
	Ativo   bool
	Bairros int32
}

type LiraaGruposData struct {
	Linhas    []LiraaGrupoLinha
	PodeGerir bool
}

type LiraaGrupoData struct {
	ID                int64
	Nome              string
	Ativo             bool
	Bairros           []CidadeOpcao
	Candidatos        []CidadeOpcao
	PodeEditar        bool
	Erro, Aviso, CSRF string
}

type LiraaAgendaLinha struct {
	IDTexto, Ciclo, Bairro, Quarteirao, Agente, Situacao string
	Semana                                               int
}

type LiraaAgendaFiltro struct{ Ciclo, Semana, Bairro, Agente, Situacao string }

type LiraaAgendaData struct {
	Linhas     []LiraaAgendaLinha
	Filtro     LiraaAgendaFiltro
	Ciclos     []CidadeOpcao
	Bairros    []CidadeOpcao
	Agentes    []CidadeOpcao
	Situacoes  []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNova   bool
}

type LiraaAgendaForm struct {
	ID                                    int64
	Semana                                string
	Ciclos, Bairros, Quarteiroes, Agentes []CidadeOpcao
	Situacoes                             []CidadeOpcao
	PodeEditar                            bool
	Erro, Aviso, CSRF                     string
}

func (d LiraaAgendaForm) Novo() bool { return d.ID == 0 }

package views

import (
	"net/url"
	"time"
)

// ---- Visitas ----

// VisitaCampos são os valores digitados no formulário da visita (texto, como chegou).
type VisitaCampos struct {
	Ciclo, Atividade, Agente, Data, Hora, Tipo, Pendencia string
	Achou                                                 bool
	A1, A2, B, C, D1, D2, E, TotalEliminado               string
	Amostra, Tratamento                                   bool
	L1Gramas, L1Tratados, L2Gramas, L2Tratados            string
	AdulticidaTipo, AdulticidaCargas, TratEliminados, Bri string
	Anotacao, Assinatura                                  string
	Lat, Lon, Precisao                                    string
}

type VisitaFotoLinha struct {
	ID      int64
	Nome    string
	Tamanho string
}

type VisitaAnotacaoLinha struct {
	Autor  string
	Quando time.Time
	Texto  string
}

type VisitaFormData struct {
	ID         int64 // 0 = visita nova
	LocalID    int64
	Local      string // endereço do local
	Campos     VisitaCampos
	Ciclos     []CidadeOpcao
	Atividade  []CidadeOpcao
	Agentes    []CidadeOpcao
	Tipos      []CidadeOpcao
	Pendencias []CidadeOpcao
	Semana     string
	Ativo      bool

	PodeEditar          bool
	PodeEditarPendencia bool
	PodeAnotar          bool
	PodeExcluir         bool
	Fotos               []VisitaFotoLinha
	Anotacoes           []VisitaAnotacaoLinha
	Aviso               string
	Erro                string
	ErroFoto            string
	CSRF                string
	Fuso                *time.Location
}

func (d VisitaFormData) Novo() bool { return d.ID == 0 }

type VisitaLinha struct {
	ID        int64
	Data      string
	Endereco  string
	Agente    string
	Ciclo     string
	Semana    string
	Atividade string
	Tipo      string
	Pendencia string
	Deposito  bool
	Amostra   bool
	Tratou    bool
	Ativo     bool
}

type VisitasData struct {
	Linhas      []VisitaLinha
	LocalID     int64
	Local       string
	PodeNova    bool
	Filtro      VisitasFiltro
	PodeLote    bool
	Aviso       string
	Pag         Pagination
	Query       url.Values
	Ciclos      []CidadeOpcao
	Atividades  []CidadeOpcao
	Agentes     []CidadeOpcao
	Bairros     []CidadeOpcao
	Quarteiroes []CidadeOpcao
	Pendencias  []CidadeOpcao
	Deposito    []CidadeOpcao
	Amostra     []CidadeOpcao
	Tratamento  []CidadeOpcao
	Foto        []CidadeOpcao
}

// VisitasFiltro são os filtros da listagem de visitas, como chegaram na URL.
type VisitasFiltro struct {
	ID, Endereco, Numero, Bairro, Quarteirao, Lado string
	Ciclo, Agente, Atividade, Semana, De, Ate      string
	Pendencia, Deposito, Amostra, Tratamento, Foto string
	Situacao                                       string
}

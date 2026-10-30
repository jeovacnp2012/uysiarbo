package views

import "net/url"

type InvestigacaoLinha struct {
	ID              int64
	IDTexto         string
	Paciente        string
	Telefone        string
	Doenca          string
	Data            string
	Endereco        string
	Situacao        string
	SituacaoRotulo  string
	Resultado       string
	ResultadoRotulo string
	Responsavel     string
	TemLocal        bool
}

type InvestigacoesFiltro struct{ Busca, Doenca, Situacao, Resultado, De, Ate string }

type InvestigacoesData struct {
	Linhas     []InvestigacaoLinha
	Filtro     InvestigacoesFiltro
	Doencas    []CidadeOpcao
	Situacoes  []CidadeOpcao
	Resultados []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNova   bool
}

// InvestigacaoFormData são os dados editáveis da investigação (nova ou existente).
type InvestigacaoFormData struct {
	ID         int64 // 0 = nova
	Doencas    []CidadeOpcao
	Paciente   string
	Telefone   string
	DataInicio string // aaaa-mm-dd
	Endereco   string
	Observacao string
	Erro       string
	CSRF       string
}

func (d InvestigacaoFormData) Novo() bool { return d.ID == 0 }

// LocalVinculo é um local encontrado na busca para vincular à investigação.
type LocalVinculo struct {
	ID       int64
	Endereco string
	Bairro   string
}

// EntrevistaCampo é uma pergunta já pronta para a tela (com a resposta atual).
type EntrevistaCampo struct {
	Nome, Label, Tipo, Placeholder string
	Obrigatorio                    bool
	Opcoes                         []string
	Valor                          string
	Valores                        []string
	CondCampo, CondOp, CondValor   string
	Oculto                         bool
}

func (c EntrevistaCampo) Marcado(op string) bool {
	if c.Tipo == "multi_select" {
		for _, v := range c.Valores {
			if v == op {
				return true
			}
		}
		return false
	}
	return c.Valor == op
}

type InvestigacaoFichaData struct {
	ID               int64
	IDTexto          string
	Doenca           string
	Paciente         string
	Telefone         string
	Data             string
	Endereco         string
	LocalID          int64
	LocalEndereco    string
	Situacao         string
	SituacaoRotulo   string
	ResultadoRotulo  string
	Resultado        string
	Responsavel      string
	Observacao       string
	Encerrada        bool
	Dados            InvestigacaoFormData
	PodeEditar       bool
	PodeLocal        bool
	PodeEntrevista   bool
	Busca            string
	Buscou           bool
	Locais           []LocalVinculo
	EntrevistaTitulo string
	Entrevista       []EntrevistaCampo
	ErroEntrevista   string
	Aviso            string
	CSRF             string
	Caso             FichaCasoData
}

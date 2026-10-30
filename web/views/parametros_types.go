package views

// ParametroCampo é um parâmetro do cliente na tela Geral ou Mobilidade.
type ParametroCampo struct {
	Chave    string
	Rotulo   string
	Ajuda    string
	Tipo     string // "bool", "int", "cep" ou "senha"
	Valor    string
	Marcado  bool // bool
	Definida bool // senha: já existe uma salva
	Min, Max int
	Opcoes   []ParametroOpcao
}

type ParametroOpcao struct{ Valor, Rotulo string }

type ParametrosData struct {
	Titulo    string
	Descricao string
	Acao      string
	Campos    []ParametroCampo
	Salvo     bool
	Erro      string
	CSRF      string
	Abas      []AbaParam
}

// AbaParam é uma aba da área Parâmetros (cada uma leva a uma tela).
type AbaParam struct {
	Rotulo string
	Href   string
	Ativa  bool
}

type SemanaLinha struct {
	Ano, Semanas     int
	Primeiro, Ultimo string
}

type SemanaDetalhe struct {
	Numero      int
	Inicio, Fim string
}

type SemanasData struct {
	Abas    []AbaParam
	Tipo    string
	Anos    []SemanaLinha
	Ano     int
	Detalhe []SemanaDetalhe
	Pode    bool
	CSRF    string
}

type NumeracaoLinha struct {
	Tipo, Prefixo, Formato, Ultimo, Proximo string
}

type NumeracaoData struct {
	Abas   []AbaParam
	Ano    int
	Linhas []NumeracaoLinha
}

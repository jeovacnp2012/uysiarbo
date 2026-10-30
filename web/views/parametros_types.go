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
}

type ParametrosData struct {
	Titulo    string
	Descricao string
	Acao      string
	Campos    []ParametroCampo
	Salvo     bool
	Erro      string
	CSRF      string
}

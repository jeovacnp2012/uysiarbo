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
	SimNao   [2]string // bool mostrado como select: rótulos de Sim e de Não
	SoSe     string    // só aparece quando todos os "campo=valor" (separados por ;) forem verdadeiros
	Cartao   bool      // vai para o cartão da direita
	Unidade  string
}

type ParametroOpcao struct{ Valor, Rotulo string }

type ParametrosData struct {
	Titulo       string
	Descricao    string
	Acao         string
	Campos       []ParametroCampo
	Salvo        bool
	Erro         string
	CSRF         string
	CartaoTitulo string
	CartaoTexto  string
	TemCartao    bool
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
	Tipo    string
	Anos    []SemanaLinha
	Ano     int // ano com as semanas abertas (?ano=)
	Detalhe []SemanaDetalhe
	CSRF    string
	Erro    string
	// Formulário (novo ou edição): Editando = ano existente, que não muda.
	Editando           bool
	FAno               string
	FPrimeiro, FUltimo string // aaaa-mm-dd
	FSemanas           string
}

type NumeracaoLinha struct {
	Tipo, Prefixo, Formato, ResetAnual, Ultimo, Proximo string
}

type NumeracaoData struct {
	Ano    int
	Linhas []NumeracaoLinha
}

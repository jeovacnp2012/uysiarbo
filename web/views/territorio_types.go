package views

import "net/url"

// ---- Cidades ----

type CidadeLinha struct {
	ID      int64
	Nome    string
	UF      string
	IBGE    string
	Ativo   bool
	Bairros int
}

type CidadeFormData struct {
	ID         int64 // 0 = cidade nova
	Nome       string
	UF         string
	IBGE       string
	Ativo      bool
	PodeEditar bool
	Erro       string
	CSRF       string
}

func (d CidadeFormData) Novo() bool { return d.ID == 0 }

// ---- Bairros ----

type CidadeOpcao struct {
	ID          int64
	Rotulo      string
	Selecionada bool
}

type BairroLinha struct {
	ID            int64
	Nome          string
	Cidade        string
	Responsaveis  string
	Status        string
	Categoria     string
	Classificacao string
	Mutirao       bool
	Quarteiroes   int
}

type BairrosFiltro struct{ Busca, Cidade, Status, Categoria string }

type BairrosData struct {
	Filtro     BairrosFiltro
	Cidades    []CidadeOpcao
	Linhas     []BairroLinha
	Pag        Pagination
	Query      url.Values
	PodeEditar bool
	Restrito   bool // a pessoa só vê os bairros atribuídos a ela
}

type ResponsavelOpcao struct {
	ID      int64
	Nome    string
	Email   string
	Marcado bool
}

type QuarteiraoLinha struct {
	ID    int64
	Nome  string
	Ativo bool
}

type BairroFormData struct {
	ID            int64 // 0 = bairro novo
	Cidades       []CidadeOpcao
	Nome          string
	Mutirao       bool
	Producao      bool
	Status        string
	Classificacao string
	Categoria     string
	Ibge          string
	Zona          string
	Latitude      string
	Longitude     string
	Altitude      string
	Distancia     string
	Responsaveis  []ResponsavelOpcao
	Quarteiroes   []QuarteiraoLinha
	PodeEditar    bool
	Aviso         string // mensagem de sucesso (ex.: quarteirões criados)
	Erro          string
	CSRF          string
}

func (d BairroFormData) Novo() bool { return d.ID == 0 }

// Modulo é um bloco do catálogo no painel inicial; Href vazio = ainda em construção.
type Modulo struct {
	Nome string
	Href string
}

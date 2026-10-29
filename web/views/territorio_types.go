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
	Valor       string // texto do option, quando não é um número (ex.: tipo de local)
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
	ID        int64
	Nome      string
	Ativo     bool
	ComLimite bool // tem polígono gravado no mapa
}

// LimiteInfo descreve o polígono gravado de um bairro.
type LimiteInfo struct {
	AreaHa    string // hectares, já formatado
	Latitude  string
	Longitude string
	GeoJSON   string
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
	Limite        *LimiteInfo // nil = bairro sem limite no mapa
	ErroLimite    string      // erro ao salvar o limite ou importar quarteirões
	GeoJSONTexto  string      // o que a pessoa colou, para não perder quando há erro
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

// ---- Locais ----

type LocalLinha struct {
	ID           int64
	Endereco     string
	Tipo         string
	Bairro       string
	Quarteirao   string
	Lado         string
	Sequencia    string
	Complemento  string
	Proprietario string
	Habitantes   string
	Ativo        bool
}

type LocaisFiltro struct{ Busca, Bairro, Tipo, Situacao string }

type LocaisData struct {
	Filtro         LocaisFiltro
	Bairros        []CidadeOpcao
	Tipos          []CidadeOpcao
	Linhas         []LocalLinha
	Pag            Pagination
	Query          url.Values
	PodeEditar     bool
	Restrito       bool // o papel mostra só os locais atribuídos ou só os pontos estratégicos
	Aviso          string
	VerDesativados bool
}

// LocalCampos são os valores do formulário (texto, como foram digitados ou vieram do banco).
type LocalCampos struct {
	Tipo, CEP, Endereco, Numero, Referencia string
	BairroID, QuarteiraoID, Lado            string
	Sequencia, Complemento                  string
	Latitude, Longitude                     string
	Proprietario, ProprietarioTelefone      string
	ProprietarioCPF, Whatsapp, Email        string
	AceitaWhatsapp, AceitaEmail             bool
	Inquilino, InquilinoTelefone            string
	Humanos, Cachorros, Gatos, Roedores     string
}

type LocalFormData struct {
	ID          int64 // 0 = local novo
	Campos      LocalCampos
	Bairros     []CidadeOpcao
	Quarteiroes []CidadeOpcao
	Tipos       []CidadeOpcao
	PodeEditar  bool
	Erro        string
	CSRF        string

	Chave         string // chave do QR Code (ficha de local já gravado)
	Ativo         bool
	PodeDesativar bool
	Aviso         string
	Localizacao   bool // botões "Buscar pelo CEP" e "Localizar no mapa" ligados

	// Ficha do local
	Grupos              []GrupoMini // grupos a que o local pertence
	GruposParaEscolha   []GrupoMini // grupos em que ele ainda pode entrar
	PodeGrupos          bool        // pode ver os grupos
	PodeGerenciarGrupos bool
	MostrarHistorico    bool
	MostrarAcesso       bool // pode gerir acessos e o método é "pela lista de acessos"
	Acessos             []AcessoDoLocal
	UsuariosAcesso      []CidadeOpcao
	Historico           []AuditoriaLinha // últimas alterações deste local
	Secoes              []SecaoFicha     // espaços reservados para os módulos seguintes
}

// AcessoDoLocal é quem tem acesso a um local, e por qual atribuição (local, quarteirão ou bairro).
type AcessoDoLocal struct {
	ID      int64
	Usuario string
	Origem  string
	Direto  bool // atribuição feita no próprio local (pode ser retirada aqui)
}

type GrupoMini struct {
	ID   int64
	Nome string
}

// SecaoFicha é um espaço da ficha do local que um módulo futuro vai preencher.
type SecaoFicha struct {
	Titulo    string
	Descricao string
	Etapa     string
}

func (d LocalFormData) Novo() bool { return d.ID == 0 }

// LoteCampos são os valores do cadastro em lote.
type LoteCampos struct {
	Tipo, BairroID, QuarteiraoID, Endereco, Lado string
	De, Ate, Passo, SequenciaInicial             string
}

type LoteFormData struct {
	Campos      LoteCampos
	Bairros     []CidadeOpcao
	Quarteiroes []CidadeOpcao
	Tipos       []CidadeOpcao
	Erro        string
	CSRF        string
}

package views

import (
	"encoding/json"
	"net/url"
)

// ---- Auditoria ----

type Mudanca struct{ Campo, Antes, Depois string }

type Observacao struct{ Campo, Valor string }

type AuditoriaLinha struct {
	ID          int64
	Quando      string
	Ator        string
	AtorTipo    string
	Acao        string // texto em português
	Codigo      string // código da ação (ex.: usuario.criar)
	Registro    string // o que foi afetado
	IP          string
	Navegador   string
	Mudancas    []Mudanca
	Observacoes []Observacao
}

type OpcaoAcao struct {
	Codigo  string
	Rotulo  string
	Marcada bool
}

type AuditoriaFiltro struct{ De, Ate, Ator, Acao, Busca string }

type AuditoriaData struct {
	Console bool // histórico da plataforma (true) ou do cliente (false)
	Filtro  AuditoriaFiltro
	Acoes   []OpcaoAcao
	Linhas  []AuditoriaLinha
	Pag     Pagination
	Query   url.Values // filtros atuais, para a paginação
}

// ---- Clientes (console) ----

type ClienteLinha struct {
	Nome     string
	Cidade   string // "Nova Mutum/MT"
	Fuso     string // nome do fuso horário
	Slug     string
	Endereco string // host de acesso, ex.: uysiarbonm.inovarsistemas.biz
	URL      string // endereço completo para abrir
	Status   string
	Schema   string
	Criado   string
}

type ClientesData struct {
	Linhas []ClienteLinha
	CSRF   string
}

type ClienteFormData struct {
	Nome        string
	Slug        string
	Subdominio  string
	Dominio     string
	Plano       string
	Cidade      string
	UF          string
	Fuso        string // vazio = automático, pelo estado
	UFs         []string
	Fusos       []FusoOpcao
	Editando    bool   // editar um cliente existente (identificador e subdomínio não mudam)
	BaseDominio string // para mostrar o endereço que o cliente terá
	Erro        string
	CSRF        string
}

// ---- Usuários da plataforma (console) ----

type MasterLinha struct {
	ID           string
	Nome         string
	Email        string
	Master       bool
	Ativo        bool
	DeveTrocar   bool
	UltimoAcesso string
	Eu           bool
}

type MastersData struct {
	Linhas []MasterLinha
	CSRF   string
}

type MasterFormData struct {
	Nome  string
	Email string
	Erro  string
	CSRF  string
}

// ---- Painel inicial ----

type Atalho struct {
	Titulo    string
	Descricao string
	Href      string
}

type PainelData struct {
	Sistema           string
	Console           bool
	Cliente           string
	Schema            string
	Usuario           string
	Master            bool
	Atalhos           []Atalho
	Modulos           []Modulo // blocos do catálogo que a pessoa pode usar
	ClientesAtivos    int
	ClientesSuspensos int
	Ultimos           []AuditoriaLinha
}

// jsString devolve o texto como literal de JavaScript entre aspas (para x-data do Alpine).
func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// FusoOpcao é um fuso horário do seletor.
type FusoOpcao struct{ Nome, Descricao string }

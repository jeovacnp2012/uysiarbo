package views

import "strings"

// PapelLinha é uma linha da lista de papéis.
type PapelLinha struct {
	ID         int64
	Nome       string
	Descricao  string
	Ativo      bool
	Permissoes int
	Usuarios   int
}

// PermItem é uma permissão no editor.
type PermItem struct {
	Codigo     string
	Rotulo     string
	Busca      string // texto minúsculo (bloco, grupo e rótulo) usado pelo filtro de busca
	Restritiva bool   // limita o escopo dos dados (ex.: "apenas as minhas")
	Marcada    bool
	Bloqueada  bool // quem edita não pode conceder esta permissão (não a tem)
}

type PermGrupo struct {
	Nome  string
	Itens []PermItem
}

type PermBloco struct {
	Nome     string
	Grupos   []PermGrupo
	Marcadas int
	Total    int
}

type PapelFormData struct {
	ID        int64 // 0 = papel novo
	Nome      string
	Descricao string
	Ativo     bool
	Blocos    []PermBloco
	Erro      string
	CSRF      string
}

func (d PapelFormData) Novo() bool { return d.ID == 0 }

// UsuarioLinha é uma linha da lista de usuários.
type UsuarioLinha struct {
	ID           int64
	Nome         string
	Email        string
	Papel        string
	Ativo        bool
	LoginWeb     bool
	LoginTablet  bool
	DeveTrocar   bool
	UltimoAcesso string
	Eu           bool
}

type OpcaoPapel struct {
	ID          int64
	Nome        string
	Selecionado bool
}

type UsuarioFormData struct {
	ID          int64 // 0 = usuário novo
	Nome        string
	Email       string
	Telefone    string
	Papeis      []OpcaoPapel
	LoginWeb    bool
	LoginTablet bool
	Ativo       bool
	Eu          bool
	Erro        string
	CSRF        string
}

func (d UsuarioFormData) Novo() bool { return d.ID == 0 }

// SenhaGeradaData é a tela que mostra, uma única vez, a senha temporária de um usuário novo ou resetado.
type SenhaGeradaData struct {
	Novo  bool // true = usuário recém-criado; false = senha resetada
	Nome  string
	Email string
	Senha string
}

// BuscaTexto junta e normaliza os textos usados no filtro de permissões.
func BuscaTexto(partes ...string) string { return strings.ToLower(strings.Join(partes, " ")) }

package views

import "net/url"

// ---- Grupos de locais ----

type GrupoLinha struct {
	ID        int64
	Nome      string
	Descricao string
	Total     int
	Criado    string
	Previa    []GrupoLocalLinha // os primeiros locais, para o cartão

	// Local principal (o grupo nasce dele); vazio em grupo antigo sem principal.
	PrincipalID                                             int64
	PrincipalEndereco, PrincipalBairro, PrincipalQuarteirao string
	PrincipalLado, PrincipalSequencia, PrincipalComplemento string
}

// GrupoPrincipal são os dados do local principal mostrados na ficha do grupo.
type GrupoPrincipal struct {
	ID                                           int64
	Tipo, Endereco, Bairro, Quarteirao, Lado     string
	Sequencia, Complemento, Cidade, Coordenadas  string
	Proprietario, CPF, Telefone, WhatsApp, Email string
	Inquilino, InquilinoTelefone                 string
	Humanos, Cachorros, Gatos, Roedores          string
}

type GruposData struct {
	Busca         string
	Linhas        []GrupoLinha
	PodeGerenciar bool
	Aviso         string
}

type GrupoLocalLinha struct {
	ID         int64
	Endereco   string
	Tipo       string
	Bairro     string
	Quarteirao string
	Lado       string
	Ativo      bool
}

type GrupoFormData struct {
	ID            int64 // 0 = grupo novo
	Nome          string
	Descricao     string
	Locais        []GrupoLocalLinha
	Pag           Pagination
	Query         url.Values
	Bairros       []CidadeOpcao
	Tipos         []CidadeOpcao
	PodeGerenciar bool
	Aviso         string
	Erro          string
	CSRF          string
	Principal     *GrupoPrincipal
	Busca         string
}

func (d GrupoFormData) Novo() bool { return d.ID == 0 }

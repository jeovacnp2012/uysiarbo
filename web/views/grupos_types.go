package views

import "net/url"

// ---- Grupos de locais ----

type GrupoLinha struct {
	ID        int64
	Nome      string
	Descricao string
	Total     int
	Criado    string
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
}

func (d GrupoFormData) Novo() bool { return d.ID == 0 }

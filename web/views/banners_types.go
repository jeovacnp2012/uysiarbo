package views

import "net/url"

// Banners (12.4).

type BannerLinha struct {
	IDTexto, Nome, Tamanho, Fundo, Situacao, Atualizado string
}

type BannerFiltro struct{ Busca, Situacao string }

type BannersData struct {
	Linhas     []BannerLinha
	Filtro     BannerFiltro
	Situacoes  []CidadeOpcao
	Pag        Pagination
	Query      url.Values
	Ordem, Dir string
	PodeNovo   bool
	Aviso      string
}

type BannerForm struct {
	ID                              int64
	Nome, Largura, Altura, CorFundo string
	Ativo, TemFundo                 bool
	Definicao                       string // JSON dos textos
	PodeEditar, PodeExcluir         bool
	Erro, Aviso, CSRF               string
}

func (d BannerForm) Novo() bool { return d.ID == 0 }

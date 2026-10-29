// Package views contém as telas e componentes em templ.
package views

import (
	"fmt"

	"github.com/a-h/templ"
)

// Version é a versão exibida no rodapé do menu. Será ligada a Novidades/Versões na etapa 1.8.
const Version = "0.5.0"

// NavItem é um link do menu.
type NavItem struct {
	Label string
	Href  string
}

// NavGrupo é uma categoria do menu lateral. Com Href, é um item simples (sem sub-itens); sem Href,
// é um grupo recolhível com Itens. O grupo da página atual abre sozinho; os demais começam recolhidos.
type NavGrupo struct {
	Label string
	Icone string // nome do ícone (ver icone em layout.templ)
	Href  string
	Itens []NavItem
}

// Contem diz se o grupo é (ou tem dentro dele) a página ativa.
func (g NavGrupo) Contem(ativa string) bool {
	if g.Href != "" {
		return g.Href == ativa
	}
	for _, it := range g.Itens {
		if it.Href == ativa {
			return true
		}
	}
	return false
}

type Layout struct {
	Title   string
	Active  string // Href do item de menu ativo
	Nav     []NavGrupo
	Console bool   // endereço do console da plataforma (sem cliente)
	Cliente string // nome do cliente (prefeitura), quando o endereço é de um cliente
	Usuario string // nome de quem está logado
	CSRF    string // token CSRF, enviado nos formulários e nas chamadas HTMX

	Versao       string // versão atual do sistema (rodapé do menu)
	NovidadeNova bool   // há uma versão nas Novidades que a pessoa ainda não viu

	Aviso string // mensagem de sucesso no topo da página (vem de ?aviso=<código>)
}

// Pagination descreve a paginação de uma listagem.
type Pagination struct {
	Page    int
	PerPage int
	Total   int
}

func (p Pagination) Pages() int {
	if p.PerPage <= 0 || p.Total <= 0 {
		return 1
	}
	return (p.Total + p.PerPage - 1) / p.PerPage
}

func (p Pagination) HasPrev() bool { return p.Page > 1 }
func (p Pagination) HasNext() bool { return p.Page < p.Pages() }

func (p Pagination) From() int {
	if p.Total == 0 {
		return 0
	}
	return (p.Page-1)*p.PerPage + 1
}

func (p Pagination) To() int {
	to := p.Page * p.PerPage
	if to > p.Total {
		return p.Total
	}
	return to
}

func (p Pagination) Summary() string {
	return fmt.Sprintf("%d–%d de %d", p.From(), p.To(), p.Total)
}

// Column é uma coluna de tabela simples.
type Column struct {
	Title string
}

// Toast é uma mensagem rápida; serve também para o cabeçalho HX-Trigger.
type Toast struct {
	Kind    string `json:"kind"` // sucesso, erro, aviso, info
	Message string `json:"message"`
}

// AssetVersion é a versão dos arquivos estáticos (definida na inicialização do servidor).
var AssetVersion string

// asset devolve o endereço de um arquivo estático com a versão, para o cache do navegador não
// segurar uma versão antiga depois de uma atualização.
func asset(caminho string) templ.SafeURL {
	if AssetVersion == "" {
		return templ.SafeURL(caminho)
	}
	return templ.SafeURL(caminho + "?v=" + AssetVersion)
}

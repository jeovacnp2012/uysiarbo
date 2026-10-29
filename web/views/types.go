// Package views contém as telas e componentes em templ.
package views

import "fmt"

// Version é a versão exibida no rodapé do menu. Será ligada a Novidades/Versões na etapa 1.8.
const Version = "0.5.0"

type NavItem struct {
	Label string
	Href  string
}

type Layout struct {
	Title  string
	Active string // Href do item de menu ativo
	Nav    []NavItem
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

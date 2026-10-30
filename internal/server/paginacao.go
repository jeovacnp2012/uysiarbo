package server

import (
	"github.com/a-h/templ"
	"net/http"
	"slices"
	"strconv"

	"uysiarbo/web/views"
)

// padraoPorPagina é o tamanho de página de toda listagem (a pessoa escolhe outro em views.PorPaginaOpcoes).
const padraoPorPagina = 10

// paginaDe lê ?pagina= e ?por_pagina= do pedido. O total e o ajuste da página ficam com quem consulta.
func paginaDe(r *http.Request) views.Pagination {
	p := views.Pagination{Page: 1, PerPage: padraoPorPagina}
	qs := r.URL.Query()
	if n, err := strconv.Atoi(qs.Get("por_pagina")); err == nil && slices.Contains(views.PorPaginaOpcoes, n) {
		p.PerPage = n
	}
	if n, err := strconv.Atoi(qs.Get("pagina")); err == nil && n > 1 {
		p.Page = n
	}
	return p
}

// paginar corta uma lista já carregada na memória e deixa o paginador (em cima e embaixo da tabela)
// no layout. Devolve só os itens da página pedida.
func paginar[T any](r *http.Request, lay *views.Layout, itens []T) []T {
	p := paginaDe(r)
	p.Total = len(itens)
	if p.Page > p.Pages() {
		p.Page = p.Pages()
	}
	ini := min((p.Page-1)*p.PerPage, len(itens))
	fim := min(ini+p.PerPage, len(itens))
	lay.Pag = &views.Paginador{Pagination: p, Path: r.URL.Path, Query: r.URL.Query()}
	return itens[ini:fim]
}

// telaAcesso monta a tela do controle de acesso com a lista paginada.
func (h *handlers) telaAcesso(r *http.Request, d views.AcessoData) templ.Component {
	lay := h.layout(r, "Controle de acesso aos locais", "/parametros/acesso")
	d.Linhas = paginar(r, &lay, d.Linhas)
	return views.Acesso(lay, d)
}

package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

// Catalog é o que as páginas leem do catálogo (schema public).
type Catalog interface {
	GetAppInfo(ctx context.Context, chave string) (sqlc.AppInfo, error)
	ListClientes(ctx context.Context) ([]sqlc.Cliente, error)
}

// layout monta o layout da página; no endereço de um cliente, mostra o nome dele no menu.
func (s *handlers) layout(r *http.Request, title, active string) views.Layout {
	l := views.Layout{Title: title, Active: active, Nav: s.menu(r), CSRF: csrfToken(r.Context())}
	if id, versao := s.ultimaVersao(r.Context()); versao != "" {
		l.Versao = versao
		if a, ok := authFrom(r.Context()); ok && a.sess != nil {
			l.NovidadeNova = id > a.sess.Conta.NovidadeVista
		}
	}
	if c, ok := tenant.FromContext(r.Context()); ok {
		l.Cliente = c.Nome
	}
	if a, ok := authFrom(r.Context()); ok && a.sess != nil {
		l.Usuario = a.sess.Conta.Nome
	}
	return l
}

func (s *handlers) components(w http.ResponseWriter, r *http.Request) {
	f, rows, p := demoPage(r)
	render(w, r, views.Components(s.layout(r, "Componentes", "/componentes"), f, rows, p))
}

// componentsTable devolve só o trecho da tabela (chamado pelo HTMX ao filtrar e paginar).
func (s *handlers) componentsTable(w http.ResponseWriter, r *http.Request) {
	f, rows, p := demoPage(r)
	render(w, r, views.DemoResults(f, rows, p))
}

// demoPage monta dados fictícios só para demonstrar os componentes.
func demoPage(r *http.Request) (views.DemoFilter, [][]string, views.Pagination) {
	f := views.DemoFilter{
		Q:      strings.TrimSpace(r.URL.Query().Get("q")),
		Status: r.URL.Query().Get("status"),
	}
	var all [][]string
	for i := 1; i <= 47; i++ {
		nome := fmt.Sprintf("Bairro Exemplo %02d", i)
		situacao := "ativo"
		if i%4 == 0 {
			situacao = "inativo"
		}
		if f.Q != "" && !strings.Contains(strings.ToLower(nome), strings.ToLower(f.Q)) {
			continue
		}
		if f.Status != "" && f.Status != situacao {
			continue
		}
		all = append(all, []string{nome, strconv.Itoa(3 + i%9), situacao})
	}

	p := views.Pagination{Page: 1, PerPage: 8, Total: len(all)}
	if n, err := strconv.Atoi(r.URL.Query().Get("pagina")); err == nil && n >= 1 {
		p.Page = n
	}
	if p.Page > p.Pages() {
		p.Page = p.Pages()
	}
	start := (p.Page - 1) * p.PerPage
	end := min(start+p.PerPage, len(all))
	return f, all[start:end], p
}

// menu monta o menu lateral conforme onde a pessoa está e o que ela pode: o master vê todas as telas
// (no console, a gestão da plataforma; num cliente, usuários, papéis e auditoria).
func (s *handlers) menu(r *http.Request) []views.NavItem {
	itens := []views.NavItem{{Label: "Início", Href: "/"}}
	a, _ := authFrom(r.Context())
	if a.sess != nil {
		pr := a.sess.Principal
		if _, cliente := tenant.FromContext(r.Context()); cliente {
			if pr.Pode("conf_usuario") {
				itens = append(itens, views.NavItem{Label: "Usuários", Href: "/usuarios"})
			}
			if pr.Pode("conf_papel") {
				itens = append(itens, views.NavItem{Label: "Papéis", Href: "/papeis"})
			}
			if pr.Pode("auditoria_sistema") {
				itens = append(itens, views.NavItem{Label: "Auditoria", Href: "/auditoria"})
			}
		} else if pr.Master {
			itens = append(itens,
				views.NavItem{Label: "Clientes", Href: "/clientes"},
				views.NavItem{Label: "Usuários da plataforma", Href: "/plataforma/usuarios"},
				views.NavItem{Label: "Auditoria", Href: "/auditoria"},
			)
		}
	}
	if a.sess != nil {
		itens = append(itens, views.NavItem{Label: "Guia", Href: "/guia"}, views.NavItem{Label: "Novidades", Href: "/novidades"})
	}
	return append(itens, views.NavItem{Label: "Componentes", Href: "/componentes"})
}

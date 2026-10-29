package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"uysiarbo/internal/db/sqlc"
	"uysiarbo/web/views"
)

// AppInfoReader lê informações da aplicação gravadas no banco.
type AppInfoReader interface {
	GetAppInfo(ctx context.Context, chave string) (sqlc.AppInfo, error)
}

var nav = []views.NavItem{
	{Label: "Início", Href: "/"},
	{Label: "Componentes", Href: "/componentes"},
}

func layout(title, active string) views.Layout {
	return views.Layout{Title: title, Active: active, Nav: nav}
}

func (s *handlers) home(w http.ResponseWriter, r *http.Request) {
	sistema := "indisponível"
	if info, err := s.info.GetAppInfo(r.Context(), "sistema"); err == nil {
		sistema = info.Valor
	} else {
		s.log.Warn("ler app_info", "erro", err)
	}
	render(w, r, views.Home(layout("Início", "/"), sistema))
}

func (s *handlers) components(w http.ResponseWriter, r *http.Request) {
	f, rows, p := demoPage(r)
	render(w, r, views.Components(layout("Componentes", "/componentes"), f, rows, p))
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

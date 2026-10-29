package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

func cidadeUF(cidade, uf string) string {
	switch {
	case cidade != "" && uf != "":
		return cidade + "/" + uf
	case cidade != "":
		return cidade
	}
	return uf
}

// opcoesDeFuso alimenta o seletor de fuso horário do formulário de cliente.
func opcoesDeFuso() []views.FusoOpcao {
	out := make([]views.FusoOpcao, len(tenant.FusosBrasil))
	for i, f := range tenant.FusosBrasil {
		out[i] = views.FusoOpcao{Nome: f.Nome, Descricao: f.Descricao}
	}
	return out
}

// formCliente completa o formulário de cliente com os seletores, o domínio base e o token CSRF.
func (h *handlers) formCliente(r *http.Request, d views.ClienteFormData) views.ClienteFormData {
	d.UFs, d.Fusos, d.BaseDominio, d.CSRF = tenant.UFs(), opcoesDeFuso(), h.resolver.BaseDomain, csrfToken(r.Context())
	return d
}

// lerLocalizacao lê e valida cidade, estado e fuso de um formulário de cliente. Sem fuso escolhido,
// vale o padrão do estado.
func lerLocalizacao(r *http.Request) (cidade, uf, fuso, erro string) {
	cidade = strings.TrimSpace(r.PostFormValue("cidade"))
	uf = strings.ToUpper(strings.TrimSpace(r.PostFormValue("uf")))
	fuso = strings.TrimSpace(r.PostFormValue("fuso"))
	switch {
	case len([]rune(cidade)) > 100:
		erro = "A cidade pode ter no máximo 100 caracteres."
	case !tenant.UFValida(uf):
		erro = "Escolha o estado (UF) da empresa: ele define o horário do sistema."
	case fuso != "" && !tenant.FusoValido(fuso):
		erro = "Fuso horário inválido."
	}
	if fuso == "" {
		fuso = tenant.FusoDaUF(uf)
	}
	return cidade, uf, fuso, erro
}

func (h *handlers) clienteEditarForm(w http.ResponseWriter, r *http.Request) {
	c, err := sqlc.New(h.tenants.Pool).GetClienteBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar cliente", err)
		return
	}
	render(w, r, views.ClienteForm(h.layout(r, "Editar cliente", "/clientes"), h.formCliente(r, views.ClienteFormData{
		Editando: true, Nome: c.Nome, Slug: c.Slug, Subdominio: c.Subdominio, Plano: c.Plano, Cidade: c.Cidade, UF: c.Uf, Fuso: c.Fuso,
	})))
}

func (h *handlers) clienteEditarSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q := sqlc.New(h.tenants.Pool)
	atual, err := q.GetClienteBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar cliente", err)
		return
	}
	nome, plano := strings.TrimSpace(r.PostFormValue("nome")), strings.TrimSpace(r.PostFormValue("plano"))
	cidade, uf, fuso, msgLocal := lerLocalizacao(r)
	falha := func(msg string) {
		renderStatus(w, r, http.StatusBadRequest, views.ClienteForm(h.layout(r, "Editar cliente", "/clientes"), h.formCliente(r, views.ClienteFormData{
			Editando: true, Nome: nome, Slug: atual.Slug, Subdominio: atual.Subdominio, Plano: plano, Cidade: cidade, UF: uf,
			Fuso: strings.TrimSpace(r.PostFormValue("fuso")), Erro: msg,
		})))
	}
	switch {
	case nome == "" || len([]rune(nome)) > 120:
		falha("Informe o nome do cliente (até 120 caracteres).")
		return
	case msgLocal != "":
		falha(msgLocal)
		return
	case plano == "" || len([]rune(plano)) > 40:
		falha("Informe o plano (até 40 caracteres).")
		return
	}
	novo, err := q.UpdateClienteDados(r.Context(), sqlc.UpdateClienteDadosParams{ID: atual.ID, Nome: nome, Cidade: cidade, Uf: uf, Fuso: fuso, Plano: plano})
	if err != nil {
		h.falhaInterna(w, r, "salvar cliente", err)
		return
	}
	h.resolver.Limpar() // o fuso novo vale na hora
	antes, depois := audit.Diferenca(
		map[string]any{"nome": atual.Nome, "cidade": atual.Cidade, "uf": atual.Uf, "fuso": atual.Fuso, "plano": atual.Plano},
		map[string]any{"nome": novo.Nome, "cidade": novo.Cidade, "uf": novo.Uf, "fuso": novo.Fuso, "plano": novo.Plano},
	)
	if len(antes)+len(depois) == 0 {
		naoAuditar(r)
	} else {
		h.auditar(r, audit.Evento{Acao: "cliente.editar", Entidade: "cliente", EntidadeID: novo.Slug, Rotulo: novo.Nome, Antes: antes, Depois: depois})
	}
	http.Redirect(w, r, "/clientes", http.StatusSeeOther)
}

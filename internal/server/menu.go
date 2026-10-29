package server

import (
	"net/http"

	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

// grupo monta uma categoria do menu com os itens permitidos; sem nenhum item, a categoria some.
func grupo(label, icone string, itens ...views.NavItem) (views.NavGrupo, bool) {
	return views.NavGrupo{Label: label, Icone: icone, Itens: itens}, len(itens) > 0
}

// menu monta o menu lateral por categoria, só com o que a pessoa pode usar (o master vê tudo).
//
// Para acrescentar uma tela nova ao menu, ponha o item na categoria certa abaixo, com a permissão
// que o libera. Um bloco novo do catálogo vira uma categoria nova aqui, quando ganhar tela.
func (s *handlers) menu(r *http.Request) []views.NavGrupo {
	menu := []views.NavGrupo{{Label: "Início", Icone: "inicio", Href: "/"}}
	a, _ := authFrom(r.Context())
	if a.sess == nil {
		return menu
	}
	pr := a.sess.Principal
	_, emCliente := tenant.FromContext(r.Context())

	var itens []views.NavItem
	add := func(permitido bool, label, href string) {
		if permitido {
			itens = append(itens, views.NavItem{Label: label, Href: href})
		}
	}
	incluir := func(label, icone string) {
		if g, ok := grupo(label, icone, itens...); ok {
			menu = append(menu, g)
		}
		itens = nil
	}

	if emCliente {
		pode := pr.Pode("bairro_visualizar")
		add(pode, "Bairros", "/bairros")
		add(pode, "Cidades", "/cidades")
		add(pr.Pode("local_visualizar"), "Locais", "/locais")
		add(pr.Pode("local_grupo_visualizar"), "Grupos de locais", "/grupos")
		incluir("Território", "territorio")

		add(pr.Pode("conf_usuario"), "Usuários", "/usuarios")
		add(pr.Pode("conf_papel"), "Papéis", "/papeis")
		add(pr.Pode("auditoria_sistema"), "Auditoria", "/auditoria")
		add(pr.Master, "Componentes", "/componentes")
		incluir("Administração", "administracao")
	} else {
		add(pr.Master, "Clientes", "/clientes")
		add(pr.Master, "Usuários da plataforma", "/plataforma/usuarios")
		incluir("Plataforma", "plataforma")

		add(pr.Master, "Auditoria", "/auditoria")
		add(pr.Master, "Componentes", "/componentes")
		incluir("Administração", "administracao")
	}

	return append(menu,
		views.NavGrupo{Label: "Guia do sistema", Icone: "guia", Href: "/guia"},
		views.NavGrupo{Label: "Novidades", Icone: "novidades", Href: "/novidades"},
	)
}

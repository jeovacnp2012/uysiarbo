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

// pontoDoMenu devolve o item se permitido, ou um item vazio (descartado pelo chamador).
func pontoDoMenu(permitido bool, label, href string) views.NavItem {
	if !permitido {
		return views.NavItem{}
	}
	return views.NavItem{Label: label, Href: href}
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

	// Painéis primeiro, logo depois de Início: Dashboard, B.I. e B.I. Epidemiológico (este ainda sem tela).
	if emCliente {
		if pr.Pode("visita_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Dashboard", Icone: "dashboard", Href: "/dashboard"})
		}
		if pr.Pode("bi_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "B.I.", Icone: "bi", Href: "/bi"})
		}
		if pr.Pode("bi_epi_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "B.I. Epidemiológico", Icone: "bi", EmBreve: true})
		}
	}

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
		incluir("Território", "territorio")

		if pr.Pode("bairro_visualizar") || pr.Pode("local_visualizar") {
			// Mapa é um grupo: cada camada de interesse vira um item. Os que ainda não têm tela aparecem
			// apagados; ao construir um, troque EmBreve por Href.
			rastro := views.NavItem{Label: "Rastro GPS", EmBreve: true}
			if pr.Pode("mapa_coleta") {
				rastro = views.NavItem{Label: "Rastro GPS", Href: "/mapa/rastro"}
			}
			menu = append(menu, views.NavGrupo{Label: "Mapa", Icone: "mapa", Itens: []views.NavItem{
				{Label: "Visitas", Href: "/mapa"},
				rastro,
				{Label: "Doenças", EmBreve: true},
				{Label: "Campanha Antirrábica", EmBreve: true},
				{Label: "Ovitrampas", EmBreve: true},
				{Label: "EDLs", EmBreve: true},
			}})
		}

		// Relatórios é um grupo, na ordem do Geo Controle; os que ainda não têm tela aparecem apagados
		// (ao construir um, troque emBreve por rel).
		rel := func(rotulo, slug string) views.NavItem {
			if d, ok := definicaoPorSlug(slug); ok && pr.Pode(d.Permissao) {
				return views.NavItem{Label: rotulo, Href: "/relatorios/" + slug}
			}
			return views.NavItem{}
		}
		emBreve := func(rotulo string) views.NavItem { return views.NavItem{Label: rotulo, EmBreve: true} }
		var relItens []views.NavItem
		temTela := false
		for _, it := range []views.NavItem{
			rel("Visita", "visitas"),
			pontoDoMenu(pr.Pode("reconhecimento_visualizar"), "Reconhecimento Geog.", "/reconhecimentos"),
			pontoDoMenu(pr.Pode("resumosemanal_visualizar"), "Resumo Semanal", "/resumosemanal"),
			rel("Local x Visita", "local-visita"),
			emBreve("Entomologica / Trat."),
			emBreve("Campanha Vacinação"),
			rel("Acomp. de Produção", "producao"),
			rel("Serviço Antivetorial", "antivetorial"),
			emBreve("Locais por Criticidade"),
			emBreve("LIRAa - Bol. de Campo"),
			rel("Visita por Dia Trab.", "dia-trabalhado"),
			rel("Investigação totais", "investigacao-total"),
			rel("Laboratório totais", "laboratorio-total"),
			rel("Animais", "animais"),
			rel("Borrifação Residual Intradomiciliar (BRI)", "bri"),
			rel("Local Incompleto", "local-incompleto"),
		} {
			if it.Label == "" {
				continue
			}
			relItens = append(relItens, it)
			temTela = temTela || !it.EmBreve
		}
		if temTela {
			menu = append(menu, views.NavGrupo{Label: "Relatórios", Icone: "visitas", Itens: relItens})
		}

		if pr.Pode("visita_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Visitas", Icone: "visitas", Href: "/visitas"})
		}

		if pr.Pode("visita_laboratorio_visualizar") {
			emAnalise := 0
			if q, _, err := s.cliente(r); err == nil {
				if n, err := q.CountAmostrasEmAnalise(r.Context(), s.escopoVisitas(r)); err == nil {
					emAnalise = int(n)
				}
			}
			menu = append(menu, views.NavGrupo{Label: "Laboratório", Icone: "visitas", Href: "/laboratorio", Contagem: emAnalise})
		}

		if pr.Pode("investigacao_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Investigações", Icone: "visitas", Href: "/investigacoes"})
		}

		if pr.Pode("denuncia_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Denúncias", Icone: "visitas", Href: "/denuncias"})
		}

		add(pr.Pode("conf_usuario"), "Usuários", "/usuarios")
		add(pr.Pode("conf_papel"), "Papéis", "/papeis")
		add(pr.Pode("param_ciclo"), "Ciclos", "/ciclos")
		add(pr.Pode("param_leis"), "Leis municipais", "/leis")
		add(pr.Pode("param_doenca"), "Doenças e agravos", "/doencas")
		add(pr.Pode("param_doenca"), "Entrevistas", "/entrevistas")
		add(pr.Pode("param_geral"), "Parâmetros gerais", "/parametros/geral")
		add(pr.Pode("param_mobilidade"), "Mobilidade (aplicativo)", "/parametros/mobilidade")
		add(pr.Pode("historico_mobilidade"), "Histórico de mobilidade", "/mobilidade/historico")
		add(pr.Pode("acesso_local"), "Controle de acesso", "/parametros/acesso")
		add(pr.Pode("conf_importacao"), "Importação de dados", "/importacoes")
		add(pr.Pode("conf_lixeira"), "Lixeira", "/lixeira")
		add(pr.Pode("conf_api"), "API e integrações", "/parametros/api")
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

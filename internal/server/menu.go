package server

import (
	"net/http"

	"uysiarbo/internal/authz"
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
			menu = append(menu, views.NavGrupo{Label: "B.I. Epidemiológico", Icone: "bi-epi", EmBreve: true})
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
		// A ordem segue a do Geo Controle: telas de trabalho, módulos de campo, grupos (Zoonose, LIRAa, Relatórios,
		// Estoque) e, por fim, Parâmetros, Auditoria e Configuração. Item sem tela aparece apagado (EmBreve): ao
		// construir um, troque EmBreve por Href e ponha a permissão em pode().
		if pr.Pode("bairro_visualizar") || pr.Pode("local_visualizar") {
			// Mapa é um grupo: cada camada de interesse vira um item.
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
		if pr.Pode("local_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Locais", Icone: "locais", Href: "/locais"})
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
			menu = append(menu, views.NavGrupo{Label: "Laboratório", Icone: "laboratorio", Href: "/laboratorio", Contagem: emAnalise})
		}
		if pr.Pode("investigacao_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Investigações", Icone: "investigacao", Href: "/investigacoes"})
		}
		if pr.Pode("denuncia_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Denúncias", Icone: "denuncia", Href: "/denuncias"})
		}
		if pr.Pode("ovitrampa_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Ovitrampas", Icone: "ovitrampa", Href: "/ovitrampas"})
		}
		if pr.Pode("edl_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "EDLs", Icone: "edl", Href: "/edls"})
		}
		if pr.Pode("dronesolicitacao_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Solicitações de drone", Icone: "drone", Href: "/drones"})
		}
		if pr.Pode("notificacao_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Notificações", Icone: "notificacao", Href: "/notificacoes"})
			menu = append(menu, views.NavGrupo{Label: "Autos de infração", Icone: "auto", Href: "/infracoes"})
		}
		if pr.Pode("historico_mobilidade") {
			menu = append(menu, views.NavGrupo{Label: "Histórico de mobilidade", Icone: "mobilidade", Href: "/mobilidade/historico"})
		}
		if pr.Pode("banner_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Banners", Icone: "banner", Href: "/banners"})
		}
		if pr.Pode("comunicacao_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Comunicações", Icone: "comunicacao", Href: "/comunicacoes"})
		}
		if pr.Pode("bairro_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Bairros", Icone: "bairros", Href: "/bairros"})
		}
		// Módulos que ainda não têm tela: aparecem apagados para quem tem a permissão de ver.
		for _, m := range []struct{ perm, rotulo, icone string }{} {
			if pr.Pode(m.perm) {
				menu = append(menu, views.NavGrupo{Label: m.rotulo, Icone: m.icone, EmBreve: true})
			}
		}
		if pr.Pode("campanha_visualizar") || pr.Pode("campanha_diaria_visualizar") || pr.Pode("zoonose_lvc_visualizar") || pr.Pode("zoonose_baap_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Zoonose", Icone: "zoonose", Itens: []views.NavItem{
				campanhaItem(pr),
				diariaItem(pr),
				lvcItem(pr),
				baapItem(pr),
			}})
		}
		if pr.Pode("liraa_agenda_visualizar") || pr.Pode("liraa_grupo") {
			menu = append(menu, views.NavGrupo{Label: "LIRAa", Icone: "liraa", Itens: []views.NavItem{
				itemSe(pr.Pode("liraa_grupo"), "Grupo", "/liraa/grupo"),
				itemSe(pr.Pode("liraa_agenda_visualizar"), "Agenda", "/liraa/agenda"),
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
			menu = append(menu, views.NavGrupo{Label: "Relatórios", Icone: "relatorios", Itens: relItens})
		}

		if pr.Pode("estoque_balanco_visualizar") || pr.Pode("estoque_produto_visualizar") || pr.Pode("estoque_retirada_visualizar") {
			menu = append(menu, views.NavGrupo{Label: "Estoque", Icone: "estoque", Itens: []views.NavItem{
				itemSe(pr.Pode("estoque_balanco_visualizar"), "Balanço Físico", "/estoque/balanco"),
				itemSe(pr.Pode("estoque_produto_visualizar"), "Produtos", "/estoque/produto"),
				itemSe(pr.Pode("estoque_entrada_visualizar"), "Entradas / Lotes", "/estoque/lote"),
				itemSe(pr.Pode("estoque_retirada_visualizar"), "Retiradas", "/estoque/retirada"),
				itemSe(pr.Pode("patrimonio_visualizar"), "Patrimônio", "/estoque/patrimonio"),
				itemSe(pr.Pode("estoque_unidade_visualizar"), "Unidades de Estoque", "/estoque/unidade"),
			}})
		}

		// Conta Ovos: grupo reservado para as telas da integração (ver MEMORIA.md, "Conta Ovos"). Por ora só a
		// configuração existe; os demais itens aparecem apagados.
		if pr.Pode("param_geral") {
			menu = append(menu, views.NavGrupo{Label: "Conta Ovos", Icone: "ovitrampa", Itens: []views.NavItem{
				{Label: "Configuração da API", Href: "/parametros/contaovos"},
				{Label: "Ovitrampas e envio de contagens", EmBreve: true},
				{Label: "Mapa de ovos", EmBreve: true},
			}})
		}

		add(pr.Pode("param_ciclo"), "Ciclos", "/ciclos")
		add(pr.Pode("param_geral"), "Geral", "/parametros/geral")
		add(pr.Pode("acesso_local"), "Controle de acesso", "/parametros/acesso")
		incluir("Parâmetros", "parametros")

		if pr.Pode("auditoria_sistema") {
			menu = append(menu, views.NavGrupo{Label: "Auditoria", Icone: "auditoria", Href: "/auditoria"})
		}

		add(pr.Pode("conf_usuario"), "Usuários", "/usuarios")
		add(pr.Pode("conf_papel"), "Papéis", "/papeis")
		add(pr.Pode("conf_importacao"), "Importação de dados", "/importacoes")
		add(pr.Pode("conf_lixeira"), "Lixeira", "/lixeira")
		add(pr.Pode("conf_api"), "API e integrações", "/parametros/api")
		add(pr.Master, "Componentes", "/componentes")
		incluir("Configuração", "configuracao")
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

// baapItem: o item de animais peçonhentos só tem tela para quem pode ver; sem a permissão fica "em construção".
func baapItem(pr authz.Principal) views.NavItem {
	if pr.Pode("zoonose_baap_visualizar") {
		return views.NavItem{Label: "Busca Ativa — Animais Peçonhentos", Href: "/zoonose/baap"}
	}
	return views.NavItem{Label: "Busca Ativa — Animais Peçonhentos", EmBreve: true}
}

func diariaItem(pr authz.Principal) views.NavItem {
	if pr.Pode("campanha_diaria_visualizar") {
		return views.NavItem{Label: "Diária de Trabalho", Href: "/zoonose/campanha/diaria"}
	}
	return views.NavItem{Label: "Diária de Trabalho", EmBreve: true}
}

// campanhaItem: o item da campanha antirrábica só tem tela para quem pode ver; sem a permissão fica "em construção".
func campanhaItem(pr authz.Principal) views.NavItem {
	if pr.Pode("campanha_visualizar") {
		return views.NavItem{Label: "Campanha Antirrábica", Href: "/zoonose/campanha"}
	}
	return views.NavItem{Label: "Campanha Antirrábica", EmBreve: true}
}

func lvcItem(pr authz.Principal) views.NavItem {
	if pr.Pode("zoonose_lvc_visualizar") {
		return views.NavItem{Label: "Leishmaniose Visceral Canina", Href: "/zoonose/lvc"}
	}
	return views.NavItem{Label: "Leishmaniose Visceral Canina", EmBreve: true}
}

// itemSe: item com tela só para quem pode; sem a permissão fica em construção.
func itemSe(pode bool, rotulo, href string) views.NavItem {
	if pode {
		return views.NavItem{Label: rotulo, Href: href}
	}
	return views.NavItem{Label: rotulo, EmBreve: true}
}

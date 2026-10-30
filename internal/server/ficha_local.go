package server

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// reincidenciaMinima é o número de visitas com depósito encontrado, nos últimos 12 meses, a partir do qual o local
// é tratado como reincidente (critério confirmado pelo usuário em 2026-10-30).
const reincidenciaMinima = 2

// visitasDoLocal preenche a ficha do local com o resumo dos últimos 12 meses (reincidência) e as últimas visitas.
// Quem só vê as próprias visitas vê só o que é seu, e não recebe o resumo geral do local.
func (h *handlers) visitasDoLocal(ctx context.Context, r *http.Request, q *sqlccliente.Queries, localID int64, form *views.LocalFormData) error {
	agora := time.Now().In(h.fuso(r))
	hoje := time.Date(agora.Year(), agora.Month(), agora.Day(), 0, 0, 0, 0, time.UTC)
	somente := h.escopoVisitas(r)

	rows, err := q.ListVisitasDoLocal(ctx, sqlccliente.ListVisitasDoLocalParams{LocalID: localID, SomenteAgente: somente})
	if err != nil {
		return err
	}
	tipo := h.tipoSemana(r, q)
	for _, v := range rows {
		semana := ""
		if v.Data.Valid {
			semana = textoDaSemana(v.Data.Time, tipo)
		}
		form.VisitasRecentes = append(form.VisitasRecentes, views.VisitaDoLocal{
			ID: v.ID, Data: dataBR(v.Data), Semana: semana, Ciclo: v.CicloNome, Atividade: v.AtividadeSigla, Agente: v.AgenteNome,
			Pendencia: rotuloDe(rotulosPendencia, v.Pendencia), Deposito: v.AchouDeposito, Amostra: v.ColetouAmostra, Tratamento: v.FezTratamento,
		})
	}
	if somente.Valid {
		return nil
	}
	res, err := q.ResumoDoLocal12Meses(ctx, sqlccliente.ResumoDoLocal12MesesParams{LocalID: localID, Hoje: pgtype.Date{Time: hoje, Valid: true}})
	if err != nil {
		return err
	}
	rc := &views.ReincidenciaLocal{
		Visitas: int(res.Visitas), Trabalhadas: int(res.Trabalhadas), ComDeposito: int(res.ComDeposito),
		AmostrasPositivas: int(res.AmostrasPositivas), Reincidente: res.ComDeposito >= reincidenciaMinima,
		CiclosVisitados: int(res.CiclosVisitados), CiclosPositivos: int(res.CiclosPositivos),
	}
	if res.UltimoDeposito.Valid {
		rc.UltimoDeposito = dataBR(res.UltimoDeposito)
	}
	form.Reincidencia = rc
	return nil
}

// dadosDaFichaDoLocal preenche os cartões e as seções de módulos da ficha (totais, armadilhas e notificações).
func (h *handlers) dadosDaFichaDoLocal(ctx context.Context, r *http.Request, q *sqlccliente.Queries, localID int64, form *views.LocalFormData) error {
	pr := h.principal(r)
	loc := h.fuso(r)
	form.PodeVisitas = pr.Pode("visita_visualizar")
	form.PodeNovaVisita = pr.Pode("visita_adicionar")
	if form.PodeVisitas {
		t, err := q.TotaisDoLocal(ctx, sqlccliente.TotaisDoLocalParams{LocalID: localID, SomenteAgente: h.escopoVisitas(r)})
		if err != nil {
			return err
		}
		form.Totais = &views.TotaisLocal{
			Visitas: int(t.Visitas), ComDeposito: int(t.ComDeposito), Amostras: int(t.ComAmostra), AmostrasPositivas: int(t.AmostrasPositivas),
			Tratamentos: int(t.ComTratamento), Ultima: dataBR(t.Ultima),
		}
	}
	armadilhas, err := q.ListArmadilhasDaFichaDoLocal(ctx, localID)
	if err != nil {
		return err
	}
	for _, tipo := range []string{"ovitrampa", "edl"} {
		t := tiposDeArmadilha[tipo]
		sec := views.ArmadilhasFicha{Pode: pr.Pode(t.Ver), PodeNova: pr.Pode(t.Novo), Caminho: t.Caminho, Rotulo: t.Rotulo}
		for _, a := range armadilhas {
			if a.Tipo != tipo || !sec.Pode {
				continue
			}
			l := views.ArmadilhaFichaLinha{ID: a.ID, Status: rotuloDe(t.statusOpcoes(), a.Status), Data: a.InstaladaEm.Time.In(loc).Format("02/01/2006 15:04"), Por: a.AgenteNome.String}
			if a.UltimoEm.Valid {
				if t.Ovos && a.UltimoOvos.Valid {
					l.Ultima = strconv.Itoa(int(a.UltimoOvos.Int32)) + " ovos"
				} else if s := a.UltimoNivel; s != "" {
					l.Ultima = "Água " + s
				}
				if l.Ultima != "" {
					l.Ultima += " em " + a.UltimoEm.Time.In(loc).Format("02/01/2006")
				}
			}
			if a.RetiradaEm.Valid {
				l.Retirada = a.RetiradaEm.Time.In(loc).Format("02/01/2006")
			}
			sec.Linhas = append(sec.Linhas, l)
		}
		if tipo == "ovitrampa" {
			form.Ovitrampas = sec
		} else {
			form.EDLs = sec
		}
	}
	form.Campanhas = views.CampanhasFicha{Pode: pr.Pode("campanha_visualizar"), PodeNova: pr.Pode("campanha_adicionar")}
	if form.Campanhas.Pode {
		cs, err := q.ListCampanhasDaFichaDoLocal(ctx, localID)
		if err != nil {
			return err
		}
		for _, c := range cs {
			form.Campanhas.Linhas = append(form.Campanhas.Linhas, views.CampanhaFichaLinha{
				ID: c.ID, Data: c.DataHora.Time.In(loc).Format("02/01/2006 15:04"), Situacao: rotuloDe(rotulosSituacaoCampanha, c.Situacao),
				Aplicador: c.Aplicador, Vacinados: int(c.VacCachorros + c.VacGatos), NaoVacinados: int(c.NaoCachorros + c.NaoGatos), Estoque: c.UsarEstoque,
			})
		}
	}
	form.Lvc = views.LvcSecaoFicha{Pode: pr.Pode("zoonose_lvc_visualizar"), PodeNovo: pr.Pode("zoonose_lvc_novo")}
	if form.Lvc.Pode {
		ts, err := q.ListLvcDaFichaDoLocal(ctx, localID)
		if err != nil {
			return err
		}
		for _, t := range ts {
			reteste := "Não"
			if t.PedidoReteste {
				reteste = rotuloDe(rotulosResultadoLvc, t.ResultadoReteste)
			}
			form.Lvc.Linhas = append(form.Lvc.Linhas, views.LvcFichaLinha{ID: t.ID, Data: dataBR(t.Data), Nome: t.Nome, Dpp: rotuloDe(rotulosResultadoLvc, t.Dpp),
				Reteste: reteste, Situacao: rotuloDe(rotulosSituacaoCachorro, t.SituacaoCachorro)})
		}
	}
	form.Notificacoes = views.NotificacoesFicha{Pode: pr.Pode("notificacao_visualizar"), PodeNova: pr.Pode("notificacao_adicionar")}
	if form.Notificacoes.Pode {
		ns, err := q.ListNotificacoesDaFichaDoLocal(ctx, localID)
		if err != nil {
			return err
		}
		for _, n := range ns {
			form.Notificacoes.Linhas = append(form.Notificacoes.Linhas, views.NotificacaoFichaLinha{
				ID: n.ID, Numero: n.Numero, Status: rotuloDe(rotulosSituacaoNotificacao, n.Situacao), Data: dataBR(n.DataNotificacao), Por: n.PorNome.String, Autuado: n.Autuado,
			})
		}
	}
	return nil
}

// secoesDaFicha são os espaços da ficha do local que os módulos seguintes vão preencher.
// Cada módulo, ao ficar pronto, troca o espaço reservado pelo conteúdo real.
var secoesDaFicha = []views.SecaoFicha{
	{Titulo: "Visitas", Descricao: "Histórico de visitas da equipe, com resultado e depósitos.", Etapa: "Etapa 6"},
	{Titulo: "Laboratório", Descricao: "Amostras coletadas neste local e seus resultados.", Etapa: "Etapa 6"},
	{Titulo: "Casos e notificações", Descricao: "Casos de dengue e outros agravos ligados ao endereço.", Etapa: "Etapa 9"},
	{Titulo: "Infrações", Descricao: "Notificações e autos de fiscalização do imóvel.", Etapa: "Etapa 9"},
	{Titulo: "Armadilhas", Descricao: "Ovitrampas e estações de monitoramento instaladas aqui.", Etapa: "Etapa 10"},
	{Titulo: "Animais", Descricao: "Cães e gatos cadastrados, vacinação e leishmaniose.", Etapa: "Etapa 10"},
	{Titulo: "Comunicação", Descricao: "Mensagens enviadas ao proprietário por WhatsApp e e-mail.", Etapa: "Etapa 12"},
}

// completarFichaDoLocal acrescenta à ficha o que não é do formulário: seções reservadas, grupos e histórico.
func (h *handlers) completarFichaDoLocal(ctx context.Context, r *http.Request, q *sqlccliente.Queries, l sqlccliente.Locai, form *views.LocalFormData) error {
	pr := h.principal(r)
	if err := h.dadosDaFichaDoLocal(ctx, r, q, l.ID, form); err != nil {
		return err
	}
	form.Secoes = make([]views.SecaoFicha, len(secoesDaFicha))
	copy(form.Secoes, secoesDaFicha)
	if pr.Pode("visita_visualizar") {
		id := strconv.FormatInt(l.ID, 10)
		links := []views.SecaoLink{{Rotulo: "Ver visitas", Href: "/visitas?local=" + id}}
		if pr.Pode("visita_adicionar") {
			links = append(links, views.SecaoLink{Rotulo: "Registrar visita", Href: "/visitas/nova?local=" + id})
		}
		form.Secoes[0].Links = links
		if err := h.visitasDoLocal(ctx, r, q, l.ID, form); err != nil {
			return err
		}
	}
	if pr.Pode("notificacao_visualizar") || pr.Pode("notificacao_adicionar") {
		var links []views.SecaoLink
		if pr.Pode("notificacao_visualizar") {
			ns, err := q.ListNotificacoesDoLocal(ctx, l.ID)
			if err != nil {
				return err
			}
			for _, n := range ns {
				links = append(links, views.SecaoLink{Rotulo: n.Numero + " — " + n.Motivo + " (" + rotuloDe(rotulosSituacaoNotificacao, n.Situacao) + ")", Href: "/notificacoes/" + strconv.FormatInt(n.ID, 10)})
			}
			as, err := q.ListAutosDoLocal(ctx, l.ID)
			if err != nil {
				return err
			}
			for _, a := range as {
				links = append(links, views.SecaoLink{Rotulo: "Auto " + a.Numero + " — " + reais(a.ValorCentavos) + " (" + rotuloDe(rotulosSituacaoAuto, a.Situacao) + ")", Href: "/infracoes/" + strconv.FormatInt(a.ID, 10)})
			}
		}
		if pr.Pode("notificacao_adicionar") {
			links = append(links, views.SecaoLink{Rotulo: "Notificar este local", Href: "/notificacoes/nova?local=" + strconv.FormatInt(l.ID, 10)})
		}
		for i := range form.Secoes {
			if form.Secoes[i].Titulo == "Infrações" {
				form.Secoes[i].Links = links
			}
		}
	}
	if pr.Pode("ovitrampa_visualizar") || pr.Pode("edl_visualizar") {
		as, err := q.ListArmadilhasDoLocal(ctx, l.ID)
		if err != nil {
			return err
		}
		var links []views.SecaoLink
		for _, a := range as {
			t := tiposDeArmadilha[a.Tipo]
			if pr.Pode(t.Ver) {
				links = append(links, views.SecaoLink{Rotulo: t.Rotulo + " " + strconv.FormatInt(a.ID, 10) + " (" + rotuloDe(t.statusOpcoes(), a.Status) + ")", Href: t.Caminho + "/" + strconv.FormatInt(a.ID, 10)})
			}
		}
		for _, t := range []armadilhaTipo{tiposDeArmadilha["ovitrampa"], tiposDeArmadilha["edl"]} {
			if pr.Pode(t.Novo) {
				links = append(links, views.SecaoLink{Rotulo: "Instalar " + t.Rotulo + " neste local", Href: t.Caminho + "/nova?local=" + strconv.FormatInt(l.ID, 10)})
			}
		}
		for i := range form.Secoes {
			if form.Secoes[i].Titulo == "Armadilhas" {
				form.Secoes[i].Links = links
			}
		}
	}
	if form.PodeGrupos = pr.Pode("local_grupo_visualizar"); form.PodeGrupos {
		do, err := q.ListGruposDoLocal(ctx, l.ID)
		if err != nil {
			return err
		}
		todos, err := q.ListGruposParaEscolher(ctx)
		if err != nil {
			return err
		}
		estao := map[int64]bool{}
		for _, g := range do {
			estao[g.ID] = true
			form.Grupos = append(form.Grupos, views.GrupoMini{ID: g.ID, Nome: g.Nome})
		}
		for _, g := range todos {
			if !estao[g.ID] {
				form.GruposParaEscolha = append(form.GruposParaEscolha, views.GrupoMini{ID: g.ID, Nome: g.Nome})
			}
		}
		form.PodeGerenciarGrupos = pr.Pode("local_grupo_gerenciar")
	}
	if pr.Pode("acesso_local") && h.metodoDeAcesso(r, q) == acessoParametro {
		form.MostrarAcesso = true
		rows, err := q.ListAcessosDoLocal(ctx, sqlccliente.ListAcessosDoLocalParams{Local: l.ID, Quarteirao: l.QuarteiraoID, Bairro: l.BairroID})
		if err != nil {
			return err
		}
		for _, a := range rows {
			origem := map[string]string{"local": "neste local", "quarteirao": "pelo quarteirão", "bairro": "pelo bairro"}[a.Tipo]
			form.Acessos = append(form.Acessos, views.AcessoDoLocal{ID: a.ID, Usuario: a.UsuarioNome, Origem: origem, Direto: a.Tipo == "local"})
		}
		us, err := q.ListUsuariosAtivosParaResponsavel(ctx)
		if err != nil {
			return err
		}
		for _, u := range us {
			form.UsuariosAcesso = append(form.UsuariosAcesso, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome + " — " + u.Email})
		}
	}
	if form.MostrarHistorico = pr.Pode("auditoria_sistema"); form.MostrarHistorico {
		regs, err := q.ListAuditoriaDaEntidade(ctx, sqlccliente.ListAuditoriaDaEntidadeParams{Entidade: "local", EntidadeID: strconv.FormatInt(l.ID, 10), Limit: 15})
		if err != nil {
			return err
		}
		loc := h.fuso(r)
		for _, g := range regs {
			form.Historico = append(form.Historico, linhaAuditoria(registroAuditoria{
				g.ID, g.Quando.Time, g.AtorTipo, g.AtorNome, g.AtorEmail, g.Ip, g.UserAgent, g.Acao, g.Entidade, g.EntidadeID, g.Rotulo, g.Antes, g.Depois, g.Detalhes,
			}, loc))
		}
	}
	return nil
}

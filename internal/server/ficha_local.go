package server

import (
	"context"
	"net/http"
	"strconv"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

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
	form.Secoes = secoesDaFicha
	pr := h.principal(r)
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

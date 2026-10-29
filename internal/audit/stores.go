package audit

import (
	"context"

	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/db/sqlccliente"
)

// Cliente grava no histórico de um cliente. Q deve usar uma conexão do schema dele (ou uma transação,
// para a auditoria entrar junto com a mudança).
type Cliente struct{ Q *sqlccliente.Queries }

func (c Cliente) Registrar(ctx context.Context, e Evento) error {
	return c.Q.InsertAuditoria(ctx, sqlccliente.InsertAuditoriaParams{
		AtorTipo: e.Ator.Tipo, AtorID: e.Ator.ID, AtorNome: e.Ator.Nome, AtorEmail: e.Ator.Email,
		Ip: e.IP, UserAgent: e.UserAgent, RequestID: e.RequestID,
		Acao: e.Acao, Entidade: e.Entidade, EntidadeID: e.EntidadeID, Rotulo: e.Rotulo,
		Antes: JSON(e.Antes), Depois: JSON(e.Depois), Detalhes: JSON(e.Detalhes),
	})
}

// Plataforma grava no histórico da plataforma (schema public).
type Plataforma struct{ Q *sqlc.Queries }

func (p Plataforma) Registrar(ctx context.Context, e Evento) error {
	return p.Q.InsertAuditoriaPlataforma(ctx, sqlc.InsertAuditoriaPlataformaParams{
		AtorTipo: e.Ator.Tipo, AtorID: e.Ator.ID, AtorNome: e.Ator.Nome, AtorEmail: e.Ator.Email,
		Ip: e.IP, UserAgent: e.UserAgent, RequestID: e.RequestID,
		Acao: e.Acao, Entidade: e.Entidade, EntidadeID: e.EntidadeID, Rotulo: e.Rotulo,
		Antes: JSON(e.Antes), Depois: JSON(e.Depois), Detalhes: JSON(e.Detalhes),
	})
}

// Sistema é o ator das ações feitas pela linha de comando e por rotinas automáticas.
var Sistema = Ator{Tipo: AtorSistema, Nome: "Sistema (linha de comando)"}

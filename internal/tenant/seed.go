package tenant

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/authz"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/db/sqlccliente"
)

// NomePapelAdministrador é o papel criado em todo cliente novo, com todas as permissões que não
// são restritivas ("apenas as minhas" etc.).
const NomePapelAdministrador = "Administrador"

// semear grava os dados iniciais dentro do schema do cliente. Pode ser repetido sem efeito colateral:
// só acrescenta o que falta (inclusive permissões novas do catálogo ao papel Administrador).
func (s *Service) semear(ctx context.Context, schema string, c sqlc.Cliente) error {
	pools := &Pools{URL: s.URL, MaxConns: 1}
	defer pools.Close()
	pool, err := pools.Get(ctx, schema)
	if err != nil {
		return err
	}
	q := sqlccliente.New(pool)

	if err := q.SetClienteInfo(ctx, sqlccliente.SetClienteInfoParams{Chave: "nome", Valor: c.Nome}); err != nil {
		return fmt.Errorf("gravar nome: %w", err)
	}

	papel, err := q.GetPapelByNome(ctx, NomePapelAdministrador)
	if errors.Is(err, pgx.ErrNoRows) {
		papel, err = q.CreatePapel(ctx, sqlccliente.CreatePapelParams{
			Nome:      NomePapelAdministrador,
			Descricao: "Acesso a tudo, exceto as restrições de escopo (apenas as minhas, apenas atribuídos).",
		})
	}
	if err != nil {
		return fmt.Errorf("papel Administrador: %w", err)
	}
	if err := q.AddPermissoesAoPapel(ctx, sqlccliente.AddPermissoesAoPapelParams{
		PapelID:    papel.ID,
		Permissoes: authz.Padrao.Codigos(false),
	}); err != nil {
		return fmt.Errorf("permissões do Administrador: %w", err)
	}
	return nil
}

// SincronizarTodos reaplica os dados iniciais (papel Administrador com as permissões do catálogo
// atual) em todos os clientes não cancelados. Use depois de acrescentar permissões ao catálogo.
func (s *Service) SincronizarTodos(ctx context.Context) error {
	clientes, err := sqlc.New(s.Pool).ListClientes(ctx)
	if err != nil {
		return err
	}
	var erros []error
	for _, c := range clientes {
		if c.Status == "cancelado" || c.Status == "provisionando" {
			continue
		}
		if err := s.semear(ctx, c.SchemaNome, c); err != nil {
			erros = append(erros, fmt.Errorf("%s: %w", c.Slug, err))
			continue
		}
		s.Log.Info("cliente sincronizado", "cliente", c.Slug)
	}
	return errors.Join(erros...)
}

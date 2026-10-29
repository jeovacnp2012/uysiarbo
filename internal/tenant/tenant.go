// Package tenant cria e mantém os clientes (prefeituras): cada um tem um schema próprio no banco.
package tenant

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
)

var ErrJaExiste = errors.New("já existe um cliente com esse identificador")

// ErrSubdominioReservado: o subdomínio já tem outro uso no servidor.
var ErrSubdominioReservado = errors.New("subdomínio reservado")

type Input struct {
	Nome       string
	Slug       string
	Subdominio string // padrão: o slug
	Dominio    string // opcional
	Plano      string // padrão: "padrao"
}

type Service struct {
	Pool *pgxpool.Pool
	URL  string
	Log  *slog.Logger
}

// Provision cria o cliente: registra no catálogo, cria o schema, aplica as migrações e ativa.
// É seguro repetir: se uma tentativa anterior parou no meio (status "provisionando"), continua dela.
func (s *Service) Provision(ctx context.Context, in Input) (sqlc.Cliente, error) {
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	if in.Subdominio == "" {
		in.Subdominio = in.Slug
	}
	if in.Plano == "" {
		in.Plano = "padrao"
	}
	in.Subdominio = strings.ToLower(strings.TrimSpace(in.Subdominio))
	if Reservados[in.Subdominio] || strings.HasPrefix(in.Subdominio, "chamados") {
		return sqlc.Cliente{}, fmt.Errorf("%w: %q", ErrSubdominioReservado, in.Subdominio)
	}
	schema := "c_" + in.Slug
	if !db.SchemaClienteRe.MatchString(schema) {
		return sqlc.Cliente{}, fmt.Errorf("identificador inválido %q: use 3 a 30 letras minúsculas e números, começando por letra", in.Slug)
	}

	q := sqlc.New(s.Pool)
	cliente, err := q.GetClienteBySlug(ctx, in.Slug)
	switch {
	case err == nil:
		if cliente.Status != "provisionando" {
			return cliente, ErrJaExiste
		}
		s.Log.Info("retomando provisionamento", "cliente", cliente.Slug)
		if _, err := s.Pool.Exec(ctx, createSchemaSQL(schema)); err != nil {
			return cliente, fmt.Errorf("criar schema: %w", err)
		}
	case errors.Is(err, pgx.ErrNoRows):
		if cliente, err = s.registrar(ctx, in, schema); err != nil {
			return cliente, err
		}
	default:
		return cliente, err
	}

	if err := db.MigrateCliente(ctx, s.URL, schema, "up"); err != nil {
		return cliente, fmt.Errorf("migrar schema %s (o cliente fica em \"provisionando\"; repita o comando): %w", schema, err)
	}
	// Grava o nome do cliente dentro do próprio schema (usado nas telas e como prova do isolamento).
	if _, err := s.Pool.Exec(ctx, "INSERT INTO "+pgx.Identifier{schema, "cliente_info"}.Sanitize()+" (chave, valor) VALUES ('nome', $1) ON CONFLICT (chave) DO UPDATE SET valor = EXCLUDED.valor", cliente.Nome); err != nil {
		return cliente, fmt.Errorf("gravar nome no schema: %w", err)
	}
	cliente, err = q.UpdateClienteStatus(ctx, sqlc.UpdateClienteStatusParams{ID: cliente.ID, Status: "ativo"})
	if err != nil {
		return cliente, err
	}
	s.Log.Info("cliente provisionado", "cliente", cliente.Slug, "schema", schema)
	return cliente, nil
}

// registrar grava o cliente e cria o schema na mesma transação: ou fica tudo, ou nada.
func (s *Service) registrar(ctx context.Context, in Input, schema string) (sqlc.Cliente, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return sqlc.Cliente{}, err
	}
	defer tx.Rollback(ctx)

	cliente, err := sqlc.New(tx).CreateCliente(ctx, sqlc.CreateClienteParams{
		Nome:       in.Nome,
		Slug:       in.Slug,
		Subdominio: in.Subdominio,
		Dominio:    pgtype.Text{String: in.Dominio, Valid: in.Dominio != ""},
		Plano:      in.Plano,
	})
	if err != nil {
		return cliente, fmt.Errorf("registrar cliente: %w", err)
	}
	if _, err := tx.Exec(ctx, createSchemaSQL(schema)); err != nil {
		return cliente, fmt.Errorf("criar schema: %w", err)
	}
	return cliente, tx.Commit(ctx)
}

// MigrateAll aplica o comando (normalmente "up") em todos os clientes que não estão cancelados.
// Um cliente com erro não impede os demais; os erros voltam juntos.
func (s *Service) MigrateAll(ctx context.Context, command string) error {
	clientes, err := sqlc.New(s.Pool).ListClientes(ctx)
	if err != nil {
		return err
	}
	var erros []error
	for _, c := range clientes {
		if c.Status == "cancelado" {
			continue
		}
		if err := db.MigrateCliente(ctx, s.URL, c.SchemaNome, command); err != nil {
			s.Log.Error("migrar cliente", "cliente", c.Slug, "erro", err)
			erros = append(erros, fmt.Errorf("%s: %w", c.Slug, err))
			continue
		}
		s.Log.Info("cliente migrado", "cliente", c.Slug, "comando", command)
	}
	return errors.Join(erros...)
}

func createSchemaSQL(schema string) string {
	return "CREATE SCHEMA IF NOT EXISTS " + pgx.Identifier{schema}.Sanitize()
}

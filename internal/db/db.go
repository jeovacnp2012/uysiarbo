// Package db cuida da conexão com o Postgres e das migrações.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"uysiarbo/migrations"
)

// SchemaClienteRe é o formato do nome do schema de um cliente ("c_" + slug).
var SchemaClienteRe = regexp.MustCompile(`^c_[a-z][a-z0-9]{2,29}$`)

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL não configurada")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("criar pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("conectar ao banco: %w", err)
	}
	return pool, nil
}

// Migrate executa um comando do goose ("up", "down", "status", "reset"...) no catálogo (schema public).
func Migrate(ctx context.Context, url, command string) error {
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer conn.Close()
	return runGoose(ctx, conn, "public", "goose_db_version", command)
}

// MigrateCliente executa um comando do goose dentro do schema de um cliente. Toda conexão usa
// search_path = <schema>, public: as tabelas (e a tabela de controle do goose) nascem no schema do
// cliente, e as funções do PostGIS, que ficam em public, continuam acessíveis.
func MigrateCliente(ctx context.Context, url, schema, command string) error {
	conn, err := OpenSchema(url, schema)
	if err != nil {
		return err
	}
	defer conn.Close()
	// Nome próprio: "public" está no search_path e já tem goose_db_version, que o goose confundiria com o do cliente.
	return runGoose(ctx, conn, "cliente", "goose_cliente_version", command)
}

// OpenSchema abre uma conexão database/sql cujo search_path é o schema do cliente.
func OpenSchema(url, schema string) (*sql.DB, error) {
	if !SchemaClienteRe.MatchString(schema) {
		return nil, fmt.Errorf("nome de schema inválido: %q", schema)
	}
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.RuntimeParams["search_path"] = pgx.Identifier{schema}.Sanitize() + ", public"
	return stdlib.OpenDB(*cfg), nil
}

// runGoose usa estado global do goose (FS e tabela de controle): não chame em paralelo.
func runGoose(ctx context.Context, conn *sql.DB, dir, table, command string) error {
	goose.SetTableName(table)
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.RunContext(ctx, command, conn, dir)
}

var _ = stdlib.GetDefaultDriver // registra o driver "pgx" para database/sql

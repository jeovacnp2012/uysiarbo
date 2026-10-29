// Package testutil ajuda os testes de integração a usar um Postgres real, sem um pisar no outro.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/db"
)

// BaseURL é o banco de testes configurado (criado por scripts/criar-banco.ps1).
func BaseURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://postgres:@localhost:5432/uysiarbo_test?sslmode=disable"
}

// NewDatabase cria um banco temporário só deste teste, com PostGIS e as migrações do catálogo,
// e o apaga no fim. Devolve a URL de conexão.
func NewDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	base, err := url.Parse(BaseURL())
	if err != nil {
		t.Fatal(err)
	}
	admin := *base
	admin.Path = "/postgres"
	conn, err := pgx.Connect(ctx, admin.String())
	if err != nil {
		t.Fatalf("conectar ao Postgres de testes: %v", err)
	}
	defer conn.Close(ctx)

	var rnd [4]byte
	_, _ = rand.Read(rnd[:])
	name := "uysiarbo_t_" + hex.EncodeToString(rnd[:])
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("criar banco temporário: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(ctx, admin.String())
		if err != nil {
			return
		}
		defer c.Close(ctx)
		_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})

	u := *base
	u.Path = "/" + name
	tmp, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	_, err = tmp.Exec(ctx, "CREATE EXTENSION postgis")
	tmp.Close(ctx)
	if err != nil {
		t.Fatalf("habilitar PostGIS: %v", err)
	}
	if err := db.Migrate(ctx, u.String(), "up"); err != nil {
		t.Fatalf("migrar catálogo: %v", err)
	}
	return u.String()
}

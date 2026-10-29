package db_test

import (
	"context"
	"testing"
	"uysiarbo/internal/testutil"

	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
)

func TestMigracoesEQueries(t *testing.T) {
	ctx := context.Background()
	url := testutil.NewDatabase(t)

	// Aplica, desfaz tudo e aplica de novo (exercita Up e Down; o reset exige a tabela de controle já criada).
	if err := db.Migrate(ctx, url, "up"); err != nil {
		t.Fatalf("up inicial: %v", err)
	}
	if err := db.Migrate(ctx, url, "reset"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if err := db.Migrate(ctx, url, "up"); err != nil {
		t.Fatalf("up: %v", err)
	}

	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	defer pool.Close()

	q := sqlc.New(pool)

	info, err := q.GetAppInfo(ctx, "sistema")
	if err != nil || info.Valor != "UYSI ARBO" {
		t.Fatalf("GetAppInfo = %+v, %v", info, err)
	}

	if err := q.SetAppInfo(ctx, sqlc.SetAppInfoParams{Chave: "teste", Valor: "ok"}); err != nil {
		t.Fatalf("SetAppInfo: %v", err)
	}
	got, err := q.GetAppInfo(ctx, "teste")
	if err != nil || got.Valor != "ok" {
		t.Fatalf("GetAppInfo(teste) = %+v, %v", got, err)
	}

	var postgis string
	if err := pool.QueryRow(ctx, "SELECT postgis_version()").Scan(&postgis); err != nil || postgis == "" {
		t.Fatalf("PostGIS indisponível: %v", err)
	}
}

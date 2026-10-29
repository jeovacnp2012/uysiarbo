package db_test

import (
	"context"
	"testing"
	"uysiarbo/internal/testutil"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
)

func novoQueries(t *testing.T) (context.Context, *sqlc.Queries) {
	t.Helper()
	ctx := context.Background()
	url := testutil.NewDatabase(t)
	if err := db.Migrate(ctx, url, "up"); err != nil {
		t.Fatalf("up: %v", err)
	}
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "DELETE FROM clientes"); err != nil {
		t.Fatal(err)
	}
	return ctx, sqlc.New(pool)
}

func params(slug, sub string) sqlc.CreateClienteParams {
	return sqlc.CreateClienteParams{Nome: "Prefeitura " + slug, Slug: slug, Subdominio: sub, Plano: "padrao"}
}

func TestClientesCriarEBuscar(t *testing.T) {
	ctx, q := novoQueries(t)

	p := params("novamutum", "novamutum")
	p.Dominio = pgtype.Text{String: "arbo.novamutum.mt.gov.br", Valid: true}
	c, err := q.CreateCliente(ctx, p)
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	if c.SchemaNome != "c_novamutum" || c.Status != "provisionando" || !c.ID.Valid {
		t.Errorf("cliente inesperado: %+v", c)
	}

	if got, err := q.GetClienteBySubdominio(ctx, "novamutum"); err != nil || got.ID != c.ID {
		t.Errorf("busca por subdomínio: %+v, %v", got, err)
	}
	if got, err := q.GetClienteByDominio(ctx, p.Dominio); err != nil || got.ID != c.ID {
		t.Errorf("busca por domínio: %+v, %v", got, err)
	}

	ativo, err := q.UpdateClienteStatus(ctx, sqlc.UpdateClienteStatusParams{ID: c.ID, Status: "ativo"})
	if err != nil || ativo.Status != "ativo" || !ativo.AtualizadoEm.Time.After(c.AtualizadoEm.Time) {
		t.Errorf("atualizar status: %+v, %v", ativo, err)
	}

	lista, err := q.ListClientes(ctx)
	if err != nil || len(lista) != 1 {
		t.Errorf("listar: %d, %v", len(lista), err)
	}
}

func TestClientesRestricoes(t *testing.T) {
	ctx, q := novoQueries(t)
	if _, err := q.CreateCliente(ctx, params("alfa", "alfa")); err != nil {
		t.Fatal(err)
	}

	invalidos := map[string]sqlc.CreateClienteParams{
		"slug repetido":         params("alfa", "outro"),
		"subdomínio repetido":   params("beta", "alfa"),
		"slug com maiúscula":    params("Beta", "beta"),
		"slug com hífen":        params("be-ta", "beta"),
		"slug curto demais":     params("ab", "ab"),
		"slug começa c/ número": params("1abc", "abc1"),
		"subdomínio inválido":   params("gama", "-gama"),
	}
	for nome, p := range invalidos {
		if _, err := q.CreateCliente(ctx, p); err == nil {
			t.Errorf("%s: deveria ser recusado pelo banco", nome)
		}
	}

	c, _ := q.GetClienteBySubdominio(ctx, "alfa")
	if _, err := q.UpdateClienteStatus(ctx, sqlc.UpdateClienteStatusParams{ID: c.ID, Status: "inexistente"}); err == nil {
		t.Error("status inválido deveria ser recusado")
	}
}

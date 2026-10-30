package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/testutil"
)

const pgLaragon = `C:\laragon\bin\postgresql\postgresql\bin`

func configDeTeste(t *testing.T, url string) Config {
	t.Helper()
	c := Config{DatabaseURL: url, Dir: t.TempDir(), Versao: "teste", PgBin: os.Getenv("PG_BIN_DIR")}
	if !c.Disponivel() {
		c.PgBin = pgLaragon
	}
	if !c.Disponivel() {
		t.Skip("pg_dump/pg_restore não encontrados (defina PG_BIN_DIR)")
	}
	return c
}

func TestRotacionarMantemAsMaisNovas(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"20260101-000000", "20260102-000000", "20260103-000000", "20260104-000000", "20260104-000000.parcial", "outra"} {
		if err := os.Mkdir(filepath.Join(dir, n), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	apagadas, err := Rotacionar(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(apagadas) != 2 || apagadas[0] != "20260102-000000" || apagadas[1] != "20260101-000000" {
		t.Fatalf("apagadas = %v", apagadas)
	}
	rest, _ := Rodadas(dir)
	if len(rest) != 2 || rest[0] != "20260104-000000" {
		t.Fatalf("restaram %v", rest)
	}
	// pastas que não são rodadas ficam como estão
	for _, n := range []string{"outra", "20260104-000000.parcial"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Fatalf("%s não deveria ser apagada", n)
		}
	}
	if _, err := Rotacionar(dir, 0); err == nil {
		t.Fatal("manter 0 deveria falhar")
	}
}

func TestExecutarVerificarERestaurar(t *testing.T) {
	origem := testutil.NewDatabase(t)
	c := configDeTeste(t, origem)
	ctx := t.Context()

	conn, err := pgx.Connect(ctx, origem)
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE SCHEMA c_demo`,
		`CREATE TABLE c_demo.itens (id int PRIMARY KEY, nome text)`,
		`INSERT INTO c_demo.itens VALUES (1, 'um'), (2, 'dois')`,
	} {
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	conn.Close(ctx)

	if _, err := c.Executar(ctx, []string{"c_demo; DROP"}, time.Now()); err == nil {
		t.Fatal("nome de schema inválido deveria ser recusado")
	}
	pasta, err := c.Executar(ctx, []string{"c_demo"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(pasta, ".parcial") {
		t.Fatal("a pasta final não pode ser parcial")
	}
	if err := c.Verificar(ctx, pasta); err != nil {
		t.Fatalf("verificar: %v", err)
	}

	// restaura num banco novo e confere os dados
	destino := testutil.NewDatabase(t)
	if err := c.Restaurar(ctx, filepath.Join(pasta, "c_demo.dump"), destino, false); err != nil {
		t.Fatalf("restaurar: %v", err)
	}
	dc, err := pgx.Connect(ctx, destino)
	if err != nil {
		t.Fatal(err)
	}
	defer dc.Close(ctx)
	var n int
	if err := dc.QueryRow(ctx, `SELECT count(*) FROM c_demo.itens`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("itens restaurados = %d (%v)", n, err)
	}

	// arquivo alterado é pego pela conferência
	f, err := os.OpenFile(filepath.Join(pasta, "c_demo.dump"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("x")
	f.Close()
	if err := c.Verificar(ctx, pasta); err == nil || !strings.Contains(err.Error(), "não confere") {
		t.Fatalf("a alteração deveria ser detectada: %v", err)
	}
}

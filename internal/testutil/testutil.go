// Package testutil ajuda os testes de integração a usar um Postgres real, sem um pisar no outro.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
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

// NovoModelo cria um banco-modelo (com PostGIS e as migrações do catálogo), deixa o chamador prepará-lo
// em montar e o devolve pronto para ser copiado com CopiaDoModelo, o que é muito mais rápido do que migrar
// do zero a cada teste. apagar remove o modelo (chame no fim do pacote de testes).
func NovoModelo(montar func(url string) error) (nome string, apagar func(), err error) {
	ctx := context.Background()
	base, err := url.Parse(BaseURL())
	if err != nil {
		return "", nil, err
	}
	admin := *base
	admin.Path = "/postgres"
	conn, err := pgx.Connect(ctx, admin.String())
	if err != nil {
		return "", nil, err
	}
	defer conn.Close(ctx)

	var rnd [4]byte
	_, _ = rand.Read(rnd[:])
	nome = "uysiarbo_m_" + hex.EncodeToString(rnd[:])
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+nome); err != nil {
		return "", nil, err
	}
	apagar = func() {
		c, err := pgx.Connect(ctx, admin.String())
		if err != nil {
			return
		}
		defer c.Close(ctx)
		_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+nome+" WITH (FORCE)")
	}
	u := *base
	u.Path = "/" + nome
	tmp, err := pgx.Connect(ctx, u.String())
	if err != nil {
		apagar()
		return "", nil, err
	}
	_, err = tmp.Exec(ctx, "CREATE EXTENSION postgis")
	tmp.Close(ctx)
	if err == nil {
		err = db.Migrate(ctx, u.String(), "up")
	}
	if err == nil {
		err = montar(u.String())
	}
	if err != nil {
		apagar()
		return "", nil, err
	}
	return nome, apagar, nil
}

// CopiaDoModelo cria um banco só deste teste a partir do modelo (sem conexões abertas nele) e o apaga no fim.
func CopiaDoModelo(t *testing.T, modelo string) string {
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
	nome := "uysiarbo_t_" + hex.EncodeToString(rnd[:])
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+nome+" TEMPLATE "+modelo); err != nil {
		t.Fatalf("copiar o banco-modelo: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(ctx, admin.String())
		if err != nil {
			return
		}
		defer c.Close(ctx)
		_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+nome+" WITH (FORCE)")
	})
	u := *base
	u.Path = "/" + nome
	return u.String()
}

// Compartilhado é um único banco de testes para todos os testes de um pacote: criar e apagar um banco
// custa alguns segundos no Windows, então o estado base é montado uma vez e restaurado entre os testes.
type Compartilhado struct {
	url     string
	apagar  func()
	tabelas []tabelaBase // tabelas do estado base, na ordem de criação
	schemas []string     // schemas existentes no estado base
}

type tabelaBase struct {
	schema, nome, copia string
	colunas             string // lista de colunas gravaveis, já com aspas
}

// NovoCompartilhado cria o banco, deixa o chamador montar o estado base em montar e guarda uma cópia dele.
func NovoCompartilhado(montar func(url string) error) (*Compartilhado, error) {
	var c Compartilhado
	_, apagar, err := NovoModelo(func(url string) error {
		if err := montar(url); err != nil {
			return err
		}
		c.url = url
		return c.fotografar(context.Background())
	})
	if err != nil {
		return nil, err
	}
	c.apagar = apagar
	return &c, nil
}

// Apagar remove o banco compartilhado (chame no fim do pacote de testes).
func (c *Compartilhado) Apagar() {
	if c != nil && c.apagar != nil {
		c.apagar()
	}
}

// consultaTabelas lista as tabelas comuns (fora das extensões) dos schemas público e de clientes.
const consultaTabelas = `
SELECT n.nspname, c.relname,
       string_agg(format('%I', a.attname), ', ' ORDER BY a.attnum)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped AND a.attgenerated = ''
WHERE c.relkind = 'r' AND (n.nspname = 'public' OR n.nspname LIKE 'c\_%')
  AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid = c.oid AND d.deptype = 'e')
GROUP BY n.nspname, c.relname
ORDER BY n.nspname, c.relname`

func (c *Compartilhado) fotografar(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, c.url)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "DROP SCHEMA IF EXISTS zz_base CASCADE; CREATE SCHEMA zz_base"); err != nil {
		return err
	}
	rows, err := conn.Query(ctx, consultaTabelas)
	if err != nil {
		return err
	}
	for rows.Next() {
		var t tabelaBase
		if err := rows.Scan(&t.schema, &t.nome, &t.colunas); err != nil {
			rows.Close()
			return err
		}
		t.copia = t.schema + "__" + t.nome
		c.tabelas = append(c.tabelas, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, t := range c.tabelas {
		sql := fmt.Sprintf(`CREATE TABLE zz_base.%s AS SELECT %s FROM %s.%s`, pgx.Identifier{t.copia}.Sanitize(), t.colunas, pgx.Identifier{t.schema}.Sanitize(), pgx.Identifier{t.nome}.Sanitize())
		if _, err := conn.Exec(ctx, sql); err != nil {
			return fmt.Errorf("fotografar %s.%s: %w", t.schema, t.nome, err)
		}
	}
	srows, err := conn.Query(ctx, `SELECT nspname FROM pg_namespace WHERE nspname LIKE 'c\_%'`)
	if err != nil {
		return err
	}
	for srows.Next() {
		var s string
		if err := srows.Scan(&s); err != nil {
			srows.Close()
			return err
		}
		c.schemas = append(c.schemas, s)
	}
	srows.Close()
	return srows.Err()
}

// Reiniciar devolve o banco ao estado base (dados, sequências e schemas de clientes) e devolve a URL.
// Só funciona quando nenhum outro teste está usando o banco (os testes de um pacote rodam em sequência).
func (c *Compartilhado) Reiniciar(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, c.url)
	if err != nil {
		t.Fatalf("conectar ao banco de testes: %v", err)
	}
	defer conn.Close(ctx)

	var sb strings.Builder
	sb.WriteString("SET LOCAL session_replication_role = replica;\n")
	// Schemas de clientes criados durante o teste.
	rows, err := conn.Query(ctx, `SELECT nspname FROM pg_namespace WHERE nspname LIKE 'c\_%'`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		if !slices.Contains(c.schemas, s) {
			fmt.Fprintf(&sb, "DROP SCHEMA %s CASCADE;\n", pgx.Identifier{s}.Sanitize())
		}
	}
	rows.Close()

	// DELETE em vez de TRUNCATE: truncar dezenas de tabelas custa segundos no Windows (arquivos novos e fsync).
	for _, tb := range c.tabelas {
		fmt.Fprintf(&sb, "DELETE FROM %s;\n", pgx.Identifier{tb.schema, tb.nome}.Sanitize())
	}
	for _, tb := range c.tabelas {
		fmt.Fprintf(&sb, "INSERT INTO %s (%s) OVERRIDING SYSTEM VALUE SELECT %s FROM zz_base.%s;\n",
			pgx.Identifier{tb.schema, tb.nome}.Sanitize(), tb.colunas, tb.colunas, pgx.Identifier{tb.copia}.Sanitize())
	}
	// As sequências voltam logo depois do maior valor do estado base.
	sb.WriteString(`DO $$
DECLARE r record; v bigint;
BEGIN
  FOR r IN
    SELECT n.nspname AS s, c.relname AS t, a.attname AS col,
           pg_get_serial_sequence(format('%I.%I', n.nspname, c.relname), a.attname) AS seq
    FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
    WHERE c.relkind = 'r' AND (n.nspname = 'public' OR n.nspname LIKE 'c\_%')
  LOOP
    IF r.seq IS NOT NULL THEN
      EXECUTE format('SELECT max(%I) FROM %I.%I', r.col, r.s, r.t) INTO v;
      IF v IS NOT NULL THEN PERFORM setval(r.seq, v, true); ELSE PERFORM setval(r.seq, 1, false); END IF;
    END IF;
  END LOOP;
END $$;`)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, sb.String()); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("restaurar o banco de testes: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("restaurar o banco de testes: %v", err)
	}
	return c.url
}

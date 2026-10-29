package tenant_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"uysiarbo/internal/auth"

	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/tenant"
	"uysiarbo/internal/testutil"
)

func novoServico(t *testing.T) (*tenant.Service, string) {
	t.Helper()
	url := testutil.NewDatabase(t)
	pool, err := db.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &tenant.Service{Pool: pool, URL: url, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, url
}

// existe diz se a tabela existe no schema indicado.
func existe(t *testing.T, s *tenant.Service, schema, tabela string) bool {
	t.Helper()
	var ok bool
	err := s.Pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)`,
		schema, tabela).Scan(&ok)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestProvisionCriaSchemaMigraEAtiva(t *testing.T) {
	s, url := novoServico(t)
	ctx := context.Background()

	c, err := s.Provision(ctx, tenant.Input{Nome: "Prefeitura de Nova Mutum", Slug: "NovaMutum"})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if c.Slug != "novamutum" || c.SchemaNome != "c_novamutum" || c.Subdominio != "novamutum" || c.Status != "ativo" {
		t.Errorf("cliente inesperado: %+v", c)
	}
	if !existe(t, s, "c_novamutum", "cliente_info") {
		t.Error("tabela cliente_info não foi criada no schema do cliente")
	}
	if !existe(t, s, "c_novamutum", "goose_cliente_version") {
		t.Error("a tabela de controle do goose deveria ficar no schema do cliente")
	}
	if existe(t, s, "public", "cliente_info") {
		t.Error("a tabela do cliente vazou para o schema public")
	}

	// As funções do PostGIS (schema public) precisam funcionar a partir do schema do cliente.
	conn, err := db.OpenSchema(url, "c_novamutum")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var wkt string
	if err := conn.QueryRowContext(ctx, "SELECT ST_AsText(ST_MakePoint(1, 2))").Scan(&wkt); err != nil || wkt != "POINT(1 2)" {
		t.Errorf("PostGIS a partir do schema do cliente: %q, %v", wkt, err)
	}
}

func TestProvisionIsolaClientes(t *testing.T) {
	s, url := novoServico(t)
	ctx := context.Background()
	for _, slug := range []string{"alfa", "beta"} {
		if _, err := s.Provision(ctx, tenant.Input{Nome: slug, Slug: slug}); err != nil {
			t.Fatal(err)
		}
	}
	grava := func(schema, valor string) {
		conn, err := db.OpenSchema(url, schema)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if _, err := conn.ExecContext(ctx, "INSERT INTO cliente_info (chave, valor) VALUES ('marca', $1)", valor); err != nil {
			t.Fatal(err)
		}
	}
	grava("c_alfa", "A")
	grava("c_beta", "B")

	for schema, quer := range map[string]string{"c_alfa": "A", "c_beta": "B"} {
		var n int
		var valor string
		err := s.Pool.QueryRow(ctx, `SELECT count(*), max(valor) FROM `+schema+`.cliente_info WHERE chave = 'marca'`).Scan(&n, &valor)
		if err != nil || n != 1 || valor != quer {
			t.Errorf("%s: n=%d valor=%q err=%v (quer 1 e %q)", schema, n, valor, err, quer)
		}
	}
}

func TestProvisionRecusaRepetidoEInvalido(t *testing.T) {
	s, _ := novoServico(t)
	ctx := context.Background()
	if _, err := s.Provision(ctx, tenant.Input{Nome: "Alfa", Slug: "alfa"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Provision(ctx, tenant.Input{Nome: "Alfa 2", Slug: "alfa"}); !errors.Is(err, tenant.ErrJaExiste) {
		t.Errorf("repetido: err = %v, quer ErrJaExiste", err)
	}
	for _, slug := range []string{"", "ab", "1abc", "a-b-c", "x; DROP SCHEMA public", strings.Repeat("a", 31)} {
		if _, err := s.Provision(ctx, tenant.Input{Nome: "X", Slug: slug}); err == nil {
			t.Errorf("slug %q deveria ser recusado", slug)
		}
	}
	// Nada além do cliente válido foi criado.
	lista, _ := sqlc.New(s.Pool).ListClientes(ctx)
	if len(lista) != 1 {
		t.Errorf("clientes = %d, quer 1", len(lista))
	}
}

func TestProvisionRetomaDeOndeParou(t *testing.T) {
	s, _ := novoServico(t)
	ctx := context.Background()

	// Simula uma tentativa interrompida: cliente registrado, schema criado, sem migrar e sem ativar.
	c, err := sqlc.New(s.Pool).CreateCliente(ctx, sqlc.CreateClienteParams{Nome: "Gama", Slug: "gama", Subdominio: "gama", Plano: "padrao"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `CREATE SCHEMA c_gama`); err != nil {
		t.Fatal(err)
	}

	got, err := s.Provision(ctx, tenant.Input{Nome: "Gama", Slug: "gama"})
	if err != nil {
		t.Fatalf("retomar: %v", err)
	}
	if got.ID != c.ID || got.Status != "ativo" || !existe(t, s, "c_gama", "cliente_info") {
		t.Errorf("não retomou corretamente: %+v", got)
	}
}

func TestMigrateAllAplicaEmTodosOsClientes(t *testing.T) {
	s, _ := novoServico(t)
	ctx := context.Background()
	for _, slug := range []string{"alfa", "beta"} {
		if _, err := s.Provision(ctx, tenant.Input{Nome: slug, Slug: slug}); err != nil {
			t.Fatal(err)
		}
	}
	// Desfaz a migração de um cliente e confere que o "up" geral a refaz, sem erro nos demais.
	if err := db.MigrateCliente(ctx, s.URL, "c_beta", "reset"); err != nil {
		t.Fatal(err)
	}
	if existe(t, s, "c_beta", "cliente_info") {
		t.Fatal("o reset não desfez a migração")
	}
	if err := s.MigrateAll(ctx, "up"); err != nil {
		t.Fatalf("MigrateAll: %v", err)
	}
	if !existe(t, s, "c_beta", "cliente_info") || !existe(t, s, "c_alfa", "cliente_info") {
		t.Error("MigrateAll não deixou todos os clientes em dia")
	}
}

func TestSubdominioReservadoERecusado(t *testing.T) {
	s, _ := novoServico(t)
	for _, sub := range []string{"www", "labpront", "perene", "uysiarbo", "chamadosvg", "WWW"} {
		_, err := s.Provision(context.Background(), tenant.Input{Nome: "X", Slug: "xyz", Subdominio: sub})
		if !errors.Is(err, tenant.ErrSubdominioReservado) {
			t.Errorf("subdomínio %q: err = %v, quer ErrSubdominioReservado", sub, err)
		}
	}
	if lista, _ := sqlc.New(s.Pool).ListClientes(context.Background()); len(lista) != 0 {
		t.Errorf("nada deveria ter sido criado, mas há %d clientes", len(lista))
	}
}

func TestPoolsIsolaPorSchema(t *testing.T) {
	s, url := novoServico(t)
	ctx := context.Background()
	for _, slug := range []string{"alfa", "beta"} {
		if _, err := s.Provision(ctx, tenant.Input{Nome: "Cliente " + slug, Slug: slug}); err != nil {
			t.Fatal(err)
		}
	}
	pools := &tenant.Pools{URL: url}
	defer pools.Close()

	for _, schema := range []string{"c_alfa", "c_beta"} {
		pool, err := pools.Get(ctx, schema)
		if err != nil {
			t.Fatal(err)
		}
		var nome, caminho string
		// Sem citar o schema: quem decide é o search_path da conexão.
		if err := pool.QueryRow(ctx, "SELECT valor FROM cliente_info WHERE chave = 'nome'").Scan(&nome); err != nil {
			t.Fatal(err)
		}
		_ = pool.QueryRow(ctx, "SHOW search_path").Scan(&caminho)
		if nome != "Cliente "+schema[2:] || !strings.Contains(caminho, schema) {
			t.Errorf("%s: nome=%q search_path=%q", schema, nome, caminho)
		}
	}
	if _, err := pools.Get(ctx, "public; DROP SCHEMA public"); err == nil {
		t.Error("nome de schema inválido deveria ser recusado")
	}
}

func TestResolverNormalizaEConsulta(t *testing.T) {
	if got := tenant.NormalizeHost("UysiarboNM.Inovarsistemas.biz:8443"); got != "uysiarbonm.inovarsistemas.biz" {
		t.Errorf("NormalizeHost = %q", got)
	}
	if got := tenant.NormalizeHost("exemplo.com."); got != "exemplo.com" {
		t.Errorf("NormalizeHost com ponto final = %q", got)
	}
}

func TestProvisionCriaPapelAdministrador(t *testing.T) {
	s, _ := novoServico(t)
	ctx := context.Background()
	if _, err := s.Provision(ctx, tenant.Input{Nome: "Alfa", Slug: "alfa"}); err != nil {
		t.Fatal(err)
	}
	var nome string
	var total, restritivas int
	err := s.Pool.QueryRow(ctx, `SELECT p.nome, count(pp.permissao) FROM c_alfa.papeis p
		JOIN c_alfa.papel_permissoes pp ON pp.papel_id = p.id GROUP BY p.nome`).Scan(&nome, &total)
	if err != nil || nome != tenant.NomePapelAdministrador || total != 159 {
		t.Fatalf("papel=%q permissões=%d err=%v (quer Administrador e 159)", nome, total, err)
	}
	// O Administrador não recebe as permissões restritivas (apenas as minhas, apenas atribuídos).
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM c_alfa.papel_permissoes
		WHERE permissao IN ('visita_visualizar_minha', 'local_meubairro', 'bairro_meubairro')`).Scan(&restritivas); err != nil || restritivas != 0 {
		t.Errorf("restritivas no Administrador = %d, quer 0 (err=%v)", restritivas, err)
	}
	// Repetir o provisionamento do seed não duplica nada.
	if err := s.MigrateAll(ctx, "up"); err != nil {
		t.Fatal(err)
	}
	var papeis int
	_ = s.Pool.QueryRow(ctx, `SELECT count(*) FROM c_alfa.papeis`).Scan(&papeis)
	if papeis != 1 {
		t.Errorf("papéis = %d, quer 1", papeis)
	}
}

func TestMasterCriarResetarEProtecao(t *testing.T) {
	s, _ := novoServico(t)
	ctx := context.Background()

	u, senha, err := s.CriarMaster(ctx, "Pessoa Teste", "Master@Teste.com")
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	if !u.Master || !u.Ativo || !u.DeveTrocarSenha || len(senha) != 16 {
		t.Errorf("master inesperado: %+v (senha %d caracteres)", u, len(senha))
	}
	if u.SenhaHash == senha || strings.Contains(u.SenhaHash, senha) {
		t.Fatal("a senha em texto puro não pode ir para o banco")
	}
	if ok, err := auth.Confere(senha, u.SenhaHash); err != nil || !ok {
		t.Errorf("a senha mostrada deveria conferir com o hash: ok=%v err=%v", ok, err)
	}

	// E-mail repetido, ignorando maiúsculas.
	if _, _, err := s.CriarMaster(ctx, "Outro", "master@teste.COM"); !errors.Is(err, tenant.ErrMasterJaExiste) {
		t.Errorf("e-mail repetido: err = %v", err)
	}
	if _, _, err := s.CriarMaster(ctx, "", "sem-arroba"); err == nil {
		t.Error("dados inválidos deveriam ser recusados")
	}

	// Reset: gera outra senha, a antiga deixa de valer e a troca volta a ser obrigatória.
	if _, err := s.Pool.Exec(ctx, `UPDATE plataforma_usuarios SET deve_trocar_senha = false`); err != nil {
		t.Fatal(err)
	}
	u2, nova, err := s.ResetarSenhaPlataforma(ctx, "master@teste.com")
	if err != nil {
		t.Fatalf("resetar: %v", err)
	}
	if nova == senha || !u2.DeveTrocarSenha {
		t.Errorf("reset: nova=%q trocar=%v", nova, u2.DeveTrocarSenha)
	}
	if ok, _ := auth.Confere(senha, u2.SenhaHash); ok {
		t.Error("a senha antiga não deveria mais valer")
	}
	if ok, _ := auth.Confere(nova, u2.SenhaHash); !ok {
		t.Error("a nova senha deveria valer")
	}
	if _, _, err := s.ResetarSenhaPlataforma(ctx, "ninguem@x.com"); err == nil {
		t.Error("reset de usuário inexistente deveria falhar")
	}

	// O banco protege o master contra exclusão, rebaixamento e desativação.
	for nome, sql := range map[string]string{
		"excluir":   `DELETE FROM plataforma_usuarios WHERE master`,
		"rebaixar":  `UPDATE plataforma_usuarios SET master = false`,
		"desativar": `UPDATE plataforma_usuarios SET ativo = false`,
	} {
		if _, err := s.Pool.Exec(ctx, sql); err == nil {
			t.Errorf("%s: o banco deveria recusar", nome)
		}
	}
	var n int
	_ = s.Pool.QueryRow(ctx, `SELECT count(*) FROM plataforma_usuarios WHERE master AND ativo`).Scan(&n)
	if n != 1 {
		t.Errorf("o master deveria continuar intacto, mas há %d", n)
	}
}

func TestSincronizarTodosAcrescentaPermissoesQueFaltam(t *testing.T) {
	s, _ := novoServico(t)
	ctx := context.Background()
	if _, err := s.Provision(ctx, tenant.Input{Nome: "Alfa", Slug: "alfa"}); err != nil {
		t.Fatal(err)
	}
	// Simula um catálogo que cresceu: o Administrador perdeu duas permissões.
	if _, err := s.Pool.Exec(ctx, `DELETE FROM c_alfa.papel_permissoes WHERE permissao IN ('visita_adicionar', 'banner_excluir')`); err != nil {
		t.Fatal(err)
	}
	if err := s.SincronizarTodos(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = s.Pool.QueryRow(ctx, `SELECT count(*) FROM c_alfa.papel_permissoes`).Scan(&n)
	if n != 159 {
		t.Errorf("permissões = %d, quer 159 (sincronizar deveria repor as que faltavam)", n)
	}
}

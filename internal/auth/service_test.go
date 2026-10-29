package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/auth"
	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/tenant"
	"uysiarbo/internal/testutil"
)

const senhaInicial = "Senha-Inicial-77"

type cenario struct {
	ctx       context.Context
	agora     time.Time // relógio falso, controlado pelo teste
	cliente   *auth.Service
	console   *auth.Service
	pubQ      *sqlc.Queries
	cliQ      *sqlccliente.Queries
	cliPool   *pgxpool.Pool
	masterPw  string
	usuarioID string
}

// novoCenario provisiona um cliente com um papel, um usuário e um master, tudo no banco temporário.
func novoCenario(t *testing.T) *cenario {
	t.Helper()
	ctx := context.Background()
	url := testutil.NewDatabase(t)
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	svc := &tenant.Service{Pool: pool, URL: url, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if _, err := svc.Provision(ctx, tenant.Input{Nome: "Alfa", Slug: "alfa"}); err != nil {
		t.Fatal(err)
	}
	_, masterPw, err := svc.CriarMaster(ctx, "Mestre", "mestre@plataforma.com")
	if err != nil {
		t.Fatal(err)
	}

	pools := &tenant.Pools{URL: url, MaxConns: 2}
	t.Cleanup(pools.Close)
	cp, err := pools.Get(ctx, "c_alfa")
	if err != nil {
		t.Fatal(err)
	}
	cliQ, pubQ := sqlccliente.New(cp), sqlc.New(pool)

	papel, err := cliQ.CreatePapel(ctx, sqlccliente.CreatePapelParams{Nome: "Agente"})
	if err != nil {
		t.Fatal(err)
	}
	if err := cliQ.AddPermissoesAoPapel(ctx, sqlccliente.AddPermissoesAoPapelParams{PapelID: papel.ID, Permissoes: []string{"visita_adicionar", "visita_visualizar"}}); err != nil {
		t.Fatal(err)
	}
	hash, _ := auth.Hash(senhaInicial)
	u, err := cliQ.CreateUsuario(ctx, sqlccliente.CreateUsuarioParams{
		Nome: "Agente Um", Email: "agente@alfa.gov.br", SenhaHash: hash, PapelID: papel.ID, LoginWeb: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	c := &cenario{ctx: ctx, agora: time.Now(), pubQ: pubQ, cliQ: cliQ, cliPool: cp, masterPw: masterPw, usuarioID: itoa(u.ID)}
	relogio := func() time.Time { return c.agora }
	c.cliente = &auth.Service{
		Contas:  []auth.ContaStore{auth.ClienteContas{Q: cliQ}, auth.PlataformaContas{Q: pubQ, SoMaster: true}},
		Sessoes: auth.ClienteSessoes{Q: cliQ},
		Agora:   relogio,
	}
	c.console = &auth.Service{
		Contas:  []auth.ContaStore{auth.PlataformaContas{Q: pubQ}},
		Sessoes: auth.PlataformaSessoes{Q: pubQ},
		Agora:   relogio,
	}
	return c
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func fmtInt(n int64) string { return strconv.FormatInt(n, 10) }

func (c *cenario) pool(*testing.T) *pgxpool.Pool { return c.cliPool }

func TestEntrarAutenticarESair(t *testing.T) {
	c := novoCenario(t)
	token, sess, err := c.cliente.Entrar(c.ctx, "AGENTE@alfa.gov.br", senhaInicial, "10.0.0.1", "teste")
	if err != nil {
		t.Fatalf("entrar: %v", err)
	}
	if len(token) != 43 || sess.Conta.Tipo != auth.TipoUsuario {
		t.Fatalf("token=%q conta=%+v", token, sess.Conta)
	}
	if !sess.Principal.Pode("visita_adicionar") || sess.Principal.Pode("visita_editar") {
		t.Error("o principal deveria ter só as permissões do papel")
	}

	again, err := c.cliente.Autenticar(c.ctx, token)
	if err != nil || again.Conta.ID != sess.Conta.ID {
		t.Fatalf("autenticar: %v", err)
	}
	if err := c.cliente.Sair(c.ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := c.cliente.Autenticar(c.ctx, token); !errors.Is(err, auth.ErrSessaoInvalida) {
		t.Errorf("depois de sair, err = %v", err)
	}
}

func TestOTokenNaoFicaGuardadoNoBanco(t *testing.T) {
	c := novoCenario(t)
	token, _, _ := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", "")
	var n int
	pool := c.pool(t)
	if err := pool.QueryRow(c.ctx, `SELECT count(*) FROM sessoes WHERE position(convert_to($1, 'UTF8') in token_hash) > 0`, token).Scan(&n); err != nil || n != 0 {
		t.Errorf("o token em texto puro não deveria estar no banco (n=%d err=%v)", n, err)
	}
	var tam int
	_ = pool.QueryRow(c.ctx, `SELECT length(token_hash) FROM sessoes LIMIT 1`).Scan(&tam)
	if tam != 32 {
		t.Errorf("tamanho do hash = %d, quer 32 (SHA-256)", tam)
	}
}

func TestCredenciaisErradasNaoRevelamNada(t *testing.T) {
	c := novoCenario(t)
	for nome, par := range map[string][2]string{
		"senha errada":       {"agente@alfa.gov.br", "senha-errada-1"},
		"e-mail inexistente": {"ninguem@alfa.gov.br", senhaInicial},
	} {
		_, _, err := c.cliente.Entrar(c.ctx, par[0], par[1], "", "")
		if !errors.Is(err, auth.ErrCredenciais) {
			t.Errorf("%s: err = %v, quer ErrCredenciais", nome, err)
		}
	}
	if _, err := c.cliente.Autenticar(c.ctx, "token-que-nao-existe"); !errors.Is(err, auth.ErrSessaoInvalida) {
		t.Errorf("token qualquer: err = %v", err)
	}
	if _, err := c.cliente.Autenticar(c.ctx, strings.Repeat("a", 43)); !errors.Is(err, auth.ErrSessaoInvalida) {
		t.Errorf("token com tamanho certo mas desconhecido: err = %v", err)
	}
}

func TestBloqueioPorTentativas(t *testing.T) {
	c := novoCenario(t)
	for i := 0; i < 5; i++ {
		if _, _, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", "errada-"+fmtInt(int64(i)), "", ""); !errors.Is(err, auth.ErrCredenciais) {
			t.Fatalf("tentativa %d: %v", i, err)
		}
	}
	// Bloqueada: nem a senha certa entra.
	_, _, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", "")
	var bloq auth.ErrBloqueada
	if !errors.As(err, &bloq) || !bloq.Ate.After(time.Now()) {
		t.Fatalf("depois de 5 falhas, err = %v, quer ErrBloqueada", err)
	}
	// Passado o tempo, entra e o contador zera.
	c.agora = time.Now().Add(16 * time.Minute)
	if _, _, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", ""); err != nil {
		t.Fatalf("depois do bloqueio: %v", err)
	}
	if _, _, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", "errada", "", ""); !errors.Is(err, auth.ErrCredenciais) {
		t.Errorf("uma falha nova não deve bloquear de novo: %v", err)
	}
}

func TestUsuarioInativoNaoEntraESessaoCai(t *testing.T) {
	c := novoCenario(t)
	token, _, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.pool(t).Exec(c.ctx, `UPDATE usuarios SET ativo = false`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.cliente.Autenticar(c.ctx, token); !errors.Is(err, auth.ErrSessaoInvalida) {
		t.Errorf("sessão de usuário desativado: err = %v", err)
	}
	if _, _, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", ""); !errors.Is(err, auth.ErrCredenciais) {
		t.Errorf("usuário desativado: err = %v", err)
	}
}

func TestSessaoExpira(t *testing.T) {
	c := novoCenario(t)
	token, _, _ := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", "")
	c.agora = c.agora.Add(11 * time.Hour)
	if _, err := c.cliente.Autenticar(c.ctx, token); err != nil {
		t.Fatalf("dentro do prazo: %v", err)
	}
	// O uso renovou a sessão por mais 12 h a partir daqui; passado esse prazo, expira.
	c.agora = c.agora.Add(13 * time.Hour)
	if _, err := c.cliente.Autenticar(c.ctx, token); !errors.Is(err, auth.ErrSessaoInvalida) {
		t.Errorf("sessão expirada: err = %v", err)
	}
}

func TestMasterEntraEmQualquerClienteEPodeTudo(t *testing.T) {
	c := novoCenario(t)
	token, sess, err := c.cliente.Entrar(c.ctx, "mestre@plataforma.com", c.masterPw, "", "")
	if err != nil {
		t.Fatalf("master no cliente: %v", err)
	}
	if sess.Conta.Tipo != auth.TipoPlataforma || !sess.Principal.Master || !sess.Principal.Pode("qualquer_coisa") {
		t.Errorf("sessão do master: %+v", sess.Conta)
	}
	if !sess.Conta.DeveTrocarSenha {
		t.Error("a senha gerada deve ser trocada no primeiro login")
	}
	if again, err := c.cliente.Autenticar(c.ctx, token); err != nil || !again.Principal.Master {
		t.Errorf("autenticar master: %v", err)
	}
	// A sessão do master fica registrada no schema do cliente.
	var n int
	_ = c.pool(t).QueryRow(c.ctx, `SELECT count(*) FROM sessoes WHERE master_id IS NOT NULL`).Scan(&n)
	if n != 1 {
		t.Errorf("sessões de master no cliente = %d, quer 1", n)
	}

	// Um usuário comum da plataforma (não master) NÃO entra no cliente.
	hash, _ := auth.Hash(senhaInicial)
	if _, err := c.pubQ.CreatePlataformaUsuario(c.ctx, sqlc.CreatePlataformaUsuarioParams{Nome: "Suporte", Email: "suporte@plataforma.com", SenhaHash: hash}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.cliente.Entrar(c.ctx, "suporte@plataforma.com", senhaInicial, "", ""); !errors.Is(err, auth.ErrCredenciais) {
		t.Errorf("não-master no cliente: err = %v", err)
	}
	// Mas entra no console.
	if _, sess, err := c.console.Entrar(c.ctx, "suporte@plataforma.com", senhaInicial, "", ""); err != nil || sess.Principal.Master {
		t.Errorf("não-master no console: err=%v", err)
	}
}

func TestTrocarSenha(t *testing.T) {
	c := novoCenario(t)
	_, sess, _ := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", "")
	outro, _, _ := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", "") // segunda sessão

	if err := c.cliente.TrocarSenha(c.ctx, sess, "errada", "Nova-Senha-123"); !errors.Is(err, auth.ErrSenhaAtual) {
		t.Errorf("senha atual errada: %v", err)
	}
	for nome, nova := range map[string]string{"curta": "Ab1", "sem número": "SomenteLetras", "sem letra": "1234567890", "igual à atual": senhaInicial} {
		var fraca auth.SenhaFracaError
		if err := c.cliente.TrocarSenha(c.ctx, sess, senhaInicial, nova); !errors.As(err, &fraca) {
			t.Errorf("%s: err = %v, quer SenhaFracaError", nome, err)
		}
	}
	if err := c.cliente.TrocarSenha(c.ctx, sess, senhaInicial, "Nova-Senha-123"); err != nil {
		t.Fatalf("trocar: %v", err)
	}
	if _, _, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", ""); !errors.Is(err, auth.ErrCredenciais) {
		t.Error("a senha antiga não deveria mais valer")
	}
	if _, _, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", "Nova-Senha-123", "", ""); err != nil {
		t.Errorf("a nova senha deveria valer: %v", err)
	}
	if _, err := c.cliente.Autenticar(c.ctx, outro); !errors.Is(err, auth.ErrSessaoInvalida) {
		t.Error("as outras sessões deveriam cair depois da troca")
	}
}

func TestResetarSenhaPorAdministrador(t *testing.T) {
	c := novoCenario(t)
	token, _, _ := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", "")
	for i := 0; i < 5; i++ { // deixa a conta bloqueada
		_, _, _ = c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", "errada", "", "")
	}

	temp, err := c.cliente.ResetarSenha(c.ctx, auth.TipoUsuario, c.usuarioID)
	if err != nil {
		t.Fatalf("resetar: %v", err)
	}
	if len(temp) != 16 {
		t.Errorf("senha temporária com %d caracteres", len(temp))
	}
	if _, err := c.cliente.Autenticar(c.ctx, token); !errors.Is(err, auth.ErrSessaoInvalida) {
		t.Error("as sessões da conta deveriam cair no reset")
	}
	if _, _, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senhaInicial, "", ""); !errors.Is(err, auth.ErrCredenciais) {
		t.Error("a senha antiga não deveria mais valer")
	}
	// O reset também tira o bloqueio; entra com a temporária e é obrigada a trocar.
	_, sess, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", temp, "", "")
	if err != nil {
		t.Fatalf("entrar com a temporária: %v", err)
	}
	if !sess.Conta.DeveTrocarSenha {
		t.Error("depois do reset, a troca de senha deve ser obrigatória")
	}
	if err := c.cliente.TrocarSenha(c.ctx, sess, temp, "Minha-Escolha-2026"); err != nil {
		t.Fatalf("trocar a temporária: %v", err)
	}
	if again, _ := c.cliente.Autenticar(c.ctx, mustToken(t, c, "Minha-Escolha-2026")); again == nil || again.Conta.DeveTrocarSenha {
		t.Error("depois de escolher a própria senha, não deve mais exigir troca")
	}

	if _, err := c.cliente.ResetarSenha(c.ctx, auth.TipoUsuario, "999999"); !errors.Is(err, auth.ErrNaoEncontrada) {
		t.Errorf("reset de conta inexistente: %v", err)
	}
}

func mustToken(t *testing.T, c *cenario, senha string) string {
	t.Helper()
	token, _, err := c.cliente.Entrar(c.ctx, "agente@alfa.gov.br", senha, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestValidarSenha(t *testing.T) {
	boas := []string{"Abcdefghi1", "uma frase longa 2026", "Ação-Número-9"}
	for _, s := range boas {
		if err := auth.ValidarSenha(s, "x@y.com"); err != nil {
			t.Errorf("%q deveria servir: %v", s, err)
		}
	}
	ruins := []string{"", "curta1A", "somenteletrasaqui", "1234567890123", "joao.silva@x.com", strings.Repeat("a1", 70)}
	for _, s := range ruins {
		if err := auth.ValidarSenha(s, "joao.silva@x.com"); err == nil {
			t.Errorf("%q deveria ser recusada", s)
		}
	}
	if err := auth.ValidarSenha("joao.silva1", "joao.silva@x.com"); err != nil {
		t.Errorf("parecida com o e-mail mas diferente, deveria servir: %v", err)
	}
}

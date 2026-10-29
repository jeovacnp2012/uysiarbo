package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/auth"
	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/tenant"
	"uysiarbo/internal/testutil"
)

const (
	hostNM      = "uysiarbonm.inovarsistemas.biz"
	hostSinop   = "uysiarbosn.inovarsistemas.biz"
	hostConsole = "uysiarbo.inovarsistemas.biz"

	emailAna   = "ana@nm.gov.br"
	senhaAna   = "Senha-Da-Ana-1"
	emailBruno = "bruno@nm.gov.br"
	senhaTemp  = "Temp-Do-Bruno-1"
)

type captchaFalso struct {
	ok  bool
	err error
}

func (c *captchaFalso) Verificar(context.Context, string, string) (bool, error) { return c.ok, c.err }

type mundo struct {
	h        http.Handler
	captcha  *captchaFalso
	masterPw string
	cliQ     *sqlccliente.Queries // consultas no schema de Nova Mutum, para montar cenários
	pool     *pgxpool.Pool        // conexão geral (schema public), para consultas diretas
	pubQ     *sqlc.Queries
}

// novoMundo sobe o servidor completo (com login) sobre um banco temporário, com dois clientes, dois
// usuários no primeiro (Ana, normal; Bruno, com troca de senha obrigatória) e um master.
func novoMundo(t *testing.T) *mundo {
	t.Helper()
	ctx := context.Background()
	url := testutil.NewDatabase(t)
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &tenant.Service{Pool: pool, URL: url, Log: log, Auditoria: audit.Plataforma{Q: sqlc.New(pool)}}
	for _, in := range []tenant.Input{
		{Nome: "Prefeitura de Nova Mutum", Slug: "novamutum", Subdominio: "uysiarbonm"},
		{Nome: "Prefeitura de Sinop", Slug: "sinop", Subdominio: "uysiarbosn"},
	} {
		if _, err := svc.Provision(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	_, masterPw, err := svc.CriarMaster(ctx, "Mestre", "mestre@plataforma.com")
	if err != nil {
		t.Fatal(err)
	}

	pools := &tenant.Pools{URL: url}
	t.Cleanup(pools.Close)
	cp, _ := pools.Get(ctx, "c_novamutum")
	q := sqlccliente.New(cp)
	papel, err := q.GetPapelByNome(ctx, tenant.NomePapelAdministrador)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct {
		nome, email, senha string
		trocar             bool
	}{{"Ana Souza", emailAna, senhaAna, false}, {"Bruno Lima", emailBruno, senhaTemp, true}} {
		hash, _ := auth.Hash(u.senha)
		if _, err := q.CreateUsuario(ctx, sqlccliente.CreateUsuarioParams{
			Nome: u.nome, Email: u.email, SenhaHash: hash, PapelID: papel.ID, LoginWeb: true, DeveTrocarSenha: u.trocar,
		}); err != nil {
			t.Fatal(err)
		}
	}

	cap := &captchaFalso{ok: true}
	return &mundo{captcha: cap, masterPw: masterPw, cliQ: q, pool: pool, pubQ: sqlc.New(pool), h: New(Deps{
		Log: log, DB: pool, Catalog: sqlc.New(pool), Pools: pools,
		Resolver: &tenant.Resolver{Pool: pool, BaseDomain: "inovarsistemas.biz", AdminHost: hostConsole},
		Public:   sqlc.New(pool), Captcha: cap, RecaptchaSite: "chave-publica-de-teste", Tenants: svc,
	})}
}

// navegador imita um navegador num endereço: guarda cookies entre as requisições.
type navegador struct {
	t       *testing.T
	h       http.Handler
	host    string
	cookies map[string]string
}

func (m *mundo) em(t *testing.T, host string) *navegador {
	return &navegador{t: t, h: m.h, host: host, cookies: map[string]string{}}
}

func (n *navegador) enviar(method, path string, form url.Values, cab map[string]string) *httptest.ResponseRecorder {
	n.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, path, body)
	req.Host = n.host
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range cab {
		req.Header.Set(k, v)
	}
	for k, v := range n.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	n.h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(n.cookies, c.Name)
		} else {
			n.cookies[c.Name] = c.Value
		}
	}
	return rec
}

func (n *navegador) get(path string) *httptest.ResponseRecorder {
	return n.enviar(http.MethodGet, path, nil, nil)
}

// post envia um formulário com o token CSRF do cookie (busca /entrar antes, se ainda não há cookie).
func (n *navegador) post(path string, form url.Values) *httptest.ResponseRecorder {
	n.t.Helper()
	if n.cookies[csrfCookie] == "" {
		n.get("/entrar")
	}
	if form.Get("csrf") == "" {
		form.Set("csrf", n.cookies[csrfCookie])
	}
	return n.enviar(http.MethodPost, path, form, nil)
}

func (n *navegador) entrar(email, senha, proximo string) *httptest.ResponseRecorder {
	return n.post("/entrar", url.Values{"email": {email}, "senha": {senha}, "proximo": {proximo}, "g-recaptcha-response": {"ok"}})
}

func local(rec *httptest.ResponseRecorder) string { return rec.Header().Get("Location") }

func quer(t *testing.T, rec *httptest.ResponseRecorder, status int, contem ...string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, quer %d (Location=%q, corpo: %.300s)", rec.Code, status, local(rec), rec.Body.String())
	}
	for _, c := range contem {
		if !strings.Contains(rec.Body.String(), c) {
			t.Errorf("a resposta deveria conter %q", c)
		}
	}
}

func TestPaginasExigemLogin(t *testing.T) {
	m := novoMundo(t)
	n := m.em(t, hostNM)

	quer(t, n.get("/"), http.StatusSeeOther)
	if got := local(n.get("/")); got != "/entrar?proximo=%2F" {
		t.Errorf("Location = %q", got)
	}
	if got := local(n.get("/componentes?x=1")); got != "/entrar?proximo=%2Fcomponentes%3Fx%3D1" {
		t.Errorf("Location = %q", got)
	}
	// HTMX recebe o redirecionamento no cabeçalho, não no corpo.
	rec := n.enviar(http.MethodGet, "/componentes/tabela", nil, map[string]string{"HX-Request": "true"})
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("HX-Redirect") != "/entrar" {
		t.Errorf("HTMX: status=%d HX-Redirect=%q", rec.Code, rec.Header().Get("HX-Redirect"))
	}
	// Monitoramento e arquivos estáticos continuam abertos.
	quer(t, n.get("/healthz"), http.StatusOK)
	quer(t, n.get("/static/css/app.css"), http.StatusOK)
}

func TestTelaDeEntrar(t *testing.T) {
	m := novoMundo(t)
	rec := m.em(t, hostNM).get("/entrar")
	quer(t, rec, http.StatusOK, `name="csrf"`, "Revelar", "g-recaptcha", "chave-publica-de-teste", "Prefeitura de Nova Mutum", "Entrar")
	if rec.Header().Get("X-Frame-Options") != "DENY" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("faltam os cabeçalhos de segurança")
	}
	var csrf *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == csrfCookie {
			csrf = c
		}
	}
	if csrf == nil || !csrf.HttpOnly || csrf.SameSite != http.SameSiteLaxMode || len(csrf.Value) != 43 {
		t.Errorf("cookie CSRF inesperado: %+v", csrf)
	}
	quer(t, m.em(t, hostConsole).get("/entrar"), http.StatusOK, "Console da plataforma")
}

func TestProtecaoCSRFEOrigem(t *testing.T) {
	m := novoMundo(t)
	n := m.em(t, hostNM)
	base := url.Values{"email": {emailAna}, "senha": {senhaAna}, "g-recaptcha-response": {"ok"}}

	// Sem cookie nem token.
	quer(t, n.enviar(http.MethodPost, "/entrar", base, nil), http.StatusForbidden)

	n.get("/entrar") // ganha o cookie
	errado := url.Values{"email": {emailAna}, "senha": {senhaAna}, "csrf": {strings.Repeat("x", 43)}}
	quer(t, n.enviar(http.MethodPost, "/entrar", errado, nil), http.StatusForbidden)

	certo := url.Values{"email": {emailAna}, "senha": {senhaAna}, "csrf": {n.cookies[csrfCookie]}, "g-recaptcha-response": {"ok"}}
	quer(t, n.enviar(http.MethodPost, "/entrar", certo, map[string]string{"Origin": "https://site-malicioso.com"}), http.StatusForbidden)
	rec := n.enviar(http.MethodPost, "/entrar", certo, map[string]string{"Origin": "https://" + hostNM})
	quer(t, rec, http.StatusSeeOther)

	// O token também vale no cabeçalho (é como o HTMX envia).
	quer(t, n.enviar(http.MethodPost, "/sair", url.Values{}, map[string]string{"X-CSRF-Token": n.cookies[csrfCookie]}), http.StatusSeeOther)
}

func TestLoginRecusaSemRevelarNada(t *testing.T) {
	m := novoMundo(t)
	n := m.em(t, hostNM)

	senhaErrada := n.entrar(emailAna, "errada-123456", "")
	quer(t, senhaErrada, http.StatusUnauthorized, "E-mail ou senha incorretos.")
	inexistente := n.entrar("ninguem@nm.gov.br", senhaAna, "")
	quer(t, inexistente, http.StatusUnauthorized, "E-mail ou senha incorretos.")
	if n.cookies[sessaoCookie] != "" {
		t.Error("login recusado não pode criar sessão")
	}

	// Desafio do captcha não resolvido: recusa mesmo com a senha certa.
	m.captcha.ok = false
	quer(t, n.entrar(emailAna, senhaAna, ""), http.StatusBadRequest, "não é um robô")
	m.captcha.ok, m.captcha.err = true, errors.New("sem rede")
	quer(t, n.entrar(emailAna, senhaAna, ""), http.StatusServiceUnavailable, "verificar o desafio")
	if n.cookies[sessaoCookie] != "" {
		t.Error("captcha recusado não pode criar sessão")
	}
}

func TestLoginEFluxoDeUsoComSessao(t *testing.T) {
	m := novoMundo(t)
	n := m.em(t, hostNM)

	rec := n.entrar(emailAna, senhaAna, "/componentes")
	quer(t, rec, http.StatusSeeOther)
	if local(rec) != "/componentes" {
		t.Errorf("destino = %q", local(rec))
	}
	var sess *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessaoCookie {
			sess = c
		}
	}
	if sess == nil || !sess.HttpOnly || sess.SameSite != http.SameSiteLaxMode || sess.Secure || len(sess.Value) != 43 {
		t.Fatalf("cookie de sessão inesperado: %+v", sess)
	}

	quer(t, n.get("/"), http.StatusOK, "Ana Souza", "Sair", "Prefeitura de Nova Mutum", "X-CSRF-Token")
	if got := local(n.get("/entrar")); got != "/" {
		t.Errorf("logada, /entrar deveria voltar para a página inicial, veio %q", got)
	}

	// Sair encerra a sessão no banco: reaproveitar o cookie antigo não funciona.
	antigo := n.cookies[sessaoCookie]
	quer(t, n.post("/sair", url.Values{}), http.StatusSeeOther)
	if n.cookies[sessaoCookie] != "" {
		t.Error("o cookie de sessão deveria ser apagado")
	}
	n.cookies[sessaoCookie] = antigo
	if got := local(n.get("/")); !strings.HasPrefix(got, "/entrar") {
		t.Errorf("sessão encerrada não pode voltar a valer: %q", got)
	}
}

func TestCookieSecureAtrasDoNginx(t *testing.T) {
	m := novoMundo(t)
	n := m.em(t, hostNM)
	n.get("/entrar")
	form := url.Values{"email": {emailAna}, "senha": {senhaAna}, "csrf": {n.cookies[csrfCookie]}, "g-recaptcha-response": {"ok"}}
	rec := n.enviar(http.MethodPost, "/entrar", form, map[string]string{"X-Forwarded-Proto": "https"})
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessaoCookie && !c.Secure {
			t.Error("com HTTPS, o cookie de sessão deve ser Secure")
		}
	}
}

func TestRedirecionamentoSoParaOMesmoSite(t *testing.T) {
	for entrada, esperado := range map[string]string{
		"": "/", "/": "/", "/componentes": "/componentes", "//evil.com": "/", "https://evil.com": "/",
		"/\\evil.com": "/", "javascript:alert(1)": "/", "/ok\r\nSet-Cookie: x=1": "/",
	} {
		if got := destinoSeguro(entrada); got != esperado {
			t.Errorf("destinoSeguro(%q) = %q, quer %q", entrada, got, esperado)
		}
	}
	m := novoMundo(t)
	rec := m.em(t, hostNM).entrar(emailAna, senhaAna, "//evil.com")
	if local(rec) != "/" {
		t.Errorf("Location = %q, quer /", local(rec))
	}
}

func TestBloqueioPelaTela(t *testing.T) {
	m := novoMundo(t)
	n := m.em(t, hostNM)
	for i := 0; i < 5; i++ {
		quer(t, n.entrar(emailAna, "errada-12345"+string(rune('a'+i)), ""), http.StatusUnauthorized)
	}
	quer(t, n.entrar(emailAna, senhaAna, ""), http.StatusTooManyRequests, "Muitas tentativas erradas")
}

func TestTrocaDeSenhaObrigatoriaDepoisDoReset(t *testing.T) {
	m := novoMundo(t)
	n := m.em(t, hostNM)

	rec := n.entrar(emailBruno, senhaTemp, "/componentes")
	if quer(t, rec, http.StatusSeeOther); local(rec) != "/trocar-senha" {
		t.Fatalf("depois de entrar com a temporária: %q", local(rec))
	}
	// Enquanto não troca, nada mais abre.
	if got := local(n.get("/")); got != "/trocar-senha" {
		t.Errorf("GET / = %q, quer /trocar-senha", got)
	}
	if got := local(n.get("/componentes")); got != "/trocar-senha" {
		t.Errorf("GET /componentes = %q", got)
	}
	quer(t, n.get("/trocar-senha"), http.StatusOK, "temporária", "Revelar", "Repita a nova senha")

	troca := func(atual, nova, conf string) *httptest.ResponseRecorder {
		return n.post("/trocar-senha", url.Values{"atual": {atual}, "nova": {nova}, "confirmar": {conf}})
	}
	quer(t, troca(senhaTemp, "Minha-Senha-2026", "outra-coisa"), http.StatusBadRequest, "confirmação não confere")
	quer(t, troca(senhaTemp, "curta1", "curta1"), http.StatusBadRequest, "pelo menos 10")
	quer(t, troca("senha-atual-errada", "Minha-Senha-2026", "Minha-Senha-2026"), http.StatusBadRequest, "senha atual não confere")
	quer(t, troca(senhaTemp, senhaTemp, senhaTemp), http.StatusBadRequest, "diferente da atual")

	if rec := troca(senhaTemp, "Minha-Senha-2026", "Minha-Senha-2026"); rec.Code != http.StatusSeeOther || local(rec) != "/" {
		t.Fatalf("troca válida: status=%d Location=%q", rec.Code, local(rec))
	}
	quer(t, n.get("/"), http.StatusOK, "Bruno Lima")

	// A temporária morreu; a nova vale.
	outro := m.em(t, hostNM)
	quer(t, outro.entrar(emailBruno, senhaTemp, ""), http.StatusUnauthorized)
	if rec := outro.entrar(emailBruno, "Minha-Senha-2026", ""); local(rec) != "/" {
		t.Errorf("com a senha nova: %d %q", rec.Code, local(rec))
	}
}

func TestMasterEntraNoClienteENoConsole(t *testing.T) {
	m := novoMundo(t)

	// No endereço de um cliente (e em outro cliente também): o master entra e vê tudo.
	for _, host := range []string{hostNM, hostSinop} {
		n := m.em(t, host)
		rec := n.entrar("mestre@plataforma.com", m.masterPw, "")
		if host == hostNM {
			// A conta é uma só: começa com a senha temporária e troca no primeiro acesso.
			if local(rec) != "/trocar-senha" {
				t.Fatalf("%s: o master começa com senha temporária: %d %q", host, rec.Code, local(rec))
			}
			quer(t, n.post("/trocar-senha", url.Values{"atual": {m.masterPw}, "nova": {"Mestre-Escolha-99"}, "confirmar": {"Mestre-Escolha-99"}}), http.StatusSeeOther)
			m.masterPw = "Mestre-Escolha-99"
		} else if local(rec) != "/" {
			t.Fatalf("%s: depois de escolher a senha, entra direto: %d %q", host, rec.Code, local(rec))
		}
		quer(t, n.get("/"), http.StatusOK, "Mestre")
	}

	// No console da plataforma.
	c := m.em(t, hostConsole)
	quer(t, c.entrar("mestre@plataforma.com", m.masterPw, ""), http.StatusSeeOther)
	quer(t, c.get("/"), http.StatusOK, "Console da plataforma", "Mestre")

	// Um usuário de cliente NÃO entra no console.
	quer(t, m.em(t, hostConsole).entrar(emailAna, senhaAna, ""), http.StatusUnauthorized)
}

func TestSessaoDeUmClienteNaoValeEmOutro(t *testing.T) {
	m := novoMundo(t)
	nm := m.em(t, hostNM)
	nm.entrar(emailAna, senhaAna, "")
	quer(t, nm.get("/"), http.StatusOK)

	outro := m.em(t, hostSinop)
	outro.cookies[sessaoCookie] = nm.cookies[sessaoCookie] // mesmo token, outro cliente
	if got := local(outro.get("/")); !strings.HasPrefix(got, "/entrar") {
		t.Errorf("a sessão de Nova Mutum não pode valer em Sinop: %q", got)
	}
	// E um usuário de Nova Mutum não entra em Sinop.
	quer(t, m.em(t, hostSinop).entrar(emailAna, senhaAna, ""), http.StatusUnauthorized)
}

func TestGoogleCaptchaFalaComOSiteVerify(t *testing.T) {
	var recebido url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		recebido = r.PostForm
		if r.PostForm.Get("response") == "valida" {
			_, _ = io.WriteString(w, `{"success": true}`)
			return
		}
		_, _ = io.WriteString(w, `{"success": false, "error-codes": ["invalid-input-response"]}`)
	}))
	defer srv.Close()

	g := GoogleCaptcha{Secret: "segredo", URL: srv.URL}
	if ok, err := g.Verificar(context.Background(), "valida", "1.2.3.4"); err != nil || !ok {
		t.Errorf("resposta válida: ok=%v err=%v", ok, err)
	}
	if recebido.Get("secret") != "segredo" || recebido.Get("remoteip") != "1.2.3.4" {
		t.Errorf("parâmetros enviados ao Google: %v", recebido)
	}
	if ok, err := g.Verificar(context.Background(), "invalida", ""); err != nil || ok {
		t.Errorf("resposta inválida: ok=%v err=%v", ok, err)
	}
	if ok, _ := g.Verificar(context.Background(), "", ""); ok {
		t.Error("resposta vazia nunca passa (nem chega a consultar o Google)")
	}
}

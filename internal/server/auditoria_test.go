package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
	"uysiarbo/internal/auth"

	"github.com/go-chi/chi/v5"

	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/tenant"
	"uysiarbo/internal/testutil"
)

// eventos devolve o histórico do cliente (do mais recente para o mais antigo).
func (m *mundo) eventos(t *testing.T) []sqlccliente.Auditorium {
	t.Helper()
	rows, err := m.cliQ.ListAuditoria(t.Context(), sqlccliente.ListAuditoriaParams{Limite: 500})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func achar(eventos []sqlccliente.Auditorium, acao string) *sqlccliente.Auditorium {
	for i := range eventos {
		if eventos[i].Acao == acao {
			return &eventos[i]
		}
	}
	return nil
}

func mapa(b []byte) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func TestAuditoriaRegistraAcoesComAtorEAntesDepois(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	papelAdm := strconv.FormatInt(m.idPapel(t, "Administrador"), 10)

	// Ações: papel novo, usuário novo, edição (muda o papel), reset de senha, troca da própria senha.
	quer(t, ana.post("/papeis", url.Values{"nome": {"Agente"}, "descricao": {"Campo"}, "ativo": {"1"}, "perm": {"visita_adicionar", "visita_visualizar"}}), http.StatusSeeOther)
	rec := ana.post("/usuarios", url.Values{"nome": {"Diego Alves"}, "email": {"diego@nm.gov.br"}, "papel_id": {papelAdm}, "login_web": {"1"}})
	quer(t, rec, http.StatusOK)
	senhaTemp := senhaDaPagina(t, rec.Body.String())
	idDiego := m.idUsuario(t, "diego@nm.gov.br")
	idAgente := strconv.FormatInt(m.idPapel(t, "Agente"), 10)
	quer(t, ana.post("/usuarios/"+idDiego, url.Values{"nome": {"Diego Alves"}, "papel_id": {idAgente}, "login_web": {"1"}, "ativo": {"1"}}), http.StatusSeeOther)
	rec = ana.post("/usuarios/"+idDiego+"/resetar-senha", url.Values{})
	quer(t, rec, http.StatusOK)
	senhaReset := senhaDaPagina(t, rec.Body.String())
	quer(t, m.comMaster(t).get("/"), http.StatusOK) // o master também deixa rastro (login)

	ev := m.eventos(t)

	// Login da Ana, com ator, IP e navegador.
	var login *sqlccliente.Auditorium
	for i := range ev {
		if ev[i].Acao == "login.sucesso" && ev[i].AtorEmail == emailAna {
			login = &ev[i]
		}
	}
	if login == nil || login.AtorEmail != emailAna || login.AtorTipo != "usuario" || login.Ip != "192.0.2.1" || login.RequestID == "" {
		t.Errorf("login.sucesso inesperado: %+v", login)
	}

	// Papel criado: o depois traz os campos e o detalhe lista as permissões acrescentadas.
	pc := achar(ev, "papel.criar")
	if pc == nil || pc.Rotulo != "Agente" || mapa(pc.Depois)["nome"] != "Agente" {
		t.Fatalf("papel.criar: %+v", pc)
	}
	if adic, _ := mapa(pc.Detalhes)["permissoes_adicionadas"].([]any); len(adic) != 2 {
		t.Errorf("permissões acrescentadas = %v", mapa(pc.Detalhes))
	}

	// Usuário criado.
	uc := achar(ev, "usuario.criar")
	if uc == nil || uc.Rotulo != "Diego Alves" || mapa(uc.Depois)["email"] != "diego@nm.gov.br" || mapa(uc.Depois)["papel"] != "Administrador" {
		t.Errorf("usuario.criar: %+v", uc)
	}

	// Edição: só o campo que mudou (o papel), com antes e depois.
	ue := achar(ev, "usuario.editar")
	if ue == nil {
		t.Fatal("faltou usuario.editar")
	}
	if a, d := mapa(ue.Antes), mapa(ue.Depois); len(a) != 1 || a["papel"] != "Administrador" || len(d) != 1 || d["papel"] != "Agente" {
		t.Errorf("antes=%v depois=%v (deveria ter só o papel)", a, d)
	}

	// Reset de senha: quem fez (Ana) e em quem (Diego).
	ur := achar(ev, "usuario.resetar_senha")
	if ur == nil || ur.AtorEmail != emailAna || ur.Rotulo != "Diego Alves" {
		t.Errorf("usuario.resetar_senha: %+v", ur)
	}

	// O master também fica registrado, como ator da plataforma.
	var master *sqlccliente.Auditorium
	for i := range ev {
		if ev[i].AtorTipo == "plataforma" && ev[i].Acao == "login.sucesso" {
			master = &ev[i]
		}
	}
	if master == nil || master.AtorEmail != "mestre@plataforma.com" {
		t.Errorf("o login do master deveria estar no histórico do cliente: %+v", master)
	}

	// Nenhum segredo no histórico, em lugar nenhum.
	for _, e := range ev {
		todo := strings.ToLower(string(e.Antes) + string(e.Depois) + string(e.Detalhes) + e.Rotulo + e.UserAgent)
		for _, proibido := range []string{strings.ToLower(senhaTemp), strings.ToLower(senhaReset), strings.ToLower(senhaAna), "argon2id", "senha_hash"} {
			if strings.Contains(todo, proibido) {
				t.Errorf("a auditoria (%s) contém %q", e.Acao, proibido)
			}
		}
	}
}

func TestAuditoriaDeLoginFalhoEBloqueio(t *testing.T) {
	m := novoMundo(t)
	n := m.em(t, hostNM)

	// E-mail que não existe: nada é registrado (evita encher o histórico com tentativas aleatórias).
	n.entrar("ninguem@nm.gov.br", "qualquer-123456", "")
	if len(m.eventos(t)) != 0 {
		t.Fatalf("e-mail inexistente não deveria gerar registro: %+v", m.eventos(t))
	}

	for i := 0; i < 5; i++ {
		n.entrar(emailAna, "errada-12345"+string(rune('a'+i)), "")
	}
	n.entrar(emailAna, senhaAna, "") // bloqueada: não gera registro novo

	ev := m.eventos(t)
	falhas, bloqueios := 0, 0
	for _, e := range ev {
		switch e.Acao {
		case "login.falha":
			falhas++
			if e.AtorEmail != emailAna {
				t.Errorf("ator da falha = %q", e.AtorEmail)
			}
		case "login.bloqueio":
			bloqueios++
		case "login.sucesso":
			t.Error("a tentativa durante o bloqueio não pode ter dado certo")
		}
	}
	if falhas != 5 || bloqueios != 1 {
		t.Errorf("falhas=%d bloqueios=%d, quer 5 e 1 (%d eventos)", falhas, bloqueios, len(ev))
	}
}

func TestAuditoriaNaoPodeSerAlteradaNemApagada(t *testing.T) {
	m := novoMundo(t)
	m.comAna(t) // gera ao menos um evento
	for nome, sql := range map[string]string{
		"alterar":               `UPDATE c_novamutum.auditoria SET acao = 'x'`,
		"apagar":                `DELETE FROM c_novamutum.auditoria`,
		"truncar":               `TRUNCATE c_novamutum.auditoria`,
		"alterar na plataforma": `UPDATE auditoria_plataforma SET acao = 'x'`,
		"apagar na plataforma":  `DELETE FROM auditoria_plataforma`,
		"truncar na plataforma": `TRUNCATE auditoria_plataforma`,
	} {
		_, err := m.pool.Exec(t.Context(), sql)
		if err == nil || !strings.Contains(err.Error(), "não pode ser alterada nem apagada") {
			t.Errorf("%s: o banco deveria recusar, veio %v", nome, err)
		}
	}
	if len(m.eventos(t)) == 0 {
		t.Error("o histórico não pode ter sumido")
	}
}

func TestRegistroGenericoCobreGravacoesSemEventoProprio(t *testing.T) {
	ctx := context.Background()
	url := testutil.NewDatabase(t)
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	h := &handlers{log: slog.New(slog.NewTextHandler(io.Discard, nil)), pub: sqlc.New(pool), pools: &tenant.Pools{URL: url}}

	r := chi.NewRouter()
	r.Use(requestID, h.auditoriaGenerica)
	r.Post("/sem-evento", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Post("/com-evento", func(w http.ResponseWriter, req *http.Request) {
		naoAuditar(req) // o handler já tratou a auditoria por conta própria
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/erro", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) })
	r.Get("/leitura", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	for _, c := range []struct{ metodo, caminho string }{
		{http.MethodPost, "/sem-evento"}, {http.MethodPost, "/com-evento"}, {http.MethodPost, "/erro"}, {http.MethodGet, "/leitura"},
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(c.metodo, c.caminho, nil))
	}

	rows, err := sqlc.New(pool).ListAuditoriaPlataforma(ctx, sqlc.ListAuditoriaPlataformaParams{Limite: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Acao != "requisicao.escrita" {
		t.Fatalf("só a gravação sem evento próprio deveria ser registrada, veio %d: %+v", len(rows), rows)
	}
	det := mapa(rows[0].Detalhes)
	if det["caminho"] != "/sem-evento" || det["metodo"] != "POST" || det["status"] != float64(200) {
		t.Errorf("detalhes do registro genérico: %v", det)
	}
}

func TestTelaDeAuditoriaDoCliente(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.post("/papeis", url.Values{"nome": {"Agente"}, "ativo": {"1"}, "perm": {"visita_adicionar"}}), http.StatusSeeOther)
	idAna := m.idUsuario(t, emailAna)
	quer(t, ana.post("/usuarios/"+idAna, url.Values{"nome": {"Ana Souza Lima"}, "papel_id": {strconv.FormatInt(m.idPapel(t, "Administrador"), 10)}, "login_web": {"1"}, "ativo": {"1"}}), http.StatusSeeOther)

	quer(t, ana.get("/auditoria"), http.StatusOK, "Auditoria", "Entrou no sistema", "Criou um papel", "Ana Souza", "Editou um usuário")
	// Detalhes: antes e depois legíveis.
	quer(t, ana.get("/auditoria?acao=usuario.editar"), http.StatusOK, "Editou um usuário", "Ana Souza Lima", "Antes", "Depois")
	rec := ana.get("/auditoria?acao=papel.criar")
	quer(t, rec, http.StatusOK, "Criou um papel", "Agente", "Permissões acrescentadas", "Adicionar (visita_adicionar)")
	if strings.Contains(rec.Body.String(), "<summary>Entrou no sistema") {
		t.Error("o filtro por ação deveria esconder os outros eventos")
	}
	quer(t, ana.get("/auditoria?ator=ana@nm"), http.StatusOK, "Entrou no sistema")
	quer(t, ana.get("/auditoria?busca=agente"), http.StatusOK, "Criou um papel")
	quer(t, ana.get("/auditoria?de=2000-01-01&ate=2000-01-02"), http.StatusOK, "Nenhum registro encontrado")
	quer(t, ana.get("/auditoria?pagina=999"), http.StatusOK, "página 1 de 1")
	hoje := time.Now().In(fusoLocal).Format("2006-01-02")
	quer(t, ana.get("/auditoria?de="+hoje+"&ate="+hoje), http.StatusOK, "Entrou no sistema")

	// Quem não tem a permissão não vê a tela nem o item do menu.
	carla := m.comCarla(t)
	quer(t, carla.get("/auditoria"), http.StatusForbidden, "Sem permissão")
	if strings.Contains(carla.get("/").Body.String(), `href="/auditoria"`) {
		t.Error("o menu da Carla não deveria ter Auditoria")
	}
	// O master vê.
	quer(t, m.comMaster(t).get("/auditoria"), http.StatusOK, "Auditoria")
}

func TestMasterVeTodasAsTelasEmCadaAmbiente(t *testing.T) {
	m := novoMundo(t)

	// Dentro de um cliente: usuários, papéis, auditoria e todos os módulos do catálogo.
	master := m.comMaster(t)
	corpo := master.get("/").Body.String()
	for _, link := range []string{`href="/usuarios"`, `href="/papeis"`, `href="/auditoria"`} {
		if !strings.Contains(corpo, link) {
			t.Errorf("o menu do master no cliente deveria ter %s", link)
		}
	}
	if n := strings.Count(corpo, `class="modulo`); n != 23 {
		t.Errorf("o master deveria ver os 23 módulos no cliente, vê %d", n)
	}
	for _, path := range []string{"/usuarios", "/papeis", "/auditoria", "/usuarios/novo", "/papeis/novo"} {
		quer(t, master.get(path), http.StatusOK)
	}

	// No console: clientes, usuários da plataforma e auditoria.
	console := m.em(t, hostConsole)
	console.entrar("mestre@plataforma.com", "Mestre-Escolha-99", "")
	corpo = console.get("/").Body.String()
	for _, link := range []string{`href="/clientes"`, `href="/plataforma/usuarios"`, `href="/auditoria"`} {
		if !strings.Contains(corpo, link) {
			t.Errorf("o menu do master no console deveria ter %s", link)
		}
	}
	for _, path := range []string{"/clientes", "/clientes/novo", "/plataforma/usuarios", "/plataforma/usuarios/novo", "/auditoria"} {
		quer(t, console.get(path), http.StatusOK)
	}
	// Telas do cliente não existem no console, e as do console não existem no cliente.
	quer(t, console.get("/usuarios"), http.StatusNotFound)
	quer(t, master.get("/clientes"), http.StatusNotFound)
}

func TestConsoleCriaClienteComSubdominioEscolhido(t *testing.T) {
	m := novoMundo(t)
	console := m.consoleMaster(t)

	rec := console.get("/clientes")
	quer(t, rec, http.StatusOK, "Prefeitura de Nova Mutum", "Prefeitura de Sinop", hostNM, "Abrir ambiente", "Suspender")
	quer(t, console.get("/clientes/novo"), http.StatusOK, "Subdomínio de acesso", "inovarsistemas.biz")

	form := func(nome, slug, sub string) url.Values {
		return url.Values{"nome": {nome}, "slug": {slug}, "subdominio": {sub}, "plano": {"padrao"}, "cidade": {"Cidade Teste"}, "uf": {"MT"}}
	}
	quer(t, console.post("/clientes", form("Prefeitura de Teste", "teste", "uysiarbotst")), http.StatusSeeOther)
	novo, err := m.pubQ.GetClienteBySlug(t.Context(), "teste")
	if err != nil || novo.Subdominio != "uysiarbotst" || novo.Status != "ativo" || novo.SchemaNome != "c_teste" {
		t.Fatalf("cliente criado: %+v, %v", novo, err)
	}
	// O ambiente já responde no endereço escolhido, com o papel Administrador pronto.
	quer(t, m.em(t, "uysiarbotst.inovarsistemas.biz").get("/entrar"), http.StatusOK, "Prefeitura de Teste")

	// Recusas.
	quer(t, console.post("/clientes", form("Outro", "teste", "uysiarbooutro")), http.StatusBadRequest, "Já existe um cliente com esse identificador")
	quer(t, console.post("/clientes", form("Outro", "outro", "uysiarbotst")), http.StatusBadRequest, "subdomínio ou domínio")
	quer(t, console.post("/clientes", form("Outro", "outro", "www")), http.StatusBadRequest, "reservado")
	quer(t, console.post("/clientes", form("Outro", "outro", "-ruim")), http.StatusBadRequest, "Subdomínio inválido")
	quer(t, console.post("/clientes", form("Outro", "A-B", "uysiarboab")), http.StatusBadRequest, "Identificador inválido")
	quer(t, console.post("/clientes", form("", "outro", "uysiarboxx")), http.StatusBadRequest, "nome do cliente")
	quer(t, console.post("/clientes", form("Outro", "outro", "")), http.StatusBadRequest, "subdomínio")

	// Quem criou fica registrado: o master, e não "sistema".
	rows, _ := m.pubQ.ListAuditoriaPlataforma(t.Context(), sqlc.ListAuditoriaPlataformaParams{Acao: "cliente.criar", Limite: 10})
	var achou bool
	for _, r := range rows {
		if r.EntidadeID == "teste" {
			achou = true
			if r.AtorTipo != "plataforma" || r.AtorEmail != "mestre@plataforma.com" || r.Ip == "" {
				t.Errorf("ator do cliente.criar: %q %q ip=%q", r.AtorTipo, r.AtorEmail, r.Ip)
			}
			if mapa(r.Depois)["subdominio"] != "uysiarbotst" {
				t.Errorf("depois = %s", r.Depois)
			}
		}
	}
	if !achou {
		t.Error("a criação do cliente deveria estar na auditoria da plataforma")
	}
	quer(t, console.get("/auditoria?acao=cliente.criar"), http.StatusOK, "Criou um cliente", "Prefeitura de Teste", "uysiarbotst")
}

func TestConsoleSuspendeEReativaClienteNaHora(t *testing.T) {
	m := novoMundo(t)
	console := m.consoleMaster(t)
	cliente := m.em(t, hostSinop)
	quer(t, cliente.get("/entrar"), http.StatusOK)

	quer(t, console.post("/clientes/sinop/status", url.Values{"status": {"suspenso"}}), http.StatusSeeOther)
	quer(t, cliente.get("/entrar"), http.StatusForbidden, "suspenso") // sem esperar o cache expirar
	quer(t, console.get("/clientes"), http.StatusOK, "Reativar")

	quer(t, console.post("/clientes/sinop/status", url.Values{"status": {"ativo"}}), http.StatusSeeOther)
	quer(t, cliente.get("/entrar"), http.StatusOK)

	quer(t, console.post("/clientes/sinop/status", url.Values{"status": {"cancelado"}}), http.StatusBadRequest)
	quer(t, console.post("/clientes/nao-existe/status", url.Values{"status": {"ativo"}}), http.StatusNotFound)

	rows, _ := m.pubQ.ListAuditoriaPlataforma(t.Context(), sqlc.ListAuditoriaPlataformaParams{Acao: "cliente.status", Limite: 10})
	if len(rows) != 2 || mapa(rows[1].Antes)["status"] != "ativo" || mapa(rows[1].Depois)["status"] != "suspenso" {
		t.Errorf("auditoria da mudança de situação: %d eventos, %+v", len(rows), rows)
	}
}

func TestConsoleSoParaMaster(t *testing.T) {
	m := novoMundo(t)
	hash, _ := auth.Hash(senhaAna)
	if _, err := m.pubQ.CreatePlataformaUsuario(t.Context(), sqlc.CreatePlataformaUsuarioParams{Nome: "Suporte", Email: "suporte@plataforma.com", SenhaHash: hash}); err != nil {
		t.Fatal(err)
	}
	c := m.em(t, hostConsole)
	quer(t, c.entrar("suporte@plataforma.com", senhaAna, ""), http.StatusSeeOther)
	for _, path := range []string{"/clientes", "/clientes/novo", "/plataforma/usuarios", "/auditoria"} {
		quer(t, c.get(path), http.StatusForbidden, "Sem permissão")
	}
	quer(t, c.post("/clientes", url.Values{"nome": {"X"}}), http.StatusForbidden)
	corpo := c.get("/").Body.String()
	if strings.Contains(corpo, `href="/clientes"`) || strings.Contains(corpo, `href="/plataforma/usuarios"`) {
		t.Error("quem não é master não deveria ver os itens de gestão da plataforma")
	}
}

func TestConsoleUsuariosDaPlataforma(t *testing.T) {
	m := novoMundo(t)
	console := m.consoleMaster(t)
	quer(t, console.get("/plataforma/usuarios"), http.StatusOK, "Mestre", "master", "você")

	rec := console.post("/plataforma/usuarios", url.Values{"nome": {"Outro Master"}, "email": {"outro@plataforma.com"}})
	quer(t, rec, http.StatusOK, "Usuário criado", "não será mostrada de novo")
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a senha nunca pode ir para cache")
	}
	senha := senhaDaPagina(t, rec.Body.String())
	quer(t, console.post("/plataforma/usuarios", url.Values{"nome": {"Outro"}, "email": {"OUTRO@plataforma.com"}}), http.StatusBadRequest, "Já existe")
	quer(t, console.post("/plataforma/usuarios", url.Values{"nome": {""}, "email": {"x"}}), http.StatusBadRequest, "nome completo")

	// O novo master entra (com troca obrigatória) em cada ambiente, e o reset derruba todas as sessões dele.
	outroConsole, outroCliente := m.em(t, hostConsole), m.em(t, hostSinop)
	if r := outroConsole.entrar("outro@plataforma.com", senha, ""); local(r) != "/trocar-senha" {
		t.Fatalf("primeiro acesso do novo master: %d %q", r.Code, local(r))
	}
	if r := outroCliente.entrar("outro@plataforma.com", senha, ""); local(r) != "/trocar-senha" {
		t.Fatalf("primeiro acesso no cliente: %d %q", r.Code, local(r))
	}
	idOutro := idDoUsuarioDaPlataforma(t, m, "outro@plataforma.com")
	rec = console.post("/plataforma/usuarios/"+idOutro+"/resetar-senha", url.Values{})
	quer(t, rec, http.StatusOK, "Senha resetada")
	if got := local(outroConsole.get("/trocar-senha")); !strings.HasPrefix(got, "/entrar") {
		t.Errorf("a sessão do console deveria ter caído: %q", got)
	}
	if got := local(outroCliente.get("/trocar-senha")); !strings.HasPrefix(got, "/entrar") {
		t.Errorf("a sessão aberta num cliente também deveria ter caído: %q", got)
	}
	quer(t, console.post("/plataforma/usuarios/"+idDoUsuarioDaPlataforma(t, m, "mestre@plataforma.com")+"/resetar-senha", url.Values{}), http.StatusBadRequest, "própria senha")

	// Tudo registrado, com o master como ator.
	for _, acao := range []string{"master.criar", "master.resetar_senha"} {
		rows, _ := m.pubQ.ListAuditoriaPlataforma(t.Context(), sqlc.ListAuditoriaPlataformaParams{Acao: acao, Limite: 10})
		var feito bool
		for _, r := range rows {
			if r.EntidadeID == "outro@plataforma.com" && r.AtorEmail == "mestre@plataforma.com" && r.AtorTipo == "plataforma" {
				feito = true
			}
			if strings.Contains(string(r.Depois)+string(r.Detalhes), senha) {
				t.Errorf("%s: a senha não pode estar na auditoria", acao)
			}
		}
		if !feito {
			t.Errorf("faltou %s com o master como ator", acao)
		}
	}
}

func TestPainelInicialDoClienteMostraModulosPermitidos(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	corpo := ana.get("/").Body.String()
	for _, quer := range []string{"Módulos do sistema", "em construção", "Atalhos", "Últimas atividades", "Entrou no sistema"} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("o painel da Ana deveria conter %q", quer)
		}
	}
	// A Carla só tem uma permissão de visitas: vê o módulo Visitas e nenhum atalho de administração.
	carla := m.comCarla(t).get("/").Body.String()
	if !strings.Contains(carla, ">Visitas<") || strings.Contains(carla, ">Estoque<") || strings.Contains(carla, ">Usuários<") {
		t.Error("o painel da Carla deveria mostrar só o que o papel dela permite")
	}
	if strings.Contains(carla, "Últimas atividades") {
		t.Error("quem não tem a permissão de auditoria não vê as últimas atividades")
	}
}

// consoleMaster devolve um navegador no console, logado como master (já com a senha escolhida).
func (m *mundo) consoleMaster(t *testing.T) *navegador {
	t.Helper()
	n := m.em(t, hostConsole)
	n.entrar("mestre@plataforma.com", m.masterPw, "")
	n.post("/trocar-senha", url.Values{"atual": {m.masterPw}, "nova": {"Mestre-Escolha-99"}, "confirmar": {"Mestre-Escolha-99"}})
	m.masterPw = "Mestre-Escolha-99"
	return n
}

func idDoUsuarioDaPlataforma(t *testing.T, m *mundo, email string) string {
	t.Helper()
	u, err := m.pubQ.GetPlataformaUsuarioByEmail(t.Context(), email)
	if err != nil {
		t.Fatal(err)
	}
	return idTexto(u.ID)
}

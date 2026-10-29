package server

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"uysiarbo/internal/auth"
	"uysiarbo/internal/db/sqlccliente"
)

const (
	emailCarla = "carla@nm.gov.br"
	senhaCarla = "Senha-Da-Carla-1"
)

// comAna devolve um navegador já logado como Ana (papel Administrador, que tem conf_usuario e conf_papel).
func (m *mundo) comAna(t *testing.T) *navegador {
	t.Helper()
	n := m.em(t, hostNM)
	if rec := n.entrar(emailAna, senhaAna, ""); local(rec) != "/" {
		t.Fatalf("login da Ana: %d %q", rec.Code, local(rec))
	}
	return n
}

// comCarla cria um papel só de leitura de visitas e a usuária Carla, e a deixa logada.
func (m *mundo) comCarla(t *testing.T) *navegador {
	t.Helper()
	ctx := t.Context()
	papel, err := m.cliQ.CreatePapel(ctx, sqlccliente.CreatePapelParams{Nome: "Leitura"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.cliQ.AddPermissoesAoPapel(ctx, sqlccliente.AddPermissoesAoPapelParams{PapelID: papel.ID, Permissoes: []string{"visita_visualizar"}}); err != nil {
		t.Fatal(err)
	}
	hash, _ := auth.Hash(senhaCarla)
	if _, err := m.cliQ.CreateUsuario(ctx, sqlccliente.CreateUsuarioParams{Nome: "Carla Dias", Email: emailCarla, SenhaHash: hash, PapelID: papel.ID, LoginWeb: true}); err != nil {
		t.Fatal(err)
	}
	n := m.em(t, hostNM)
	if rec := n.entrar(emailCarla, senhaCarla, ""); local(rec) != "/" {
		t.Fatalf("login da Carla: %d %q", rec.Code, local(rec))
	}
	return n
}

// comMaster entra como master em Nova Mutum, já com a senha temporária trocada.
func (m *mundo) comMaster(t *testing.T) *navegador {
	t.Helper()
	n := m.em(t, hostNM)
	n.entrar("mestre@plataforma.com", m.masterPw, "")
	n.post("/trocar-senha", url.Values{"atual": {m.masterPw}, "nova": {"Mestre-Escolha-99"}, "confirmar": {"Mestre-Escolha-99"}})
	return n
}

var reSenha = regexp.MustCompile(`senha-gerada[^>]*>([^<]+)<`)

func senhaDaPagina(t *testing.T, corpo string) string {
	t.Helper()
	m := reSenha.FindStringSubmatch(corpo)
	if m == nil {
		t.Fatalf("a página não mostra a senha gerada: %.300s", corpo)
	}
	// A página escapa o HTML (o "&" da senha vira "&amp;"); o usuário vê o caractere original.
	return html.UnescapeString(strings.TrimSpace(m[1]))
}

func (m *mundo) idUsuario(t *testing.T, email string) string {
	t.Helper()
	u, err := m.cliQ.GetUsuarioByEmail(t.Context(), email)
	if err != nil {
		t.Fatal(err)
	}
	return strconv.FormatInt(u.ID, 10)
}

func (m *mundo) idPapel(t *testing.T, nome string) int64 {
	t.Helper()
	p, err := m.cliQ.GetPapelByNome(t.Context(), nome)
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func TestListasDePapeisEUsuarios(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	quer(t, ana.get("/papeis"), http.StatusOK, "Administrador", "159", "Novo papel")
	quer(t, ana.get("/usuarios"), http.StatusOK, "Ana Souza", "você", "Bruno Lima", "Troca de senha pendente", "Novo usuário")
	// O menu mostra as duas opções.
	corpo := ana.get("/").Body.String()
	if !strings.Contains(corpo, `href="/usuarios"`) || !strings.Contains(corpo, `href="/papeis"`) {
		t.Error("o menu deveria ter Usuários e Papéis para quem tem as permissões")
	}
}

func TestSemPermissaoNaoAbreNemMostraNoMenu(t *testing.T) {
	m := novoMundo(t)
	carla := m.comCarla(t)

	for _, path := range []string{"/usuarios", "/usuarios/novo", "/papeis", "/papeis/novo"} {
		quer(t, carla.get(path), http.StatusForbidden, "Sem permissão")
	}
	quer(t, carla.post("/usuarios", url.Values{"nome": {"X"}}), http.StatusForbidden)
	quer(t, carla.post("/papeis", url.Values{"nome": {"X"}}), http.StatusForbidden)
	corpo := carla.get("/").Body.String()
	if strings.Contains(corpo, `href="/usuarios"`) || strings.Contains(corpo, `href="/papeis"`) {
		t.Error("o menu não deveria mostrar Usuários e Papéis para quem não tem permissão")
	}

	// No console da plataforma essas telas não existem, nem para o master.
	console := m.em(t, hostConsole)
	console.entrar("mestre@plataforma.com", m.masterPw, "")
	console.post("/trocar-senha", url.Values{"atual": {m.masterPw}, "nova": {"Mestre-Escolha-99"}, "confirmar": {"Mestre-Escolha-99"}})
	quer(t, console.get("/usuarios"), http.StatusNotFound)
	quer(t, console.get("/papeis"), http.StatusNotFound)
}

func TestEditorDePermissoesMostraBlocosEBusca(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	rec := ana.get("/papeis/novo")
	quer(t, rec, http.StatusOK, `value="visita_adicionar"`, "restringe", "Marcar todas", "Buscar permissão", "Marcar bloco", `x-data="editorPermissoes"`)
	if n := strings.Count(rec.Body.String(), `<details class="bloco"`); n != 23 {
		t.Errorf("blocos = %d, quer 23", n)
	}
	if n := strings.Count(rec.Body.String(), `name="perm"`); n != 170 {
		t.Errorf("caixas de permissão = %d, quer 170", n)
	}
	// Ana não tem as restritivas, então não pode concedê-las: aparecem desabilitadas.
	if !regexp.MustCompile(`value="visita_visualizar_minha"[^>]*disabled`).MatchString(rec.Body.String()) {
		t.Error("a permissão restritiva deveria aparecer desabilitada para quem não a tem")
	}
}

func TestCriarPapelValidaEGrava(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	form := func(nome string, perms ...string) url.Values {
		return url.Values{"nome": {nome}, "descricao": {"Equipe de campo"}, "ativo": {"1"}, "perm": perms}
	}
	// Ana não tem a permissão restritiva: ela é ignorada, o resto é gravado.
	rec := ana.post("/papeis", form("Agente de campo", "visita_adicionar", "visita_visualizar", "local_visualizar", "visita_visualizar_minha"))
	if rec.Code != http.StatusSeeOther || local(rec) != "/papeis" {
		t.Fatalf("criar: status=%d Location=%q corpo=%.300s", rec.Code, local(rec), rec.Body.String())
	}
	perms, _ := m.cliQ.ListPermissoesDoPapel(t.Context(), m.idPapel(t, "Agente de campo"))
	if strings.Join(perms, ",") != "local_visualizar,visita_adicionar,visita_visualizar" {
		t.Errorf("permissões gravadas = %v", perms)
	}

	quer(t, ana.post("/papeis", form("agente DE campo", "visita_adicionar")), http.StatusBadRequest, "Já existe um papel com esse nome")
	quer(t, ana.post("/papeis", form("   ")), http.StatusBadRequest, "Informe o nome do papel")
	quer(t, ana.post("/papeis", form("Outro", "permissao_inventada")), http.StatusBadRequest, "Permissão desconhecida")
	// Depois de um erro, o formulário volta preenchido.
	quer(t, ana.post("/papeis", form("Outro", "permissao_inventada")), http.StatusBadRequest, `value="Outro"`)
}

func TestSoOMasterConcedeRestritivasEQuemEditaNaoMexeNoQueNaoTem(t *testing.T) {
	m := novoMundo(t)
	master := m.comMaster(t)

	rec := master.post("/papeis", url.Values{"nome": {"Supervisor"}, "ativo": {"1"}, "perm": {"visita_visualizar", "visita_visualizar_minha"}})
	quer(t, rec, http.StatusSeeOther)
	id := m.idPapel(t, "Supervisor")
	perms, _ := m.cliQ.ListPermissoesDoPapel(t.Context(), id)
	if strings.Join(perms, ",") != "visita_visualizar,visita_visualizar_minha" {
		t.Fatalf("o master deveria poder conceder a restritiva: %v", perms)
	}

	// Ana abre o papel: a restritiva aparece marcada e desabilitada.
	ana := m.comAna(t)
	editor := ana.get("/papeis/" + strconv.FormatInt(id, 10)).Body.String()
	if !regexp.MustCompile(`value="visita_visualizar_minha"[^>]*checked[^>]*disabled`).MatchString(editor) {
		t.Error("a restritiva deveria aparecer marcada e desabilitada para a Ana")
	}
	// Ana salva (o navegador não envia caixas desabilitadas): a restritiva fica como estava.
	quer(t, ana.post("/papeis/"+strconv.FormatInt(id, 10), url.Values{"nome": {"Supervisor"}, "ativo": {"1"}, "perm": {"visita_visualizar", "local_visualizar"}}), http.StatusSeeOther)
	perms, _ = m.cliQ.ListPermissoesDoPapel(t.Context(), id)
	if strings.Join(perms, ",") != "local_visualizar,visita_visualizar,visita_visualizar_minha" {
		t.Errorf("a Ana não podia retirar nem conceder a restritiva: %v", perms)
	}
	// Ana retira uma permissão que ela tem.
	quer(t, ana.post("/papeis/"+strconv.FormatInt(id, 10), url.Values{"nome": {"Supervisor"}, "ativo": {"1"}, "perm": {"visita_visualizar"}}), http.StatusSeeOther)
	perms, _ = m.cliQ.ListPermissoesDoPapel(t.Context(), id)
	if strings.Join(perms, ",") != "visita_visualizar,visita_visualizar_minha" {
		t.Errorf("depois de retirar local_visualizar: %v", perms)
	}
	quer(t, ana.get("/papeis/99999"), http.StatusNotFound)
	quer(t, ana.get("/papeis/abc"), http.StatusNotFound)
}

func TestNaoDesativaPapelComUsuarios(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	id := strconv.FormatInt(m.idPapel(t, "Administrador"), 10)
	quer(t, ana.post("/papeis/"+id, url.Values{"nome": {"Administrador"}, "perm": {"conf_usuario", "conf_papel"}}),
		http.StatusBadRequest, "tem 2 usuário(s)")
}

func TestCriarUsuarioGeraSenhaTemporariaUmaVez(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	papel := strconv.FormatInt(m.idPapel(t, "Administrador"), 10)
	form := func(email, papel string) url.Values {
		return url.Values{"nome": {"Diego Alves"}, "email": {email}, "telefone": {"65 99999-0000"}, "papel_id": {papel}, "login_web": {"1"}}
	}

	rec := ana.post("/usuarios", form("Diego@NM.gov.br", papel))
	quer(t, rec, http.StatusOK, "Usuário criado", "Diego Alves", "não será mostrada de novo", "Copiar senha")
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, quer no-store", rec.Header().Get("Cache-Control"))
	}
	senha := senhaDaPagina(t, rec.Body.String())
	if len(senha) != 16 {
		t.Fatalf("senha com %d caracteres", len(senha))
	}
	// O e-mail é guardado em minúsculas; o primeiro acesso exige a troca.
	novo := m.em(t, hostNM)
	if r := novo.entrar("diego@nm.gov.br", senha, ""); local(r) != "/trocar-senha" {
		t.Errorf("primeiro acesso: %d %q", r.Code, local(r))
	}
	// A senha não aparece de novo na lista.
	if strings.Contains(ana.get("/usuarios").Body.String(), senha) {
		t.Error("a senha temporária não pode aparecer na lista")
	}

	quer(t, ana.post("/usuarios", form("diego@nm.gov.br", papel)), http.StatusBadRequest, "Já existe um usuário com esse e-mail")
	quer(t, ana.post("/usuarios", form("mestre@plataforma.com", papel)), http.StatusBadRequest, "pertence a um usuário da plataforma")
	quer(t, ana.post("/usuarios", form("sem-arroba", papel)), http.StatusBadRequest, "e-mail válido")
	quer(t, ana.post("/usuarios", form("x@nm.gov.br", "999999")), http.StatusBadRequest, "papel ativo")
	quer(t, ana.post("/usuarios", url.Values{"nome": {"Sem Acesso"}, "email": {"y@nm.gov.br"}, "papel_id": {papel}}), http.StatusBadRequest, "forma de acesso")
}

func TestResetarSenhaDeOutroUsuario(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	carla := m.comCarla(t)
	idCarla := m.idUsuario(t, emailCarla)

	rec := ana.post("/usuarios/"+idCarla+"/resetar-senha", url.Values{})
	quer(t, rec, http.StatusOK, "Senha resetada", "Carla Dias", "não será mostrada de novo")
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a senha resetada nunca pode ir para cache")
	}
	nova := senhaDaPagina(t, rec.Body.String())

	// A sessão aberta da Carla caiu, a senha antiga morreu, a nova exige troca.
	if got := local(carla.get("/")); !strings.HasPrefix(got, "/entrar") {
		t.Errorf("a sessão da Carla deveria ter caído: %q", got)
	}
	novo := m.em(t, hostNM)
	quer(t, novo.entrar(emailCarla, senhaCarla, ""), http.StatusUnauthorized)
	if r := novo.entrar(emailCarla, nova, ""); local(r) != "/trocar-senha" {
		t.Errorf("com a senha resetada: %d %q", r.Code, local(r))
	}

	// Ninguém reseta a própria senha por aqui, nem usuário inexistente.
	quer(t, ana.post("/usuarios/"+m.idUsuario(t, emailAna)+"/resetar-senha", url.Values{}), http.StatusBadRequest, "trocar a sua própria senha")
	quer(t, ana.post("/usuarios/999999/resetar-senha", url.Values{}), http.StatusNotFound)
}

func TestEditarUsuarioDesativaEEncerraSessoes(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	carla := m.comCarla(t)
	idCarla := m.idUsuario(t, emailCarla)
	papelAdm := strconv.FormatInt(m.idPapel(t, "Administrador"), 10)
	papelLeitura := strconv.FormatInt(m.idPapel(t, "Leitura"), 10)

	quer(t, ana.get("/usuarios/"+idCarla), http.StatusOK, "Carla Dias", "Resetar senha de Carla Dias", "Leitura")

	// Trocar o papel encerra a sessão aberta (a pessoa volta a entrar já com o acesso novo).
	rec := ana.post("/usuarios/"+idCarla, url.Values{"nome": {"Carla Dias"}, "papel_id": {papelAdm}, "login_web": {"1"}, "ativo": {"1"}})
	quer(t, rec, http.StatusSeeOther)
	if got := local(carla.get("/")); !strings.HasPrefix(got, "/entrar") {
		t.Errorf("a sessão deveria cair ao trocar o papel: %q", got)
	}
	novo := m.em(t, hostNM)
	novo.entrar(emailCarla, senhaCarla, "")
	quer(t, novo.get("/usuarios"), http.StatusOK) // agora é Administradora

	// Desativar: não entra mais.
	quer(t, ana.post("/usuarios/"+idCarla, url.Values{"nome": {"Carla Dias"}, "papel_id": {papelLeitura}, "login_web": {"1"}}), http.StatusSeeOther)
	quer(t, m.em(t, hostNM).entrar(emailCarla, senhaCarla, ""), http.StatusUnauthorized)

	// Ana não se desativa nem troca o próprio papel.
	idAna := m.idUsuario(t, emailAna)
	quer(t, ana.post("/usuarios/"+idAna, url.Values{"nome": {"Ana Souza"}, "papel_id": {papelAdm}, "login_web": {"1"}}), http.StatusBadRequest, "desativar a si mesmo")
	quer(t, ana.post("/usuarios/"+idAna, url.Values{"nome": {"Ana Souza"}, "papel_id": {papelLeitura}, "login_web": {"1"}, "ativo": {"1"}}), http.StatusBadRequest, "trocar o seu próprio papel")
	// Mas pode corrigir o próprio nome.
	quer(t, ana.post("/usuarios/"+idAna, url.Values{"nome": {"Ana Souza Lima"}, "papel_id": {papelAdm}, "login_web": {"1"}, "ativo": {"1"}}), http.StatusSeeOther)
	quer(t, ana.get("/usuarios"), http.StatusOK, "Ana Souza Lima")
}

func TestMasterAdministraQualquerCliente(t *testing.T) {
	m := novoMundo(t)
	master := m.comMaster(t)
	quer(t, master.get("/usuarios"), http.StatusOK, "Ana Souza")
	quer(t, master.get("/papeis"), http.StatusOK, "Administrador")
	// O master não está na lista de usuários do cliente (ele é da plataforma).
	if strings.Contains(master.get("/usuarios").Body.String(), "mestre@plataforma.com") {
		t.Error("o master não pertence à lista de usuários do cliente")
	}
	rec := master.post("/usuarios/"+m.idUsuario(t, emailBruno)+"/resetar-senha", url.Values{})
	quer(t, rec, http.StatusOK, "Senha resetada")
}

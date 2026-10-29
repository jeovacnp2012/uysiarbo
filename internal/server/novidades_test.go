package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"uysiarbo/internal/auth"
	"uysiarbo/internal/db/sqlc"
)

func TestGuiaPorPermissaoEBusca(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)     // Administrador: usuários, papéis e auditoria
	carla := m.comCarla(t) // só leitura de visitas
	master := m.comMaster(t)

	// Ana vê os artigos de administração, mas não os do console.
	rec := ana.get("/guia")
	quer(t, rec, http.StatusOK, "Guia do sistema", "Primeiros passos", "Senhas", "Usuários", "Papéis e permissões", "Auditoria", "Novidades e versões", "Glossário")
	if strings.Contains(rec.Body.String(), "/guia/clientes") {
		t.Error("o artigo do console é só do master")
	}
	// Carla só vê o que não exige permissão.
	corpo := carla.get("/guia").Body.String()
	if !strings.Contains(corpo, "/guia/primeiros-passos") || strings.Contains(corpo, "/guia/usuarios") || strings.Contains(corpo, "/guia/papeis") || strings.Contains(corpo, "/guia/auditoria") {
		t.Error("a Carla só deveria ver os artigos livres")
	}
	// O master vê todos, inclusive os do console.
	quer(t, master.get("/guia"), http.StatusOK, "Clientes (console)", "Usuários da plataforma (console)")

	// Busca sem diferenciar acento nem caixa, com trecho.
	quer(t, ana.get("/guia?q=SENHA+temporaria"), http.StatusOK, "resultado(s)", "Senhas")
	quer(t, ana.get("/guia?q=resetar"), http.StatusOK, "Usuários")
	// A busca não revela o que a pessoa não pode ler.
	if strings.Contains(carla.get("/guia?q=resetar").Body.String(), "/guia/usuarios") {
		t.Error("a busca da Carla não pode devolver o artigo de usuários")
	}
	quer(t, ana.get("/guia?q=zzzinexistente"), http.StatusOK, "Nada encontrado")

	// Artigos: o texto convertido, e o mesmo 404 para "não existe" e "sem permissão".
	quer(t, ana.get("/guia/primeiros-passos"), http.StatusOK, "Entrar", "Outros artigos", "<strong>")
	quer(t, ana.get("/guia/usuarios"), http.StatusOK, "Resetar senha")
	quer(t, carla.get("/guia/usuarios"), http.StatusNotFound)
	quer(t, ana.get("/guia/clientes"), http.StatusNotFound)
	quer(t, master.get("/guia/clientes"), http.StatusOK, "Subdomínio de acesso")
	quer(t, ana.get("/guia/nao-existe"), http.StatusNotFound)

	// O menu leva ao Guia e às Novidades.
	if c := ana.get("/").Body.String(); !strings.Contains(c, `href="/guia"`) || !strings.Contains(c, `href="/novidades"`) {
		t.Error("o menu deveria ter Guia e Novidades")
	}
}

func TestGuiaENovidadesExigemLogin(t *testing.T) {
	m := novoMundo(t)
	n := m.em(t, hostNM)
	for _, p := range []string{"/guia", "/guia/senhas", "/novidades"} {
		if got := local(n.get(p)); !strings.HasPrefix(got, "/entrar") {
			t.Errorf("%s sem login deveria ir para /entrar, foi para %q", p, got)
		}
	}
}

func TestNovidadesEAvisoDeVersaoNova(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	// Logo depois de entrar, há versões que a Ana nunca viu: aviso no topo e a versão no rodapé do menu.
	inicio := ana.get("/").Body.String()
	if !strings.Contains(inicio, `href="/novidades">Novidades <span class="selo selo-aviso">nova</span>`) {
		t.Error("o aviso de versão nova deveria aparecer")
	}
	if !strings.Contains(inicio, "Versão 1.0.005") {
		t.Error("o rodapé do menu deveria mostrar a versão atual (1.0.005)")
	}

	// A página lista as versões, da mais nova para a mais antiga, com os itens por tipo.
	rec := ana.get("/novidades")
	quer(t, rec, http.StatusOK, "Versão 1.0.004", "Guia do sistema e Novidades", "Versão 1.0.001", "Fundação do sistema", "Novo", "Corrigido", "29/10/2026")
	corpo := rec.Body.String()
	if strings.Index(corpo, "Versão 1.0.004") > strings.Index(corpo, "Versão 1.0.001") {
		t.Error("a versão mais nova deveria vir primeiro")
	}
	if n := strings.Count(corpo, `<span class="selo selo-aviso">nova</span>`); n != 5 {
		t.Errorf("na primeira visita as 5 versões são novas, vi %d selos", n)
	}

	// Depois de abrir a página, o aviso some (e as versões deixam de ser "novas").
	if strings.Contains(ana.get("/").Body.String(), `selo selo-aviso">nova`) {
		t.Error("o aviso deveria sumir depois de abrir as Novidades")
	}
	if strings.Contains(ana.get("/novidades").Body.String(), `selo selo-aviso">nova`) {
		t.Error("nenhuma versão deveria estar marcada como nova na segunda visita")
	}
	// E continua assim ao entrar de novo (a marca fica guardada na conta).
	outra := m.em(t, hostNM)
	outra.entrar(emailAna, senhaAna, "")
	if strings.Contains(outra.get("/").Body.String(), `selo selo-aviso">nova`) {
		t.Error("a marca de 'já vi' deveria valer em outra sessão")
	}
}

func TestMasterRegistraVersaoEOAvisoVoltaParaTodos(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	ana.get("/novidades") // Ana viu tudo
	if strings.Contains(ana.get("/").Body.String(), `selo selo-aviso">nova`) {
		t.Fatal("sem versão nova, sem aviso")
	}

	console := m.consoleMaster(t)
	quer(t, console.get("/novidades"), http.StatusOK, "Registrar versão")
	quer(t, console.get("/novidades/nova"), http.StatusOK, `value="1.0.006"`, "Publicar versão")

	form := func(versao, titulo, data, novo, melhorado, corrigido string) url.Values {
		return url.Values{"versao": {versao}, "titulo": {titulo}, "data": {data}, "novo": {novo}, "melhorado": {melhorado}, "corrigido": {corrigido}}
	}
	quer(t, console.post("/novidades", form("1.5", "X", "2026-11-01", "a", "", "")), http.StatusBadRequest, "Número de versão inválido")
	quer(t, console.post("/novidades", form("1.0.006", "", "2026-11-01", "a", "", "")), http.StatusBadRequest, "título")
	quer(t, console.post("/novidades", form("1.0.006", "X", "ontem", "a", "", "")), http.StatusBadRequest, "data válida")
	quer(t, console.post("/novidades", form("1.0.006", "X", "2026-11-01", "  \n ", "", "")), http.StatusBadRequest, "pelo menos uma novidade")
	quer(t, console.post("/novidades", form("1.0.004", "Repetida", "2026-11-01", "a", "", "")), http.StatusBadRequest, "Já existe uma versão")
	quer(t, console.post("/novidades", form("1.0.006", "X", "2026-11-01", strings.Repeat("a", 501), "", "")), http.StatusBadRequest, "500 caracteres")
	// Depois de um erro o formulário volta preenchido.
	quer(t, console.post("/novidades", form("1.0.006", "Meu título", "2026-11-01", "", "", "")), http.StatusBadRequest, `value="Meu título"`)

	ok := form("1.0.006", "Cadastro de bairros", "2026-11-05", "Cadastro de bairros e quarteirões.\nMapa dos bairros.", "Telas mais rápidas.", "Corrigido o filtro da lista.")
	quer(t, console.post("/novidades", ok), http.StatusSeeOther)

	// A Ana, num cliente, vê o aviso de volta, a versão nova no rodapé e o conteúdo.
	inicio := ana.get("/").Body.String()
	if !strings.Contains(inicio, `selo selo-aviso">nova`) || !strings.Contains(inicio, "Versão 1.0.006") {
		t.Error("a versão registrada deveria acender o aviso para quem já tinha visto tudo")
	}
	quer(t, ana.get("/novidades"), http.StatusOK, "Versão 1.0.006", "Cadastro de bairros", "Mapa dos bairros.", "Telas mais rápidas.", "Corrigido o filtro da lista.", "05/11/2026")

	// Fica na auditoria da plataforma, com o master como autor.
	rows, _ := m.pubQ.ListAuditoriaPlataforma(t.Context(), sqlc.ListAuditoriaPlataformaParams{Acao: "versao.criar", Limite: 5})
	if len(rows) != 1 || rows[0].EntidadeID != "1.0.006" || rows[0].AtorEmail != "mestre@plataforma.com" {
		t.Errorf("auditoria da versão: %+v", rows)
	}
	quer(t, console.get("/auditoria?acao=versao.criar"), http.StatusOK, "Registrou uma versão nas Novidades")
}

func TestSoOMasterNoConsoleRegistraVersao(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	// Num cliente nem o botão nem a tela existem (nem para o master).
	quer(t, ana.get("/novidades/nova"), http.StatusNotFound)
	quer(t, ana.post("/novidades", url.Values{"versao": {"1.0.009"}}), http.StatusNotFound)
	if strings.Contains(ana.get("/novidades").Body.String(), "Registrar versão") {
		t.Error("o botão de registrar versão é só do master, no console")
	}
	if strings.Contains(m.comMaster(t).get("/novidades").Body.String(), "Registrar versão") {
		t.Error("o registro de versões fica no console, não dentro de um cliente")
	}
	// Uma conta da plataforma que não é master também não registra.
	m.criarSuporte(t)
	c := m.em(t, hostConsole)
	c.entrar("suporte@plataforma.com", senhaAna, "")
	quer(t, c.get("/novidades"), http.StatusOK)
	quer(t, c.get("/novidades/nova"), http.StatusForbidden)
}

func TestProximaVersao(t *testing.T) {
	for in, esperado := range map[string]string{"1.0.004": "1.0.005", "1.0.009": "1.0.010", "2.3.099": "2.3.100", "": "1.0.001", "x": "1.0.001", "1.0.x": "1.0.001"} {
		if got := proximaVersao(in); got != esperado {
			t.Errorf("proximaVersao(%q) = %q, quer %q", in, got, esperado)
		}
	}
}

// criarSuporte cria uma conta da plataforma que não é master, com a senha da Ana.
func (m *mundo) criarSuporte(t *testing.T) {
	t.Helper()
	hash, _ := auth.Hash(senhaAna)
	if _, err := m.pubQ.CreatePlataformaUsuario(t.Context(), sqlc.CreatePlataformaUsuarioParams{Nome: "Suporte", Email: "suporte@plataforma.com", SenhaHash: hash}); err != nil {
		t.Fatal(err)
	}
}

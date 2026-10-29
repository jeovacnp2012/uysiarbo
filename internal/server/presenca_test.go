package server

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
	"uysiarbo/internal/auth"
	"uysiarbo/internal/db/sqlccliente"

	"uysiarbo/internal/presenca"
)

const (
	abaA = "aba00000A1"
	abaB = "aba00000B2"
	abaC = "aba00000C3"
)

func pedidoPresenca(recurso, id, modo, aba string) string {
	return "/presenca?" + url.Values{"recurso": {recurso}, "id": {id}, "modo": {modo}, "aba": {aba}}.Encode()
}

func TestFichasTrazemAFaixaDePresenca(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	bid := m.criarBairro(t, cid, "Centro")

	for recurso, caminho := range map[string]string{
		"bairro":  "/bairros/" + s64(bid),
		"cidade":  "/cidades/" + s64(cid),
		"usuario": "/usuarios/" + m.idUsuario(t, emailBruno),
		"papel":   "/papeis/" + s64(m.idPapel(t, "Administrador")),
	} {
		corpo := ana.get(caminho).Body.String()
		if !strings.Contains(corpo, `class="presenca"`) || !strings.Contains(corpo, `data-recurso="`+recurso+`"`) || !strings.Contains(corpo, `hx-trigger="load, every 15s"`) {
			t.Errorf("a ficha de %s deveria ter a faixa de presença", recurso)
		}
	}
	// Cadastro novo não tem registro ainda: sem faixa.
	for _, caminho := range []string{"/bairros/novo", "/cidades/nova", "/usuarios/novo", "/papeis/novo"} {
		if strings.Contains(ana.get(caminho).Body.String(), `data-recurso=`) {
			t.Errorf("%s não deveria ter faixa de presença", caminho)
		}
	}
	// Quem pode editar entra como "editar"; quem só consulta, como "ver".
	if !strings.Contains(ana.get("/bairros/"+s64(bid)).Body.String(), "modo=editar") {
		t.Error("quem pode editar abre a ficha no modo editar")
	}
}

func TestAvisaQuemMaisEstaNoRegistro(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	bid := s64(m.criarBairro(t, cid, "Centro"))
	ana, master := m.comAna(t), m.comMaster(t)

	// Sozinha: faixa vazia (mas o batimento continua).
	rec := ana.get(pedidoPresenca("bairro", bid, "editar", abaA))
	quer(t, rec, http.StatusOK, `hx-trigger="every 15s"`)
	if strings.Contains(rec.Body.String(), "Este registro está sendo") || strings.Contains(rec.Body.String(), "presenca-visivel") {
		t.Errorf("sozinha, a faixa deve ficar vazia: %.300s", rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a faixa nunca vai para cache")
	}

	// O master chega só para ver: vê a Ana editando; a Ana vê o master visualizando.
	rec = master.get(pedidoPresenca("bairro", bid, "ver", abaB))
	corpo := rec.Body.String()
	if !regexp.MustCompile(`Este registro está sendo editado por Ana Souza \(desde \d\d:\d\d\)\.`).MatchString(corpo) || !strings.Contains(corpo, "presenca-alerta") {
		t.Errorf("o master deveria ver a Ana editando, em destaque: %.400s", corpo)
	}
	if strings.Contains(corpo, "sobrescrever") {
		t.Error("o aviso de sobrescrever só vale para quem também está editando")
	}
	corpo = ana.get(pedidoPresenca("bairro", bid, "editar", abaA)).Body.String()
	if !regexp.MustCompile(`visualizado por Mestre \(desde \d\d:\d\d\)\.`).MatchString(corpo) {
		t.Errorf("a Ana deveria ver o master visualizando: %.400s", corpo)
	}
	if strings.Contains(corpo, "presenca-alerta") {
		t.Error("só visualizando: sem destaque")
	}

	// O master passa a editar: os dois editando, com o alerta de sobrescrita; e a mesma pessoa em outra janela.
	master.get(pedidoPresenca("bairro", bid, "editar", abaB))
	corpo = ana.get(pedidoPresenca("bairro", bid, "editar", abaA)).Body.String()
	if !strings.Contains(corpo, "editado por Mestre") || !strings.Contains(corpo, "sobrescrever") || !strings.Contains(corpo, "presenca-alerta") {
		t.Errorf("os dois editando: %.500s", corpo)
	}
	ana.get(pedidoPresenca("bairro", bid, "ver", abaC)) // a Ana abre uma segunda janela
	corpo = ana.get(pedidoPresenca("bairro", bid, "editar", abaA)).Body.String()
	if !strings.Contains(corpo, "você, em outra janela") {
		t.Errorf("a outra janela da mesma pessoa deveria aparecer: %.500s", corpo)
	}
	// Várias pessoas no mesmo papel de ação: lista "A e B".
	corpo = master.get(pedidoPresenca("bairro", bid, "editar", abaB)).Body.String()
	if !regexp.MustCompile(`editado por Ana Souza \(desde \d\d:\d\d\)`).MatchString(corpo) || !strings.Contains(corpo, "visualizado por Ana Souza") {
		t.Errorf("o master vê a Ana editando e a outra janela da Ana só vendo: %.500s", corpo)
	}

	// Sair tira a janela do aviso.
	quer(t, ana.post("/presenca/sair", url.Values{"recurso": {"bairro"}, "id": {bid}, "aba": {abaA}}), http.StatusNoContent)
	quer(t, ana.post("/presenca/sair", url.Values{"recurso": {"bairro"}, "id": {bid}, "aba": {abaC}}), http.StatusNoContent)
	corpo = master.get(pedidoPresenca("bairro", bid, "editar", abaB)).Body.String()
	if strings.Contains(corpo, "Este registro está sendo") {
		t.Errorf("depois de a Ana sair, ninguém mais: %.300s", corpo)
	}
	// O pedido de sair não gera auditoria (não é ação sobre os dados).
	if m.contarGenericos(t) != 0 {
		t.Errorf("sair da presença não deve cair no registro genérico (%d)", m.contarGenericos(t))
	}
}

func TestPresencaNaoMisturaRegistrosNemClientes(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b1, b2 := s64(m.criarBairro(t, cid, "Centro")), s64(m.criarBairro(t, cid, "Jardim"))
	ana, master := m.comAna(t), m.comMaster(t)

	ana.get(pedidoPresenca("bairro", b1, "editar", abaA))
	if strings.Contains(master.get(pedidoPresenca("bairro", b2, "ver", abaB)).Body.String(), "Este registro está sendo") {
		t.Error("outro bairro não deve receber o aviso")
	}
	if strings.Contains(master.get(pedidoPresenca("cidade", b1, "ver", abaB)).Body.String(), "Este registro está sendo") {
		t.Error("outro tipo de registro (cidade nº igual) não deve receber o aviso")
	}

	// O mesmo id em outro cliente é outro registro.
	outro := m.em(t, hostSinop)
	outro.entrar("mestre@plataforma.com", "Mestre-Escolha-99", "")
	if strings.Contains(outro.get(pedidoPresenca("bairro", b1, "ver", abaC)).Body.String(), "Este registro está sendo") {
		t.Error("os clientes são isolados: o aviso não pode vazar para o outro cliente")
	}
}

func TestPresencaRespeitaPermissoesEPedidosInvalidos(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	bid := s64(m.criarBairro(t, cid, "Centro"))
	ana, carla := m.comAna(t), m.comCarla(t) // a Carla não tem permissão nenhuma do território

	ana.get(pedidoPresenca("bairro", bid, "editar", abaA))
	// Sem permissão para ver o registro: nem o aviso, nem os nomes.
	quer(t, carla.get(pedidoPresenca("bairro", bid, "ver", abaB)), http.StatusForbidden)
	quer(t, carla.get(pedidoPresenca("usuario", "1", "ver", abaB)), http.StatusForbidden)
	quer(t, carla.post("/presenca/sair", url.Values{"recurso": {"bairro"}, "id": {bid}, "aba": {abaB}}), http.StatusForbidden)

	// Pedidos malformados.
	for nome, p := range map[string]string{
		"recurso desconhecido": pedidoPresenca("segredo", "1", "ver", abaA),
		"id inválido":          pedidoPresenca("bairro", "1; DROP", "ver", abaA),
		"id vazio":             pedidoPresenca("bairro", "", "ver", abaA),
		"aba curta":            pedidoPresenca("bairro", bid, "ver", "x"),
		"aba com símbolo":      pedidoPresenca("bairro", bid, "ver", "abc<script>1"),
	} {
		if rec := ana.get(p); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, quer 400", nome, rec.Code)
		}
	}
	// Sem login, nada.
	if got := local(m.em(t, hostNM).get(pedidoPresenca("bairro", bid, "ver", abaA))); !strings.HasPrefix(got, "/entrar") {
		t.Errorf("sem login deveria ir para /entrar: %q", got)
	}
	// Recurso de cliente no console não existe (e o contrário também).
	console := m.consoleMaster(t)
	quer(t, console.get(pedidoPresenca("bairro", bid, "ver", abaA)), http.StatusNotFound)

	// Quem só consulta não consegue se passar por editor: o servidor reduz para "ver".
	papel, _ := m.cliQ.CreatePapel(t.Context(), papelParams("Consulta"))
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(papel.ID, "bairro_visualizar"))
	edu := m.novoUsuarioLogado(t, "Edu Lima", "edu@nm.gov.br", papel.ID)
	edu.get(pedidoPresenca("bairro", bid, "editar", abaC))
	corpo := ana.get(pedidoPresenca("bairro", bid, "editar", abaA)).Body.String()
	if !strings.Contains(corpo, "visualizado por Edu Lima") || strings.Contains(corpo, "editado por Edu") {
		t.Errorf("quem não pode editar deve aparecer como visualizando: %.400s", corpo)
	}
}

func TestPresencaDeBairroRespeitaApenasAtribuidos(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	a, b := m.criarBairro(t, cid, "Bairro A"), m.criarBairro(t, cid, "Bairro B")
	papel, _ := m.cliQ.CreatePapel(t.Context(), papelParams("Supervisor"))
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(papel.ID, "bairro_visualizar", "bairro_meubairro"))
	dora := m.novoUsuarioLogado(t, "Dora Reis", "dora@nm.gov.br", papel.ID)
	u, _ := m.cliQ.GetUsuarioByEmail(t.Context(), "dora@nm.gov.br")
	_ = m.cliQ.AddResponsaveisAoBairro(t.Context(), addRespParams(a, u.ID))

	m.comAna(t).get(pedidoPresenca("bairro", s64(b), "editar", abaA))
	quer(t, dora.get(pedidoPresenca("bairro", s64(b), "ver", abaB)), http.StatusNotFound) // bairro que não é dela
	quer(t, dora.get(pedidoPresenca("bairro", s64(a), "ver", abaB)), http.StatusOK)
}

func TestFrasePresenca(t *testing.T) {
	desde := time.Date(2026, 10, 29, 17, 35, 0, 0, time.UTC) // 13:35 em Cuiabá
	eu := presenca.Pessoa{Tipo: "usuario", ID: "1", Nome: "Eu"}
	p := func(id, nome string, modo presenca.Modo) presenca.Presenca {
		return presenca.Presenca{Pessoa: presenca.Pessoa{Tipo: "usuario", ID: id, Nome: nome}, Modo: modo, Desde: desde}
	}
	casos := []struct {
		nome   string
		outros []presenca.Presenca
		meu    presenca.Modo
		frase  string
		alerta bool
	}{
		{"ninguém", nil, presenca.Ver, "", false},
		{"um vendo", []presenca.Presenca{p("2", "Maria", presenca.Ver)}, presenca.Ver,
			"Este registro está sendo visualizado por Maria (desde 13:35).", false},
		{"um editando", []presenca.Presenca{p("2", "João", presenca.Editar)}, presenca.Ver,
			"Este registro está sendo editado por João (desde 13:35).", true},
		{"vários e os dois modos", []presenca.Presenca{p("2", "João", presenca.Editar), p("3", "Ana", presenca.Editar), p("4", "Bia", presenca.Ver), p("5", "Caio", presenca.Ver), p("6", "Davi", presenca.Ver)}, presenca.Ver,
			"Este registro está sendo editado por João (desde 13:35) e Ana (desde 13:35) e visualizado por Bia (desde 13:35), Caio (desde 13:35) e Davi (desde 13:35).", true},
		{"eu em outra janela", []presenca.Presenca{p("1", "Eu", presenca.Ver)}, presenca.Editar,
			"Este registro está sendo visualizado por você, em outra janela (desde 13:35).", false},
		{"os dois editando", []presenca.Presenca{p("2", "João", presenca.Editar)}, presenca.Editar,
			"Este registro está sendo editado por João (desde 13:35). Cuidado: se você salvar, pode sobrescrever o que a outra pessoa alterou.", true},
	}
	for _, c := range casos {
		frase, alerta := frasePresenca(c.outros, eu, c.meu, fusoLocal)
		if frase != c.frase || alerta != c.alerta {
			t.Errorf("%s:\n  veio  %q (%v)\n  quer  %q (%v)", c.nome, frase, alerta, c.frase, c.alerta)
		}
	}
	if got := listaEmPortugues([]string{"a", "b", "c"}); got != "a, b e c" {
		t.Errorf("lista = %q", got)
	}
}

// ---- auxiliares dos testes de presença ----

func papelParams(nome string) sqlccliente.CreatePapelParams {
	return sqlccliente.CreatePapelParams{Nome: nome}
}

func permsParams(papel int64, codigos ...string) sqlccliente.AddPermissoesAoPapelParams {
	return sqlccliente.AddPermissoesAoPapelParams{PapelID: papel, Permissoes: codigos}
}

func addRespParams(bairro int64, usuarios ...int64) sqlccliente.AddResponsaveisAoBairroParams {
	return sqlccliente.AddResponsaveisAoBairroParams{BairroID: bairro, Usuarios: usuarios}
}

// novoUsuarioLogado cria um usuário (senha conhecida) com o papel dado e devolve um navegador já logado.
func (m *mundo) novoUsuarioLogado(t *testing.T, nome, email string, papelID int64) *navegador {
	t.Helper()
	const senha = "Senha-Do-Teste-9"
	hash, _ := auth.Hash(senha)
	if _, err := m.cliQ.CreateUsuario(t.Context(), sqlccliente.CreateUsuarioParams{Nome: nome, Email: email, SenhaHash: hash, PapelID: papelID, LoginWeb: true}); err != nil {
		t.Fatal(err)
	}
	n := m.em(t, hostNM)
	if r := n.entrar(email, senha, ""); local(r) != "/" {
		t.Fatalf("login de %s: %d %q", nome, r.Code, local(r))
	}
	return n
}

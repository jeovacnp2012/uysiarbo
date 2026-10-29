package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/tenant"
)

// contemHora diz se o corpo traz a hora atual (HH:MM de agora ou do minuto anterior) no fuso dado.
func contemHora(corpo string, loc *time.Location, prefixo string) bool {
	agora := time.Now()
	for _, t := range []time.Time{agora, agora.Add(-time.Minute), agora.Add(-2 * time.Minute)} {
		if strings.Contains(corpo, prefixo+t.In(loc).Format("15:04")) {
			return true
		}
	}
	return false
}

// masterEm devolve o master logado num endereço (a senha já foi trocada pelo console).
func (m *mundo) masterEm(t *testing.T, host string) *navegador {
	t.Helper()
	m.consoleMaster(t)
	n := m.em(t, host)
	if r := n.entrar("mestre@plataforma.com", m.masterPw, ""); local(r) != "/" {
		t.Fatalf("login do master em %s: %d %q", host, r.Code, local(r))
	}
	return n
}

func TestCadaClienteMostraOHorarioDoSeuFuso(t *testing.T) {
	m := novoMundo(t)
	cuiaba, saoPaulo := tenant.Local("America/Cuiaba"), tenant.Local("America/Sao_Paulo")
	if cuiaba.String() == saoPaulo.String() {
		t.Fatal("o teste precisa de dois fusos diferentes")
	}
	nm, sinop := m.masterEm(t, hostNM), m.masterEm(t, hostSinop)

	// O mesmo instante, dois clientes: a hora do aviso de presença segue o fuso de cada um.
	nm.get(pedidoPresenca("bairro", "1", "editar", abaA))
	sinop.get(pedidoPresenca("bairro", "1", "editar", abaA))
	corpoNM := nm.get(pedidoPresenca("bairro", "1", "ver", abaB)).Body.String()
	corpoSinop := sinop.get(pedidoPresenca("bairro", "1", "ver", abaB)).Body.String()
	if !contemHora(corpoNM, cuiaba, "(desde ") {
		t.Errorf("Nova Mutum (MT) deveria mostrar o horário de Cuiabá: %.400s", corpoNM)
	}
	if !contemHora(corpoSinop, saoPaulo, "(desde ") {
		t.Errorf("o cliente de SP deveria mostrar o horário de Brasília: %.400s", corpoSinop)
	}
	if contemHora(corpoNM, saoPaulo, "(desde ") {
		t.Error("o cliente de MT não pode mostrar a hora de Brasília")
	}

	// A auditoria também: o "quando" de cada cliente no seu fuso.
	hora := time.Now().In(cuiaba).Format("02/01/2006 15:")
	horaAnt := time.Now().Add(-time.Hour).In(cuiaba).Format("02/01/2006 15:")
	corpo := nm.get("/auditoria").Body.String()
	if !strings.Contains(corpo, hora) && !strings.Contains(corpo, horaAnt) {
		t.Errorf("a auditoria deveria ter data e hora de Cuiabá (%s)", hora)
	}
	if !strings.Contains(sinop.get("/auditoria").Body.String(), time.Now().In(saoPaulo).Format("02/01/2006 15:")) &&
		!strings.Contains(sinop.get("/auditoria").Body.String(), time.Now().Add(-time.Minute).In(saoPaulo).Format("02/01/2006 15:")) {
		t.Error("a auditoria do cliente de SP deveria ter data e hora de Brasília")
	}
}

func TestConsoleUsaOFusoDoConsole(t *testing.T) {
	m := novoMundo(t)
	console := m.consoleMaster(t)
	corpo := console.get("/auditoria").Body.String()
	if !contemHora(corpo, fusoLocal, "/2026 ") && !strings.Contains(corpo, time.Now().In(fusoLocal).Format("02/01/2006 15:")) {
		t.Errorf("o console deveria usar o fuso do console (Cuiabá): %.200s", corpo)
	}
}

func TestCriarClienteExigeEstadoEGravaFuso(t *testing.T) {
	m := novoMundo(t)
	console := m.consoleMaster(t)
	form := func(slug, sub, uf, fuso string) url.Values {
		return url.Values{"nome": {"Cliente " + slug}, "slug": {slug}, "subdominio": {sub}, "plano": {"padrao"}, "cidade": {"Cidade X"}, "uf": {uf}, "fuso": {fuso}}
	}
	quer(t, console.get("/clientes/novo"), http.StatusOK, "Cidade da empresa", "Estado (UF)", "Fuso horário", "Automático, pelo estado", "Mato Grosso")

	quer(t, console.post("/clientes", form("semuf", "uysiarbosu", "", "")), http.StatusBadRequest, "Escolha o estado")
	quer(t, console.post("/clientes", form("ufruim", "uysiarbouf", "XX", "")), http.StatusBadRequest, "Escolha o estado")
	quer(t, console.post("/clientes", form("fusoruim", "uysiarbofu", "MT", "Europe/Paris")), http.StatusBadRequest, "Fuso horário inválido")

	// Sem fuso escolhido: o do estado. Com fuso escolhido: o escolhido (exceções dentro do estado).
	quer(t, console.post("/clientes", form("manaus", "uysiarbomn", "AM", "")), http.StatusSeeOther)
	quer(t, console.post("/clientes", form("eirune", "uysiarboei", "AM", "America/Eirunepe")), http.StatusSeeOther)
	quer(t, console.post("/clientes", form("acre", "uysiarboac", "AC", "")), http.StatusSeeOther)
	for slug, quer := range map[string][2]string{"manaus": {"America/Manaus", "AM"}, "eirune": {"America/Eirunepe", "AM"}, "acre": {"America/Rio_Branco", "AC"}} {
		c, err := m.pubQ.GetClienteBySlug(t.Context(), slug)
		if err != nil || c.Fuso != quer[0] || c.Uf != quer[1] || c.Cidade != "Cidade X" {
			t.Errorf("%s: fuso=%q uf=%q cidade=%q err=%v, quer %v", slug, c.Fuso, c.Uf, c.Cidade, err, quer)
		}
	}
	// A criação fica na auditoria com o fuso.
	rows, _ := m.pubQ.ListAuditoriaPlataforma(t.Context(), sqlc.ListAuditoriaPlataformaParams{Acao: "cliente.criar", Limite: 20})
	var achou bool
	for _, r := range rows {
		if r.EntidadeID == "acre" && mapa(r.Depois)["fuso"] == "America/Rio_Branco" && mapa(r.Depois)["uf"] == "AC" {
			achou = true
		}
	}
	if !achou {
		t.Error("o cliente.criar deveria registrar cidade, estado e fuso")
	}
	// A lista mostra cidade e fuso.
	quer(t, console.get("/clientes"), http.StatusOK, "Cidade X/AC", "America/Rio_Branco", "America/Cuiaba", "Editar")
}

func TestEditarClienteMudaOFusoNaHora(t *testing.T) {
	m := novoMundo(t)
	console := m.consoleMaster(t)
	nm := m.masterEm(t, hostNM)
	rioBranco := tenant.Local("America/Rio_Branco")

	quer(t, console.get("/clientes/novamutum/editar"), http.StatusOK, "Editar cliente", "Prefeitura de Nova Mutum", `value="Nova Mutum"`, "não mudam depois de criados")
	quer(t, console.get("/clientes/nao-existe/editar"), http.StatusNotFound)

	// Antes: Cuiabá. Depois de trocar o estado para AC, o próximo pedido já usa Rio Branco.
	nm.get(pedidoPresenca("bairro", "1", "editar", abaA))
	if !contemHora(nm.get(pedidoPresenca("bairro", "1", "ver", abaB)).Body.String(), tenant.Local("America/Cuiaba"), "(desde ") {
		t.Fatal("antes da troca deveria ser o horário de Cuiabá")
	}
	form := url.Values{"nome": {"Prefeitura de Nova Mutum"}, "cidade": {"Nova Mutum"}, "uf": {"AC"}, "plano": {"padrao"}}
	quer(t, console.post("/clientes/novamutum/editar", form), http.StatusSeeOther)
	c, _ := m.pubQ.GetClienteBySlug(t.Context(), "novamutum")
	if c.Uf != "AC" || c.Fuso != "America/Rio_Branco" {
		t.Fatalf("depois de editar: uf=%q fuso=%q", c.Uf, c.Fuso)
	}
	nm.get(pedidoPresenca("bairro", "2", "editar", abaA))
	if !contemHora(nm.get(pedidoPresenca("bairro", "2", "ver", abaB)).Body.String(), rioBranco, "(desde ") {
		t.Error("depois da troca, o horário já deveria ser o de Rio Branco, sem esperar cache")
	}

	// Auditoria do antes e depois; salvar sem mudar não registra.
	rows, _ := m.pubQ.ListAuditoriaPlataforma(t.Context(), sqlc.ListAuditoriaPlataformaParams{Acao: "cliente.editar", Limite: 10})
	if len(rows) != 1 || mapa(rows[0].Antes)["fuso"] != "America/Cuiaba" || mapa(rows[0].Depois)["fuso"] != "America/Rio_Branco" {
		t.Errorf("auditoria da edição: %+v", rows)
	}
	quer(t, console.post("/clientes/novamutum/editar", form), http.StatusSeeOther)
	if rows, _ := m.pubQ.ListAuditoriaPlataforma(t.Context(), sqlc.ListAuditoriaPlataformaParams{Acao: "cliente.editar", Limite: 10}); len(rows) != 1 {
		t.Errorf("salvar sem mudança não deveria registrar (%d eventos)", len(rows))
	}

	// Validações.
	ruim := func(campo, valor string) url.Values {
		v := url.Values{"nome": {"Prefeitura de Nova Mutum"}, "cidade": {"Nova Mutum"}, "uf": {"MT"}, "plano": {"padrao"}}
		v.Set(campo, valor)
		return v
	}
	quer(t, console.post("/clientes/novamutum/editar", ruim("nome", "")), http.StatusBadRequest, "nome do cliente")
	quer(t, console.post("/clientes/novamutum/editar", ruim("uf", "")), http.StatusBadRequest, "Escolha o estado")
	quer(t, console.post("/clientes/novamutum/editar", ruim("fuso", "Asia/Tokyo")), http.StatusBadRequest, "Fuso horário inválido")
	quer(t, console.post("/clientes/novamutum/editar", ruim("plano", "")), http.StatusBadRequest, "plano")
	quer(t, console.post("/clientes/nao-existe/editar", ruim("nome", "X")), http.StatusNotFound)
	// Só o master edita clientes, e só no console.
	quer(t, nm.get("/clientes/novamutum/editar"), http.StatusNotFound)
}

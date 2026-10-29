package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
)

// cenarioDeAcesso: dois bairros, o A com dois quarteirões; um local em cada quarteirão e um no B.
type cenarioDeAcesso struct {
	a, b, qa1, qa2, qb1 int64
	l1, l2, l3          string // ids dos locais (A/q1, A/q2, B/q1)
}

func (m *mundo) cenarioDeAcesso(t *testing.T, ana *navegador) cenarioDeAcesso {
	t.Helper()
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	c := cenarioDeAcesso{a: m.criarBairro(t, cid, "Bairro A"), b: m.criarBairro(t, cid, "Bairro B")}
	c.qa1, c.qa2, c.qb1 = m.criarQuarteirao(t, c.a, "1"), m.criarQuarteirao(t, c.a, "2"), m.criarQuarteirao(t, c.b, "1")
	c.l1 = s64(umLocal(t, m, ana, c.a, c.qa1, "endereco", "Rua Um", "numero", "1", "latitude", "-13.82", "longitude", "-56.08"))
	c.l2 = s64(umLocal(t, m, ana, c.a, c.qa2, "endereco", "Rua Dois", "numero", "2", "latitude", "-13.821", "longitude", "-56.081"))
	c.l3 = s64(umLocal(t, m, ana, c.b, c.qb1, "endereco", "Rua Tres", "numero", "3", "latitude", "-13.822", "longitude", "-56.082"))
	return c
}

// camposDeCampo cria a Dora com permissão de ver só o que lhe foi atribuído.
func (m *mundo) doraDeCampo(t *testing.T) (*navegador, int64) {
	t.Helper()
	pd, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Campo"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pd.ID, "bairro_visualizar", "bairro_meubairro", "local_visualizar", "local_adicionar", "local_meubairro"))
	dora := m.novoUsuarioLogado(t, "Dora Reis", "dora@nm.gov.br", pd.ID)
	u, _ := m.cliQ.GetUsuarioByEmail(t.Context(), "dora@nm.gov.br")
	return dora, u.ID
}

func (m *mundo) metodoAcesso(t *testing.T, ana *navegador, metodo string) {
	t.Helper()
	quer(t, ana.post("/parametros/acesso/metodo", url.Values{"metodo": {metodo}}), http.StatusSeeOther)
}

func (m *mundo) concederAcesso(t *testing.T, ana *navegador, usuario, bairro, quarteirao int64) {
	t.Helper()
	f := url.Values{"usuario_id": {s64(usuario)}, "bairro_id": {s64(bairro)}}
	if quarteirao != 0 {
		f.Set("quarteirao_id", s64(quarteirao))
	}
	quer(t, ana.post("/parametros/acesso", f), http.StatusSeeOther)
}

func visivel(n *navegador, caminho string) bool { return n.get(caminho).Code == http.StatusOK }

func TestTelaDeControleDeAcesso(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioDeAcesso(t, ana)
	_, did := m.doraDeCampo(t)

	quer(t, ana.get("/parametros/acesso"), http.StatusOK, "Controle de acesso aos locais", "Pelo responsável do bairro", "Pela lista de acessos", "Nenhum acesso concedido", "Dora Reis")
	quer(t, ana.get("/"), http.StatusOK, `href="/parametros/acesso"`)
	carla := m.comCarla(t)
	quer(t, carla.get("/parametros/acesso"), http.StatusForbidden)
	quer(t, carla.post("/parametros/acesso/metodo", url.Values{"metodo": {"parametro"}}), http.StatusForbidden)

	// Método: inválido recusa; igual ao atual não registra; mudar registra antes e depois.
	quer(t, ana.post("/parametros/acesso/metodo", url.Values{"metodo": {"x"}}), http.StatusBadRequest, "método válido")
	quer(t, ana.post("/parametros/acesso/metodo", url.Values{"metodo": {"responsavel_bairro"}}), http.StatusSeeOther)
	if achar(m.eventos(t), "acesso.metodo") != nil {
		t.Error("manter o método não deveria registrar")
	}
	m.metodoAcesso(t, ana, "parametro")
	ev := achar(m.eventos(t), "acesso.metodo")
	if ev == nil || mapa(ev.Antes)["metodo"] != "Pelo responsável do bairro" || mapa(ev.Depois)["metodo"] != "Pela lista de acessos" {
		t.Fatalf("acesso.metodo: %+v", ev)
	}
	quer(t, ana.get("/parametros/acesso"), http.StatusOK, "Salvar método", `value="parametro" checked`)

	// Conceder: validações.
	for msg, f := range map[string]url.Values{
		"pessoa ativa":         {"usuario_id": {"999"}, "bairro_id": {s64(c.a)}},
		"Escolha o bairro":     {"usuario_id": {s64(did)}, "bairro_id": {"0"}},
		"quarteirão do bairro": {"usuario_id": {s64(did)}, "bairro_id": {s64(c.a)}, "quarteirao_id": {s64(c.qb1)}},
		"no máximo 200":        {"usuario_id": {s64(did)}, "bairro_id": {s64(c.a)}, "descricao": {strings.Repeat("x", 201)}},
	} {
		quer(t, ana.post("/parametros/acesso", f), http.StatusBadRequest, msg)
	}
	m.concederAcesso(t, ana, did, c.a, 0)
	rec := ana.post("/parametros/acesso", url.Values{"usuario_id": {s64(did)}, "bairro_id": {s64(c.a)}})
	if local(rec) != "/parametros/acesso?ok=ja" {
		t.Errorf("repetir: %q", local(rec))
	}
	m.concederAcesso(t, ana, did, c.a, c.qa2)
	quer(t, ana.get("/parametros/acesso"), http.StatusOK, "Dora Reis", "Bairro A", "Quarteirão 2 · Bairro A")
	quer(t, ana.get("/parametros/acesso?usuario="+s64(did)), http.StatusOK, "Dora Reis")
	quer(t, ana.get("/parametros/acesso/quarteiroes?bairro_id="+s64(c.a)), http.StatusOK, ">1<", ">2<")
	if e := achar(m.eventos(t), "acesso.criar"); e == nil || mapa(e.Depois)["tipo"] != "Quarteirão" || e.Rotulo != "Dora Reis" {
		t.Errorf("acesso.criar: %+v", e)
	}

	// Retirar.
	as, _ := m.cliQ.ListAcessosLocal(t.Context(), pgtype.Int8{})
	if len(as) != 2 {
		t.Fatalf("acessos = %d", len(as))
	}
	quer(t, ana.post("/parametros/acesso/"+s64(as[0].ID)+"/remover", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/parametros/acesso/999999/remover", url.Values{}), http.StatusNotFound)
	if achar(m.eventos(t), "acesso.remover") == nil {
		t.Error("faltou acesso.remover")
	}
	// O destino "voltar" só aceita caminhos de locais do próprio sistema.
	rec = ana.post("/parametros/acesso/"+s64(as[1].ID)+"/remover", url.Values{"voltar": {"//evil.example.com"}})
	if local(rec) != "/parametros/acesso?ok=removido" {
		t.Errorf("voltar externo redirecionou para %q", local(rec))
	}
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

func TestAcessoPorBairroQuarteiraoELocal(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioDeAcesso(t, ana)
	dora, did := m.doraDeCampo(t)
	// A Dora é responsável pelo Bairro B; no método "parametro" isso não vale nada.
	_ = m.cliQ.AddResponsaveisAoBairro(t.Context(), addRespParams(c.b, did))
	m.metodoAcesso(t, ana, "parametro")

	semNada := dora.get("/locais").Body.String()
	if strings.Contains(semNada, "Rua Um") || strings.Contains(semNada, "Rua Tres") {
		t.Error("sem nenhum acesso concedido, a Dora não enxerga locais (nem os do bairro em que é responsável)")
	}
	if visivel(dora, "/locais/"+c.l3) || visivel(dora, "/bairros/"+s64(c.b)) {
		t.Error("ser responsável não vale no método da lista de acessos")
	}

	// Acesso ao Bairro A inteiro.
	m.concederAcesso(t, ana, did, c.a, 0)
	corpo := dora.get("/locais").Body.String()
	if !strings.Contains(corpo, "Rua Um") || !strings.Contains(corpo, "Rua Dois") || strings.Contains(corpo, "Rua Tres") {
		t.Error("com acesso ao Bairro A, a Dora vê os locais dele e só eles")
	}
	if !visivel(dora, "/bairros/"+s64(c.a)) || visivel(dora, "/bairros/"+s64(c.b)) || visivel(dora, "/locais/"+c.l3) {
		t.Error("escopo do bairro")
	}
	as, _ := m.cliQ.ListAcessosLocal(t.Context(), pgtype.Int8{})
	quer(t, ana.post("/parametros/acesso/"+s64(as[0].ID)+"/remover", url.Values{}), http.StatusSeeOther)
	if visivel(dora, "/locais/"+c.l1) {
		t.Error("retirado o acesso, o local some")
	}

	// Acesso só ao Quarteirão 2 do Bairro A.
	m.concederAcesso(t, ana, did, c.a, c.qa2)
	corpo = dora.get("/locais").Body.String()
	if !strings.Contains(corpo, "Rua Dois") || strings.Contains(corpo, "Rua Um") || strings.Contains(corpo, "Rua Tres") {
		t.Error("com acesso ao quarteirão 2, só o local dele aparece")
	}
	if !visivel(dora, "/locais/"+c.l2) || visivel(dora, "/locais/"+c.l1) {
		t.Errorf("escopo do quarteirão na ficha do local: l2=%d l1=%d", dora.get("/locais/"+c.l2).Code, dora.get("/locais/"+c.l1).Code)
	}
	// O bairro é visto, mas só com o quarteirão dela.
	fichaBairro := dora.get("/bairros/" + s64(c.a))
	if fichaBairro.Code != http.StatusOK || !strings.Contains(fichaBairro.Body.String(), "Quarteirões <span class=\"selo\">1</span>") {
		t.Errorf("a ficha do bairro deveria listar só 1 quarteirão: %d", fichaBairro.Code)
	}
	opcoes := dora.get("/locais/quarteiroes?bairro_id=" + s64(c.a)).Body.String()
	if !strings.Contains(opcoes, ">2<") || strings.Contains(opcoes, ">1<") {
		t.Errorf("opções de quarteirão da Dora: %s", opcoes)
	}
	quer(t, dora.post("/locais", formLocal(c.a, c.qa1, "numero", "50")), http.StatusBadRequest, "ao qual você tem acesso")
	quer(t, dora.post("/locais", formLocal(c.a, c.qa2, "numero", "51")), http.StatusSeeOther)
	lote := url.Values{"tipo": {"residencial"}, "endereco": {"Rua X"}, "lado": {"Par"}, "bairro_id": {s64(c.a)}, "quarteirao_id": {s64(c.qa1)}, "numero_de": {"1"}, "numero_ate": {"2"}, "passo": {"1"}}
	quer(t, dora.post("/locais/lote", lote), http.StatusBadRequest, "ao qual você tem acesso")
	lote.Set("quarteirao_id", s64(c.qa2))
	quer(t, dora.post("/locais/lote", lote), http.StatusSeeOther)
	// Mapa: só os pontos do quarteirão 2 (o novo local não tem coordenadas).
	if cl := lerColecao(t, dora, "/mapa/locais.geojson"); len(cl.Features) != 1 || cl.Features[0].Properties["endereco"] != "Rua Dois, 2" {
		t.Errorf("mapa da Dora: %+v", cl)
	}
	// Grupo: acrescentar por bairro só traz o que ela enxerga.
	pg, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Grupos"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pg.ID, "bairro_visualizar", "local_visualizar", "local_meubairro", "local_grupo_visualizar", "local_grupo_gerenciar"))
	gabi := m.novoUsuarioLogado(t, "Gabi Luz", "gabi@nm.gov.br", pg.ID)
	gu, _ := m.cliQ.GetUsuarioByEmail(t.Context(), "gabi@nm.gov.br")
	m.concederAcesso(t, ana, gu.ID, c.a, c.qa1)
	g := m.criarGrupoPelaTela(t, gabi, "Rota da Gabi")
	rec := gabi.post("/grupos/"+g+"/locais", url.Values{"bairro_id": {s64(c.a)}})
	if !strings.Contains(local(rec), "adicionados=1") {
		t.Errorf("a Gabi só tem o quarteirão 1: %q", local(rec))
	}
	quer(t, gabi.post("/grupos/"+g+"/locais", url.Values{"bairro_id": {s64(c.a)}, "quarteirao_id": {s64(c.qa2)}}), http.StatusBadRequest, "ao qual você tem acesso")

	// Acesso a um único local.
	as, _ = m.cliQ.ListAcessosLocal(t.Context(), pgtype.Int8{})
	for _, a := range as {
		if a.UsuarioID == did {
			quer(t, ana.post("/parametros/acesso/"+s64(a.ID)+"/remover", url.Values{}), http.StatusSeeOther)
		}
	}
	quer(t, ana.post("/locais/"+c.l3+"/acesso", url.Values{"usuario_id": {s64(did)}}), http.StatusSeeOther)
	corpo = dora.get("/locais").Body.String()
	if !strings.Contains(corpo, "Rua Tres") || strings.Contains(corpo, "Rua Um") || strings.Contains(corpo, "Rua Dois") {
		t.Error("com acesso a um único local, só ele aparece")
	}
	if !visivel(dora, "/locais/"+c.l3) || !visivel(dora, "/bairros/"+s64(c.b)) {
		t.Error("o bairro do local é visível")
	}

	// De volta ao método do responsável: vale o bairro em que ela é responsável (B).
	m.metodoAcesso(t, ana, "responsavel_bairro")
	corpo = dora.get("/locais").Body.String()
	if !strings.Contains(corpo, "Rua Tres") || strings.Contains(corpo, "Rua Um") {
		t.Error("no método do responsável, a Dora vê o Bairro B")
	}
}

func TestFichaDoLocalMostraQuemTemAcesso(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioDeAcesso(t, ana)
	_, did := m.doraDeCampo(t)

	// No método do responsável, a seção não aparece.
	if strings.Contains(ana.get("/locais/"+c.l1).Body.String(), "Quem tem acesso a este local") {
		t.Error("a seção de acesso só existe no método da lista de acessos")
	}
	m.metodoAcesso(t, ana, "parametro")
	quer(t, ana.get("/locais/"+c.l1), http.StatusOK, "Quem tem acesso a este local", "Ninguém recebeu acesso", "Dar acesso a este local", "Dora Reis")

	m.concederAcesso(t, ana, did, c.a, 0)
	rec := ana.post("/locais/"+c.l1+"/acesso", url.Values{"usuario_id": {s64(did)}})
	if local(rec) != "/locais/"+c.l1+"?ok=acesso" {
		t.Fatalf("dar acesso: %q", local(rec))
	}
	quer(t, ana.post("/locais/"+c.l1+"/acesso", url.Values{"usuario_id": {"999"}}), http.StatusSeeOther)
	corpo := ana.get("/locais/" + c.l1).Body.String()
	if !strings.Contains(corpo, "pelo bairro") || !strings.Contains(corpo, "neste local") {
		t.Errorf("a ficha deveria mostrar as duas origens de acesso")
	}
	// Só o acesso direto (neste local) tem botão de retirar na ficha.
	if n := strings.Count(corpo, `name="voltar"`); n != 1 {
		t.Errorf("botões de retirar na ficha = %d, queria 1", n)
	}
	as, _ := m.cliQ.ListAcessosDoLocal(t.Context(), sqlccliente.ListAcessosDoLocalParams{Local: mustID(c.l1), Quarteirao: c.qa1, Bairro: c.a})
	var direto int64
	for _, a := range as {
		if a.Tipo == "local" {
			direto = a.ID
		}
	}
	rec = ana.post("/parametros/acesso/"+s64(direto)+"/remover", url.Values{"voltar": {"/locais/" + c.l1}})
	if local(rec) != "/locais/"+c.l1 {
		t.Errorf("voltar para a ficha: %q", local(rec))
	}
	quer(t, m.comCarla(t).post("/locais/"+c.l1+"/acesso", url.Values{"usuario_id": {s64(did)}}), http.StatusForbidden)
}

func mustID(s string) int64 {
	var n int64
	for _, c := range s {
		n = n*10 + int64(c-'0')
	}
	return n
}

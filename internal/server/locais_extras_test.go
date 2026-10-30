package server

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"uysiarbo/internal/db/sqlccliente"
)

// umLocal cria um local pela tela e devolve o id.
func umLocal(t *testing.T, m *mundo, n *navegador, b, q int64, extras ...string) int64 {
	t.Helper()
	quer(t, n.post("/locais", formLocal(b, q, extras...)), http.StatusSeeOther)
	ls, _ := m.cliQ.ListLocais(t.Context(), sqlccliente.ListLocaisParams{Limite: 100, VerDesativados: true})
	var maior int64
	for _, l := range ls { // a lista vem ordenada por endereço: o novo é o de maior id
		maior = max(maior, l.ID)
	}
	return maior
}

func TestChaveEQRCodeDoLocal(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	q := m.criarQuarteirao(t, b, "1")
	quer(t, ana.post("/locais", formLocal(b, q, "numero", "1")), http.StatusSeeOther)
	quer(t, ana.post("/locais", formLocal(b, q, "numero", "2")), http.StatusSeeOther)
	l1, _ := m.cliQ.GetLocal(t.Context(), 1)
	l2, _ := m.cliQ.GetLocal(t.Context(), 2)
	if len(l1.QrChave) != 20 || l1.QrChave == l2.QrChave {
		t.Fatalf("chaves: %q e %q", l1.QrChave, l2.QrChave)
	}
	quer(t, ana.get("/locais/1"), http.StatusOK, "Chave e QR Code", l1.QrChave, "/locais/1/qr.png", "Gerar nova chave")

	rec := ana.get("/locais/1/qr.png")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !bytes.HasPrefix(rec.Body.Bytes(), []byte("\x89PNG")) {
		t.Fatalf("qr.png: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	quer(t, ana.get("/locais/999/qr.png"), http.StatusNotFound)

	rec = ana.post("/locais/1/chave", url.Values{})
	if rec.Code != http.StatusSeeOther || local(rec) != "/locais/1?ok=chave" {
		t.Fatalf("nova chave: %d %q", rec.Code, local(rec))
	}
	novo, _ := m.cliQ.GetLocal(t.Context(), 1)
	if novo.QrChave == l1.QrChave || len(novo.QrChave) != 20 {
		t.Error("a chave deveria mudar")
	}
	if _, err := m.cliQ.GetLocalPorChave(t.Context(), l1.QrChave); err == nil {
		t.Error("a chave antiga não deveria mais achar o local")
	}
	quer(t, ana.get("/locais/1?ok=chave"), http.StatusOK, "Nova chave gerada")
	ev := achar(m.eventos(t), "local.chave_nova")
	if ev == nil || strings.Contains(string(ev.Depois)+string(ev.Antes)+string(ev.Detalhes), novo.QrChave) {
		t.Errorf("local.chave_nova: %+v (a chave não pode ir para o histórico)", ev)
	}
}

func TestDesativarEReativarLocal(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	q := m.criarQuarteirao(t, b, "1")
	id := umLocal(t, m, ana, b, q, "endereco", "Rua Velha")
	url1 := "/locais/" + s64(id)

	quer(t, ana.get(url1), http.StatusOK, "Desativar local")
	rec := ana.post(url1+"/ativo", url.Values{"ativo": {"0"}})
	if rec.Code != http.StatusSeeOther || local(rec) != url1+"?ok=desativado" {
		t.Fatalf("desativar: %d %q", rec.Code, local(rec))
	}
	quer(t, ana.get(url1+"?ok=desativado"), http.StatusOK, "Local desativado.", "Este local está desativado", "Reativar local")
	quer(t, ana.get("/locais"), http.StatusOK, "Rua Velha", "Inativo")
	quer(t, ana.get("/locais?situacao=ativo"), http.StatusOK, "Nenhum local encontrado")
	// repetir não gera novo registro
	quer(t, ana.post(url1+"/ativo", url.Values{"ativo": {"0"}}), http.StatusSeeOther)
	ev := achar(m.eventos(t), "local.desativar")
	if ev == nil || mapa(ev.Antes)["ativo"] != true || mapa(ev.Depois)["ativo"] != false {
		t.Fatalf("local.desativar: %+v", ev)
	}
	n := 0
	for _, e := range m.eventos(t) {
		if e.Acao == "local.desativar" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("local.desativar = %d, queria 1", n)
	}

	// Quem não tem "ver desativados" não enxerga o local desativado, nem pela ficha.
	pa, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Sem desativados"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pa.ID, "local_visualizar", "local_adicionar"))
	leo := m.novoUsuarioLogado(t, "Leo Dias", "leo@nm.gov.br", pa.ID)
	quer(t, leo.get("/locais"), http.StatusOK, "Nenhum local encontrado")
	quer(t, leo.get(url1), http.StatusNotFound)
	quer(t, leo.get(url1+"/qr.png"), http.StatusNotFound)
	quer(t, leo.post(url1+"/ativo", url.Values{"ativo": {"1"}}), http.StatusForbidden) // não tem local_desativar

	// Reativar.
	rec = ana.post(url1+"/ativo", url.Values{"ativo": {"1"}})
	if rec.Code != http.StatusSeeOther || local(rec) != url1+"?ok=reativado" {
		t.Fatalf("reativar: %d %q", rec.Code, local(rec))
	}
	if achar(m.eventos(t), "local.reativar") == nil {
		t.Error("faltou local.reativar na auditoria")
	}
	quer(t, leo.get("/locais"), http.StatusOK, "Rua Velha")
	quer(t, leo.get(url1), http.StatusOK)
	if strings.Contains(leo.get(url1).Body.String(), "Desativar local") {
		t.Error("quem não pode desativar não deveria ver o botão")
	}
	quer(t, ana.post("/locais/999/ativo", url.Values{"ativo": {"0"}}), http.StatusNotFound)
}

func TestCadastroDeLocaisEmLote(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	q := m.criarQuarteirao(t, b, "1")
	outro := m.criarBairro(t, cid, "Jardim")
	qo := m.criarQuarteirao(t, outro, "9")

	quer(t, ana.get("/locais/lote"), http.StatusOK, "Primeiro número", "Último número", "Sequência inicial", "Cadastro em lote")
	// o acesso ao cadastro em lote está escondido da tela de Locais (pedido do usuário); a rota segue existindo
	nao(t, ana.get("/locais"), "Cadastro em lote")

	lote := func(extras ...string) url.Values {
		v := url.Values{
			"tipo": {"residencial"}, "endereco": {"Rua das Acácias"}, "lado": {"Par"}, "bairro_id": {s64(b)}, "quarteirao_id": {s64(q)},
			"numero_de": {"2"}, "numero_ate": {"10"}, "passo": {"2"},
		}
		for i := 0; i+1 < len(extras); i += 2 {
			v.Set(extras[i], extras[i+1])
		}
		return v
	}
	ruim := map[string]url.Values{
		"Escolha o tipo":             lote("tipo", "casa"),
		"Informe o endereço":         lote("endereco", ""),
		"Informe o lado":             lote("lado", ""),
		"primeiro e o último":        lote("numero_de", "x"),
		"de 1 em 1 ou de 2 em 2":     lote("passo", "3"),
		"não pode ser menor":         lote("numero_de", "10", "numero_ate", "2"),
		"Muitos locais de uma vez":   lote("numero_de", "1", "numero_ate", "5000", "passo", "1"),
		"Sequência inicial":          lote("sequencia_inicial", "abc"),
		"quarteirão ativo do bairro": lote("quarteirao_id", s64(qo)),
		"bairro ativo":               lote("bairro_id", "0"),
	}
	for msg, f := range ruim {
		quer(t, ana.post("/locais/lote", f), http.StatusBadRequest, msg)
	}
	if n := contarLocais(t, m); n != 0 {
		t.Fatalf("recusas não deveriam criar nada (%d)", n)
	}

	// 2,4,6,8,10 com sequência 100..104.
	rec := ana.post("/locais/lote", lote("sequencia_inicial", "100"))
	if rec.Code != http.StatusSeeOther || local(rec) != "/locais?bairro="+s64(b)+"&lote=5-0" {
		t.Fatalf("lote: %d %q", rec.Code, local(rec))
	}
	quer(t, ana.get(local(rec)), http.StatusOK, "5 local(is) criado(s).", "Rua das Acácias, 6", "seq. 102")
	ls, _ := m.cliQ.ListLocais(t.Context(), sqlccliente.ListLocaisParams{Limite: 50})
	if len(ls) != 5 {
		t.Fatalf("locais = %d", len(ls))
	}
	ev := achar(m.eventos(t), "local.lote")
	if ev == nil || mapa(ev.Detalhes)["criados"] != float64(5) || mapa(ev.Detalhes)["de"] != float64(2) || mapa(ev.Detalhes)["passo"] != float64(2) {
		t.Fatalf("local.lote: %+v", ev)
	}

	// Repetir: nada novo, avisa quantos já existiam, sem novo registro de auditoria.
	rec = ana.post("/locais/lote", lote("sequencia_inicial", "100"))
	if local(rec) != "/locais?bairro="+s64(b)+"&lote=0-5" {
		t.Fatalf("repetir: %q", local(rec))
	}
	quer(t, ana.get(local(rec)), http.StatusOK, "0 local(is) criado(s) e 5 já existia(m).")
	n := 0
	for _, e := range m.eventos(t) {
		if e.Acao == "local.lote" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("local.lote = %d, queria 1", n)
	}

	// Trecho sobreposto cria só o que falta (8, 10 existem; 12, 14 são novos).
	rec = ana.post("/locais/lote", lote("numero_de", "8", "numero_ate", "14", "sequencia_inicial", "103"))
	if local(rec) != "/locais?bairro="+s64(b)+"&lote=2-2" {
		t.Errorf("sobreposto: %q", local(rec))
	}
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

func TestLoteRespeitaPermissoesEEscopo(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	a, b := m.criarBairro(t, cid, "Bairro A"), m.criarBairro(t, cid, "Bairro B")
	qa, qb := m.criarQuarteirao(t, a, "1"), m.criarQuarteirao(t, b, "1")
	form := func(bairro, quart int64) url.Values {
		return url.Values{"tipo": {"residencial"}, "endereco": {"Rua X"}, "lado": {"Par"}, "bairro_id": {s64(bairro)},
			"quarteirao_id": {s64(quart)}, "numero_de": {"1"}, "numero_ate": {"3"}, "passo": {"1"}}
	}
	carla := m.comCarla(t)
	quer(t, carla.get("/locais/lote"), http.StatusForbidden)
	quer(t, carla.post("/locais/lote", form(a, qa)), http.StatusForbidden)

	pa, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Campo restrito"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pa.ID, "local_visualizar", "local_adicionar", "local_meubairro"))
	dora := m.novoUsuarioLogado(t, "Dora Reis", "dora@nm.gov.br", pa.ID)
	did, _ := m.cliQ.GetUsuarioByEmail(t.Context(), "dora@nm.gov.br")
	_ = m.cliQ.AddResponsaveisAoBairro(t.Context(), addRespParams(a, did.ID))
	quer(t, dora.post("/locais/lote", form(b, qb)), http.StatusBadRequest, "em que você é responsável")
	quer(t, dora.post("/locais/lote", form(a, qa)), http.StatusSeeOther)
	if n := contarLocais(t, m); n != 3 {
		t.Errorf("locais = %d, queria 3", n)
	}
}

package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"uysiarbo/internal/db/sqlccliente"
)

// novoPapelComPermissoes cria um papel e um usuário logado com as permissões dadas.
func (m *mundo) usuarioCom(t *testing.T, nome, email string, permissoes ...string) *navegador {
	t.Helper()
	p, err := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Papel de " + nome})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(p.ID, permissoes...)); err != nil {
		t.Fatal(err)
	}
	return m.novoUsuarioLogado(t, nome, email, p.ID)
}

func TestExclusaoLogicaDoTerritorio(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	q := m.criarQuarteirao(t, b, "1")
	l1 := umLocal(t, m, ana, b, q, "numero", "1")
	l2 := umLocal(t, m, ana, b, q, "numero", "2")

	// Pai com filhos ativos: recusa, com aviso do que está ligado; nada muda.
	rec := ana.post("/bairros/"+s64(b)+"/excluir", url.Values{})
	quer(t, rec, http.StatusConflict, "Não foi possível excluir", "1 quarteirão e 2 locais", "Exclua ou mova esses registros antes")
	quer(t, ana.post("/bairros/"+s64(b)+"/quarteiroes/"+s64(q)+"/excluir", url.Values{}), http.StatusConflict, "2 locais")
	quer(t, ana.post("/cidades/"+s64(cid)+"/excluir", url.Values{}), http.StatusConflict, "1 bairro")
	quer(t, ana.get("/bairros/"+s64(b)), http.StatusOK)
	if achar(m.eventos(t), "bairro.excluir") != nil {
		t.Error("a recusa não gera evento de exclusão")
	}

	// Local: exclusão lógica. Some das listas, da ficha, do mapa e da API; o registro continua no banco.
	rec = ana.post("/locais/"+s64(l1)+"/excluir", url.Values{})
	if rec.Code != http.StatusSeeOther || local(rec) != "/locais?aviso=excluido" {
		t.Fatalf("excluir local: %d %q", rec.Code, local(rec))
	}
	quer(t, ana.get("/locais?aviso=excluido"), http.StatusOK, "Enviado para a Lixeira")
	if strings.Contains(ana.get("/locais").Body.String(), "Rua das Palmeiras, 1<") {
		t.Error("o local excluído não deveria aparecer na lista")
	}
	quer(t, ana.get("/locais/"+s64(l1)), http.StatusNotFound)
	quer(t, ana.post("/locais/"+s64(l1)+"/excluir", url.Values{}), http.StatusNotFound) // já excluído: some até das ações
	l, err := m.cliQ.GetLocalQualquer(t.Context(), l1)
	if err != nil || !l.DeletedAt.Valid || !l.DeletedBy.Valid {
		t.Fatalf("o registro deveria continuar no banco, marcado como excluído: %+v %v", l, err)
	}
	ev := achar(m.eventos(t), "local.excluir")
	if ev == nil || ev.Rotulo != "Rua das Palmeiras, 1" || mapa(ev.Detalhes)["para_a_lixeira"] != true {
		t.Errorf("local.excluir: %+v", ev)
	}
	// o endereço pode ser cadastrado de novo (o índice único só vale para os ativos)
	l1b := umLocal(t, m, ana, b, q, "numero", "1")
	if l1b == l1 {
		t.Error("deveria ter criado um registro novo")
	}

	// Lixeira: lista, restaura e avisa quando o nome já está em uso.
	quer(t, ana.get("/lixeira?tipo=local"), http.StatusOK, "Lixeira", "Rua das Palmeiras, 1", "Restaurar", "Excluir definitivamente")
	quer(t, ana.get("/lixeira?tipo=bairro"), http.StatusOK, "Nada na lixeira neste tipo")
	quer(t, ana.get("/lixeira?tipo=qualquer"), http.StatusOK, "Lixeira") // tipo desconhecido cai no primeiro
	quer(t, ana.post("/lixeira/local/"+s64(l1)+"/restaurar", url.Values{}), http.StatusConflict, "Não foi possível restaurar", "mesmo nome ou identificação")
	quer(t, ana.post("/locais/"+s64(l1b)+"/excluir", url.Values{}), http.StatusSeeOther)
	rec = ana.post("/lixeira/local/"+s64(l1)+"/restaurar", url.Values{})
	if rec.Code != http.StatusSeeOther || local(rec) != "/lixeira?tipo=local&ok=restaurado" {
		t.Fatalf("restaurar: %d %q", rec.Code, local(rec))
	}
	quer(t, ana.get("/locais/"+s64(l1)), http.StatusOK)
	if e := achar(m.eventos(t), "local.restaurar"); e == nil {
		t.Error("faltou local.restaurar")
	}

	// Árvore: do filho para o pai. Restaurar o filho antes do pai é recusado.
	quer(t, ana.post("/locais/"+s64(l1)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/locais/"+s64(l2)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/bairros/"+s64(b)+"/quarteiroes/"+s64(q)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/bairros/"+s64(b)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/cidades/"+s64(cid)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/cidades"), http.StatusOK, "Nenhuma cidade cadastrada")
	quer(t, ana.get("/bairros"), http.StatusOK, "Nenhum bairro encontrado")
	quer(t, ana.post("/lixeira/bairro/"+s64(b)+"/restaurar", url.Values{}), http.StatusConflict, "Restaure a cidade")
	quer(t, ana.post("/lixeira/local/"+s64(l2)+"/restaurar", url.Values{}), http.StatusConflict, "Restaure o quarteirão")
	quer(t, ana.post("/lixeira/cidade/"+s64(cid)+"/restaurar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/lixeira/bairro/"+s64(b)+"/restaurar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/lixeira/quarteirao/"+s64(q)+"/restaurar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/lixeira/local/"+s64(l2)+"/restaurar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/locais/"+s64(l2)), http.StatusOK)
	quer(t, ana.get("/cidades"), http.StatusOK, "Nova Mutum")

	// Pedidos inválidos.
	quer(t, ana.post("/lixeira/foo/1/restaurar", url.Values{}), http.StatusNotFound)
	quer(t, ana.post("/lixeira/local/999999/restaurar", url.Values{}), http.StatusNotFound)
	quer(t, ana.post("/locais/999999/excluir", url.Values{}), http.StatusNotFound)
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

func TestExclusaoDefinitivaSoComAutorizacaoESemVinculos(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	q := m.criarQuarteirao(t, b, "1")
	l1 := umLocal(t, m, ana, b, q, "numero", "1")

	// Quem só vê e restaura a lixeira não exclui definitivamente.
	so := m.usuarioCom(t, "Lia Restaura", "lia@nm.gov.br", "conf_lixeira", "local_visualizar", "local_adicionar")
	quer(t, so.post("/locais/"+s64(l1)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, so.get("/lixeira?tipo=local"), http.StatusOK, "Restaurar")
	if strings.Contains(so.get("/lixeira?tipo=local").Body.String(), "Excluir definitivamente") {
		t.Error("sem a permissão, o botão de exclusão definitiva não aparece")
	}
	quer(t, so.post("/lixeira/local/"+s64(l1)+"/excluir-definitivo", url.Values{}), http.StatusForbidden)
	quer(t, m.comCarla(t).get("/lixeira"), http.StatusForbidden)
	if _, err := m.cliQ.GetLocalQualquer(t.Context(), l1); err != nil {
		t.Fatal("o local deveria continuar no banco")
	}

	// Registro que NÃO está na lixeira não pode ser excluído definitivamente.
	l2 := umLocal(t, m, ana, b, q, "numero", "2")
	quer(t, ana.post("/lixeira/local/"+s64(l2)+"/excluir-definitivo", url.Values{}), http.StatusConflict, "Só dá para excluir definitivamente o que já está na lixeira")

	// Vínculo com grupo de locais impede, com aviso; sem vínculo, apaga de verdade.
	g := m.criarGrupoPelaTela(t, ana, "Rota")
	// o local já está na lixeira; o vínculo com o grupo foi feito antes de excluir
	if _, err := m.cliQ.AddLocalAoGrupo(t.Context(), sqlccliente.AddLocalAoGrupoParams{GrupoID: mustID(g), LocalID: l1}); err != nil {
		t.Fatal(err)
	}
	rec := ana.post("/lixeira/local/"+s64(l1)+"/excluir-definitivo", url.Values{})
	quer(t, rec, http.StatusConflict, "tem vínculo com outros registros", "1 grupo de locais", "só é permitida quando nada mais depende")
	if _, err := m.cliQ.GetLocalQualquer(t.Context(), l1); err != nil {
		t.Fatal("com vínculo, o registro não pode ser apagado")
	}
	gid := mustID(g)
	if _, err := m.cliQ.RemoveLocalDoGrupo(t.Context(), sqlccliente.RemoveLocalDoGrupoParams{GrupoID: gid, LocalID: l1}); err != nil {
		t.Fatal(err)
	}
	rec = ana.post("/lixeira/local/"+s64(l1)+"/excluir-definitivo", url.Values{})
	if rec.Code != http.StatusSeeOther || local(rec) != "/lixeira?tipo=local&ok=definitivo" {
		t.Fatalf("definitivo: %d %q", rec.Code, local(rec))
	}
	if _, err := m.cliQ.GetLocalQualquer(t.Context(), l1); err == nil {
		t.Error("agora o registro deveria ter sido apagado de verdade")
	}
	ev := achar(m.eventos(t), "local.excluir_definitivo")
	if ev == nil || ev.Rotulo != "Rua das Palmeiras, 1" || mapa(ev.Detalhes)["definitivo"] != true {
		t.Errorf("local.excluir_definitivo: %+v", ev)
	}

	// Pais: contam também os filhos já excluídos logicamente.
	l3 := umLocal(t, m, ana, b, q, "numero", "3")
	quer(t, ana.post("/locais/"+s64(l3)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/locais/"+s64(l2)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/bairros/"+s64(b)+"/quarteiroes/"+s64(q)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/lixeira/quarteirao/"+s64(q)+"/excluir-definitivo", url.Values{}), http.StatusConflict, "2 locais (contando os excluídos)")
	quer(t, ana.post("/bairros/"+s64(b)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/lixeira/bairro/"+s64(b)+"/excluir-definitivo", url.Values{}), http.StatusConflict, "1 quarteirão", "2 locais")
	quer(t, ana.post("/cidades/"+s64(cid)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/lixeira/cidade/"+s64(cid)+"/excluir-definitivo", url.Values{}), http.StatusConflict, "1 bairro (contando os excluídos)")
	// de baixo para cima, tudo pode ir embora
	for _, tp := range []struct{ tipo, id string }{{"local", s64(l2)}, {"local", s64(l3)}, {"quarteirao", s64(q)}, {"bairro", s64(b)}, {"cidade", s64(cid)}} {
		quer(t, ana.post("/lixeira/"+tp.tipo+"/"+tp.id+"/excluir-definitivo", url.Values{}), http.StatusSeeOther)
	}
	if _, err := m.cliQ.GetCidadeQualquer(t.Context(), cid); err == nil {
		t.Error("a cidade deveria ter sido apagada de verdade")
	}
	quer(t, ana.get("/lixeira"), http.StatusOK, "Nada na lixeira")
}

func TestExclusaoRespeitaEscopoDoPapel(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	a, b := m.criarBairro(t, cid, "Bairro A"), m.criarBairro(t, cid, "Bairro B")
	qa, qb := m.criarQuarteirao(t, a, "1"), m.criarQuarteirao(t, b, "1")
	la := umLocal(t, m, ana, a, qa, "endereco", "Rua A")
	lb := umLocal(t, m, ana, b, qb, "endereco", "Rua B")

	dora := m.usuarioCom(t, "Dora Reis", "dora@nm.gov.br", "bairro_visualizar", "bairro_adicionar", "bairro_meubairro", "local_visualizar", "local_adicionar", "local_meubairro")
	did, _ := m.cliQ.GetUsuarioByEmail(t.Context(), "dora@nm.gov.br")
	_ = m.cliQ.AddResponsaveisAoBairro(t.Context(), addRespParams(a, did.ID))
	quer(t, dora.post("/locais/"+s64(lb)+"/excluir", url.Values{}), http.StatusNotFound)
	quer(t, dora.post("/bairros/"+s64(b)+"/excluir", url.Values{}), http.StatusNotFound)
	quer(t, dora.post("/bairros/"+s64(b)+"/quarteiroes/"+s64(qb)+"/excluir", url.Values{}), http.StatusNotFound)
	quer(t, dora.post("/bairros/"+s64(a)+"/quarteiroes/"+s64(qb)+"/excluir", url.Values{}), http.StatusNotFound) // quarteirão de outro bairro
	quer(t, dora.post("/locais/"+s64(la)+"/excluir", url.Values{}), http.StatusSeeOther)
	l, _ := m.cliQ.GetLocalQualquer(t.Context(), lb)
	if l.DeletedAt.Valid {
		t.Error("o local do Bairro B não podia ser excluído pela Dora")
	}
	// sem permissão de editar, nem a exclusão lógica
	soVe := m.usuarioCom(t, "Edu Lima", "edu@nm.gov.br", "bairro_visualizar", "local_visualizar")
	quer(t, soVe.post("/locais/"+s64(lb)+"/excluir", url.Values{}), http.StatusForbidden)
	quer(t, soVe.post("/bairros/"+s64(b)+"/excluir", url.Values{}), http.StatusForbidden)
}

func TestExclusaoDeGrupoVaiParaALixeira(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	q := m.criarQuarteirao(t, b, "1")
	umLocal(t, m, ana, b, q, "numero", "1")
	g := m.criarGrupoPelaTela(t, ana, "Rota")
	quer(t, ana.post("/grupos/"+g+"/locais", url.Values{"bairro_id": {s64(b)}}), http.StatusSeeOther)

	rec := ana.post("/grupos/"+g+"/excluir", url.Values{})
	if rec.Code != http.StatusSeeOther || local(rec) != "/grupos?aviso=excluido" {
		t.Fatalf("excluir grupo: %d %q", rec.Code, local(rec))
	}
	quer(t, ana.get("/grupos"), http.StatusOK, "Nenhum grupo cadastrado")
	quer(t, ana.get("/grupos/"+g), http.StatusNotFound)
	quer(t, ana.get("/lixeira?tipo=grupo"), http.StatusOK, "Rota")
	// o nome pode ser reutilizado; restaurar com o nome em uso é recusado
	m.criarGrupoPelaTela(t, ana, "Rota")
	quer(t, ana.post("/lixeira/grupo/"+g+"/restaurar", url.Values{}), http.StatusConflict, "mesmo nome")
	// restaurar traz os locais do grupo de volta
	quer(t, ana.post("/grupos/"+s64(mustID(g)+1)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/lixeira/grupo/"+g+"/restaurar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/grupos/"+g), http.StatusOK, "Rua das Palmeiras, 1")
	if achar(m.eventos(t), "grupo.restaurar") == nil {
		t.Error("faltou grupo.restaurar")
	}
	// grupo na lixeira pode ser apagado de vez (a lista de membros sai junto)
	quer(t, ana.post("/grupos/"+g+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/lixeira/grupo/"+g+"/excluir-definitivo", url.Values{}), http.StatusSeeOther)
}

func TestAPISincronizaExcluidos(t *testing.T) {
	m := novoMundo(t)
	api := novaAPI(t, m)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	q := m.criarQuarteirao(t, b, "1")
	l := umLocal(t, m, ana, b, q, "numero", "1")
	desde := url.QueryEscape(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))

	quer(t, ana.post("/locais/"+s64(l)+"/excluir", url.Values{}), http.StatusSeeOther)
	// Sem updated_since o excluído não aparece; com ele, aparece marcado, para o aplicativo apagar do tablet.
	if ids, _ := api.lista("/api/v1/locais"); len(ids) != 0 {
		t.Errorf("sem updated_since: %v", ids)
	}
	rec := api.req(http.MethodGet, "/api/v1/locais?updated_since="+desde, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"excluido_em":"`) {
		t.Fatalf("com updated_since: %d %s", rec.Code, rec.Body.String())
	}
	quemAPI(t, api.req(http.MethodGet, "/api/v1/locais/"+s64(l), ""), http.StatusNotFound, "not_found")
	if ids, _ := api.lista("/api/v1/quarteiroes?updated_since=" + desde); len(ids) != 1 {
		t.Errorf("quarteirões: %v", ids)
	}
	// Ativos trazem excluido_em nulo.
	rec = api.req(http.MethodGet, "/api/v1/bairros?updated_since="+desde, "")
	if !strings.Contains(rec.Body.String(), `"excluido_em":null`) {
		t.Errorf("bairro ativo: %s", rec.Body.String())
	}
}

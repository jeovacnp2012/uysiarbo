package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"uysiarbo/internal/db/sqlccliente"
)

// cenarioDeGrupos cria dois bairros com locais e devolve os ids.
type cenarioDeGrupos struct {
	cid, a, b, qa1, qa2, qb1 int64
}

func (m *mundo) cenarioDeGrupos(t *testing.T, n *navegador) cenarioDeGrupos {
	t.Helper()
	c := cenarioDeGrupos{cid: m.criarCidade(t, "Nova Mutum", "MT")}
	c.a, c.b = m.criarBairro(t, c.cid, "Bairro A"), m.criarBairro(t, c.cid, "Bairro B")
	c.qa1, c.qa2, c.qb1 = m.criarQuarteirao(t, c.a, "1"), m.criarQuarteirao(t, c.a, "2"), m.criarQuarteirao(t, c.b, "1")
	quer(t, n.post("/locais", formLocal(c.a, c.qa1, "endereco", "Rua Alfa", "numero", "1")), http.StatusSeeOther)
	quer(t, n.post("/locais", formLocal(c.a, c.qa1, "endereco", "Rua Alfa", "numero", "2", "tipo", "comercio")), http.StatusSeeOther)
	quer(t, n.post("/locais", formLocal(c.a, c.qa2, "endereco", "Rua Gama", "numero", "3")), http.StatusSeeOther)
	quer(t, n.post("/locais", formLocal(c.b, c.qb1, "endereco", "Rua Beta", "numero", "4")), http.StatusSeeOther)
	return c
}

func (m *mundo) criarGrupoPelaTela(t *testing.T, n *navegador, nome string) string {
	t.Helper()
	rec := n.post("/grupos", url.Values{"nome": {nome}, "descricao": {"para teste"}})
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(local(rec), "/grupos/") {
		t.Fatalf("criar grupo %q: %d %q", nome, rec.Code, local(rec))
	}
	return strings.TrimSuffix(strings.Split(strings.TrimPrefix(local(rec), "/grupos/"), "?")[0], "/")
}

func TestGrupoCriarEditarEValidar(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	quer(t, ana.get("/grupos"), http.StatusOK, "Nenhum grupo cadastrado", "Novo grupo")
	quer(t, ana.get("/grupos/novo"), http.StatusOK, "Dê um nome")
	quer(t, ana.post("/grupos", url.Values{"nome": {"X"}}), http.StatusBadRequest, "nome do grupo")
	quer(t, ana.post("/grupos", url.Values{"nome": {"Grupo válido"}, "descricao": {strings.Repeat("x", 301)}}), http.StatusBadRequest, "no máximo 300")

	id := m.criarGrupoPelaTela(t, ana, "Rota de segunda")
	quer(t, ana.post("/grupos", url.Values{"nome": {"rota DE segunda"}}), http.StatusBadRequest, "Já existe um grupo")
	quer(t, ana.get("/grupos"), http.StatusOK, "Rota de segunda", "para teste")
	quer(t, ana.get("/grupos/"+id), http.StatusOK, "Rota de segunda", "Adicionar Local", "Exportar planilha", "Clonar grupo", "Nenhum registro encontrado")
	quer(t, ana.get("/grupos/"+id+"?ok=criado"), http.StatusOK, "Grupo criado")

	// Editar registra antes e depois; salvar igual não registra.
	quer(t, ana.post("/grupos/"+id, url.Values{"nome": {"Rota de terça"}, "descricao": {"para teste"}}), http.StatusSeeOther)
	ev := achar(m.eventos(t), "grupo.editar")
	if ev == nil || mapa(ev.Antes)["nome"] != "Rota de segunda" || mapa(ev.Depois)["nome"] != "Rota de terça" {
		t.Fatalf("grupo.editar: %+v", ev)
	}
	quer(t, ana.post("/grupos/"+id, url.Values{"nome": {"Rota de terça"}, "descricao": {"para teste"}}), http.StatusSeeOther)
	n := 0
	for _, e := range m.eventos(t) {
		if e.Acao == "grupo.editar" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("grupo.editar = %d, queria 1", n)
	}
	quer(t, ana.get("/grupos/999"), http.StatusNotFound)
	quer(t, ana.get("/grupos/abc"), http.StatusNotFound)
	quer(t, ana.post("/grupos/999", url.Values{"nome": {"Qualquer"}}), http.StatusNotFound)
	if achar(m.eventos(t), "grupo.criar") == nil {
		t.Error("faltou grupo.criar")
	}
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

func TestGrupoAcrescentarRetirarEExcluir(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioDeGrupos(t, ana)
	id := m.criarGrupoPelaTela(t, ana, "Rota")
	postAdd := func(f url.Values) string {
		rec := ana.post("/grupos/"+id+"/locais", f)
		return local(rec)
	}

	// Recusas.
	quer(t, ana.post("/grupos/"+id+"/locais", url.Values{"bairro_id": {"0"}}), http.StatusBadRequest, "Escolha o bairro")
	quer(t, ana.post("/grupos/"+id+"/locais", url.Values{"bairro_id": {s64(c.a)}, "quarteirao_id": {s64(c.qb1)}}), http.StatusBadRequest, "quarteirão do bairro")
	quer(t, ana.post("/grupos/"+id+"/locais", url.Values{"bairro_id": {s64(c.a)}, "tipo": {"casa"}}), http.StatusBadRequest, "Tipo de local inválido")

	// Só o comércio do Bairro A.
	if got := postAdd(url.Values{"bairro_id": {s64(c.a)}, "tipo": {"comercio"}}); got != "/grupos/"+id+"?adicionados=1" {
		t.Fatalf("filtro por tipo: %q", got)
	}
	// O Bairro A inteiro: 3 locais, 1 já estava.
	if got := postAdd(url.Values{"bairro_id": {s64(c.a)}}); got != "/grupos/"+id+"?adicionados=2" {
		t.Fatalf("bairro inteiro: %q", got)
	}
	// Repetir não acrescenta nada (e não registra de novo).
	if got := postAdd(url.Values{"bairro_id": {s64(c.a)}}); got != "/grupos/"+id+"?adicionados=0" {
		t.Fatalf("repetir: %q", got)
	}
	// Um quarteirão do outro bairro.
	if got := postAdd(url.Values{"bairro_id": {s64(c.b)}, "quarteirao_id": {s64(c.qb1)}}); got != "/grupos/"+id+"?adicionados=1" {
		t.Fatalf("quarteirão: %q", got)
	}
	corpo := ana.get("/grupos/" + id + "?adicionados=2").Body.String()
	for _, quer := range []string{"2 local(is) acrescentado(s)", "Rua Alfa, 1", "Rua Alfa, 2", "Rua Gama, 3", "Rua Beta, 4", "Comércio"} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("a ficha do grupo deveria conter %q", quer)
		}
	}
	quer(t, ana.get("/grupos"), http.StatusOK, ">4<")
	adds := 0
	for _, e := range m.eventos(t) {
		if e.Acao == "grupo.locais_adicionar" {
			adds++
		}
	}
	if adds != 3 {
		t.Errorf("grupo.locais_adicionar = %d, queria 3 (a repetição não registra)", adds)
	}

	// Local desativado não entra por filtro.
	if _, err := m.cliQ.SetLocalAtivo(t.Context(), sqlccliente.SetLocalAtivoParams{ID: 3, Ativo: false}); err != nil {
		t.Fatal(err)
	}
	g2 := m.criarGrupoPelaTela(t, ana, "Outra rota")
	rec := ana.post("/grupos/"+g2+"/locais", url.Values{"bairro_id": {s64(c.a)}, "quarteirao_id": {s64(c.qa2)}})
	if local(rec) != "/grupos/"+g2+"?adicionados=0" {
		t.Errorf("desativado não deveria entrar: %q", local(rec))
	}

	// Retirar um local, e depois esvaziar.
	quer(t, ana.post("/grupos/"+id+"/locais/1/remover", url.Values{}), http.StatusSeeOther)
	if strings.Contains(ana.get("/grupos/"+id).Body.String(), "Rua Alfa, 1<") {
		t.Error("o local retirado não deveria aparecer")
	}
	quer(t, ana.post("/grupos/"+id+"/locais/1/remover", url.Values{}), http.StatusSeeOther) // de novo: nada a fazer
	quer(t, ana.post("/grupos/"+id+"/locais/999/remover", url.Values{}), http.StatusNotFound)
	rem := 0
	for _, e := range m.eventos(t) {
		if e.Acao == "grupo.local_remover" {
			rem++
		}
	}
	if rem != 1 {
		t.Errorf("grupo.local_remover = %d, queria 1", rem)
	}
	quer(t, ana.post("/grupos/"+id+"/limpar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/grupos/"+id), http.StatusOK, "Nenhum registro encontrado")

	// Excluir o grupo não apaga os locais.
	rec = ana.post("/grupos/"+id+"/excluir", url.Values{})
	if rec.Code != http.StatusSeeOther || local(rec) != "/grupos?aviso=excluido" {
		t.Fatalf("excluir: %d %q", rec.Code, local(rec))
	}
	quer(t, ana.get("/grupos/"+id), http.StatusNotFound)
	if n, _ := m.cliQ.CountLocais(t.Context(), sqlccliente.CountLocaisParams{VerDesativados: true}); n != 4 {
		t.Errorf("os locais deveriam continuar (%d)", n)
	}
	if achar(m.eventos(t), "grupo.excluir") == nil || achar(m.eventos(t), "grupo.limpar") == nil {
		t.Error("faltou grupo.excluir ou grupo.limpar")
	}
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

func TestClonarEExportarGrupo(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioDeGrupos(t, ana)
	id := m.criarGrupoPelaTela(t, ana, "Rota")
	quer(t, ana.post("/grupos/"+id+"/locais", url.Values{"bairro_id": {s64(c.a)}}), http.StatusSeeOther)

	// Clonar: "Cópia de Rota", depois "(2)", com os mesmos locais.
	r1 := ana.post("/grupos/"+id+"/clonar", url.Values{})
	r2 := ana.post("/grupos/"+id+"/clonar", url.Values{})
	if r1.Code != http.StatusSeeOther || r2.Code != http.StatusSeeOther || local(r1) == local(r2) {
		t.Fatalf("clonar: %q %q", local(r1), local(r2))
	}
	quer(t, ana.get("/grupos"), http.StatusOK, "Cópia de Rota", "Cópia de Rota (2)")
	corpo := ana.get(local(r1)).Body.String()
	for _, quer := range []string{"Rua Alfa, 1", "Rua Alfa, 2", "Rua Gama, 3"} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("a cópia deveria conter %q", quer)
		}
	}
	ev := achar(m.eventos(t), "grupo.clonar")
	if ev == nil || mapa(ev.Detalhes)["origem"] != "Rota" || mapa(ev.Detalhes)["locais"] != float64(3) {
		t.Errorf("grupo.clonar: %+v", ev)
	}

	// Exportar: CSV com BOM, ponto e vírgula, só endereços (nada de dados pessoais).
	completo := camposCompletos(c.a, c.qa1, "1")
	completo.Set("endereco", "Rua Alfa")
	quer(t, ana.post("/locais/1", completo), http.StatusSeeOther) // dá dados pessoais ao local 1
	rec := ana.get("/grupos/" + id + "/exportar.csv")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") || !strings.Contains(rec.Header().Get("Content-Disposition"), "grupo-"+id+".csv") {
		t.Fatalf("exportar: %d %v", rec.Code, rec.Header())
	}
	csv := rec.Body.String()
	if !strings.HasPrefix(csv, "\xEF\xBB\xBFBairro;Quarteirão;Endereço;Número;Lado;Sequência;Complemento;Tipo;Situação;Latitude;Longitude;Chave") {
		t.Errorf("cabeçalho do CSV: %.150q", csv)
	}
	if got := strings.Count(csv, "\n"); got != 4 {
		t.Errorf("linhas do CSV = %d, queria 4 (cabeçalho + 3)", got)
	}
	for _, proibido := range []string{"Maria Souza", "52998224725", "529.982.247-25", "maria@exemplo.com", "(65) 3000-0000"} {
		if strings.Contains(csv, proibido) {
			t.Errorf("o CSV não pode conter dado pessoal: %q", proibido)
		}
	}
	if !strings.Contains(csv, "Bairro A;1;Rua Alfa;1;Par;5;2;Residencial;Ativo;-13.8247;-56.0813;") {
		t.Errorf("linha do local 1 no CSV: %q", csv)
	}
	if e := achar(m.eventos(t), "grupo.exportar"); e == nil || mapa(e.Detalhes)["locais"] != float64(3) {
		t.Errorf("grupo.exportar: %+v", e)
	}
	quer(t, ana.get("/grupos/999/exportar.csv"), http.StatusNotFound)
}

func TestTextoSeguroCSV(t *testing.T) {
	for entrada, esperado := range map[string]string{"Rua A": "Rua A", "=SOMA(A1)": "'=SOMA(A1)", "+55": "'+55", "-1": "'-1", "@x": "'@x", "": ""} {
		if got := textoSeguroCSV(entrada); got != esperado {
			t.Errorf("textoSeguroCSV(%q) = %q, queria %q", entrada, got, esperado)
		}
	}
}

func TestGruposRespeitamEscopoEPermissoes(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioDeGrupos(t, ana)
	id := m.criarGrupoPelaTela(t, ana, "Rota")
	quer(t, ana.post("/grupos/"+id+"/locais", url.Values{"bairro_id": {s64(c.a)}}), http.StatusSeeOther)
	quer(t, ana.post("/grupos/"+id+"/locais", url.Values{"bairro_id": {s64(c.b)}}), http.StatusSeeOther) // 4 locais no total

	carla := m.comCarla(t)
	for _, p := range []string{"/grupos", "/grupos/novo", "/grupos/" + id, "/grupos/" + id + "/exportar.csv", "/grupos/quarteiroes?bairro_id=1"} {
		quer(t, carla.get(p), http.StatusForbidden)
	}
	quer(t, carla.post("/grupos", url.Values{"nome": {"X"}}), http.StatusForbidden)
	if strings.Contains(carla.get("/").Body.String(), `href="/grupos"`) {
		t.Error("o menu da Carla não deveria ter Grupos de locais")
	}

	// Só consulta: vê e exporta, não altera.
	pv, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Consulta de grupos"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pv.ID, "local_grupo_visualizar", "local_visualizar"))
	edu := m.novoUsuarioLogado(t, "Edu Lima", "edu@nm.gov.br", pv.ID)
	quer(t, edu.get("/grupos/"+id), http.StatusOK, "Rua Alfa, 1")
	quer(t, edu.get("/grupos/"+id+"/exportar.csv"), http.StatusOK)
	if strings.Contains(edu.get("/grupos").Body.String(), "Novo grupo") || strings.Contains(edu.get("/grupos/"+id).Body.String(), "Clonar grupo") {
		t.Error("quem só consulta não deveria ver os botões de gerenciar")
	}
	quer(t, edu.get("/grupos/novo"), http.StatusForbidden)
	quer(t, edu.post("/grupos/"+id+"/excluir", url.Values{}), http.StatusForbidden)
	quer(t, edu.post("/grupos/"+id+"/clonar", url.Values{}), http.StatusForbidden)

	// "Apenas atribuídos": a Dora só enxerga (conta, lista, exporta, copia e remove) o Bairro A.
	pd, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Campo"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pd.ID, "local_grupo_visualizar", "local_grupo_gerenciar", "local_visualizar", "local_adicionar", "local_meubairro"))
	dora := m.novoUsuarioLogado(t, "Dora Reis", "dora@nm.gov.br", pd.ID)
	did, _ := m.cliQ.GetUsuarioByEmail(t.Context(), "dora@nm.gov.br")
	_ = m.cliQ.AddResponsaveisAoBairro(t.Context(), addRespParams(c.a, did.ID))
	quer(t, dora.get("/grupos"), http.StatusOK, ">3<")
	corpo := dora.get("/grupos/" + id).Body.String()
	if !strings.Contains(corpo, "Rua Alfa, 1") || strings.Contains(corpo, "Rua Beta") {
		t.Error("a Dora deveria ver só o Bairro A no grupo")
	}
	if csv := dora.get("/grupos/" + id + "/exportar.csv").Body.String(); strings.Contains(csv, "Rua Beta") || !strings.Contains(csv, "Rua Alfa") {
		t.Error("a exportação da Dora deveria ter só o Bairro A")
	}
	quer(t, dora.post("/grupos/"+id+"/locais", url.Values{"bairro_id": {s64(c.b)}}), http.StatusBadRequest, "em que você é responsável")
	quer(t, dora.get("/grupos/quarteiroes?bairro_id="+s64(c.b)), http.StatusNotFound)
	quer(t, dora.post("/grupos/"+id+"/locais/4/remover", url.Values{}), http.StatusNotFound) // local do Bairro B

	rec := dora.post("/grupos/"+id+"/clonar", url.Values{})
	copia := ana.get(local(rec)).Body.String()
	if !strings.Contains(copia, "Rua Alfa, 1") || strings.Contains(copia, "Rua Beta") {
		t.Error("a cópia da Dora deveria ter só os locais que ela enxerga")
	}
	quer(t, dora.post("/grupos/"+id+"/limpar", url.Values{}), http.StatusSeeOther)
	if !strings.Contains(ana.get("/grupos/"+id).Body.String(), "Rua Beta, 4") {
		t.Error("o esvaziamento da Dora não pode tirar o que ela não enxerga")
	}
	if strings.Contains(ana.get("/grupos/"+id).Body.String(), "Rua Alfa") {
		t.Error("o esvaziamento da Dora deveria tirar o Bairro A")
	}
}

func TestFichaDoLocalComSecoesGruposEHistorico(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioDeGrupos(t, ana)
	g1 := m.criarGrupoPelaTela(t, ana, "Rota 1")
	m.criarGrupoPelaTela(t, ana, "Rota 2")

	corpo := ana.get("/locais/1").Body.String()
	for _, quer := range []string{"Dados do Local", "Visitas", "Zoonose — Campanha Antirrábica", "Notificações", "Ovitrampa", "EDL — Estação",
		"Criar Grupo", "Histórico de alterações", "Cadastrou um local"} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("a ficha do local deveria conter %q", quer)
		}
	}

	gr := ana.get("/locais/1/grupos").Body.String()
	for _, quer := range []string{"Grupos de locais", "não está em nenhum grupo", "Acrescentar a um grupo", "Rota 1", "Rota 2", "Dados do Local"} {
		if !strings.Contains(gr, quer) {
			t.Errorf("a tela de grupos do local deveria conter %q", quer)
		}
	}
	rec := ana.post("/locais/1/grupo", url.Values{"grupo_id": {g1}})
	if rec.Code != http.StatusSeeOther || local(rec) != "/locais/1/grupos?ok=nogrupo" {
		t.Fatalf("acrescentar ao grupo: %d %q", rec.Code, local(rec))
	}
	corpo = ana.get("/locais/1/grupos?ok=nogrupo").Body.String()
	if !strings.Contains(corpo, "Local acrescentado ao grupo.") || !strings.Contains(corpo, `href="/grupos/`+g1+`"`) {
		t.Error("a ficha deveria mostrar o grupo e o aviso")
	}
	if strings.Contains(corpo, `<option value="`+g1+`">`) {
		t.Error("o grupo em que o local já está não deveria ser oferecido de novo")
	}
	quer(t, ana.get("/grupos/"+g1), http.StatusOK, "Rua Alfa, 1")
	quer(t, ana.post("/locais/1/grupo", url.Values{"grupo_id": {g1}}), http.StatusSeeOther) // repetir não duplica
	quer(t, ana.post("/locais/1/grupo", url.Values{"grupo_id": {"999"}}), http.StatusSeeOther)
	quer(t, ana.post("/locais/999/grupo", url.Values{"grupo_id": {g1}}), http.StatusNotFound)

	// Quem não tem a permissão de grupos nem a de auditoria não vê essas seções.
	pa, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Só locais"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pa.ID, "local_visualizar", "local_adicionar"))
	leo := m.novoUsuarioLogado(t, "Leo Dias", "leo@nm.gov.br", pa.ID)
	corpo = leo.get("/locais/1").Body.String()
	if !strings.Contains(corpo, "Dados do Local") || strings.Contains(corpo, "Histórico de alterações") || strings.Contains(corpo, "Acrescentar a um grupo") {
		t.Error("a ficha do Leo deveria ter as seções reservadas, mas não grupos nem histórico")
	}
	quer(t, leo.post("/locais/1/grupo", url.Values{"grupo_id": {g1}}), http.StatusForbidden)
	_ = c
}

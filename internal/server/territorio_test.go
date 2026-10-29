package server

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/auth"
	"uysiarbo/internal/db/sqlccliente"
)

func (m *mundo) criarCidade(t *testing.T, nome, uf string) int64 {
	t.Helper()
	c, err := m.cliQ.CreateCidade(t.Context(), sqlccliente.CreateCidadeParams{Nome: nome, Uf: uf})
	if err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func (m *mundo) criarBairro(t *testing.T, cidade int64, nome string) int64 {
	t.Helper()
	b, err := m.cliQ.CreateBairro(t.Context(), sqlccliente.CreateBairroParams{
		CidadeID: cidade, Nome: nome, Status: "ativo", Classificacao: "urbano", Categoria: "bairro", Producao: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.ID
}

func s64(n int64) string { return strconv.FormatInt(n, 10) }

// formBairro monta um formulário de bairro válido; os testes mudam só o que querem testar.
func formBairro(cidade int64, nome string, extras ...string) url.Values {
	v := url.Values{
		"nome": {nome}, "cidade_id": {s64(cidade)}, "status": {"ativo"}, "categoria": {"bairro"}, "classificacao": {"urbano"},
		"producao": {"1"},
	}
	for i := 0; i+1 < len(extras); i += 2 {
		if extras[i] == "responsavel" { // pode repetir
			v[extras[i]] = append(v[extras[i]], extras[i+1])
		} else {
			v[extras[i]] = []string{extras[i+1]}
		}
	}
	return v
}

func TestCidades(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	quer(t, ana.get("/cidades"), http.StatusOK, "Nenhuma cidade cadastrada", "Nova cidade")
	quer(t, ana.get("/cidades/nova"), http.StatusOK, "Nome da cidade", "UF")

	form := func(nome, uf, ibge string) url.Values { return url.Values{"nome": {nome}, "uf": {uf}, "ibge": {ibge}} }
	quer(t, ana.post("/cidades", form("N", "MT", "")), http.StatusBadRequest, "nome da cidade")
	quer(t, ana.post("/cidades", form("Nova Mutum", "XX", "")), http.StatusBadRequest, "estado brasileiro")
	quer(t, ana.post("/cidades", form("Nova Mutum", "MT", "abc")), http.StatusBadRequest, "código IBGE")
	quer(t, ana.post("/cidades", form("Nova Mutum", "mt", "5106240")), http.StatusSeeOther) // UF em minúsculas é normalizada
	quer(t, ana.post("/cidades", form("nova mutum", "MT", "")), http.StatusBadRequest, "Já existe uma cidade")

	quer(t, ana.get("/cidades"), http.StatusOK, "Nova Mutum", "5106240", "Ativa")
	cs, _ := m.cliQ.ListCidades(t.Context())
	if len(cs) != 1 || cs[0].Uf != "MT" {
		t.Fatalf("cidades = %+v", cs)
	}
	id := s64(cs[0].ID)

	// Editar: desmarcar "ativa" a desativa; sem mudança nada é registrado.
	quer(t, ana.get("/cidades/"+id), http.StatusOK, "Nova Mutum", "Cidade ativa")
	quer(t, ana.post("/cidades/"+id, url.Values{"nome": {"Nova Mutum"}, "uf": {"MT"}, "ibge": {"5106240"}, "ativo": {"1"}}), http.StatusSeeOther)
	quer(t, ana.post("/cidades/"+id, url.Values{"nome": {"Nova Mutum"}, "uf": {"MT"}, "ibge": {"5106240"}}), http.StatusSeeOther)
	quer(t, ana.get("/cidades"), http.StatusOK, "Inativa")
	quer(t, ana.get("/cidades/999999"), http.StatusNotFound)

	ev := m.eventos(t)
	if achar(ev, "cidade.criar") == nil {
		t.Error("faltou cidade.criar na auditoria")
	}
	edit := achar(ev, "cidade.editar")
	if edit == nil || mapa(edit.Antes)["ativo"] != true || mapa(edit.Depois)["ativo"] != false {
		t.Errorf("cidade.editar: %+v", edit)
	}
	n := 0
	for _, e := range ev {
		if e.Acao == "cidade.editar" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("a edição sem mudança não deve gerar registro (cidade.editar = %d)", n)
	}
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d: as telas novas devem registrar os próprios eventos", got)
	}
}

func (m *mundo) contarGenericos(t *testing.T) int {
	t.Helper()
	n := 0
	for _, e := range m.eventos(t) {
		if e.Acao == "requisicao.escrita" {
			n++
		}
	}
	return n
}

func TestBairroCriarEditarEValidar(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	idAna, idBruno := m.idUsuario(t, emailAna), m.idUsuario(t, emailBruno)

	rec := ana.get("/bairros/novo")
	quer(t, rec, http.StatusOK, "Novo bairro", "Nova Mutum/MT", "Responsáveis", "Ana Souza", "Bruno Lima", "Bairro de mutirão", "relatório de produção")
	if strings.Contains(rec.Body.String(), "quarteiroes") {
		t.Error("os quarteirões só aparecem depois de salvar o bairro")
	}

	// Criar, com coordenadas com vírgula e dois responsáveis.
	rec = ana.post("/bairros", formBairro(cid, "Jardim das Flores",
		"mutirao", "1", "zona", "Norte", "ibge", "123", "latitude", "-13,8247", "longitude", "-56.0813", "altitude", "420,5",
		"distancia_centro", "2,5", "categoria", "povoado", "classificacao", "rural", "responsavel", idAna, "responsavel", idBruno))
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(local(rec), "/bairros/") || !strings.HasSuffix(local(rec), "#quarteiroes") {
		t.Fatalf("criar: %d %q %.300s", rec.Code, local(rec), rec.Body.String())
	}
	lista, _ := m.cliQ.ListBairros(t.Context(), sqlccliente.ListBairrosParams{Limite: 50})
	if len(lista) != 1 {
		t.Fatalf("bairros = %d", len(lista))
	}
	b, _ := m.cliQ.GetBairro(t.Context(), lista[0].ID)
	if b.Nome != "Jardim das Flores" || !b.Mutirao || b.Categoria != "povoado" || b.Classificacao != "rural" || b.Zona != "Norte" ||
		b.Latitude.Float64 != -13.8247 || b.Longitude.Float64 != -56.0813 || b.Altitude.Float64 != 420.5 || b.DistanciaCentro.Float64 != 2.5 {
		t.Errorf("bairro gravado: %+v", b)
	}
	id := s64(b.ID)
	resp, _ := m.cliQ.ListResponsaveisDoBairro(t.Context(), b.ID)
	if len(resp) != 2 {
		t.Errorf("responsáveis = %v", resp)
	}
	quer(t, ana.get("/bairros"), http.StatusOK, "Jardim das Flores", "Ana Souza, Bruno Lima", "Nova Mutum/MT", "povoado")

	// Validações (cada uma volta ao formulário com a mensagem e o que foi digitado).
	erros := map[string]struct {
		form url.Values
		msg  string
	}{
		"nome curto":       {formBairro(cid, "A"), "nome do bairro"},
		"situação":         {formBairro(cid, "Outro", "status", "talvez"), "Situação inválida"},
		"classificação":    {formBairro(cid, "Outro", "classificacao", "x"), "Classificação inválida"},
		"categoria":        {formBairro(cid, "Outro", "categoria", "x"), "Categoria inválida"},
		"cidade inválida":  {formBairro(99999, "Outro"), "cidade ativa"},
		"latitude sozinha": {formBairro(cid, "Outro", "latitude", "-13.8"), "latitude e longitude juntas"},
		"latitude fora":    {formBairro(cid, "Outro", "latitude", "95", "longitude", "10"), "fora do limite"},
		"latitude texto":   {formBairro(cid, "Outro", "latitude", "abc", "longitude", "10"), "inválida"},
		"distância":        {formBairro(cid, "Outro", "distancia_centro", "-3"), "fora do limite"},
		"responsável":      {formBairro(cid, "Outro", "responsavel", "99999"), "usuários ativos"},
		"nome repetido":    {formBairro(cid, "jardim das flores"), "Já existe um bairro com esse nome"},
	}
	for nome, c := range erros {
		rec := ana.post("/bairros", c.form)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), c.msg) {
			t.Errorf("%s: status %d, quer 400 com %q (%.200s)", nome, rec.Code, c.msg, rec.Body.String())
		}
	}
	if rec := ana.post("/bairros", formBairro(cid, "Outro", "status", "talvez")); !strings.Contains(rec.Body.String(), `value="Outro"`) {
		t.Error("depois de um erro o formulário volta preenchido")
	}
	// O banco também protege, mesmo que a tela falhasse.
	if _, err := m.cliQ.CreateBairro(t.Context(), sqlccliente.CreateBairroParams{CidadeID: cid, Nome: "Fora", Status: "ativo", Classificacao: "urbano", Categoria: "bairro",
		Latitude: pgtype.Float8{Float64: 100, Valid: true}}); err == nil {
		t.Error("o banco deveria recusar latitude 100")
	}

	// Editar: muda dados e troca um responsável; a auditoria mostra o antes/depois e quem entrou e saiu.
	quer(t, ana.get("/bairros/"+id), http.StatusOK, "Jardim das Flores", "Quarteirões", "Adicionar quarteirões")
	rec = ana.post("/bairros/"+id, formBairro(cid, "Jardim das Flores II", "zona", "Norte", "ibge", "123", "latitude", "-13.8247", "longitude", "-56.0813",
		"altitude", "420.5", "distancia_centro", "2.5", "mutirao", "1", "categoria", "povoado", "classificacao", "rural", "responsavel", idAna))
	if rec.Code != http.StatusSeeOther || local(rec) != "/bairros" {
		t.Fatalf("editar: %d %q %.200s", rec.Code, local(rec), rec.Body.String())
	}
	ev := achar(m.eventos(t), "bairro.editar")
	if ev == nil {
		t.Fatal("faltou bairro.editar")
	}
	if a, d := mapa(ev.Antes), mapa(ev.Depois); a["nome"] != "Jardim das Flores" || d["nome"] != "Jardim das Flores II" || len(a) != 1 {
		t.Errorf("antes=%v depois=%v (só o nome mudou)", a, d)
	}
	det := mapa(ev.Detalhes)
	if rem, _ := det["responsaveis_removidos"].([]any); len(rem) != 1 || rem[0] != "Bruno Lima" {
		t.Errorf("responsáveis removidos: %v", det)
	}
	// Salvar de novo sem mudar nada não gera registro.
	antes := len(m.eventos(t))
	quer(t, ana.post("/bairros/"+id, formBairro(cid, "Jardim das Flores II", "zona", "Norte", "ibge", "123", "latitude", "-13.8247", "longitude", "-56.0813",
		"altitude", "420.5", "distancia_centro", "2.5", "mutirao", "1", "categoria", "povoado", "classificacao", "rural", "responsavel", idAna)), http.StatusSeeOther)
	if depois := len(m.eventos(t)); depois != antes {
		t.Errorf("sem mudança não deveria registrar (eventos %d -> %d)", antes, depois)
	}
	cb := achar(m.eventos(t), "bairro.criar")
	if cb == nil || mapa(cb.Depois)["cidade"] != "Nova Mutum/MT" {
		t.Errorf("bairro.criar: %+v", cb)
	}
	quer(t, ana.get("/bairros/999999"), http.StatusNotFound)
	quer(t, ana.get("/bairros/abc"), http.StatusNotFound)
	if m.contarGenericos(t) != 0 {
		t.Error("as telas do território devem registrar eventos próprios, sem cair no genérico")
	}
}

func TestQuarteiroesEmLote(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	bid := m.criarBairro(t, m.criarCidade(t, "Nova Mutum", "MT"), "Centro")
	url1 := "/bairros/" + s64(bid) + "/quarteiroes"

	rec := ana.post(url1, url.Values{"nomes": {"1-20\nA1\n5\n  \nB2"}}) // o 5 repete, as linhas em branco somem
	if rec.Code != http.StatusSeeOther || !strings.Contains(local(rec), "criados=22") {
		t.Fatalf("lote: %d %q", rec.Code, local(rec))
	}
	corpo := ana.get("/bairros/" + s64(bid) + "?criados=22&existiam=0").Body.String()
	for _, quer := range []string{"22 quarteirão(ões) criado(s)", ">A1<", ">B2<"} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("faltou %q", quer)
		}
	}
	// Ordem natural: 2 vem antes de 10.
	if strings.Index(corpo, "<strong>2</strong>") > strings.Index(corpo, "<strong>10</strong>") {
		t.Error("os quarteirões deveriam vir em ordem numérica (2 antes de 10)")
	}
	// Repetir não duplica.
	rec = ana.post(url1, url.Values{"nomes": {"1-5\nA1"}})
	if !strings.Contains(local(rec), "criados=0&existiam=6") {
		t.Errorf("repetição: %q", local(rec))
	}
	quer(t, ana.get("/bairros/"+s64(bid)+"?criados=0&existiam=6"), http.StatusOK, "0 quarteirão(ões) criado(s) e 6 já existia(m)")
	if qs, _ := m.cliQ.ListQuarteiroesDoBairro(t.Context(), bid); len(qs) != 22 {
		t.Errorf("quarteirões = %d, quer 22", len(qs))
	}

	for nome, c := range map[string]struct{ nomes, msg string }{
		"vazio":             {"  \n ", "pelo menos um"},
		"faixa invertida":   {"20-10", "Faixa inválida"},
		"faixa enorme":      {"1-9999", "Faixa inválida"},
		"nome comprido":     {strings.Repeat("x", 31), "30 caracteres"},
		"muitos de uma vez": {"1-400\n1000-1200", "Muitos quarteirões"},
	} {
		quer(t, ana.post(url1, url.Values{"nomes": {c.nomes}}), http.StatusBadRequest, c.msg)
		_ = nome
	}

	// Desativar e reativar.
	qs, _ := m.cliQ.ListQuarteiroesDoBairro(t.Context(), bid)
	qid := s64(qs[0].ID)
	quer(t, ana.post(url1+"/"+qid+"/ativo", url.Values{"ativo": {"0"}}), http.StatusSeeOther)
	quer(t, ana.get("/bairros/"+s64(bid)), http.StatusOK, "quarteirao quarteirao-inativo", "Reativar")
	quer(t, ana.post(url1+"/"+qid+"/ativo", url.Values{"ativo": {"1"}}), http.StatusSeeOther)
	quer(t, ana.post(url1+"/"+qid+"/ativo", url.Values{"ativo": {"1"}}), http.StatusSeeOther) // repetido: sem efeito
	quer(t, ana.post(url1+"/999999/ativo", url.Values{"ativo": {"0"}}), http.StatusNotFound)

	ev := m.eventos(t)
	qc := achar(ev, "quarteirao.criar")
	if qc == nil || mapa(qc.Detalhes)["criados"] != float64(22) || qc.Rotulo != "Centro" {
		t.Errorf("quarteirao.criar: %+v", qc)
	}
	n := 0
	for _, e := range ev {
		if e.Acao == "quarteirao.status" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("quarteirao.status = %d, quer 2 (desativar e reativar; o repetido não conta)", n)
	}
	if m.contarGenericos(t) != 0 {
		t.Error("registros genéricos inesperados")
	}
	// Um quarteirão de outro bairro não é alcançável por este endereço.
	outro := m.criarBairro(t, m.criarCidade(t, "Sinop", "MT"), "Outro")
	quer(t, ana.post("/bairros/"+s64(outro)+"/quarteiroes/"+qid+"/ativo", url.Values{"ativo": {"0"}}), http.StatusNotFound)
}

func TestApenasBairrosAtribuidos(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	a, b := m.criarBairro(t, cid, "Bairro A"), m.criarBairro(t, cid, "Bairro B")

	papel, err := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Supervisor de campo"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.cliQ.AddPermissoesAoPapel(t.Context(), sqlccliente.AddPermissoesAoPapelParams{
		PapelID: papel.ID, Permissoes: []string{"bairro_visualizar", "bairro_adicionar", "bairro_meubairro"},
	}); err != nil {
		t.Fatal(err)
	}
	hash, _ := auth.Hash("Senha-Da-Dora-1")
	dora, err := m.cliQ.CreateUsuario(t.Context(), sqlccliente.CreateUsuarioParams{Nome: "Dora Reis", Email: "dora@nm.gov.br", SenhaHash: hash, PapelID: papel.ID, LoginWeb: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.cliQ.AddResponsaveisAoBairro(t.Context(), sqlccliente.AddResponsaveisAoBairroParams{BairroID: a, Usuarios: []int64{dora.ID}}); err != nil {
		t.Fatal(err)
	}

	d := m.em(t, hostNM)
	if r := d.entrar("dora@nm.gov.br", "Senha-Da-Dora-1", ""); local(r) != "/" {
		t.Fatalf("login da Dora: %d %q", r.Code, local(r))
	}
	rec := d.get("/bairros")
	quer(t, rec, http.StatusOK, "Bairro A", "apenas os bairros atribuídos")
	if strings.Contains(rec.Body.String(), "Bairro B") {
		t.Error("a Dora não deveria ver o Bairro B")
	}
	quer(t, d.get("/bairros/"+s64(a)), http.StatusOK, "Bairro A")
	quer(t, d.get("/bairros/"+s64(b)), http.StatusNotFound)
	quer(t, d.post("/bairros/"+s64(b), formBairro(cid, "Bairro B alterado")), http.StatusNotFound)
	quer(t, d.post("/bairros/"+s64(b)+"/quarteiroes", url.Values{"nomes": {"1"}}), http.StatusNotFound)
	quer(t, d.post("/bairros/"+s64(a)+"/quarteiroes", url.Values{"nomes": {"1-3"}}), http.StatusSeeOther)
	if qs, _ := m.cliQ.ListQuarteiroesDoBairro(t.Context(), b); len(qs) != 0 {
		t.Error("nenhum quarteirão deveria ter sido criado no Bairro B")
	}
	// Quem cria um bairro novo começa como responsável (senão ele sumiria da própria lista).
	quer(t, d.get("/bairros/novo"), http.StatusOK)
	if !regexp.MustCompile(`value="` + s64(dora.ID) + `"[^>]*checked`).MatchString(d.get("/bairros/novo").Body.String()) {
		t.Error("quem só vê os atribuídos deveria começar marcado como responsável do bairro novo")
	}

	// Quem não tem a restrição (Ana, Administrador) vê os dois.
	quer(t, m.comAna(t).get("/bairros"), http.StatusOK, "Bairro A", "Bairro B")
	// O master também.
	quer(t, m.comMaster(t).get("/bairros"), http.StatusOK, "Bairro A", "Bairro B")
}

func TestPermissoesDoTerritorio(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	bid := m.criarBairro(t, cid, "Centro")

	// Sem nenhuma permissão do bloco.
	carla := m.comCarla(t)
	for _, p := range []string{"/bairros", "/bairros/novo", "/bairros/" + s64(bid), "/cidades", "/cidades/nova"} {
		quer(t, carla.get(p), http.StatusForbidden, "Sem permissão")
	}
	quer(t, carla.post("/bairros", formBairro(cid, "X")), http.StatusForbidden)
	if corpo := carla.get("/").Body.String(); strings.Contains(corpo, `href="/bairros"`) || strings.Contains(corpo, `href="/cidades"`) {
		t.Error("o menu da Carla não deveria ter Bairros nem Cidades")
	}

	// Só ler: vê tudo, mas não edita.
	papel, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Consulta"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), sqlccliente.AddPermissoesAoPapelParams{PapelID: papel.ID, Permissoes: []string{"bairro_visualizar"}})
	hash, _ := auth.Hash("Senha-Do-Edu-1")
	if _, err := m.cliQ.CreateUsuario(t.Context(), sqlccliente.CreateUsuarioParams{Nome: "Edu Lima", Email: "edu@nm.gov.br", SenhaHash: hash, PapelID: papel.ID, LoginWeb: true}); err != nil {
		t.Fatal(err)
	}
	edu := m.em(t, hostNM)
	edu.entrar("edu@nm.gov.br", "Senha-Do-Edu-1", "")
	rec := edu.get("/bairros")
	quer(t, rec, http.StatusOK, "Centro")
	if strings.Contains(rec.Body.String(), "Novo bairro") {
		t.Error("quem só consulta não vê o botão Novo bairro")
	}
	quer(t, edu.get("/bairros/new"), http.StatusNotFound)
	quer(t, edu.get("/bairros/novo"), http.StatusForbidden)
	ficha := edu.get("/bairros/" + s64(bid))
	quer(t, ficha, http.StatusOK, "Centro", "disabled")
	if strings.Contains(ficha.Body.String(), "Salvar bairro") || strings.Contains(ficha.Body.String(), "Adicionar quarteirões") {
		t.Error("a ficha de quem só consulta não deve ter botões de gravar")
	}
	quer(t, edu.post("/bairros/"+s64(bid), formBairro(cid, "Centro alterado")), http.StatusForbidden)
	quer(t, edu.post("/bairros/"+s64(bid)+"/quarteiroes", url.Values{"nomes": {"1"}}), http.StatusForbidden)
	quer(t, edu.post("/cidades", url.Values{"nome": {"Outra"}, "uf": {"MT"}}), http.StatusForbidden)
	quer(t, edu.get("/cidades"), http.StatusOK, "Nova Mutum")
	if bd, _ := m.cliQ.GetBairro(t.Context(), bid); bd.Nome != "Centro" {
		t.Errorf("o bairro não podia ter mudado: %q", bd.Nome)
	}
}

func TestPainelMostraBairrosDisponivel(t *testing.T) {
	m := novoMundo(t)
	corpo := m.comAna(t).get("/").Body.String()
	if !strings.Contains(corpo, `class="modulo modulo-pronto" href="/bairros"`) || !strings.Contains(corpo, "disponível") {
		t.Error("o módulo Bairros deveria aparecer como disponível, com link")
	}
	if !strings.Contains(corpo, `href="/bairros"`) || !strings.Contains(corpo, `href="/cidades"`) {
		t.Error("o menu e os atalhos deveriam ter Bairros e Cidades")
	}
	for _, href := range []string{"/auditoria", "/usuarios"} {
		if !strings.Contains(corpo, `class="modulo modulo-pronto" href="`+href+`"`) {
			t.Errorf("os módulos Auditoria e Configuração também já têm tela (%s)", href)
		}
	}
	// Os outros módulos continuam em construção.
	if !strings.Contains(corpo, "em construção") {
		t.Error("os demais módulos ainda estão em construção")
	}
}

func TestMigracaoDoTerritorioTemAsRestricoes(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	if _, err := m.cliQ.CreateCidade(t.Context(), sqlccliente.CreateCidadeParams{Nome: "NOVA MUTUM", Uf: "MT"}); err == nil {
		t.Error("cidade repetida (ignorando caixa) deveria ser recusada")
	}
	if _, err := m.cliQ.CreateCidade(t.Context(), sqlccliente.CreateCidadeParams{Nome: "Outra", Uf: "mt"}); err == nil {
		t.Error("UF em minúsculas deveria ser recusada pelo banco")
	}
	b := m.criarBairro(t, cid, "Centro")
	if _, err := m.pool.Exec(t.Context(), `DELETE FROM c_novamutum.cidades WHERE id = $1`, cid); err == nil {
		t.Error("não se apaga uma cidade que tem bairros")
	}
	if _, err := m.cliQ.CreateQuarteirao(t.Context(), sqlccliente.CreateQuarteiraoParams{BairroID: b, Nome: ""}); err == nil {
		t.Error("quarteirão sem nome deveria ser recusado")
	}
	if _, err := m.pool.Exec(t.Context(), `DELETE FROM c_novamutum.bairros WHERE id = $1`, b); err != nil {
		t.Errorf("apagar um bairro sem quarteirões deveria ser possível: %v", err)
	}
}

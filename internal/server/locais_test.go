package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"uysiarbo/internal/cifra"
	"uysiarbo/internal/db/sqlccliente"
)

func (m *mundo) criarQuarteirao(t *testing.T, bairro int64, nome string) int64 {
	t.Helper()
	if _, err := m.cliQ.CreateQuarteirao(t.Context(), sqlccliente.CreateQuarteiraoParams{BairroID: bairro, Nome: nome}); err != nil {
		t.Fatal(err)
	}
	qs, _ := m.cliQ.ListQuarteiroesDoBairro(t.Context(), bairro)
	for _, q := range qs {
		if q.Nome == nome {
			return q.ID
		}
	}
	t.Fatalf("quarteirão %q não encontrado", nome)
	return 0
}

// formLocal monta um formulário de local válido; os testes mudam só o que querem testar.
func formLocal(bairro, quarteirao int64, extras ...string) url.Values {
	v := url.Values{
		"tipo": {"residencial"}, "endereco": {"Rua das Palmeiras"}, "numero": {"120"}, "lado": {"Par"},
		"bairro_id": {s64(bairro)}, "quarteirao_id": {s64(quarteirao)},
	}
	for i := 0; i+1 < len(extras); i += 2 {
		v.Set(extras[i], extras[i+1])
	}
	return v
}

const cpfBom = "529.982.247-25"

// camposCompletos é o formulário completo da Maria, reaproveitado para salvar "sem mudança".
func camposCompletos(b, q int64, numero string) url.Values {
	return formLocal(b, q, "numero", numero, "sequencia", "5", "complemento", "2", "cep", "78450-000", "referencia", "Perto da praça",
		"latitude", "-13.8247", "longitude", "-56.0813", "proprietario", "Maria Souza", "proprietario_cpf", cpfBom,
		"proprietario_telefone", "(65) 3000-0000", "proprietario_email", "maria@exemplo.com", "aceita_whatsapp", "1",
		"inquilino", "João Lima", "humanos", "4", "cachorros", "1", "gatos", "0")
}

func contarLocais(t *testing.T, m *mundo) int {
	t.Helper()
	n, err := m.cliQ.CountLocais(t.Context(), sqlccliente.CountLocaisParams{})
	if err != nil {
		t.Fatal(err)
	}
	return int(n)
}

func TestLocalCriarEditarEValidar(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b1, b2 := m.criarBairro(t, cid, "Centro"), m.criarBairro(t, cid, "Jardim")
	q1, q2 := m.criarQuarteirao(t, b1, "1"), m.criarQuarteirao(t, b2, "7")

	quer(t, ana.get("/locais"), http.StatusOK, "Nenhum local encontrado", "Novo local")
	quer(t, ana.get("/locais/novo"), http.StatusOK, "Rua ou avenida", "Residencial", "Terreno baldio", "Ponto estratégico", "Centro", "Jardim", "Habitantes", "Moradores")

	ruim := map[string]url.Values{
		"Escolha o tipo":               formLocal(b1, q1, "tipo", "casa"),
		"Informe o endereço":           formLocal(b1, q1, "endereco", "R"),
		"Informe o número":             formLocal(b1, q1, "numero", ""),
		"Informe o lado":               formLocal(b1, q1, "lado", ""),
		"CEP inválido":                 formLocal(b1, q1, "cep", "123"),
		"Escolha o bairro":             formLocal(0, q1),
		"quarteirão do bairro":         formLocal(b1, q2),
		"Sequência":                    formLocal(b1, q1, "sequencia", "-3"),
		"e longitude juntas":           formLocal(b1, q1, "latitude", "-13.8"),
		"Humanos":                      formLocal(b1, q1, "humanos", "abc"),
		"CPF inválido":                 formLocal(b1, q1, "proprietario_cpf", "111.111.111-11"),
		"E-mail do proprietário":       formLocal(b1, q1, "proprietario_email", "nao-e-email"),
		"Longitude fora do limite (de": formLocal(b1, q1, "latitude", "-13.8", "longitude", "-500"),
	}
	for msg, f := range ruim {
		quer(t, ana.post("/locais", f), http.StatusBadRequest, msg)
	}
	if n := contarLocais(t, m); n != 0 {
		t.Fatalf("nenhum local deveria existir, há %d", n)
	}

	// Criar com todos os campos.
	rec := ana.post("/locais", formLocal(b1, q1, "cep", "78450000", "referencia", "Perto da praça", "sequencia", "5", "complemento", "2",
		"latitude", "-13,8247", "longitude", "-56.0813", "proprietario", "Maria Souza", "proprietario_cpf", cpfBom,
		"proprietario_telefone", "(65) 3000-0000", "proprietario_email", "maria@exemplo.com", "aceita_whatsapp", "1",
		"inquilino", "João Lima", "humanos", "4", "cachorros", "1", "gatos", "0"))
	if rec.Code != http.StatusSeeOther || local(rec) != "/locais" {
		t.Fatalf("criar: %d %q %s", rec.Code, local(rec), rec.Body.String())
	}
	ls, _ := m.cliQ.ListLocais(t.Context(), sqlccliente.ListLocaisParams{Limite: 10})
	if len(ls) != 1 {
		t.Fatalf("locais = %d", len(ls))
	}
	id := s64(ls[0].ID)
	l, _ := m.cliQ.GetLocal(t.Context(), ls[0].ID)
	if l.Cep != "78450-000" || !cifra.Cifrado(l.ProprietarioCpf) || cifra.Decifrar(l.ProprietarioCpf) != "52998224725" || strings.Contains(l.ProprietarioCpf, "52998224725") || !l.Latitude.Valid || l.Latitude.Float64 != -13.8247 || !l.AceitaWhatsapp || l.AceitaEmail || !l.CriadoPor.Valid {
		t.Errorf("gravado: %+v", l)
	}
	quer(t, ana.get("/locais"), http.StatusOK, "Rua das Palmeiras, 120", "Centro", "Maria Souza", "seq. 5", "compl. 2", "Residencial", "Ativo")
	quer(t, ana.get("/locais/"+id), http.StatusOK, "Perto da praça", cpfBom, "Maria Souza", "João Lima", `value="78450-000"`, "Salvar local")

	// Abrir a ficha com dados pessoais fica registrado (LGPD).
	if v := achar(m.eventos(t), "dados_pessoais.ver"); v == nil || v.Entidade != "local" || v.EntidadeID != id {
		t.Errorf("dados_pessoais.ver: %+v", v)
	}
	// A auditoria guarda o CPF mascarado, nunca inteiro.
	ev := achar(m.eventos(t), "local.criar")
	if ev == nil || mapa(ev.Depois)["proprietario_cpf"] != "***.982.247-**" || mapa(ev.Depois)["bairro"] != "Centro" {
		t.Fatalf("local.criar: %+v", ev)
	}
	if strings.Contains(string(ev.Depois), "52998224725") {
		t.Error("o CPF completo não pode ir para a auditoria")
	}

	// Duplicado: mesmo endereço, número, lado, sequência e complemento no quarteirão.
	quer(t, ana.post("/locais", formLocal(b1, q1, "sequencia", "5", "complemento", "2")), http.StatusBadRequest, "Já existe um local")
	// Outra sequência é outro local.
	quer(t, ana.post("/locais", formLocal(b1, q1, "sequencia", "6", "complemento", "2")), http.StatusSeeOther)

	// Editar: mudar de bairro/quarteirão e o número; sem mudança não registra.
	quer(t, ana.post("/locais/"+id, camposCompletos(b2, q2, "130")), http.StatusSeeOther)
	ed := achar(m.eventos(t), "local.editar")
	if ed == nil || mapa(ed.Antes)["numero"] != "120" || mapa(ed.Depois)["numero"] != "130" || mapa(ed.Depois)["bairro"] != "Jardim" {
		t.Fatalf("local.editar: %+v", ed)
	}
	if _, ok := mapa(ed.Depois)["proprietario"]; ok {
		t.Error("campo que não mudou não deveria estar no depois")
	}
	quer(t, ana.post("/locais/"+id, camposCompletos(b2, q2, "130")), http.StatusSeeOther)
	n := 0
	for _, e := range m.eventos(t) {
		if e.Acao == "local.editar" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("salvar sem mudança não deve registrar (local.editar = %d)", n)
	}

	// Filtros da lista.
	quer(t, ana.get("/locais?busca=palmeiras"), http.StatusOK, "Rua das Palmeiras")
	quer(t, ana.get("/locais?busca=maria"), http.StatusOK, "Maria Souza")
	quer(t, ana.get("/locais?busca=zzz"), http.StatusOK, "Nenhum local encontrado")
	quer(t, ana.get("/locais?bairro="+s64(b2)), http.StatusOK, "Rua das Palmeiras, 130")
	quer(t, ana.get("/locais?tipo=comercio"), http.StatusOK, "Nenhum local encontrado")
	quer(t, ana.get("/locais?situacao=inativo"), http.StatusOK, "Nenhum local encontrado")
	quer(t, ana.get("/locais/999999"), http.StatusNotFound)
	quer(t, ana.get("/locais/abc"), http.StatusNotFound)
	quer(t, ana.post("/locais/999999", formLocal(b1, q1)), http.StatusNotFound)

	// Quarteirão desativado: não aceita para local novo, mas o local que já estava nele continua aparecendo.
	if _, err := m.cliQ.SetQuarteiraoAtivo(t.Context(), sqlccliente.SetQuarteiraoAtivoParams{ID: q2, BairroID: b2, Ativo: false}); err != nil {
		t.Fatal(err)
	}
	quer(t, ana.post("/locais", formLocal(b2, q2, "numero", "999")), http.StatusBadRequest, "desativado")
	quer(t, ana.get("/locais/"+id), http.StatusOK, "(desativado)")

	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

func TestQuarteiroesDoBairroNoFormularioDoLocal(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	m.criarQuarteirao(t, b, "11")
	desat := m.criarQuarteirao(t, b, "12")
	if _, err := m.cliQ.SetQuarteiraoAtivo(t.Context(), sqlccliente.SetQuarteiraoAtivoParams{ID: desat, BairroID: b, Ativo: false}); err != nil {
		t.Fatal(err)
	}
	rec := ana.get("/locais/quarteiroes?bairro_id=" + s64(b))
	quer(t, rec, http.StatusOK, "Escolha o quarteirão", ">11<")
	if strings.Contains(rec.Body.String(), ">12<") {
		t.Error("quarteirão desativado não deveria ser oferecido")
	}
	quer(t, ana.get("/locais/quarteiroes?bairro_id=abc"), http.StatusOK, "Escolha o quarteirão")
	// O formulário já abre com os quarteirões do bairro pedido.
	quer(t, ana.get("/locais/novo?bairro="+s64(b)), http.StatusOK, ">11<")
}

func TestLocaisRespeitamEscopoDoPapel(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	a, b := m.criarBairro(t, cid, "Bairro A"), m.criarBairro(t, cid, "Bairro B")
	qa, qb := m.criarQuarteirao(t, a, "1"), m.criarQuarteirao(t, b, "1")
	ana := m.comAna(t)
	quer(t, ana.post("/locais", formLocal(a, qa, "endereco", "Rua Alfa")), http.StatusSeeOther)
	quer(t, ana.post("/locais", formLocal(b, qb, "endereco", "Rua Beta")), http.StatusSeeOther)
	quer(t, ana.post("/locais", formLocal(a, qa, "endereco", "Praça Central", "tipo", "ponto_estrategico")), http.StatusSeeOther)
	ls, _ := m.cliQ.ListLocais(t.Context(), sqlccliente.ListLocaisParams{Limite: 10})
	var idAlfa, idBeta, idPE string
	for _, l := range ls {
		switch l.Endereco {
		case "Rua Alfa":
			idAlfa = s64(l.ID)
		case "Rua Beta":
			idBeta = s64(l.ID)
		case "Praça Central":
			idPE = s64(l.ID)
		}
	}

	// "Apenas atribuídos": a Dora só enxerga o Bairro A.
	pa, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Campo"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pa.ID, "local_visualizar", "local_adicionar", "local_meubairro"))
	dora := m.novoUsuarioLogado(t, "Dora Reis", "dora@nm.gov.br", pa.ID)
	did, _ := strconv.ParseInt(m.idUsuario(t, "dora@nm.gov.br"), 10, 64)
	_ = m.cliQ.AddResponsaveisAoBairro(t.Context(), addRespParams(a, did))
	rec := dora.get("/locais")
	quer(t, rec, http.StatusOK, "Rua Alfa", "apenas parte")
	if strings.Contains(rec.Body.String(), "Rua Beta") {
		t.Error("a Dora não deveria ver o local do Bairro B")
	}
	quer(t, dora.get("/locais/"+idAlfa), http.StatusOK)
	quer(t, dora.get("/locais/"+idBeta), http.StatusNotFound)
	quer(t, dora.post("/locais/"+idBeta, formLocal(b, qb, "endereco", "Rua Beta", "numero", "1")), http.StatusNotFound)
	quer(t, dora.post("/locais", formLocal(b, qb, "numero", "55")), http.StatusBadRequest, "em que você é responsável")
	quer(t, dora.get("/locais/quarteiroes?bairro_id="+s64(b)), http.StatusNotFound)
	quer(t, dora.get("/locais/quarteiroes?bairro_id="+s64(a)), http.StatusOK)
	quer(t, dora.post("/locais", formLocal(a, qa, "numero", "77")), http.StatusSeeOther)
	if strings.Contains(dora.get("/locais/novo").Body.String(), "Bairro B") {
		t.Error("o formulário da Dora não deveria oferecer o Bairro B")
	}

	// "Apenas ponto estratégico": só vê (e só cadastra) pontos estratégicos.
	pp, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Pontos"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(pp.ID, "local_visualizar", "local_adicionar", "local_pe"))
	pe := m.novoUsuarioLogado(t, "Paulo Dias", "paulo@nm.gov.br", pp.ID)
	rec = pe.get("/locais")
	quer(t, rec, http.StatusOK, "Praça Central")
	if strings.Contains(rec.Body.String(), "Rua Alfa") || strings.Contains(rec.Body.String(), "Rua Beta") {
		t.Error("quem só vê ponto estratégico não deveria ver os outros tipos")
	}
	quer(t, pe.get("/locais/"+idPE), http.StatusOK)
	quer(t, pe.get("/locais/"+idAlfa), http.StatusNotFound)
	quer(t, pe.post("/locais", formLocal(a, qa, "numero", "88")), http.StatusBadRequest, "só cadastra pontos estratégicos")
	quer(t, pe.post("/locais", formLocal(a, qa, "numero", "89", "tipo", "ponto_estrategico")), http.StatusSeeOther)
}

func TestPermissoesDosLocais(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	q := m.criarQuarteirao(t, b, "1")

	carla := m.comCarla(t) // sem nenhuma permissão
	for _, p := range []string{"/locais", "/locais/novo", "/locais/quarteiroes?bairro_id=" + s64(b)} {
		quer(t, carla.get(p), http.StatusForbidden)
	}
	quer(t, carla.post("/locais", formLocal(b, q)), http.StatusForbidden)
	if strings.Contains(carla.get("/").Body.String(), `href="/locais"`) {
		t.Error("o menu da Carla não deveria ter Locais")
	}

	// Só consulta: vê, mas não edita.
	papel, _ := m.cliQ.CreatePapel(t.Context(), sqlccliente.CreatePapelParams{Nome: "Consulta de locais"})
	_ = m.cliQ.AddPermissoesAoPapel(t.Context(), permsParams(papel.ID, "local_visualizar"))
	edu := m.novoUsuarioLogado(t, "Edu Lima", "edu@nm.gov.br", papel.ID)
	quer(t, edu.get("/locais"), http.StatusOK)
	if strings.Contains(edu.get("/locais").Body.String(), "Novo local") {
		t.Error("quem só consulta não deveria ver o botão Novo local")
	}
	quer(t, edu.get("/locais/novo"), http.StatusForbidden)
	quer(t, edu.post("/locais", formLocal(b, q)), http.StatusForbidden)

	// O menu do administrador mostra Locais.
	quer(t, m.comAna(t).get("/"), http.StatusOK, `href="/locais"`)
}

func TestCPFValido(t *testing.T) {
	for cpf, ok := range map[string]bool{"52998224725": true, "11144477735": true, "11111111111": false, "52998224724": false, "123": false, "": false} {
		if cpfValido(cpf) != ok {
			t.Errorf("cpfValido(%q) = %v", cpf, !ok)
		}
	}
	if mascararCPF("52998224725") != "***.982.247-**" || formatarCPF("52998224725") != cpfBom {
		t.Error("máscara ou formatação do CPF")
	}
}

func TestListaDeLocaisFiltros(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b1, b2 := m.criarBairro(t, cid, "Centro"), m.criarBairro(t, cid, "Jardim")
	q1, q2 := m.criarQuarteirao(t, b1, "1"), m.criarQuarteirao(t, b2, "7")
	quer(t, ana.post("/locais", formLocal(b1, q1, "endereco", "Rua das Palmeiras", "numero", "10", "lado", "Par", "sequencia", "3")), http.StatusSeeOther)
	quer(t, ana.post("/locais", formLocal(b2, q2, "endereco", "Avenida Brasil", "numero", "20", "lado", "Impar", "tipo", "comercio", "referencia", "Perto do posto")), http.StatusSeeOther)

	quer(t, ana.get("/locais"), http.StatusOK, "Rua das Palmeiras", "Avenida Brasil", "Visualizar Grupos", "filtros-vivos")
	quer(t, ana.get("/locais?endereco=palmeiras"), http.StatusOK, "Rua das Palmeiras")
	nao(t, ana.get("/locais?endereco=palmeiras"), "Avenida Brasil")
	nao(t, ana.get("/locais?numero=20"), "Rua das Palmeiras")
	quer(t, ana.get("/locais?referencia=posto"), http.StatusOK, "Avenida Brasil")
	nao(t, ana.get("/locais?referencia=posto"), "Rua das Palmeiras")
	nao(t, ana.get("/locais?quarteirao="+s64(q1)), "Avenida Brasil")
	nao(t, ana.get("/locais?lado=impar"), "Rua das Palmeiras")
	nao(t, ana.get("/locais?sequencia=3"), "Avenida Brasil")
	nao(t, ana.get("/locais?tipo=comercio"), "Rua das Palmeiras")
	quer(t, ana.get("/locais/quarteiroes?filtro=1&bairro="+s64(b1)), http.StatusOK, "Todos os quarteirões", ">1<")
}

// nao confere que a resposta não traz o texto (útil para provar que um filtro escondeu o registro).
func nao(t *testing.T, rec *httptest.ResponseRecorder, texto string) {
	t.Helper()
	if strings.Contains(rec.Body.String(), texto) {
		t.Errorf("a resposta não deveria conter %q", texto)
	}
}

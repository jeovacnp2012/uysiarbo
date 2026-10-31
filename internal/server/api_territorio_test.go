package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"uysiarbo/internal/cifra"
	"uysiarbo/internal/db/sqlccliente"
)

// cliAPI facilita chamadas à API de um cliente com um token já obtido.
type cliAPI struct {
	t   *testing.T
	m   *mundo
	tok string
}

func (c cliAPI) req(metodo, caminho, corpo string, cab ...string) *httptest.ResponseRecorder {
	h := map[string]string{}
	for i := 0; i+1 < len(cab); i += 2 {
		h[cab[i]] = cab[i+1]
	}
	return c.m.chamar(hostNM, metodo, caminho, c.tok, corpo, h)
}

func (c cliAPI) json(rec *httptest.ResponseRecorder, destino any) {
	c.t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), destino); err != nil {
		c.t.Fatalf("JSON inválido: %v\n%s", err, rec.Body.String())
	}
}

func (c cliAPI) id(rec *httptest.ResponseRecorder) int64 {
	c.t.Helper()
	var x struct {
		ID int64 `json:"id"`
	}
	c.json(rec, &x)
	return x.ID
}

func (c cliAPI) lista(caminho string) (ids []int64, proximo string) {
	c.t.Helper()
	rec := c.req(http.MethodGet, caminho, "")
	if rec.Code != http.StatusOK {
		c.t.Fatalf("%s: %d %s", caminho, rec.Code, rec.Body.String())
	}
	var l struct {
		Data       []struct{ ID int64 } `json:"data"`
		NextCursor *string              `json:"next_cursor"`
	}
	c.json(rec, &l)
	for _, d := range l.Data {
		ids = append(ids, d.ID)
	}
	if l.NextCursor != nil {
		proximo = *l.NextCursor
	}
	return
}

func novaAPI(t *testing.T, m *mundo, escopos ...string) cliAPI {
	t.Helper()
	if len(escopos) == 0 {
		escopos = []string{"cidades:read", "cidades:write", "bairros:read", "bairros:write", "quarteiroes:read", "quarteiroes:write", "locais:read", "locais:write"}
	}
	return cliAPI{t: t, m: m, tok: m.token(t, m.novaCredencial(t, "Tablet "+strconv.Itoa(len(escopos)), escopos...), "")}
}

func TestAPIDeBairros(t *testing.T) {
	m := novoMundo(t)
	api := novaAPI(t, m)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	cidS := s64(cid)

	rec := api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"Centro","id_externo":"tab-b-1","latitude":-13.82,"longitude":-56.08,"mutirao":true}`)
	if rec.Code != http.StatusCreated || !strings.HasPrefix(rec.Header().Get("Location"), "/api/v1/bairros/") {
		t.Fatalf("criar: %d %s", rec.Code, rec.Body.String())
	}
	var b apiBairro
	api.json(rec, &b)
	if b.Nome != "Centro" || b.Categoria != "bairro" || b.Classificacao != "urbano" || b.Status != "ativo" || !b.Producao || !b.Mutirao || b.IDExterno != "tab-b-1" || b.Latitude.(float64) != -13.82 {
		t.Errorf("padrões e campos: %+v", b)
	}

	// Repetir o envio com o mesmo id_externo (tablet sem sinal) devolve o mesmo bairro, sem duplicar.
	rep := api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"Centro","id_externo":"tab-b-1"}`)
	if rep.Code != http.StatusOK || api.id(rep) != b.ID {
		t.Errorf("repetição por id_externo: %d %s", rep.Code, rep.Body.String())
	}
	if bs, _ := m.cliQ.ListBairrosApi(t.Context(), listaTodosBairros()); len(bs) != 1 {
		t.Errorf("bairros = %d, queria 1", len(bs))
	}

	// Recusas.
	quemAPI(t, api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"Centro"}`), http.StatusConflict, "conflict")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"Outro","id_externo":"tab-b-1"}`), http.StatusOK, "") // id_externo já existe: devolve o existente
	quemAPI(t, api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":999,"nome":"Novo"}`), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"N"}`), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"Novo","categoria":"cidade"}`), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"Novo","latitude":-13.8}`), http.StatusUnprocessableEntity, "validation_error") // latitude sem longitude
	quemAPI(t, api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"Novo","segredo":1}`), http.StatusBadRequest, "invalid_json")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"Novo","mutirao":"sim"}`), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/bairros", `[1]`), http.StatusBadRequest, "invalid_json")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"Novo","id_externo":""}`), http.StatusUnprocessableEntity, "validation_error")

	// Alterar: só o que veio; null limpa; o id_externo não muda depois de definido.
	url1 := "/api/v1/bairros/" + s64(b.ID)
	rec = api.req(http.MethodPatch, url1, `{"zona":"Norte","status":"extinto","latitude":null,"longitude":null}`)
	api.json(rec, &b)
	if rec.Code != http.StatusOK || b.Zona != "Norte" || b.Status != "extinto" || b.Latitude != nil || b.Longitude != nil || b.Nome != "Centro" || !b.Mutirao {
		t.Fatalf("alterar: %d %+v", rec.Code, b)
	}
	quemAPI(t, api.req(http.MethodPatch, url1, `{"id_externo":"outro"}`), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPatch, url1, `{"id_externo":"tab-b-1","zona":"Sul"}`), http.StatusOK, "")
	quemAPI(t, api.req(http.MethodPatch, "/api/v1/bairros/999", `{"zona":"x"}`), http.StatusNotFound, "not_found")
	quemAPI(t, api.req(http.MethodGet, "/api/v1/bairros/abc", ""), http.StatusNotFound, "not_found")

	// Filtros da lista.
	segundo := api.id(api.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+cidS+`,"nome":"Jardim das Flores","categoria":"povoado"}`))
	if ids, _ := api.lista("/api/v1/bairros"); len(ids) != 2 {
		t.Errorf("todos: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/bairros?status=extinto"); len(ids) != 1 || ids[0] != b.ID {
		t.Errorf("status: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/bairros?q=flores"); len(ids) != 1 || ids[0] != segundo {
		t.Errorf("busca: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/bairros?id_externo=tab-b-1"); len(ids) != 1 || ids[0] != b.ID {
		t.Errorf("id_externo: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/bairros?cidade_id=" + cidS); len(ids) != 2 {
		t.Errorf("cidade: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/bairros?cidade_id=999"); len(ids) != 0 {
		t.Errorf("outra cidade: %v", ids)
	}
	futuro := url.QueryEscape(time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	if ids, _ := api.lista("/api/v1/bairros?updated_since=" + futuro); len(ids) != 0 {
		t.Errorf("updated_since no futuro: %v", ids)
	}
	passado := url.QueryEscape(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
	if ids, _ := api.lista("/api/v1/bairros?updated_since=" + passado); len(ids) != 2 {
		t.Errorf("updated_since no passado: %v", ids)
	}
	for _, ruim := range []string{"cidade_id=abc", "updated_since=ontem", "cursor=x"} {
		quemAPI(t, api.req(http.MethodGet, "/api/v1/bairros?"+ruim, ""), http.StatusBadRequest, "")
	}
	// Paginação.
	ids, prox := api.lista("/api/v1/bairros?limit=1")
	if len(ids) != 1 || prox == "" {
		t.Fatalf("página 1: %v %q", ids, prox)
	}
	if ids2, prox2 := api.lista("/api/v1/bairros?limit=1&cursor=" + url.QueryEscape(prox)); len(ids2) != 1 || ids2[0] == ids[0] || prox2 != "" {
		t.Errorf("página 2: %v %q", ids2, prox2)
	}

	// Auditoria: o autor é a credencial.
	var criou, editou bool
	for _, e := range m.eventos(t) {
		if e.Acao == "bairro.criar" && e.Rotulo == "Centro" && e.AtorTipo == "api" && mapa(e.Detalhes)["id_externo"] == "tab-b-1" {
			criou = true
		}
		if e.Acao == "bairro.editar" && e.AtorTipo == "api" && mapa(e.Antes)["status"] == "ativo" && mapa(e.Depois)["status"] == "extinto" {
			editou = true
		}
	}
	if !criou || !editou {
		t.Errorf("auditoria do bairro: criou=%v editou=%v", criou, editou)
	}
	// A tela enxerga o que veio pela API.
	quer(t, m.comAna(t).get("/bairros"), http.StatusOK, "Centro", "Jardim das Flores")
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

func TestAPIDeQuarteiroes(t *testing.T) {
	m := novoMundo(t)
	api := novaAPI(t, m)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	a, b := m.criarBairro(t, cid, "Bairro A"), m.criarBairro(t, cid, "Bairro B")

	rec := api.req(http.MethodPost, "/api/v1/quarteiroes", `{"bairro_id":`+s64(a)+`,"nome":"12","id_externo":"tab-q-12"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar: %d %s", rec.Code, rec.Body.String())
	}
	var q apiQuarteirao
	api.json(rec, &q)
	if q.BairroID != a || q.Nome != "12" || !q.Ativo || q.IDExterno != "tab-q-12" {
		t.Errorf("quarteirão: %+v", q)
	}
	if rep := api.req(http.MethodPost, "/api/v1/quarteiroes", `{"bairro_id":`+s64(a)+`,"nome":"12","id_externo":"tab-q-12"}`); rep.Code != http.StatusOK || api.id(rep) != q.ID {
		t.Errorf("repetição: %d", rep.Code)
	}
	quemAPI(t, api.req(http.MethodPost, "/api/v1/quarteiroes", `{"bairro_id":`+s64(a)+`,"nome":"12"}`), http.StatusConflict, "conflict")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/quarteiroes", `{"bairro_id":`+s64(b)+`,"nome":"12"}`), http.StatusCreated, "") // o mesmo nome em outro bairro vale
	quemAPI(t, api.req(http.MethodPost, "/api/v1/quarteiroes", `{"bairro_id":999,"nome":"1"}`), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/quarteiroes", `{"bairro_id":`+s64(a)+`,"nome":""}`), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/quarteiroes", `{"bairro_id":`+s64(a)+`,"nome":"`+strings.Repeat("x", 31)+`"}`), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/quarteiroes", `{"nome":"1"}`), http.StatusUnprocessableEntity, "validation_error")

	// Alterar: desativar muda atualizado_em e entra no updated_since; o bairro não troca.
	time.Sleep(1100 * time.Millisecond)
	antes := time.Now().UTC().Format(time.RFC3339) // arredonda para baixo, mas fica depois da criação (passou mais de 1 s)
	time.Sleep(1100 * time.Millisecond)
	rec = api.req(http.MethodPatch, "/api/v1/quarteiroes/"+s64(q.ID), `{"ativo":false}`)
	var q2 apiQuarteirao
	api.json(rec, &q2)
	if rec.Code != http.StatusOK || q2.Ativo || !q2.AtualizadoEm.After(q.AtualizadoEm) {
		t.Fatalf("desativar: %d %+v", rec.Code, q2)
	}
	quemAPI(t, api.req(http.MethodPatch, "/api/v1/quarteiroes/"+s64(q.ID), `{"bairro_id":`+s64(b)+`}`), http.StatusBadRequest, "invalid_json")
	if ids, _ := api.lista("/api/v1/quarteiroes?updated_since=" + url.QueryEscape(antes)); len(ids) != 1 || ids[0] != q.ID {
		t.Errorf("updated_since: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/quarteiroes?ativo=false"); len(ids) != 1 {
		t.Errorf("inativos: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/quarteiroes?ativo=true"); len(ids) != 1 {
		t.Errorf("ativos: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/quarteiroes?bairro_id=" + s64(a)); len(ids) != 1 {
		t.Errorf("por bairro: %v", ids)
	}
	quemAPI(t, api.req(http.MethodGet, "/api/v1/quarteiroes?ativo=talvez", ""), http.StatusBadRequest, "invalid_parameter")
	if e := achar(m.eventos(t), "quarteirao.editar"); e == nil || e.AtorTipo != "api" || mapa(e.Depois)["ativo"] != false {
		t.Errorf("quarteirao.editar: %+v", e)
	}
}

func TestAPIDeLocais(t *testing.T) {
	m := novoMundo(t)
	api := novaAPI(t, m)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	a, b := m.criarBairro(t, cid, "Bairro A"), m.criarBairro(t, cid, "Bairro B")
	qa, qb := m.criarQuarteirao(t, a, "1"), m.criarQuarteirao(t, b, "1")
	novo := func(extras string) string {
		return `{"tipo":"residencial","endereco":"Rua das Palmeiras","numero":"120","lado":"Par","bairro_id":` + s64(a) + `,"quarteirao_id":` + s64(qa) + extras + `}`
	}

	rec := api.req(http.MethodPost, "/api/v1/locais", novo(`,"id_externo":"tab-l-1","latitude":-13.82,"longitude":-56.08,"sequencia":5,"cep":"78450000","humanos":3,"proprietario":"Maria Souza","aceita_email":true`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar: %d %s", rec.Code, rec.Body.String())
	}
	var l apiLocal
	api.json(rec, &l)
	if l.Tipo != "residencial" || l.CEP != "78450-000" || l.Sequencia.(float64) != 5 || l.Humanos.(float64) != 3 || !l.AceitaEmail || !l.Ativo || len(l.Chave) != 20 || l.IDExterno != "tab-l-1" {
		t.Errorf("local: %+v", l)
	}
	if strings.Contains(rec.Body.String(), "cpf") {
		t.Error("a API não devolve CPF")
	}
	if rep := api.req(http.MethodPost, "/api/v1/locais", novo(`,"id_externo":"tab-l-1"`)); rep.Code != http.StatusOK || api.id(rep) != l.ID {
		t.Errorf("repetição por id_externo: %d", rep.Code)
	}
	if n := contarLocais(t, m); n != 1 {
		t.Errorf("locais = %d, queria 1", n)
	}

	// Recusas (as mesmas regras das telas).
	quemAPI(t, api.req(http.MethodPost, "/api/v1/locais", novo(`,"sequencia":5`)), http.StatusConflict, "conflict")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/locais", strings.Replace(novo(""), `"tipo":"residencial",`, "", 1)), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/locais", strings.Replace(novo(`,"numero":"1"`), `"numero":"120",`, "", 1)), http.StatusCreated, "") // o último campo repetido vence
	quemAPI(t, api.req(http.MethodPost, "/api/v1/locais", novo(`,"cep":"123"`)), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/locais", novo(`,"latitude":-13.8`)), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/locais", novo(`,"humanos":-1`)), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/locais", novo(`,"proprietario_cpf":"52998224725"`)), http.StatusBadRequest, "invalid_json") // CPF não entra pela API
	quemAPI(t, api.req(http.MethodPost, "/api/v1/locais", strings.Replace(novo(`,"numero":"7"`), `"quarteirao_id":`+s64(qa), `"quarteirao_id":`+s64(qb), 1)), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPost, "/api/v1/locais", novo(`,"numero":"8","aceita_email":"sim"`)), http.StatusUnprocessableEntity, "validation_error")

	// Alterar parcialmente; null limpa; desativar entra na auditoria; CPF cadastrado na tela é preservado.
	quer(t, m.comAna(t).post("/locais", formLocal(a, qa, "numero", "300", "proprietario_cpf", "529.982.247-25")), http.StatusSeeOther)
	ls, _ := m.cliQ.ListLocaisApi(t.Context(), listaTodosLocais())
	var comCPF int64
	for _, x := range ls {
		if x.Numero == "300" {
			comCPF = x.ID
		}
	}
	rec = api.req(http.MethodPatch, "/api/v1/locais/"+s64(comCPF), `{"proprietario":"João","humanos":null,"referencia":"Perto da praça"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("alterar: %d %s", rec.Code, rec.Body.String())
	}
	var l2 apiLocal
	api.json(rec, &l2)
	if l2.Proprietario != "João" || l2.Humanos != nil || l2.Referencia != "Perto da praça" || l2.Numero != "300" {
		t.Errorf("alterado: %+v", l2)
	}
	banco, _ := m.cliQ.GetLocal(t.Context(), comCPF)
	if cifra.Decifrar(banco.ProprietarioCpf) != "52998224725" {
		t.Errorf("o CPF não pode se perder numa alteração pela API: %q", banco.ProprietarioCpf)
	}
	quemAPI(t, api.req(http.MethodPatch, "/api/v1/locais/"+s64(comCPF), `{"ativo":false}`), http.StatusOK, "")
	if e := achar(m.eventos(t), "local.desativar"); e == nil || e.AtorTipo != "api" {
		t.Errorf("local.desativar: %+v", e)
	}
	if e := achar(m.eventos(t), "local.editar"); e == nil || e.AtorTipo != "api" || mapa(e.Antes)["proprietario"] == nil {
		t.Errorf("local.editar: %+v", e)
	}
	// Mudar o id_externo de quem já tem é recusado; passar o mesmo é aceito.
	quemAPI(t, api.req(http.MethodPatch, "/api/v1/locais/"+s64(l.ID), `{"id_externo":"outro"}`), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api.req(http.MethodPatch, "/api/v1/locais/"+s64(l.ID), `{"id_externo":"tab-l-1"}`), http.StatusOK, "")
	// Mover para outro bairro e quarteirão.
	rec = api.req(http.MethodPatch, "/api/v1/locais/"+s64(l.ID), `{"bairro_id":`+s64(b)+`,"quarteirao_id":`+s64(qb)+`}`)
	api.json(rec, &l2)
	if rec.Code != http.StatusOK || l2.BairroID != b || l2.QuarteiraoID != qb {
		t.Errorf("mover: %d %+v", rec.Code, l2)
	}
	quemAPI(t, api.req(http.MethodPatch, "/api/v1/locais/"+s64(l.ID), `{"quarteirao_id":`+s64(qa)+`}`), http.StatusUnprocessableEntity, "validation_error") // quarteirão de outro bairro
	quemAPI(t, api.req(http.MethodPatch, "/api/v1/locais/999", `{"numero":"1"}`), http.StatusNotFound, "not_found")

	// Listas e filtros (inclui o que foi desativado: a sincronização precisa saber).
	todos, _ := api.lista("/api/v1/locais")
	if len(todos) != 3 {
		t.Fatalf("todos = %v", todos)
	}
	if ids, _ := api.lista("/api/v1/locais?ativo=false"); len(ids) != 1 || ids[0] != comCPF {
		t.Errorf("inativos: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/locais?bairro_id=" + s64(b)); len(ids) != 1 || ids[0] != l.ID {
		t.Errorf("por bairro: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/locais?quarteirao_id=" + s64(qa)); len(ids) != 2 {
		t.Errorf("por quarteirão: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/locais?q=300"); len(ids) != 1 || ids[0] != comCPF {
		t.Errorf("busca: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/locais?id_externo=tab-l-1"); len(ids) != 1 || ids[0] != l.ID {
		t.Errorf("id_externo: %v", ids)
	}
	if ids, _ := api.lista("/api/v1/locais?tipo=comercio"); len(ids) != 0 {
		t.Errorf("tipo: %v", ids)
	}
	quemAPI(t, api.req(http.MethodGet, "/api/v1/locais?tipo=casa", ""), http.StatusBadRequest, "invalid_parameter")
	quemAPI(t, api.req(http.MethodGet, "/api/v1/locais?quarteirao_id=0", ""), http.StatusBadRequest, "invalid_parameter")
	// Sincronização em páginas pequenas percorre tudo sem repetir.
	var vistos []int64
	cursor := ""
	for i := 0; i < 5; i++ {
		caminho := "/api/v1/locais?limit=2"
		if cursor != "" {
			caminho += "&cursor=" + url.QueryEscape(cursor)
		}
		ids, prox := api.lista(caminho)
		vistos = append(vistos, ids...)
		if prox == "" {
			break
		}
		cursor = prox
	}
	if len(vistos) != 3 || vistos[0] == vistos[1] || vistos[1] == vistos[2] {
		t.Errorf("paginação: %v", vistos)
	}
	// O local da API aparece na tela, com o QR Code.
	quer(t, m.comAna(t).get("/locais/"+s64(l.ID)), http.StatusOK, "Rua das Palmeiras", l.Chave)
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

func TestEscoposDaAPIDoTerritorio(t *testing.T) {
	m := novoMundo(t)
	leitura := novaAPI(t, m, "bairros:read", "quarteiroes:read", "locais:read")
	soBairros := novaAPI(t, m, "bairros:read", "bairros:write")
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	corpo := `{"cidade_id":` + s64(cid) + `,"nome":"X Bairro"}`

	for _, c := range []struct{ metodo, caminho string }{
		{http.MethodPost, "/api/v1/bairros"}, {http.MethodPost, "/api/v1/quarteiroes"}, {http.MethodPost, "/api/v1/locais"},
		{http.MethodPatch, "/api/v1/bairros/1"}, {http.MethodPatch, "/api/v1/quarteiroes/1"}, {http.MethodPatch, "/api/v1/locais/1"},
	} {
		quemAPI(t, leitura.req(c.metodo, c.caminho, corpo), http.StatusForbidden, "insufficient_scope")
	}
	for _, caminho := range []string{"/api/v1/cidades", "/api/v1/cidades/1"} {
		quemAPI(t, leitura.req(http.MethodGet, caminho, ""), http.StatusForbidden, "insufficient_scope")
	}
	for _, caminho := range []string{"/api/v1/quarteiroes", "/api/v1/locais"} {
		quemAPI(t, soBairros.req(http.MethodGet, caminho, ""), http.StatusForbidden, "insufficient_scope")
	}
	quemAPI(t, soBairros.req(http.MethodPost, "/api/v1/bairros", corpo), http.StatusCreated, "")
	quemAPI(t, leitura.req(http.MethodGet, "/api/v1/bairros", ""), http.StatusOK, "")
	// Idempotency-Key também vale nas gravações do território.
	k := []string{"Idempotency-Key", "k-1"}
	b1 := soBairros.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+s64(cid)+`,"nome":"Y Bairro"}`, k...)
	b2 := soBairros.req(http.MethodPost, "/api/v1/bairros", `{"cidade_id":`+s64(cid)+`,"nome":"Y Bairro"}`, k...)
	if b1.Code != http.StatusCreated || b2.Code != http.StatusCreated || b2.Header().Get("Idempotent-Replayed") != "true" || b1.Body.String() != b2.Body.String() {
		t.Errorf("idempotência: %d %d %q", b1.Code, b2.Code, b2.Header().Get("Idempotent-Replayed"))
	}
}

func listaTodosBairros() sqlccliente.ListBairrosApiParams {
	return sqlccliente.ListBairrosApiParams{Limite: 1000}
}

func listaTodosLocais() sqlccliente.ListLocaisApiParams {
	return sqlccliente.ListLocaisApiParams{Limite: 1000}
}

package server

import (
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/apiauth"
	"uysiarbo/internal/db/sqlccliente"
)

// credTeste é uma credencial de API criada direto no banco, com o segredo em mãos.
type credTeste struct {
	ID       int64
	ClientID string
	Segredo  string
}

func (m *mundo) novaCredencial(t *testing.T, nome string, escopos ...string) credTeste {
	t.Helper()
	id, segredo, hash, err := apiauth.NovaCredencial()
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.cliQ.CreateApiCredencial(t.Context(), sqlccliente.CreateApiCredencialParams{
		Nome: nome, ClientID: id, SegredoHash: hash, Escopos: escopos, LimitePorMinuto: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	return credTeste{ID: c.ID, ClientID: id, Segredo: segredo}
}

// chamar faz uma chamada à API (sem cookies, como um sistema externo).
func (m *mundo) chamar(host, metodo, caminho, token string, corpo string, cab map[string]string) *httptest.ResponseRecorder {
	var body io.Reader
	if corpo != "" {
		body = strings.NewReader(corpo)
	}
	req := httptest.NewRequest(metodo, caminho, body)
	req.Host = host
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if corpo != "" && req.Header.Get("Content-Type") == "" {
		if metodo == http.MethodPost && caminho == "/api/v1/token" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	for k, v := range cab {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	m.h.ServeHTTP(rec, req)
	return rec
}

func (m *mundo) token(t *testing.T, c credTeste, escopo string) string {
	t.Helper()
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {c.ClientID}, "client_secret": {c.Segredo}}
	if escopo != "" {
		form.Set("scope", escopo)
	}
	rec := m.chamar(hostNM, http.MethodPost, "/api/v1/token", "", form.Encode(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("token: %d %s", rec.Code, rec.Body.String())
	}
	var r struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	return r.AccessToken
}

func codigoDeErro(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("erro não é JSON: %v\n%s", err, rec.Body.String())
	}
	return e.Error.Code
}

func quemAPI(t *testing.T, rec *httptest.ResponseRecorder, status int, codigo string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, quer %d: %s", rec.Code, status, rec.Body.String())
	}
	if codigo != "" && codigoDeErro(t, rec) != codigo {
		t.Fatalf("code = %q, quer %q", codigoDeErro(t, rec), codigo)
	}
}

func TestEmissaoDeToken(t *testing.T) {
	m := novoMundo(t)
	c := m.novaCredencial(t, "Sistema A", "cidades:read", "cidades:write")
	form := func(extras ...string) string {
		v := url.Values{"grant_type": {"client_credentials"}, "client_id": {c.ClientID}, "client_secret": {c.Segredo}}
		for i := 0; i+1 < len(extras); i += 2 {
			if extras[i+1] == "-" {
				v.Del(extras[i])
			} else {
				v.Set(extras[i], extras[i+1])
			}
		}
		return v.Encode()
	}
	pedir := func(corpo string) *httptest.ResponseRecorder {
		return m.chamar(hostNM, http.MethodPost, "/api/v1/token", "", corpo, nil)
	}

	rec := pedir(form())
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("token: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	var r struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if r.TokenType != "Bearer" || r.ExpiresIn != 900 || r.Scope != "cidades:read cidades:write" {
		t.Errorf("resposta: %+v", r)
	}
	claims, err := apiauth.Verificar(r.AccessToken, chaveDaAPITeste, time.Now())
	if err != nil || claims.Cliente != "novamutum" || claims.Credencial != c.ID || claims.Versao != 1 {
		t.Errorf("claims: %+v %v", claims, err)
	}

	// HTTP Basic e subconjunto de escopos.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/token", strings.NewReader("grant_type=client_credentials&scope=cidades:read"))
	req.Host = hostNM
	req.SetBasicAuth(c.ClientID, c.Segredo)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rb := httptest.NewRecorder()
	m.h.ServeHTTP(rb, req)
	_ = json.Unmarshal(rb.Body.Bytes(), &r)
	if rb.Code != http.StatusOK || r.Scope != "cidades:read" {
		t.Errorf("basic: %d %+v", rb.Code, r)
	}

	// Recusas.
	quemAPI(t, pedir(form("grant_type", "password")), http.StatusBadRequest, "unsupported_grant_type")
	quemAPI(t, pedir(form("client_secret", "-")), http.StatusUnauthorized, "invalid_client")
	quemAPI(t, pedir(form("client_secret", "uas_errado")), http.StatusUnauthorized, "invalid_client")
	quemAPI(t, pedir(form("client_id", "uac_naoexiste")), http.StatusUnauthorized, "invalid_client")
	quemAPI(t, pedir(form("scope", "locais:read")), http.StatusBadRequest, "invalid_scope")
	quemAPI(t, pedir(form("scope", "x:read")), http.StatusBadRequest, "invalid_scope")
	quemAPI(t, m.chamar(hostConsole, http.MethodPost, "/api/v1/token", "", form(), nil), http.StatusNotFound, "not_found")
	// o segredo de um cliente não funciona em outro cliente (cada um tem o seu banco)
	quemAPI(t, m.chamar(hostSinop, http.MethodPost, "/api/v1/token", "", form(), nil), http.StatusUnauthorized, "invalid_client")

	// Desativada ou vencida.
	if _, err := m.cliQ.SetApiCredencialAtivo(t.Context(), sqlccliente.SetApiCredencialAtivoParams{ID: c.ID, Ativo: false}); err != nil {
		t.Fatal(err)
	}
	quemAPI(t, pedir(form()), http.StatusUnauthorized, "invalid_client")
	_, _ = m.cliQ.SetApiCredencialAtivo(t.Context(), sqlccliente.SetApiCredencialAtivoParams{ID: c.ID, Ativo: true})
	cred, _ := m.cliQ.GetApiCredencial(t.Context(), c.ID)
	_, _ = m.cliQ.UpdateApiCredencial(t.Context(), sqlccliente.UpdateApiCredencialParams{
		ID: c.ID, Nome: cred.Nome, Escopos: cred.Escopos, LimitePorMinuto: 120,
		ValidaAte: pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
	})
	quemAPI(t, pedir(form()), http.StatusUnauthorized, "invalid_client")
}

func TestAutenticacaoDasChamadas(t *testing.T) {
	m := novoMundo(t)
	leitura := m.novaCredencial(t, "Leitura", "cidades:read")
	tudo := m.novaCredencial(t, "Tudo", "cidades:read", "cidades:write")
	listar := func(tok string) *httptest.ResponseRecorder {
		return m.chamar(hostNM, http.MethodGet, "/api/v1/cidades", tok, "", nil)
	}

	// Sem token, token ruim, cookie de sessão não vale.
	rec := listar("")
	quemAPI(t, rec, http.StatusUnauthorized, "invalid_token")
	if !strings.Contains(rec.Header().Get("WWW-Authenticate"), "Bearer") {
		t.Error("faltou WWW-Authenticate")
	}
	quemAPI(t, listar("lixo"), http.StatusUnauthorized, "invalid_token")
	ana := m.comAna(t)
	quemAPI(t, ana.enviar(http.MethodGet, "/api/v1/cidades", nil, nil), http.StatusUnauthorized, "invalid_token")

	// Token válido; escopo insuficiente para gravar.
	tl := m.token(t, leitura, "")
	quemAPI(t, listar(tl), http.StatusOK, "")
	corpo := `{"nome":"Nova Mutum","uf":"MT"}`
	quemAPI(t, m.chamar(hostNM, http.MethodPost, "/api/v1/cidades", tl, corpo, nil), http.StatusForbidden, "insufficient_scope")
	// Token pedido só com leitura não grava mesmo que a credencial possa.
	tr := m.token(t, tudo, "cidades:read")
	quemAPI(t, m.chamar(hostNM, http.MethodPost, "/api/v1/cidades", tr, corpo, nil), http.StatusForbidden, "insufficient_scope")
	// Gravar não dá direito de ler.
	soGravar := m.novaCredencial(t, "Só grava", "cidades:write")
	quemAPI(t, listar(m.token(t, soGravar, "")), http.StatusForbidden, "insufficient_scope")

	// Token vencido, de outro cliente, de credencial desativada e de segredo trocado.
	agora := time.Now()
	vencido, _ := apiauth.Assinar(apiauth.Claims{Credencial: leitura.ID, Cliente: "novamutum", Versao: 1, Escopos: []string{"cidades:read"}, Emitido: agora.Add(-time.Hour).Unix(), Expira: agora.Add(-time.Minute).Unix()}, chaveDaAPITeste)
	rec = listar(vencido)
	quemAPI(t, rec, http.StatusUnauthorized, "invalid_token")
	if !strings.Contains(rec.Body.String(), "expirado") {
		t.Error("deveria dizer que o token expirou")
	}
	deOutro, _ := apiauth.Assinar(apiauth.Claims{Credencial: leitura.ID, Cliente: "sinop", Versao: 1, Escopos: []string{"cidades:read"}, Expira: agora.Add(time.Hour).Unix()}, chaveDaAPITeste)
	quemAPI(t, listar(deOutro), http.StatusUnauthorized, "invalid_token")
	assinadoPorOutraChave, _ := apiauth.Assinar(apiauth.Claims{Credencial: leitura.ID, Cliente: "novamutum", Versao: 1, Escopos: []string{"cidades:read"}, Expira: agora.Add(time.Hour).Unix()}, []byte("outra-chave-com-mais-de-32-bytes-ok!"))
	quemAPI(t, listar(assinadoPorOutraChave), http.StatusUnauthorized, "invalid_token")

	quemAPI(t, listar(tl), http.StatusOK, "")
	_, _ = m.cliQ.SetApiCredencialAtivo(t.Context(), sqlccliente.SetApiCredencialAtivoParams{ID: leitura.ID, Ativo: false})
	quemAPI(t, listar(tl), http.StatusUnauthorized, "invalid_token") // vale na hora, sem esperar o token vencer
	_, _ = m.cliQ.SetApiCredencialAtivo(t.Context(), sqlccliente.SetApiCredencialAtivoParams{ID: leitura.ID, Ativo: true})
	quemAPI(t, listar(tl), http.StatusOK, "")
	_, hash, _ := apiauth.NovoSegredo()
	if _, err := m.cliQ.RotacionarApiCredencial(t.Context(), sqlccliente.RotacionarApiCredencialParams{ID: leitura.ID, SegredoHash: hash}); err != nil {
		t.Fatal(err)
	}
	quemAPI(t, listar(tl), http.StatusUnauthorized, "invalid_token") // rotacionar invalida os tokens emitidos

	// A API não existe no console da plataforma.
	quemAPI(t, m.chamar(hostConsole, http.MethodGet, "/api/v1/cidades", tl, "", nil), http.StatusNotFound, "not_found")
	// O contrato é público.
	rec = m.chamar(hostNM, http.MethodGet, "/api/v1/openapi.yaml", "", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "openapi: 3.0.3") || !strings.Contains(rec.Body.String(), "/cidades/{id}") {
		t.Errorf("openapi: %d", rec.Code)
	}
}

func TestLimiteDeChamadasDaAPI(t *testing.T) {
	m := novoMundo(t)
	c := m.novaCredencial(t, "Limitada", "cidades:read")
	if _, err := m.pool.Exec(t.Context(), `UPDATE c_novamutum.api_credenciais SET limite_por_minuto = 3 WHERE id = $1`, c.ID); err != nil {
		t.Fatal(err)
	}
	tok := m.token(t, c, "")
	var rec *httptest.ResponseRecorder
	for i := 0; i < 3; i++ {
		rec = m.chamar(hostNM, http.MethodGet, "/api/v1/cidades", tok, "", nil)
		if rec.Code != http.StatusOK || rec.Header().Get("X-RateLimit-Limit") != "3" || rec.Header().Get("X-RateLimit-Remaining") != strconv.Itoa(2-i) {
			t.Fatalf("chamada %d: %d limit=%q restam=%q", i+1, rec.Code, rec.Header().Get("X-RateLimit-Limit"), rec.Header().Get("X-RateLimit-Remaining"))
		}
	}
	rec = m.chamar(hostNM, http.MethodGet, "/api/v1/cidades", tok, "", nil)
	quemAPI(t, rec, http.StatusTooManyRequests, "rate_limited")
	if rec.Header().Get("Retry-After") == "" {
		t.Error("faltou Retry-After")
	}
}

func TestCidadesPelaAPI(t *testing.T) {
	m := novoMundo(t)
	c := m.novaCredencial(t, "Integração do SMS", "cidades:read", "cidades:write")
	tok := m.token(t, c, "")
	api := func(metodo, caminho, corpo string, cab map[string]string) *httptest.ResponseRecorder {
		return m.chamar(hostNM, metodo, caminho, tok, corpo, cab)
	}

	// Criar.
	rec := api(http.MethodPost, "/api/v1/cidades", `{"nome":" Nova Mutum ","uf":"mt","ibge":"5106240"}`, nil)
	if rec.Code != http.StatusCreated || rec.Header().Get("Location") != "/api/v1/cidades/1" {
		t.Fatalf("criar: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	var cid apiCidade
	_ = json.Unmarshal(rec.Body.Bytes(), &cid)
	if cid.ID != 1 || cid.Nome != "Nova Mutum" || cid.UF != "MT" || cid.IBGE != "5106240" || !cid.Ativo || cid.CriadoEm.IsZero() {
		t.Errorf("cidade criada: %+v", cid)
	}
	// Recusas.
	quemAPI(t, api(http.MethodPost, "/api/v1/cidades", `{"nome":"Nova Mutum","uf":"MT"}`, nil), http.StatusConflict, "conflict")
	quemAPI(t, api(http.MethodPost, "/api/v1/cidades", `{"nome":"X","uf":"MT"}`, nil), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api(http.MethodPost, "/api/v1/cidades", `{"nome":"Sinop","uf":"XX"}`, nil), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api(http.MethodPost, "/api/v1/cidades", `{"nome":"Sinop","uf":"MT","ibge":"abc"}`, nil), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api(http.MethodPost, "/api/v1/cidades", `{nome}`, nil), http.StatusBadRequest, "invalid_json")
	quemAPI(t, api(http.MethodPost, "/api/v1/cidades", `{"nome":"Sinop","uf":"MT","outro":1}`, nil), http.StatusBadRequest, "invalid_json")

	// Ler e alterar.
	rec = api(http.MethodGet, "/api/v1/cidades/1", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"Nova Mutum"`) {
		t.Fatalf("ler: %d %s", rec.Code, rec.Body.String())
	}
	quemAPI(t, api(http.MethodGet, "/api/v1/cidades/99", "", nil), http.StatusNotFound, "not_found")
	quemAPI(t, api(http.MethodGet, "/api/v1/cidades/abc", "", nil), http.StatusNotFound, "not_found")
	rec = api(http.MethodPatch, "/api/v1/cidades/1", `{"ativo":false,"ibge":"5106241"}`, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &cid)
	if rec.Code != http.StatusOK || cid.Ativo || cid.IBGE != "5106241" || cid.Nome != "Nova Mutum" {
		t.Fatalf("alterar: %d %+v", rec.Code, cid)
	}
	quemAPI(t, api(http.MethodPatch, "/api/v1/cidades/1", `{"uf":"ZZ"}`, nil), http.StatusUnprocessableEntity, "validation_error")
	quemAPI(t, api(http.MethodPatch, "/api/v1/cidades/99", `{"ativo":true}`, nil), http.StatusNotFound, "not_found")

	// Paginação por cursor: 5 cidades, de 2 em 2.
	for _, n := range []string{"Sinop", "Sorriso", "Lucas do Rio Verde", "Cuiabá"} {
		quemAPI(t, api(http.MethodPost, "/api/v1/cidades", `{"nome":"`+n+`","uf":"MT"}`, nil), http.StatusCreated, "")
	}
	var nomes []string
	cursor := ""
	for pagina := 0; pagina < 5; pagina++ {
		caminho := "/api/v1/cidades?limit=2"
		if cursor != "" {
			caminho += "&cursor=" + url.QueryEscape(cursor)
		}
		rec = api(http.MethodGet, caminho, "", nil)
		var l struct {
			Data       []apiCidade `json:"data"`
			NextCursor *string     `json:"next_cursor"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("lista: %d %s", rec.Code, rec.Body.String())
		}
		for _, x := range l.Data {
			nomes = append(nomes, x.Nome)
		}
		if l.NextCursor == nil {
			if pagina != 2 || len(l.Data) != 1 {
				t.Errorf("última página inesperada: pagina=%d itens=%d", pagina, len(l.Data))
			}
			break
		}
		cursor = *l.NextCursor
	}
	if strings.Join(nomes, "|") != "Nova Mutum|Sinop|Sorriso|Lucas do Rio Verde|Cuiabá" {
		t.Errorf("ordem e conteúdo da paginação: %v", nomes)
	}
	quemAPI(t, api(http.MethodGet, "/api/v1/cidades?cursor=lixo", "", nil), http.StatusBadRequest, "invalid_cursor")
	if rec = api(http.MethodGet, "/api/v1/cidades?limit=1000", "", nil); rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"nome"`) != 5 {
		t.Errorf("limit alto: %d", rec.Code)
	}

	// Auditoria: o autor é a credencial, e o registro guarda o antes e o depois.
	ev := achar(m.eventos(t), "cidade.editar")
	if ev == nil || ev.AtorTipo != "api" || ev.AtorNome != "Integração do SMS" || ev.AtorEmail != c.ClientID || mapa(ev.Antes)["ativo"] != true || mapa(ev.Depois)["ativo"] != false {
		t.Errorf("cidade.editar: %+v", ev)
	}
	if e := achar(m.eventos(t), "cidade.criar"); e == nil || e.AtorTipo != "api" {
		t.Errorf("cidade.criar: %+v", e)
	}
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
	// As cidades criadas pela API aparecem na tela.
	quer(t, m.comAna(t).get("/cidades"), http.StatusOK, "Sorriso", "Cuiabá")
}

func TestIdempotenciaDaAPI(t *testing.T) {
	m := novoMundo(t)
	c := m.novaCredencial(t, "Com chave", "cidades:read", "cidades:write")
	outra := m.novaCredencial(t, "Outra", "cidades:read", "cidades:write")
	tok := m.token(t, c, "")
	post := func(tok, chave, corpo string) *httptest.ResponseRecorder {
		cab := map[string]string{}
		if chave != "" {
			cab["Idempotency-Key"] = chave
		}
		return m.chamar(hostNM, http.MethodPost, "/api/v1/cidades", tok, corpo, cab)
	}
	contar := func() int {
		cs, _ := m.cliQ.ListCidades(t.Context())
		return len(cs)
	}

	primeira := post(tok, "pedido-1", `{"nome":"Sinop","uf":"MT"}`)
	if primeira.Code != http.StatusCreated || primeira.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("primeira: %d", primeira.Code)
	}
	repetida := post(tok, "pedido-1", `{"nome":"Sinop","uf":"MT"}`)
	if repetida.Code != http.StatusCreated || repetida.Header().Get("Idempotent-Replayed") != "true" || repetida.Body.String() != primeira.Body.String() || contar() != 1 {
		t.Fatalf("repetida: %d replay=%q cidades=%d", repetida.Code, repetida.Header().Get("Idempotent-Replayed"), contar())
	}
	// Mesma chave com outro pedido.
	quemAPI(t, post(tok, "pedido-1", `{"nome":"Sorriso","uf":"MT"}`), http.StatusUnprocessableEntity, "idempotency_key_reuse")
	// Sem chave, o segundo pedido não é protegido (conflito de nome).
	quemAPI(t, post(tok, "", `{"nome":"Sinop","uf":"MT"}`), http.StatusConflict, "conflict")
	// Erros de negócio também ficam guardados e se repetem.
	e1 := post(tok, "erro-1", `{"nome":"X","uf":"MT"}`)
	e2 := post(tok, "erro-1", `{"nome":"X","uf":"MT"}`)
	if e1.Code != http.StatusUnprocessableEntity || e2.Code != http.StatusUnprocessableEntity || e2.Header().Get("Idempotent-Replayed") != "true" {
		t.Errorf("erro repetido: %d %d %q", e1.Code, e2.Code, e2.Header().Get("Idempotent-Replayed"))
	}
	// A chave pertence à credencial: outra credencial pode usar o mesmo texto.
	if rec := post(m.token(t, outra, ""), "pedido-1", `{"nome":"Lucas","uf":"MT"}`); rec.Code != http.StatusCreated || contar() != 2 {
		t.Errorf("outra credencial: %d", rec.Code)
	}
	// Chave inválida; e chave "em andamento" (outra requisição ainda não terminou).
	quemAPI(t, post(tok, "tem espaço", `{"nome":"Y","uf":"MT"}`), http.StatusBadRequest, "invalid_idempotency_key")
	soma := sha256Hex(`POST /api/v1/cidades` + "\n" + `{"nome":"Z","uf":"MT"}`)
	if _, err := m.pool.Exec(t.Context(), `INSERT INTO c_novamutum.api_idempotencia (credencial_id, chave, hash_pedido) VALUES ($1, 'andando', $2)`, c.ID, soma); err != nil {
		t.Fatal(err)
	}
	rec := post(tok, "andando", `{"nome":"Z","uf":"MT"}`)
	quemAPI(t, rec, http.StatusConflict, "request_in_progress")
	if rec.Header().Get("Retry-After") == "" {
		t.Error("faltou Retry-After")
	}
	// Idempotência também vale para PATCH.
	cab := map[string]string{"Idempotency-Key": "patch-1"}
	p1 := m.chamar(hostNM, http.MethodPatch, "/api/v1/cidades/1", tok, `{"ibge":"5107909"}`, cab)
	p2 := m.chamar(hostNM, http.MethodPatch, "/api/v1/cidades/1", tok, `{"ibge":"5107909"}`, cab)
	if p1.Code != http.StatusOK || p2.Code != http.StatusOK || p2.Header().Get("Idempotent-Replayed") != "true" {
		t.Errorf("patch: %d %d", p1.Code, p2.Code)
	}
}

func TestRegistroDeChamadasDaAPI(t *testing.T) {
	m := novoMundo(t)
	c := m.novaCredencial(t, "Registrada", "cidades:read", "cidades:write")
	tok := m.token(t, c, "")
	m.chamar(hostNM, http.MethodGet, "/api/v1/cidades", tok, "", nil)
	m.chamar(hostNM, http.MethodPost, "/api/v1/cidades", tok, `{"nome":"X","uf":"MT"}`, nil)
	m.chamar(hostNM, http.MethodGet, "/api/v1/cidades/99", tok, "", nil)
	m.chamar(hostNM, http.MethodPost, "/api/v1/token", "", url.Values{"grant_type": {"client_credentials"}, "client_id": {c.ClientID}, "client_secret": {"uas_errado"}}.Encode(), nil)

	chs, err := m.cliQ.ListApiChamadas(t.Context(), sqlccliente.ListApiChamadasParams{CredencialID: c.ID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	var status []string
	for _, x := range chs { // da mais nova para a mais antiga
		status = append(status, x.Metodo+" "+strconv.Itoa(int(x.Status)))
	}
	if strings.Join(status, ",") != "POST 401,GET 404,POST 422,GET 200,POST 200" {
		t.Errorf("chamadas registradas: %v", status)
	}
	if chs[0].Ip == "" || chs[0].Caminho != "/api/v1/token" {
		t.Errorf("campos: %+v", chs[0])
	}
	r, _ := m.cliQ.ResumoApiChamadas(t.Context(), sqlccliente.ResumoApiChamadasParams{CredencialID: c.ID, Quando: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true}})
	if r.Total != 5 || r.ComErro != 3 {
		t.Errorf("resumo: %+v", r)
	}
	cred, _ := m.cliQ.GetApiCredencial(t.Context(), c.ID)
	if !cred.UltimoUsoEm.Valid {
		t.Error("o último uso deveria estar preenchido")
	}
}

func sha256Hex(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

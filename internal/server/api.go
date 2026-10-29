package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/apiauth"
	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/httpx"
	"uysiarbo/internal/tenant"
)

// ehAPI diz se o caminho é da API para sistemas externos: ela tem autenticação própria (token Bearer),
// sem cookie, sem CSRF e sem as telas de login.
func ehAPI(path string) bool { return strings.HasPrefix(path, "/api/") }

const (
	duracaoDoToken  = 15 * time.Minute
	limiteDeIPToken = 30 // pedidos de token por minuto, por endereço
	maxCorpoAPI     = 1 << 20
)

type credencialKey struct{}

// credencialDaAPI é a credencial que fez a chamada (preenchida pelo middleware de autenticação).
type credencialDaAPI struct {
	ID       int64
	Nome     string
	ClientID string
	Escopos  []string
}

func credencialDe(ctx context.Context) (credencialDaAPI, bool) {
	c, ok := ctx.Value(credencialKey{}).(credencialDaAPI)
	return c, ok
}

// apiEstado guarda o que a API precisa entre chamadas (limites e último uso).
type apiEstado struct {
	chave      []byte
	porCred    apiauth.Limitador
	porIP      apiauth.Limitador
	mu         sync.Mutex
	tocadoEm   map[int64]time.Time
	limpezaIdp int
}

func (h *handlers) apiErro(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	httpx.Error(w, r, status, code, msg)
}

func (h *handlers) apiNaoAutorizado(w http.ResponseWriter, r *http.Request, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="uysiarbo", error="invalid_token"`)
	h.apiErro(w, r, http.StatusUnauthorized, "invalid_token", msg)
}

// registrarChamada grava a chamada no log da credencial e atualiza o último uso (no máximo 1 vez por minuto).
func (h *handlers) registrarChamada(ctx context.Context, q *sqlccliente.Queries, credID int64, r *http.Request, status int, inicio time.Time) {
	err := q.InsertApiChamada(ctx, sqlccliente.InsertApiChamadaParams{
		CredencialID: credID, Metodo: r.Method, Caminho: r.URL.Path, Status: int32(status),
		DuracaoMs: int32(time.Since(inicio).Milliseconds()), Ip: ipDe(r), RequestID: r.Header.Get("X-Request-Id"),
	})
	if err != nil {
		h.log.Error("registrar chamada da API", "erro", err, "request_id", r.Header.Get("X-Request-Id"))
	}
	h.api.mu.Lock()
	ultimo := h.api.tocadoEm[credID]
	tocar := time.Since(ultimo) > time.Minute
	if tocar {
		h.api.tocadoEm[credID] = time.Now()
	}
	h.api.mu.Unlock()
	if tocar {
		_ = q.TocarApiCredencial(ctx, credID)
	}
}

// apiAuth autentica a chamada (Bearer), confere o cliente, a credencial, o escopo e o limite de uso.
func (h *handlers) apiAuth(escopo string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		cliente, ok := tenant.FromContext(r.Context())
		if !ok {
			h.apiErro(w, r, http.StatusNotFound, "not_found", "A API fica no endereço de cada cliente.")
			return
		}
		cab := r.Header.Get("Authorization")
		tok, tem := strings.CutPrefix(cab, "Bearer ")
		if !tem || strings.TrimSpace(tok) == "" {
			h.apiNaoAutorizado(w, r, "Envie o token no cabeçalho Authorization: Bearer <token>.")
			return
		}
		claims, err := apiauth.Verificar(strings.TrimSpace(tok), h.api.chave, time.Now())
		if err != nil || claims.Cliente != cliente.Slug {
			msg := "Token inválido."
			if errors.Is(err, apiauth.ErrTokenExpirado) {
				msg = "Token expirado. Peça um novo em /api/v1/token."
			}
			h.apiNaoAutorizado(w, r, msg)
			return
		}
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		cred, err := q.GetApiCredencial(r.Context(), claims.Credencial)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!cred.Ativo || int(cred.VersaoSegredo) != claims.Versao ||
			(cred.ValidaAte.Valid && !cred.ValidaAte.Time.After(time.Now())))) {
			h.apiNaoAutorizado(w, r, "Credencial desativada, vencida ou com segredo trocado. Peça um novo token.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar credencial da API", err)
			return
		}
		ctx := context.WithValue(r.Context(), credencialKey{}, credencialDaAPI{ID: cred.ID, Nome: cred.Nome, ClientID: cred.ClientID, Escopos: claims.Escopos})
		r = r.WithContext(ctx)
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		finalizar := func() { h.registrarChamada(context.WithoutCancel(ctx), q, cred.ID, r, ww.Status(), inicio) }
		defer finalizar()

		if !apiauth.Permite(cred.Escopos, escopo) || !apiauth.Permite(claims.Escopos, escopo) {
			h.apiErro(ww, r, http.StatusForbidden, "insufficient_scope", "Esta credencial não tem o escopo "+escopo+".")
			return
		}
		limite := int(cred.LimitePorMinuto)
		permitido, restam, espera := h.api.porCred.Permitir("cred:"+strconv.FormatInt(cred.ID, 10), limite)
		ww.Header().Set("X-RateLimit-Limit", strconv.Itoa(limite))
		ww.Header().Set("X-RateLimit-Remaining", strconv.Itoa(restam))
		if !permitido {
			ww.Header().Set("Retry-After", strconv.Itoa(int(espera.Seconds())+1))
			h.apiErro(ww, r, http.StatusTooManyRequests, "rate_limited", "Limite de chamadas por minuto excedido. Tente de novo em instantes.")
			return
		}
		next(ww, r)
	}
}

// ------------------------------------------------------------------- Token

func (h *handlers) apiToken(w http.ResponseWriter, r *http.Request) {
	inicio := time.Now()
	cliente, ok := tenant.FromContext(r.Context())
	if !ok {
		h.apiErro(w, r, http.StatusNotFound, "not_found", "A API fica no endereço de cada cliente.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if okIP, _, espera := h.api.porIP.Permitir("ip:"+ipDe(r), limiteDeIPToken); !okIP {
		w.Header().Set("Retry-After", strconv.Itoa(int(espera.Seconds())+1))
		h.apiErro(w, r, http.StatusTooManyRequests, "rate_limited", "Muitos pedidos de token deste endereço. Tente de novo em instantes.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := r.ParseForm(); err != nil {
		h.apiErro(w, r, http.StatusBadRequest, "invalid_request", "Envie o pedido como application/x-www-form-urlencoded.")
		return
	}
	if r.PostFormValue("grant_type") != "client_credentials" {
		h.apiErro(w, r, http.StatusBadRequest, "unsupported_grant_type", "Use grant_type=client_credentials.")
		return
	}
	id, segredo, basico := r.BasicAuth()
	if !basico {
		id, segredo = r.PostFormValue("client_id"), r.PostFormValue("client_secret")
	}
	if id == "" || segredo == "" {
		h.apiErro(w, r, http.StatusUnauthorized, "invalid_client", "Informe client_id e client_secret.")
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	cred, err := q.GetApiCredencialPorClientID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		apiauth.SegredoConfere(segredo, make([]byte, 32)) // gasta o mesmo tempo, exista ou não o client_id
		h.apiErro(w, r, http.StatusUnauthorized, "invalid_client", "Credencial inválida.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar credencial da API", err)
		return
	}
	ctx := context.WithoutCancel(r.Context())
	if !apiauth.SegredoConfere(segredo, cred.SegredoHash) {
		h.registrarChamada(ctx, q, cred.ID, r, http.StatusUnauthorized, inicio)
		h.apiErro(w, r, http.StatusUnauthorized, "invalid_client", "Credencial inválida.")
		return
	}
	if !cred.Ativo || (cred.ValidaAte.Valid && !cred.ValidaAte.Time.After(time.Now())) {
		h.registrarChamada(ctx, q, cred.ID, r, http.StatusUnauthorized, inicio)
		h.apiErro(w, r, http.StatusUnauthorized, "invalid_client", "Credencial desativada ou vencida.")
		return
	}
	escopos := cred.Escopos
	if pedido := strings.Fields(r.PostFormValue("scope")); len(pedido) > 0 {
		pedido, err := apiauth.Normalizar(pedido)
		if err != nil {
			h.registrarChamada(ctx, q, cred.ID, r, http.StatusBadRequest, inicio)
			h.apiErro(w, r, http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		for _, e := range pedido {
			if !apiauth.Permite(cred.Escopos, e) {
				h.registrarChamada(ctx, q, cred.ID, r, http.StatusBadRequest, inicio)
				h.apiErro(w, r, http.StatusBadRequest, "invalid_scope", "A credencial não tem o escopo "+e+".")
				return
			}
		}
		escopos = pedido
	}
	agora := time.Now()
	tok, err := apiauth.Assinar(apiauth.Claims{
		Emissor: "uysiarbo", Credencial: cred.ID, Cliente: cliente.Slug, Versao: int(cred.VersaoSegredo), Escopos: escopos,
		Emitido: agora.Unix(), Expira: agora.Add(duracaoDoToken).Unix(),
	}, h.api.chave)
	if err != nil {
		h.falhaInterna(w, r, "assinar token da API", err)
		return
	}
	h.registrarChamada(ctx, q, cred.ID, r, http.StatusOK, inicio)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"access_token": tok, "token_type": "Bearer", "expires_in": int(duracaoDoToken.Seconds()), "scope": strings.Join(escopos, " "),
	})
}

// ------------------------------------------------------------------- Idempotência

var reChaveIdempotencia = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,64}$`)

// gravador guarda status e corpo da resposta, para repetir quando o mesmo Idempotency-Key voltar.
type gravador struct {
	http.ResponseWriter
	status int
	corpo  bytes.Buffer
}

func (g *gravador) WriteHeader(s int) {
	if g.status == 0 {
		g.status = s
	}
	g.ResponseWriter.WriteHeader(s)
}

func (g *gravador) Write(b []byte) (int, error) {
	if g.status == 0 {
		g.status = http.StatusOK
	}
	if g.corpo.Len() < maxCorpoAPI {
		g.corpo.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

// idempotente faz um pedido de gravação com Idempotency-Key acontecer uma vez só: repetir o mesmo
// pedido devolve a mesma resposta (com Idempotent-Replayed: true); a mesma chave com outro conteúdo é recusada.
func (h *handlers) idempotente(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chave := r.Header.Get("Idempotency-Key")
		cred, tem := credencialDe(r.Context())
		if chave == "" || !tem {
			next(w, r)
			return
		}
		if !reChaveIdempotencia.MatchString(chave) {
			h.apiErro(w, r, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key deve ter de 1 a 64 caracteres: letras, números e _ . : -")
			return
		}
		corpo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxCorpoAPI))
		if err != nil {
			h.apiErro(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Corpo grande demais (máximo 1 MB).")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(corpo))
		soma := sha256.Sum256(append([]byte(r.Method+" "+r.URL.Path+"\n"), corpo...))
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		ctx := r.Context()
		h.api.mu.Lock()
		h.api.limpezaIdp++
		limpar := h.api.limpezaIdp%50 == 0
		h.api.mu.Unlock()
		if limpar {
			_, _ = q.LimparIdempotenciaAntiga(context.WithoutCancel(ctx))
		}
		n, err := q.ReservarIdempotencia(ctx, sqlccliente.ReservarIdempotenciaParams{CredencialID: cred.ID, Chave: chave, HashPedido: soma[:]})
		if err != nil {
			h.falhaInterna(w, r, "reservar idempotência", err)
			return
		}
		if n == 0 { // a chave já existe: repete, recusa ou pede para esperar
			reg, err := q.GetIdempotencia(ctx, sqlccliente.GetIdempotenciaParams{CredencialID: cred.ID, Chave: chave})
			if err != nil {
				h.falhaInterna(w, r, "ler idempotência", err)
				return
			}
			switch {
			case !bytes.Equal(reg.HashPedido, soma[:]):
				h.apiErro(w, r, http.StatusUnprocessableEntity, "idempotency_key_reuse", "Esta Idempotency-Key já foi usada com outro pedido.")
			case reg.Status == 0:
				w.Header().Set("Retry-After", "1")
				h.apiErro(w, r, http.StatusConflict, "request_in_progress", "O pedido com esta Idempotency-Key ainda está sendo processado.")
			default:
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Idempotent-Replayed", "true")
				w.WriteHeader(int(reg.Status))
				_, _ = w.Write(reg.Resposta)
			}
			return
		}
		g := &gravador{ResponseWriter: w}
		defer func() {
			fim := context.WithoutCancel(ctx)
			if g.status == 0 || g.status >= 500 { // falha do servidor: libera a chave para tentar de novo
				_ = q.LiberarIdempotencia(fim, sqlccliente.LiberarIdempotenciaParams{CredencialID: cred.ID, Chave: chave})
				return
			}
			_ = q.ConcluirIdempotencia(fim, sqlccliente.ConcluirIdempotenciaParams{CredencialID: cred.ID, Chave: chave, Status: int32(g.status), Resposta: g.corpo.Bytes()})
		}()
		next(g, r)
	}
}

// ------------------------------------------------------------------- Utilidades de recurso

// lerJSON lê o corpo como JSON, recusando campos desconhecidos e corpo grande demais.
func (h *handlers) lerJSON(w http.ResponseWriter, r *http.Request, destino any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCorpoAPI))
	dec.DisallowUnknownFields()
	if err := dec.Decode(destino); err != nil || dec.More() {
		h.apiErro(w, r, http.StatusBadRequest, "invalid_json", "O corpo precisa ser um JSON válido, só com os campos documentados.")
		return false
	}
	return true
}

func limiteDaURL(r *http.Request) int32 {
	n, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || n <= 0 {
		return 50
	}
	return int32(min(n, 200))
}

// cursor é opaco para quem consome: hoje é o último id lido, em base64.
func lerCursor(r *http.Request) (int64, bool) {
	c := r.URL.Query().Get("cursor")
	if c == "" {
		return 0, true
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimPrefix(string(b), "i:"), 10, 64)
	return n, err == nil && strings.HasPrefix(string(b), "i:") && n >= 0
}

func montarCursor(ultimo int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte("i:" + strconv.FormatInt(ultimo, 10)))
}

// auditarAPI registra uma gravação feita pela API (o autor é a credencial).
func (h *handlers) auditarAPI(r *http.Request, e audit.Evento) { h.auditar(r, e) }

// ------------------------------------------------------------------- Cidades

type apiCidade struct {
	ID           int64     `json:"id"`
	Nome         string    `json:"nome"`
	UF           string    `json:"uf"`
	IBGE         string    `json:"ibge"`
	Ativo        bool      `json:"ativo"`
	CriadoEm     time.Time `json:"criado_em"`
	AtualizadoEm time.Time `json:"atualizado_em"`
}

func cidadeParaAPI(c sqlccliente.Cidade) apiCidade {
	return apiCidade{ID: c.ID, Nome: c.Nome, UF: c.Uf, IBGE: c.Ibge, Ativo: c.Ativo, CriadoEm: c.CriadoEm.Time, AtualizadoEm: c.AtualizadoEm.Time}
}

func validarCidadeAPI(nome, uf, ibge string) (string, string) {
	switch {
	case len([]rune(nome)) < 2 || len([]rune(nome)) > 100:
		return "nome", "O nome da cidade deve ter de 2 a 100 caracteres."
	case !ufsValidas[uf]:
		return "uf", "uf deve ser a sigla de um estado brasileiro, como MT."
	case !reIBGE.MatchString(ibge):
		return "ibge", "ibge deve ter até 7 números."
	}
	return "", ""
}

func (h *handlers) apiErroCampo(w http.ResponseWriter, r *http.Request, campo, msg string) {
	h.apiErro(w, r, http.StatusUnprocessableEntity, "validation_error", campo+": "+msg)
}

func (h *handlers) apiCidadesLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	depois, ok := lerCursor(r)
	if !ok {
		h.apiErro(w, r, http.StatusBadRequest, "invalid_cursor", "cursor inválido: use o next_cursor da resposta anterior.")
		return
	}
	limite := limiteDaURL(r)
	rows, err := q.ListCidadesApi(r.Context(), sqlccliente.ListCidadesApiParams{ID: depois, Limit: limite + 1})
	if err != nil {
		h.falhaInterna(w, r, "listar cidades", err)
		return
	}
	var proximo any
	if int32(len(rows)) > limite {
		rows = rows[:limite]
		proximo = montarCursor(rows[len(rows)-1].ID)
	}
	dados := make([]apiCidade, len(rows))
	for i, c := range rows {
		dados[i] = cidadeParaAPI(c)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": dados, "next_cursor": proximo})
}

func (h *handlers) apiCidadeDaURL(w http.ResponseWriter, r *http.Request) (sqlccliente.Cidade, *sqlccliente.Queries, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		h.apiErro(w, r, http.StatusNotFound, "not_found", "Cidade não encontrada.")
		return sqlccliente.Cidade{}, nil, false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return sqlccliente.Cidade{}, nil, false
	}
	c, err := q.GetCidade(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		h.apiErro(w, r, http.StatusNotFound, "not_found", "Cidade não encontrada.")
		return c, nil, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar cidade", err)
		return c, nil, false
	}
	return c, q, true
}

func (h *handlers) apiCidadeObter(w http.ResponseWriter, r *http.Request) {
	if c, _, ok := h.apiCidadeDaURL(w, r); ok {
		httpx.JSON(w, http.StatusOK, cidadeParaAPI(c))
	}
}

func dadosAuditoriaCidade(c sqlccliente.Cidade) map[string]any {
	return map[string]any{"nome": c.Nome, "uf": c.Uf, "ibge": c.Ibge, "ativo": c.Ativo}
}

func (h *handlers) apiCidadeCriar(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Nome  string `json:"nome"`
		UF    string `json:"uf"`
		IBGE  string `json:"ibge"`
		Ativo *bool  `json:"ativo"`
	}
	if !h.lerJSON(w, r, &in) {
		return
	}
	in.Nome, in.UF, in.IBGE = strings.TrimSpace(in.Nome), strings.ToUpper(strings.TrimSpace(in.UF)), strings.TrimSpace(in.IBGE)
	if campo, msg := validarCidadeAPI(in.Nome, in.UF, in.IBGE); campo != "" {
		h.apiErroCampo(w, r, campo, msg)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	c, err := q.CreateCidade(r.Context(), sqlccliente.CreateCidadeParams{Nome: in.Nome, Uf: in.UF, Ibge: in.IBGE})
	if violacaoUnica(err) {
		h.apiErro(w, r, http.StatusConflict, "conflict", "Já existe uma cidade com esse nome e UF.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "criar cidade", err)
		return
	}
	if in.Ativo != nil && !*in.Ativo {
		if c, err = q.UpdateCidade(r.Context(), sqlccliente.UpdateCidadeParams{ID: c.ID, Nome: c.Nome, Uf: c.Uf, Ibge: c.Ibge, Ativo: false}); err != nil {
			h.falhaInterna(w, r, "desativar cidade", err)
			return
		}
	}
	h.auditarAPI(r, audit.Evento{Acao: "cidade.criar", Entidade: "cidade", EntidadeID: strconv.FormatInt(c.ID, 10), Rotulo: c.Nome + "/" + c.Uf, Depois: dadosAuditoriaCidade(c)})
	w.Header().Set("Location", "/api/v1/cidades/"+strconv.FormatInt(c.ID, 10))
	httpx.JSON(w, http.StatusCreated, cidadeParaAPI(c))
}

func (h *handlers) apiCidadeAlterar(w http.ResponseWriter, r *http.Request) {
	atual, q, ok := h.apiCidadeDaURL(w, r)
	if !ok {
		return
	}
	var in struct {
		Nome  *string `json:"nome"`
		UF    *string `json:"uf"`
		IBGE  *string `json:"ibge"`
		Ativo *bool   `json:"ativo"`
	}
	if !h.lerJSON(w, r, &in) {
		return
	}
	novo := atual
	if in.Nome != nil {
		novo.Nome = strings.TrimSpace(*in.Nome)
	}
	if in.UF != nil {
		novo.Uf = strings.ToUpper(strings.TrimSpace(*in.UF))
	}
	if in.IBGE != nil {
		novo.Ibge = strings.TrimSpace(*in.IBGE)
	}
	if in.Ativo != nil {
		novo.Ativo = *in.Ativo
	}
	if campo, msg := validarCidadeAPI(novo.Nome, novo.Uf, novo.Ibge); campo != "" {
		h.apiErroCampo(w, r, campo, msg)
		return
	}
	c, err := q.UpdateCidade(r.Context(), sqlccliente.UpdateCidadeParams{ID: atual.ID, Nome: novo.Nome, Uf: novo.Uf, Ibge: novo.Ibge, Ativo: novo.Ativo})
	if violacaoUnica(err) {
		h.apiErro(w, r, http.StatusConflict, "conflict", "Já existe uma cidade com esse nome e UF.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "alterar cidade", err)
		return
	}
	a, dep := audit.Diferenca(dadosAuditoriaCidade(atual), dadosAuditoriaCidade(c))
	if len(a)+len(dep) > 0 {
		h.auditarAPI(r, audit.Evento{Acao: "cidade.editar", Entidade: "cidade", EntidadeID: strconv.FormatInt(c.ID, 10), Rotulo: c.Nome + "/" + c.Uf, Antes: a, Depois: dep})
	}
	httpx.JSON(w, http.StatusOK, cidadeParaAPI(c))
}

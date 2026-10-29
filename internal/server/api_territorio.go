package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/httpx"
	"uysiarbo/web/views"
)

func idDaURLAPI(r *http.Request) (int64, bool) {
	n, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return n, err == nil && n > 0
}

func (h *handlers) apiNaoEncontrado(w http.ResponseWriter, r *http.Request, o string) {
	h.apiErro(w, r, http.StatusNotFound, "not_found", o+" não encontrado(a).")
}

// resolverConflito traduz violação de unicidade (nome repetido ou id_externo já usado).
func (h *handlers) apiConflito(w http.ResponseWriter, r *http.Request, msg string) {
	h.apiErro(w, r, http.StatusConflict, "conflict", msg)
}

// paginaAPI responde uma lista paginada por cursor: itens já cortados em `limite` e o id do último.
func (h *handlers) paginaAPI(w http.ResponseWriter, itens any, ultimoID int64, temMais bool) {
	var proximo any
	if temMais {
		proximo = montarCursor(ultimoID)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": itens, "next_cursor": proximo})
}

func tempo(t pgtype.Timestamptz) time.Time { return t.Time }

// ------------------------------------------------------------------- Bairros

type apiBairro struct {
	ExcluidoEm      any       `json:"excluido_em"` // preenchido se o registro foi excluído (só aparece com updated_since)
	ID              int64     `json:"id"`
	CidadeID        int64     `json:"cidade_id"`
	Nome            string    `json:"nome"`
	Categoria       string    `json:"categoria"`
	Classificacao   string    `json:"classificacao"`
	Status          string    `json:"status"`
	Ibge            string    `json:"ibge"`
	Zona            string    `json:"zona"`
	Mutirao         bool      `json:"mutirao"`
	Producao        bool      `json:"producao"`
	Latitude        any       `json:"latitude"`
	Longitude       any       `json:"longitude"`
	Altitude        any       `json:"altitude"`
	DistanciaCentro any       `json:"distancia_centro"`
	IDExterno       any       `json:"id_externo"`
	CriadoEm        time.Time `json:"criado_em"`
	AtualizadoEm    time.Time `json:"atualizado_em"`
}

func bairroParaAPI(b sqlccliente.Bairro) apiBairro {
	return apiBairro{
		ID: b.ID, CidadeID: b.CidadeID, Nome: b.Nome, Categoria: b.Categoria, Classificacao: b.Classificacao, Status: b.Status, Ibge: b.Ibge,
		Zona: b.Zona, Mutirao: b.Mutirao, Producao: b.Producao, Latitude: optFloat(b.Latitude), Longitude: optFloat(b.Longitude),
		Altitude: optFloat(b.Altitude), DistanciaCentro: optFloat(b.DistanciaCentro), IDExterno: optTexto(b.IDExterno),
		CriadoEm: tempo(b.CriadoEm), AtualizadoEm: tempo(b.AtualizadoEm), ExcluidoEm: optTempo(b.DeletedAt),
	}
}

func (h *handlers) apiBairrosLista(w http.ResponseWriter, r *http.Request) {
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
	cidade, ok1 := inteiroDaURL(r, "cidade_id")
	desde, ok2 := desdeDaURL(r)
	if !ok1 {
		h.apiParametroInvalido(w, r, "cidade_id")
		return
	}
	if !ok2 {
		h.apiParametroInvalido(w, r, "updated_since (use RFC 3339, como 2026-10-29T12:00:00Z)")
		return
	}
	qs := r.URL.Query()
	limite := limiteDaURL(r)
	rows, err := q.ListBairrosApi(r.Context(), sqlccliente.ListBairrosApiParams{
		Depois: depois, CidadeID: cidade, Status: qs.Get("status"), Busca: strings.TrimSpace(qs.Get("q")), IDExterno: qs.Get("id_externo"),
		AtualizadoDesde: desde, Limite: limite + 1, IncluirExcluidos: desde.Valid,
	})
	if err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	mais := int32(len(rows)) > limite
	if mais {
		rows = rows[:limite]
	}
	dados := make([]apiBairro, len(rows))
	var ultimo int64
	for i, b := range rows {
		dados[i], ultimo = bairroParaAPI(b), b.ID
	}
	h.paginaAPI(w, dados, ultimo, mais)
}

func (h *handlers) apiBairroDaURL(w http.ResponseWriter, r *http.Request) (sqlccliente.Bairro, *sqlccliente.Queries, bool) {
	id, ok := idDaURLAPI(r)
	if !ok {
		h.apiNaoEncontrado(w, r, "Bairro")
		return sqlccliente.Bairro{}, nil, false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return sqlccliente.Bairro{}, nil, false
	}
	b, err := q.GetBairro(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		h.apiNaoEncontrado(w, r, "Bairro")
		return b, nil, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar bairro", err)
		return b, nil, false
	}
	return b, q, true
}

func (h *handlers) apiBairroObter(w http.ResponseWriter, r *http.Request) {
	if b, _, ok := h.apiBairroDaURL(w, r); ok {
		httpx.JSON(w, http.StatusOK, bairroParaAPI(b))
	}
}

var camposDeBairro = []string{"cidade_id", "nome", "categoria", "classificacao", "status", "ibge", "zona", "mutirao", "producao",
	"latitude", "longitude", "altitude", "distancia_centro", "id_externo"}

// dadosDoBairroAPI aplica o corpo sobre os dados atuais do bairro (ou sobre o padrão, na criação).
func dadosDoBairroAPI(c corpoAPI, d dadosBairro) (dadosBairro, error) {
	var err error
	cid, err := c.inteiro("cidade_id", d.cidadeID)
	if err != nil {
		return d, err
	}
	d.cidadeID = cid
	campos := []struct {
		nome string
		dest *string
	}{{"nome", &d.nome}, {"categoria", &d.categoria}, {"classificacao", &d.classificacao}, {"status", &d.status}, {"ibge", &d.ibge}, {"zona", &d.zona},
		{"latitude", &d.latitude}, {"longitude", &d.longitude}, {"altitude", &d.altitude}, {"distancia_centro", &d.distancia}}
	for _, f := range campos {
		if *f.dest, err = c.texto(f.nome, *f.dest); err != nil {
			return d, err
		}
	}
	if d.mutirao, err = c.logico("mutirao", d.mutirao); err != nil {
		return d, err
	}
	if d.producao, err = c.logico("producao", d.producao); err != nil {
		return d, err
	}
	return d, nil
}

func (h *handlers) apiBairroCriar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, ok := h.lerCorpo(w, r, camposDeBairro...)
	if !ok {
		return
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	ext, err := c.idExterno()
	if err != nil {
		h.apiErroCampo(w, r, "id_externo", err.Error())
		return
	}
	if ext != "" { // já criado antes (o tablet repetiu o envio): devolve o que existe
		if b, err := q.GetBairroPorIdExterno(ctx, pgtype.Text{String: ext, Valid: true}); err == nil {
			httpx.JSON(w, http.StatusOK, bairroParaAPI(b))
			return
		} else if !errors.Is(err, pgx.ErrNoRows) {
			h.falhaInterna(w, r, "buscar por id_externo", err)
			return
		}
	}
	d, err := dadosDoBairroAPI(c, dadosBairro{producao: true, status: "ativo", classificacao: "urbano", categoria: "bairro"})
	if err != nil {
		h.apiErroCampo(w, r, "corpo", err.Error())
		return
	}
	cidades, err := q.ListCidadesAtivas(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar cidades", err)
		return
	}
	params, msg := validarBairro(d, cidades, 0)
	if msg != "" {
		h.apiErroCampo(w, r, "bairro", msg)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	b, err := qt.CreateBairro(ctx, params)
	if violacaoUnica(err) {
		h.apiConflito(w, r, "Já existe um bairro com esse nome nessa cidade.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "criar bairro", err)
		return
	}
	if ext != "" {
		if b, err = qt.SetBairroIdExterno(ctx, sqlccliente.SetBairroIdExternoParams{ID: b.ID, IDExterno: pgtype.Text{String: ext, Valid: true}}); violacaoUnica(err) {
			h.apiConflito(w, r, "Este id_externo já está em uso por outro bairro.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "gravar id_externo", err)
			return
		}
	}
	cidade, _ := q.GetCidade(ctx, b.CidadeID)
	ev := audit.Evento{Acao: "bairro.criar", Entidade: "bairro", EntidadeID: strconv.FormatInt(b.ID, 10), Rotulo: b.Nome, Depois: dadosAuditoriaBairro(params, cidade.Nome+"/"+cidade.Uf)}
	if ext != "" {
		ev.Detalhes = map[string]any{"id_externo": ext}
	}
	h.auditarCom(auditCli(qt), r, ev)
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar bairro", err)
		return
	}
	w.Header().Set("Location", "/api/v1/bairros/"+strconv.FormatInt(b.ID, 10))
	httpx.JSON(w, http.StatusCreated, bairroParaAPI(b))
}

func auditCli(q *sqlccliente.Queries) audit.Registrador { return audit.Cliente{Q: q} }

func (h *handlers) apiBairroAlterar(w http.ResponseWriter, r *http.Request) {
	atual, q, ok := h.apiBairroDaURL(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	c, ok := h.lerCorpo(w, r, camposDeBairro...)
	if !ok {
		return
	}
	ext, err := c.idExterno()
	if err != nil {
		h.apiErroCampo(w, r, "id_externo", err.Error())
		return
	}
	if ext != "" && atual.IDExterno.Valid && atual.IDExterno.String != ext {
		h.apiErroCampo(w, r, "id_externo", "o id_externo de um registro não muda depois de definido")
		return
	}
	d, err := dadosDoBairroAPI(c, dadosDoBairro(atual, nil))
	if err != nil {
		h.apiErroCampo(w, r, "corpo", err.Error())
		return
	}
	cidades, err := q.ListCidadesAtivas(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar cidades", err)
		return
	}
	params, msg := validarBairro(d, cidades, atual.CidadeID)
	if msg != "" {
		h.apiErroCampo(w, r, "bairro", msg)
		return
	}
	_, pool, _ := h.cliente(r)
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	b, err := qt.UpdateBairro(ctx, sqlccliente.UpdateBairroParams{
		ID: atual.ID, CidadeID: params.CidadeID, Nome: params.Nome, Mutirao: params.Mutirao, Status: params.Status, Ibge: params.Ibge,
		Classificacao: params.Classificacao, Zona: params.Zona, Categoria: params.Categoria, Latitude: params.Latitude, Longitude: params.Longitude,
		Altitude: params.Altitude, DistanciaCentro: params.DistanciaCentro, Producao: params.Producao,
	})
	if violacaoUnica(err) {
		h.apiConflito(w, r, "Já existe um bairro com esse nome nessa cidade.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "alterar bairro", err)
		return
	}
	if ext != "" && !atual.IDExterno.Valid {
		if b, err = qt.SetBairroIdExterno(ctx, sqlccliente.SetBairroIdExternoParams{ID: b.ID, IDExterno: pgtype.Text{String: ext, Valid: true}}); violacaoUnica(err) {
			h.apiConflito(w, r, "Este id_externo já está em uso por outro bairro.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "gravar id_externo", err)
			return
		}
	}
	nomeCidade := func(id int64) string {
		if x, err := q.GetCidade(ctx, id); err == nil {
			return x.Nome + "/" + x.Uf
		}
		return ""
	}
	antesP := sqlccliente.CreateBairroParams{
		CidadeID: atual.CidadeID, Nome: atual.Nome, Mutirao: atual.Mutirao, Status: atual.Status, Ibge: atual.Ibge, Classificacao: atual.Classificacao,
		Zona: atual.Zona, Categoria: atual.Categoria, Latitude: atual.Latitude, Longitude: atual.Longitude, Altitude: atual.Altitude,
		DistanciaCentro: atual.DistanciaCentro, Producao: atual.Producao,
	}
	a, dep := audit.Diferenca(dadosAuditoriaBairro(antesP, nomeCidade(atual.CidadeID)), dadosAuditoriaBairro(params, nomeCidade(b.CidadeID)))
	if len(a)+len(dep) > 0 {
		h.auditarCom(auditCli(qt), r, audit.Evento{Acao: "bairro.editar", Entidade: "bairro", EntidadeID: strconv.FormatInt(b.ID, 10), Rotulo: b.Nome, Antes: a, Depois: dep})
	}
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar bairro", err)
		return
	}
	httpx.JSON(w, http.StatusOK, bairroParaAPI(b))
}

// ------------------------------------------------------------------- Quarteirões

type apiQuarteirao struct {
	ExcluidoEm   any       `json:"excluido_em"` // preenchido se o registro foi excluído (só aparece com updated_since)
	ID           int64     `json:"id"`
	BairroID     int64     `json:"bairro_id"`
	Nome         string    `json:"nome"`
	Ativo        bool      `json:"ativo"`
	IDExterno    any       `json:"id_externo"`
	CriadoEm     time.Time `json:"criado_em"`
	AtualizadoEm time.Time `json:"atualizado_em"`
}

func quarteiraoParaAPI(x sqlccliente.Quarteiro) apiQuarteirao {
	return apiQuarteirao{ID: x.ID, BairroID: x.BairroID, Nome: x.Nome, Ativo: x.Ativo, IDExterno: optTexto(x.IDExterno), CriadoEm: tempo(x.CriadoEm), AtualizadoEm: tempo(x.AtualizadoEm), ExcluidoEm: optTempo(x.DeletedAt)}
}

func (h *handlers) apiQuarteiroesLista(w http.ResponseWriter, r *http.Request) {
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
	bairro, ok1 := inteiroDaURL(r, "bairro_id")
	ativo, ok2 := logicoDaURL(r, "ativo")
	desde, ok3 := desdeDaURL(r)
	switch {
	case !ok1:
		h.apiParametroInvalido(w, r, "bairro_id")
		return
	case !ok2:
		h.apiParametroInvalido(w, r, "ativo (true ou false)")
		return
	case !ok3:
		h.apiParametroInvalido(w, r, "updated_since (use RFC 3339, como 2026-10-29T12:00:00Z)")
		return
	}
	limite := limiteDaURL(r)
	rows, err := q.ListQuarteiroesApi(r.Context(), sqlccliente.ListQuarteiroesApiParams{
		Depois: depois, BairroID: bairro, Ativo: ativo, IDExterno: r.URL.Query().Get("id_externo"), AtualizadoDesde: desde, Limite: limite + 1, IncluirExcluidos: desde.Valid,
	})
	if err != nil {
		h.falhaInterna(w, r, "listar quarteirões", err)
		return
	}
	mais := int32(len(rows)) > limite
	if mais {
		rows = rows[:limite]
	}
	dados := make([]apiQuarteirao, len(rows))
	var ultimo int64
	for i, x := range rows {
		dados[i], ultimo = quarteiraoParaAPI(x), x.ID
	}
	h.paginaAPI(w, dados, ultimo, mais)
}

func (h *handlers) apiQuarteiraoDaURL(w http.ResponseWriter, r *http.Request) (sqlccliente.Quarteiro, *sqlccliente.Queries, bool) {
	id, ok := idDaURLAPI(r)
	if !ok {
		h.apiNaoEncontrado(w, r, "Quarteirão")
		return sqlccliente.Quarteiro{}, nil, false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return sqlccliente.Quarteiro{}, nil, false
	}
	x, err := q.GetQuarteirao(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		h.apiNaoEncontrado(w, r, "Quarteirão")
		return x, nil, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar quarteirão", err)
		return x, nil, false
	}
	return x, q, true
}

func (h *handlers) apiQuarteiraoObter(w http.ResponseWriter, r *http.Request) {
	if x, _, ok := h.apiQuarteiraoDaURL(w, r); ok {
		httpx.JSON(w, http.StatusOK, quarteiraoParaAPI(x))
	}
}

func validarNomeQuarteirao(nome string) string {
	if n := len([]rune(nome)); n < 1 || n > 30 {
		return "O nome do quarteirão deve ter de 1 a 30 caracteres."
	}
	return ""
}

func (h *handlers) apiQuarteiraoCriar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, ok := h.lerCorpo(w, r, "bairro_id", "nome", "ativo", "id_externo")
	if !ok {
		return
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	ext, err := c.idExterno()
	if err != nil {
		h.apiErroCampo(w, r, "id_externo", err.Error())
		return
	}
	if ext != "" {
		if x, err := q.GetQuarteiraoPorIdExterno(ctx, pgtype.Text{String: ext, Valid: true}); err == nil {
			httpx.JSON(w, http.StatusOK, quarteiraoParaAPI(x))
			return
		} else if !errors.Is(err, pgx.ErrNoRows) {
			h.falhaInterna(w, r, "buscar por id_externo", err)
			return
		}
	}
	bid, e1 := c.inteiro("bairro_id", 0)
	nome, e2 := c.texto("nome", "")
	ativo, e3 := c.logico("ativo", true)
	if err := errors.Join(e1, e2, e3); err != nil {
		h.apiErroCampo(w, r, "corpo", err.Error())
		return
	}
	if msg := validarNomeQuarteirao(nome); msg != "" {
		h.apiErroCampo(w, r, "nome", msg)
		return
	}
	b, err := q.GetBairro(ctx, bid)
	if errors.Is(err, pgx.ErrNoRows) {
		h.apiErroCampo(w, r, "bairro_id", "informe um bairro existente")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar bairro", err)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	x, err := qt.CreateQuarteiraoApi(ctx, sqlccliente.CreateQuarteiraoApiParams{BairroID: b.ID, Nome: nome, Ativo: ativo, IDExterno: pgtype.Text{String: ext, Valid: ext != ""}})
	if violacaoUnica(err) {
		h.apiConflito(w, r, "Já existe um quarteirão com esse nome (ou esse id_externo) neste bairro.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "criar quarteirão", err)
		return
	}
	det := map[string]any{"criados": 1, "ja_existiam": 0, "quarteiroes": []string{nome}}
	if ext != "" {
		det["id_externo"] = ext
	}
	h.auditarCom(auditCli(qt), r, audit.Evento{Acao: "quarteirao.criar", Entidade: "bairro", EntidadeID: strconv.FormatInt(b.ID, 10), Rotulo: b.Nome, Detalhes: det})
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar quarteirão", err)
		return
	}
	w.Header().Set("Location", "/api/v1/quarteiroes/"+strconv.FormatInt(x.ID, 10))
	httpx.JSON(w, http.StatusCreated, quarteiraoParaAPI(x))
}

func (h *handlers) apiQuarteiraoAlterar(w http.ResponseWriter, r *http.Request) {
	atual, q, ok := h.apiQuarteiraoDaURL(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	c, ok := h.lerCorpo(w, r, "nome", "ativo", "id_externo")
	if !ok {
		return
	}
	ext, err := c.idExterno()
	if err != nil {
		h.apiErroCampo(w, r, "id_externo", err.Error())
		return
	}
	if ext != "" && atual.IDExterno.Valid && atual.IDExterno.String != ext {
		h.apiErroCampo(w, r, "id_externo", "o id_externo de um registro não muda depois de definido")
		return
	}
	nome, e1 := c.texto("nome", atual.Nome)
	ativo, e2 := c.logico("ativo", atual.Ativo)
	if err := errors.Join(e1, e2); err != nil {
		h.apiErroCampo(w, r, "corpo", err.Error())
		return
	}
	if msg := validarNomeQuarteirao(nome); msg != "" {
		h.apiErroCampo(w, r, "nome", msg)
		return
	}
	x, err := q.UpdateQuarteirao(ctx, sqlccliente.UpdateQuarteiraoParams{ID: atual.ID, Nome: nome, Ativo: ativo})
	if violacaoUnica(err) {
		h.apiConflito(w, r, "Já existe um quarteirão com esse nome neste bairro.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "alterar quarteirão", err)
		return
	}
	if ext != "" && !atual.IDExterno.Valid {
		if x, err = q.SetQuarteiraoIdExterno(ctx, sqlccliente.SetQuarteiraoIdExternoParams{ID: x.ID, IDExterno: pgtype.Text{String: ext, Valid: true}}); violacaoUnica(err) {
			h.apiConflito(w, r, "Este id_externo já está em uso por outro quarteirão.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "gravar id_externo", err)
			return
		}
	}
	a, dep := audit.Diferenca(map[string]any{"nome": atual.Nome, "ativo": atual.Ativo}, map[string]any{"nome": x.Nome, "ativo": x.Ativo})
	if len(a)+len(dep) > 0 {
		b, _ := q.GetBairro(ctx, x.BairroID)
		h.auditar(r, audit.Evento{Acao: "quarteirao.editar", Entidade: "bairro", EntidadeID: strconv.FormatInt(x.BairroID, 10), Rotulo: b.Nome + " · quarteirão " + atual.Nome, Antes: a, Depois: dep})
	}
	httpx.JSON(w, http.StatusOK, quarteiraoParaAPI(x))
}

// ------------------------------------------------------------------- Locais

type apiLocal struct {
	ExcluidoEm           any       `json:"excluido_em"` // preenchido se o registro foi excluído (só aparece com updated_since)
	ID                   int64     `json:"id"`
	Tipo                 string    `json:"tipo"`
	CEP                  string    `json:"cep"`
	Endereco             string    `json:"endereco"`
	Numero               string    `json:"numero"`
	Referencia           string    `json:"referencia"`
	BairroID             int64     `json:"bairro_id"`
	QuarteiraoID         int64     `json:"quarteirao_id"`
	Lado                 string    `json:"lado"`
	Sequencia            any       `json:"sequencia"`
	Complemento          any       `json:"complemento"`
	Latitude             any       `json:"latitude"`
	Longitude            any       `json:"longitude"`
	Ativo                bool      `json:"ativo"`
	Chave                string    `json:"chave_qr"`
	Proprietario         string    `json:"proprietario"`
	ProprietarioTelefone string    `json:"proprietario_telefone"`
	ProprietarioWhatsapp string    `json:"proprietario_whatsapp"`
	ProprietarioEmail    string    `json:"proprietario_email"`
	AceitaWhatsapp       bool      `json:"aceita_whatsapp"`
	AceitaEmail          bool      `json:"aceita_email"`
	Inquilino            string    `json:"inquilino"`
	InquilinoTelefone    string    `json:"inquilino_telefone"`
	Humanos              any       `json:"humanos"`
	Cachorros            any       `json:"cachorros"`
	Gatos                any       `json:"gatos"`
	Roedores             any       `json:"roedores"`
	IDExterno            any       `json:"id_externo"`
	CriadoEm             time.Time `json:"criado_em"`
	AtualizadoEm         time.Time `json:"atualizado_em"`
}

// localParaAPI não leva o CPF do proprietário: dado pessoal sensível fica só na tela, até a LGPD (etapa 13.3).
func localParaAPI(l sqlccliente.Locai) apiLocal {
	return apiLocal{
		ID: l.ID, Tipo: l.Tipo, CEP: l.Cep, Endereco: l.Endereco, Numero: l.Numero, Referencia: l.Referencia, BairroID: l.BairroID,
		QuarteiraoID: l.QuarteiraoID, Lado: l.Lado, Sequencia: optInt(l.Sequencia), Complemento: optInt(l.Complemento),
		Latitude: optFloat(l.Latitude), Longitude: optFloat(l.Longitude), Ativo: l.Ativo, Chave: l.QrChave, Proprietario: l.Proprietario,
		ProprietarioTelefone: l.ProprietarioTelefone, ProprietarioWhatsapp: l.ProprietarioWhatsapp, ProprietarioEmail: l.ProprietarioEmail,
		AceitaWhatsapp: l.AceitaWhatsapp, AceitaEmail: l.AceitaEmail, Inquilino: l.Inquilino, InquilinoTelefone: l.InquilinoTelefone,
		Humanos: optInt(l.Humanos), Cachorros: optInt(l.Cachorros), Gatos: optInt(l.Gatos), Roedores: optInt(l.Roedores),
		IDExterno: optTexto(l.IDExterno), CriadoEm: tempo(l.CriadoEm), AtualizadoEm: tempo(l.AtualizadoEm), ExcluidoEm: optTempo(l.DeletedAt),
	}
}

func (h *handlers) apiLocaisLista(w http.ResponseWriter, r *http.Request) {
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
	bairro, ok1 := inteiroDaURL(r, "bairro_id")
	quart, ok2 := inteiroDaURL(r, "quarteirao_id")
	ativo, ok3 := logicoDaURL(r, "ativo")
	desde, ok4 := desdeDaURL(r)
	switch {
	case !ok1:
		h.apiParametroInvalido(w, r, "bairro_id")
		return
	case !ok2:
		h.apiParametroInvalido(w, r, "quarteirao_id")
		return
	case !ok3:
		h.apiParametroInvalido(w, r, "ativo (true ou false)")
		return
	case !ok4:
		h.apiParametroInvalido(w, r, "updated_since (use RFC 3339, como 2026-10-29T12:00:00Z)")
		return
	}
	qs := r.URL.Query()
	tipo := qs.Get("tipo")
	if tipo != "" && rotuloTipoLocal(tipo) == tipo {
		h.apiParametroInvalido(w, r, "tipo")
		return
	}
	limite := limiteDaURL(r)
	rows, err := q.ListLocaisApi(r.Context(), sqlccliente.ListLocaisApiParams{
		Depois: depois, BairroID: bairro, QuarteiraoID: quart, Tipo: tipo, Ativo: ativo, Busca: strings.TrimSpace(qs.Get("q")),
		IDExterno: qs.Get("id_externo"), AtualizadoDesde: desde, Limite: limite + 1, IncluirExcluidos: desde.Valid,
	})
	if err != nil {
		h.falhaInterna(w, r, "listar locais", err)
		return
	}
	mais := int32(len(rows)) > limite
	if mais {
		rows = rows[:limite]
	}
	dados := make([]apiLocal, len(rows))
	var ultimo int64
	for i, l := range rows {
		dados[i], ultimo = localParaAPI(l), l.ID
	}
	h.paginaAPI(w, dados, ultimo, mais)
}

func (h *handlers) apiLocalDaURL(w http.ResponseWriter, r *http.Request) (sqlccliente.Locai, *sqlccliente.Queries, bool) {
	id, ok := idDaURLAPI(r)
	if !ok {
		h.apiNaoEncontrado(w, r, "Local")
		return sqlccliente.Locai{}, nil, false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return sqlccliente.Locai{}, nil, false
	}
	l, err := q.GetLocal(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		h.apiNaoEncontrado(w, r, "Local")
		return l, nil, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar local", err)
		return l, nil, false
	}
	return l, q, true
}

func (h *handlers) apiLocalObter(w http.ResponseWriter, r *http.Request) {
	if l, _, ok := h.apiLocalDaURL(w, r); ok {
		httpx.JSON(w, http.StatusOK, localParaAPI(l))
	}
}

var camposDeLocal = []string{"tipo", "cep", "endereco", "numero", "referencia", "bairro_id", "quarteirao_id", "lado", "sequencia", "complemento",
	"latitude", "longitude", "proprietario", "proprietario_telefone", "proprietario_whatsapp", "proprietario_email", "aceita_whatsapp",
	"aceita_email", "inquilino", "inquilino_telefone", "humanos", "cachorros", "gatos", "roedores", "ativo", "id_externo"}

// camposDoLocalAPI aplica o corpo sobre os campos atuais do local (ou vazios, na criação).
func camposDoLocalAPI(c corpoAPI, cur views.LocalCampos) (views.LocalCampos, error) {
	var erros []error
	t := func(nome string, dest *string) {
		v, err := c.texto(nome, *dest)
		erros = append(erros, err)
		*dest = v
	}
	t("tipo", &cur.Tipo)
	t("cep", &cur.CEP)
	t("endereco", &cur.Endereco)
	t("numero", &cur.Numero)
	t("referencia", &cur.Referencia)
	t("bairro_id", &cur.BairroID)
	t("quarteirao_id", &cur.QuarteiraoID)
	t("lado", &cur.Lado)
	t("sequencia", &cur.Sequencia)
	t("complemento", &cur.Complemento)
	t("latitude", &cur.Latitude)
	t("longitude", &cur.Longitude)
	t("proprietario", &cur.Proprietario)
	t("proprietario_telefone", &cur.ProprietarioTelefone)
	t("proprietario_whatsapp", &cur.Whatsapp)
	t("proprietario_email", &cur.Email)
	t("inquilino", &cur.Inquilino)
	t("inquilino_telefone", &cur.InquilinoTelefone)
	t("humanos", &cur.Humanos)
	t("cachorros", &cur.Cachorros)
	t("gatos", &cur.Gatos)
	t("roedores", &cur.Roedores)
	var err error
	if cur.AceitaWhatsapp, err = c.logico("aceita_whatsapp", cur.AceitaWhatsapp); err != nil {
		erros = append(erros, err)
	}
	if cur.AceitaEmail, err = c.logico("aceita_email", cur.AceitaEmail); err != nil {
		erros = append(erros, err)
	}
	return cur, errors.Join(erros...)
}

func (h *handlers) apiLocalCriar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, ok := h.lerCorpo(w, r, camposDeLocal...)
	if !ok {
		return
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	ext, err := c.idExterno()
	if err != nil {
		h.apiErroCampo(w, r, "id_externo", err.Error())
		return
	}
	if ext != "" {
		if l, err := q.GetLocalPorIdExterno(ctx, pgtype.Text{String: ext, Valid: true}); err == nil {
			httpx.JSON(w, http.StatusOK, localParaAPI(l))
			return
		} else if !errors.Is(err, pgx.ErrNoRows) {
			h.falhaInterna(w, r, "buscar por id_externo", err)
			return
		}
	}
	campos, err := camposDoLocalAPI(c, views.LocalCampos{})
	if err != nil {
		h.apiErroCampo(w, r, "corpo", err.Error())
		return
	}
	p, msg, err := validarLocal(ctx, q, campos, 0, 0)
	if err != nil {
		h.falhaInterna(w, r, "validar local", err)
		return
	}
	if msg != "" {
		h.apiErroCampo(w, r, "local", msg)
		return
	}
	ativo, err := c.logico("ativo", true)
	if err != nil {
		h.apiErroCampo(w, r, "ativo", err.Error())
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	l, err := qt.CreateLocal(ctx, p)
	if violacaoUnica(err) {
		h.apiConflito(w, r, "Já existe um local com esse endereço, número, lado, sequência e complemento nesse quarteirão.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "criar local", err)
		return
	}
	if ext != "" {
		if l, err = qt.SetLocalIdExterno(ctx, sqlccliente.SetLocalIdExternoParams{ID: l.ID, IDExterno: pgtype.Text{String: ext, Valid: true}}); violacaoUnica(err) {
			h.apiConflito(w, r, "Este id_externo já está em uso por outro local.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "gravar id_externo", err)
			return
		}
	}
	if !ativo {
		if _, err := qt.SetLocalAtivo(ctx, sqlccliente.SetLocalAtivoParams{ID: l.ID, Ativo: false}); err != nil {
			h.falhaInterna(w, r, "desativar local", err)
			return
		}
		l.Ativo = false
	}
	nb, nq := nomesDoLocal(ctx, qt, p.BairroID, p.QuarteiraoID)
	ev := audit.Evento{Acao: "local.criar", Entidade: "local", EntidadeID: strconv.FormatInt(l.ID, 10), Rotulo: enderecoCompleto(l.Endereco, l.Numero), Depois: dadosAuditoriaLocal(p, nb, nq)}
	if ext != "" {
		ev.Detalhes = map[string]any{"id_externo": ext}
	}
	h.auditarCom(auditCli(qt), r, ev)
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar local", err)
		return
	}
	w.Header().Set("Location", "/api/v1/locais/"+strconv.FormatInt(l.ID, 10))
	httpx.JSON(w, http.StatusCreated, localParaAPI(l))
}

func (h *handlers) apiLocalAlterar(w http.ResponseWriter, r *http.Request) {
	atual, q, ok := h.apiLocalDaURL(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	c, ok := h.lerCorpo(w, r, camposDeLocal...)
	if !ok {
		return
	}
	ext, err := c.idExterno()
	if err != nil {
		h.apiErroCampo(w, r, "id_externo", err.Error())
		return
	}
	if ext != "" && atual.IDExterno.Valid && atual.IDExterno.String != ext {
		h.apiErroCampo(w, r, "id_externo", "o id_externo de um registro não muda depois de definido")
		return
	}
	campos, err := camposDoLocalAPI(c, camposDoLocal(atual))
	if err != nil {
		h.apiErroCampo(w, r, "corpo", err.Error())
		return
	}
	p, msg, err := validarLocal(ctx, q, campos, atual.BairroID, atual.QuarteiraoID)
	if err != nil {
		h.falhaInterna(w, r, "validar local", err)
		return
	}
	if msg != "" {
		h.apiErroCampo(w, r, "local", msg)
		return
	}
	ativo, err := c.logico("ativo", atual.Ativo)
	if err != nil {
		h.apiErroCampo(w, r, "ativo", err.Error())
		return
	}
	_, pool, _ := h.cliente(r)
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	l, err := qt.UpdateLocal(ctx, sqlccliente.UpdateLocalParams{
		ID: atual.ID, Tipo: p.Tipo, Cep: p.Cep, Endereco: p.Endereco, Numero: p.Numero, Referencia: p.Referencia, BairroID: p.BairroID,
		QuarteiraoID: p.QuarteiraoID, Lado: p.Lado, Sequencia: p.Sequencia, Complemento: p.Complemento, Latitude: p.Latitude,
		Longitude: p.Longitude, Proprietario: p.Proprietario, ProprietarioTelefone: p.ProprietarioTelefone, ProprietarioCpf: p.ProprietarioCpf,
		ProprietarioWhatsapp: p.ProprietarioWhatsapp, ProprietarioEmail: p.ProprietarioEmail, AceitaWhatsapp: p.AceitaWhatsapp,
		AceitaEmail: p.AceitaEmail, Inquilino: p.Inquilino, InquilinoTelefone: p.InquilinoTelefone, Humanos: p.Humanos,
		Cachorros: p.Cachorros, Gatos: p.Gatos, Roedores: p.Roedores,
	})
	if violacaoUnica(err) {
		h.apiConflito(w, r, "Já existe um local com esse endereço, número, lado, sequência e complemento nesse quarteirão.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "alterar local", err)
		return
	}
	if ext != "" && !atual.IDExterno.Valid {
		if l, err = qt.SetLocalIdExterno(ctx, sqlccliente.SetLocalIdExternoParams{ID: l.ID, IDExterno: pgtype.Text{String: ext, Valid: true}}); violacaoUnica(err) {
			h.apiConflito(w, r, "Este id_externo já está em uso por outro local.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "gravar id_externo", err)
			return
		}
	}
	rotulo := enderecoCompleto(l.Endereco, l.Numero)
	if ativo != atual.Ativo {
		if _, err := qt.SetLocalAtivo(ctx, sqlccliente.SetLocalAtivoParams{ID: l.ID, Ativo: ativo}); err != nil {
			h.falhaInterna(w, r, "mudar situação do local", err)
			return
		}
		l.Ativo = ativo
		acao := "local.desativar"
		if ativo {
			acao = "local.reativar"
		}
		h.auditarCom(auditCli(qt), r, audit.Evento{Acao: acao, Entidade: "local", EntidadeID: strconv.FormatInt(l.ID, 10), Rotulo: rotulo, Antes: map[string]any{"ativo": atual.Ativo}, Depois: map[string]any{"ativo": ativo}})
	}
	nbA, nqA := nomesDoLocal(ctx, qt, atual.BairroID, atual.QuarteiraoID)
	nbD, nqD := nomesDoLocal(ctx, qt, p.BairroID, p.QuarteiraoID)
	a, dep := audit.Diferenca(dadosAuditoriaLocal(paramsDoLocal(atual), nbA, nqA), dadosAuditoriaLocal(p, nbD, nqD))
	if len(a)+len(dep) > 0 {
		h.auditarCom(auditCli(qt), r, audit.Evento{Acao: "local.editar", Entidade: "local", EntidadeID: strconv.FormatInt(l.ID, 10), Rotulo: rotulo, Antes: a, Depois: dep})
	}
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar local", err)
		return
	}
	httpx.JSON(w, http.StatusOK, localParaAPI(l))
}

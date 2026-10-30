package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/apiauth"
	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/httpx"
	"uysiarbo/internal/importacao"
	"uysiarbo/internal/tenant"
)

// Importação pela API: outro sistema empurra linhas em JSON. Passa pela MESMA análise, prévia e execução das telas.
// Fluxo: POST /importacoes (analisa e devolve a prévia) -> GET /importacoes/{id} e /linhas (conferir) -> POST /importacoes/{id}/executar.
// A credencial precisa do escopo importacoes:write E do escopo de gravação do tipo de dado (por exemplo, locais:write).

type apiImportacao struct {
	ID               int64        `json:"id"`
	Tipo             string       `json:"tipo"`
	Status           string       `json:"status"`
	Origem           string       `json:"origem"`
	AtualizarExist   bool         `json:"atualizar_existentes"`
	TotalLinhas      int32        `json:"total_linhas"`
	Previa           apiPrevia    `json:"previa"`
	Andamento        apiAndamento `json:"andamento"`
	ColunasIgnoradas []string     `json:"colunas_ignoradas"`
	Mensagem         string       `json:"mensagem"`
	CriadoEm         time.Time    `json:"criado_em"`
	ConcluidoEm      any          `json:"concluido_em"`
}

type apiPrevia struct {
	Criar     int32 `json:"criar"`
	Atualizar int32 `json:"atualizar"`
	Ignorar   int32 `json:"ignorar"`
	Erro      int32 `json:"erro"`
}

type apiAndamento struct {
	Processadas int32 `json:"processadas"`
	Criados     int32 `json:"criados"`
	Atualizados int32 `json:"atualizados"`
	Ignorados   int32 `json:"ignorados"`
	Erros       int32 `json:"erros"`
	Percentual  int32 `json:"percentual"`
}

func importacaoParaAPI(i sqlccliente.Importaco) apiImportacao {
	var ign []string
	if i.ColunasIgnoradas != "" {
		ign = strings.Split(i.ColunasIgnoradas, ", ")
	}
	var pct int32
	if i.TotalLinhas > 0 {
		pct = i.Processadas * 100 / i.TotalLinhas
	}
	return apiImportacao{
		ID: i.ID, Tipo: i.Tipo, Status: i.Status, Origem: i.Conector, AtualizarExist: i.AtualizarExistentes, TotalLinhas: i.TotalLinhas,
		Previa:           apiPrevia{Criar: i.PrevCriar, Atualizar: i.PrevAtualizar, Ignorar: i.PrevIgnorar, Erro: i.PrevErro},
		Andamento:        apiAndamento{Processadas: i.Processadas, Criados: i.Criados, Atualizados: i.Atualizados, Ignorados: i.Ignorados, Erros: i.Erros, Percentual: pct},
		ColunasIgnoradas: ign, Mensagem: i.Mensagem, CriadoEm: tempo(i.CriadoEm), ConcluidoEm: optTempo(i.ConcluidoEm),
	}
}

// apiImportacaoDaURL carrega a importação do endereço (e acerta a que ficou "executando" sem motor depois de um reinício).
func (h *handlers) apiImportacaoDaURL(w http.ResponseWriter, r *http.Request) (sqlccliente.Importaco, *sqlccliente.Queries, string, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		h.apiNaoEncontrado(w, r, "Importação")
		return sqlccliente.Importaco{}, nil, "", false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return sqlccliente.Importaco{}, nil, "", false
	}
	imp, err := q.GetImportacao(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && imp.Status == "mapeando") {
		h.apiNaoEncontrado(w, r, "Importação")
		return imp, nil, "", false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar importação", err)
		return imp, nil, "", false
	}
	schema := h.schemaDoCliente(r)
	if imp.Status == "executando" && !h.imp.esta(chaveImp(schema, imp.ID)) {
		_ = q.SetImportacaoStatus(r.Context(), sqlccliente.SetImportacaoStatusParams{ID: imp.ID, Status: "interrompida", Mensagem: "Interrompida (o servidor reiniciou)."})
		imp.Status, imp.Mensagem = "interrompida", "Interrompida (o servidor reiniciou)."
	}
	return imp, q, schema, true
}

func (h *handlers) schemaDoCliente(r *http.Request) string {
	c, _ := tenant.FromContext(r.Context())
	return c.SchemaNome
}

func (h *handlers) apiImportacaoCriar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, ok := h.lerCorpo(w, r, "tipo", "atualizar_existentes", "linhas")
	if !ok {
		return
	}
	tipoCod, e1 := c.texto("tipo", "")
	atualizar, e2 := c.logico("atualizar_existentes", false)
	if err := errors.Join(e1, e2); err != nil {
		h.apiErroCampo(w, r, "corpo", err.Error())
		return
	}
	tipo, ok := importacao.TipoPorCodigo(tipoCod)
	if !ok {
		h.apiErroCampo(w, r, "tipo", "use bairros, quarteiroes, locais ou ciclos")
		return
	}
	cred, _ := credencialDe(ctx)
	if !apiauth.Permite(cred.Escopos, tipoCod+":write") {
		h.apiErro(w, r, http.StatusForbidden, "insufficient_scope", "Importar "+tipoCod+" também exige o escopo "+tipoCod+":write.")
		return
	}
	var objetos []map[string]any
	dec := json.NewDecoder(strings.NewReader(string(c["linhas"])))
	dec.UseNumber()
	if err := dec.Decode(&objetos); err != nil {
		h.apiErroCampo(w, r, "linhas", "envie uma lista de objetos, um por linha, com as colunas do modelo")
		return
	}
	plan, err := importacao.PlanilhaDeObjetos(objetos)
	if err != nil {
		h.apiErroCampo(w, r, "linhas", err.Error())
		return
	}
	mapa := tipo.Mapear(plan.Cabecalho)
	if len(mapa.Faltando) > 0 {
		h.apiErroCampo(w, r, "linhas", "faltam colunas obrigatórias: "+strings.Join(mapa.Faltando, ", "))
		return
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	soma := sha256.Sum256(c["linhas"])
	imp, err := h.gravarAnalise(ctx, r, pool, q, paramsAnalise{
		Tipo: tipoCod, Plan: plan, Mapa: mapa, Atualizar: atualizar, Conector: "api_sistema", Info: cred.ClientID, Nome: "API: " + cred.Nome, Hash: soma[:],
	})
	if err != nil {
		h.falhaInterna(w, r, "analisar", err)
		return
	}
	w.Header().Set("Location", "/api/v1/importacoes/"+strconv.FormatInt(imp.ID, 10))
	httpx.JSON(w, http.StatusCreated, importacaoParaAPI(imp))
}

func (h *handlers) apiImportacaoObter(w http.ResponseWriter, r *http.Request) {
	if imp, _, _, ok := h.apiImportacaoDaURL(w, r); ok {
		httpx.JSON(w, http.StatusOK, importacaoParaAPI(imp))
	}
}

type apiImportacaoLinha struct {
	Linha        int32  `json:"linha"`
	Acao         string `json:"acao"`
	Mensagem     string `json:"mensagem"`
	Resultado    any    `json:"resultado"`
	ResultadoMsg string `json:"resultado_msg"`
	RegistroID   any    `json:"registro_id"`
}

func (h *handlers) apiImportacaoLinhas(w http.ResponseWriter, r *http.Request) {
	imp, q, _, ok := h.apiImportacaoDaURL(w, r)
	if !ok {
		return
	}
	depois, okc := lerCursor(r)
	if !okc {
		h.apiErro(w, r, http.StatusBadRequest, "invalid_cursor", "cursor inválido: use o next_cursor da resposta anterior.")
		return
	}
	acao := r.URL.Query().Get("acao")
	if !slices.Contains([]string{"", "criar", "atualizar", "ignorar", "erro"}, acao) {
		h.apiParametroInvalido(w, r, "acao (criar, atualizar, ignorar ou erro)")
		return
	}
	limite := limiteDaURL(r)
	rows, err := q.ListImportacaoLinhasApi(r.Context(), sqlccliente.ListImportacaoLinhasApiParams{ImportacaoID: imp.ID, Acao: acao, Depois: int32(depois), Limite: limite + 1})
	if err != nil {
		h.falhaInterna(w, r, "listar linhas", err)
		return
	}
	mais := int32(len(rows)) > limite
	if mais {
		rows = rows[:limite]
	}
	dados := make([]apiImportacaoLinha, len(rows))
	var ultimo int64
	for i, x := range rows {
		var res, reg any
		if x.Resultado.Valid {
			res = x.Resultado.String
		}
		if x.RegistroID.Valid {
			reg = x.RegistroID.Int64
		}
		dados[i] = apiImportacaoLinha{Linha: x.Linha, Acao: x.Acao, Mensagem: x.Mensagem, Resultado: res, ResultadoMsg: x.ResultadoMsg, RegistroID: reg}
		ultimo = int64(x.Linha)
	}
	h.paginaAPI(w, dados, ultimo, mais)
}

func (h *handlers) apiImportacaoExecutar(w http.ResponseWriter, r *http.Request) {
	imp, q, schema, ok := h.apiImportacaoDaURL(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	_, pool, _ := h.cliente(r)
	if imp.Status != "analisada" && imp.Status != "interrompida" {
		h.apiConflito(w, r, "A importação está "+rotulosDeStatusImp[imp.Status]+" e não pode ser iniciada agora.")
		return
	}
	if n, err := q.ContarImportacoesEmExecucao(ctx); err != nil {
		h.falhaInterna(w, r, "contar execuções", err)
		return
	} else if n > 0 {
		w.Header().Set("Retry-After", "5")
		h.apiConflito(w, r, "Já há uma importação em andamento neste ambiente.")
		return
	}
	if n, _ := q.ContarImportacaoLinhasAFazer(ctx, imp.ID); n == 0 {
		h.apiConflito(w, r, "Não há linhas a gravar nesta importação (confira a prévia).")
		return
	}
	if n, err := q.IniciarImportacao(ctx, imp.ID); err != nil {
		h.falhaInterna(w, r, "iniciar importação", err)
		return
	} else if n == 0 {
		h.apiConflito(w, r, "A importação não pode ser iniciada agora.")
		return
	}
	if !h.iniciarExecucao(pool, schema, imp.ID, origemDaImportacao{Ator: atorDaSessao(r), IP: ipDe(r), UserAgent: r.UserAgent(), RequestID: r.Header.Get("X-Request-Id")}) {
		_ = q.SetImportacaoStatus(ctx, sqlccliente.SetImportacaoStatusParams{ID: imp.ID, Status: imp.Status, Mensagem: ""})
		h.apiConflito(w, r, "Esta importação já está rodando.")
		return
	}
	h.auditar(r, audit.Evento{Acao: "importacao.iniciar", Entidade: "importacao", EntidadeID: strconv.FormatInt(imp.ID, 10), Rotulo: imp.ArquivoNome,
		Detalhes: map[string]any{"tipo": imp.Tipo, "a_criar": imp.PrevCriar, "a_atualizar": imp.PrevAtualizar}})
	atual, _ := q.GetImportacao(ctx, imp.ID)
	httpx.JSON(w, http.StatusAccepted, importacaoParaAPI(atual))
}

func (h *handlers) apiImportacaoDescartar(w http.ResponseWriter, r *http.Request) {
	imp, q, _, ok := h.apiImportacaoDaURL(w, r)
	if !ok {
		return
	}
	if imp.Status != "analisada" {
		h.apiConflito(w, r, "Só dá para descartar uma importação que ainda aguarda confirmação.")
		return
	}
	if err := q.CancelarImportacao(r.Context(), sqlccliente.CancelarImportacaoParams{ID: imp.ID, Mensagem: "Descartada antes de gravar: nada foi importado."}); err != nil {
		h.falhaInterna(w, r, "descartar importação", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "importacao.cancelar", Entidade: "importacao", EntidadeID: strconv.FormatInt(imp.ID, 10), Rotulo: imp.ArquivoNome})
	atual, _ := q.GetImportacao(r.Context(), imp.ID)
	httpx.JSON(w, http.StatusOK, importacaoParaAPI(atual))
}

var _ = context.Background
var _ = fmt.Sprintf

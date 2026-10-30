package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/importacao"
	"uysiarbo/web/views"
)

// ------------------------------------------------------------------- Planilha guardada (etapa "mapeando")

type linhaGuardada struct {
	N int      `json:"n"`
	C []string `json:"c"`
}

type planilhaGuardada struct {
	Cabecalho []string        `json:"c"`
	Linhas    []linhaGuardada `json:"l"`
}

func guardarPlanilha(p importacao.Planilha) []byte {
	g := planilhaGuardada{Cabecalho: p.Cabecalho, Linhas: make([]linhaGuardada, len(p.Linhas))}
	for i, l := range p.Linhas {
		g.Linhas[i] = linhaGuardada{N: l.N, C: l.Celulas}
	}
	return emJSON(g)
}

func lerPlanilhaGuardada(b []byte) (importacao.Planilha, error) {
	var g planilhaGuardada
	if err := json.Unmarshal(b, &g); err != nil {
		return importacao.Planilha{}, err
	}
	p := importacao.Planilha{Cabecalho: g.Cabecalho, Linhas: make([]importacao.Linha, len(g.Linhas))}
	for i, l := range g.Linhas {
		p.Linhas[i] = importacao.Linha{N: l.N, Celulas: l.C}
	}
	return p, nil
}

// ------------------------------------------------------------------- Formulário

func (h *handlers) dadosDoFormularioDeImportacao(r *http.Request, q *sqlccliente.Queries, conector, tipo string, atualizar, revisar bool, urlApi, caminho, erro string) views.ImportacaoNovaData {
	pr := h.principal(r)
	if _, ok := importacao.ConectorPorCodigo(conector); !ok {
		conector = "planilha"
	}
	d := views.ImportacaoNovaData{
		Conector: conector, Atualizar: atualizar, Revisar: revisar, URL: urlApi, Caminho: caminho,
		Colunas: map[string][]views.ImportacaoColunaAjuda{}, MaxMB: importacao.MaxBytes >> 20, MaxLinhas: importacao.MaxLinhas,
		Erro: erro, CSRF: csrfToken(r.Context()),
	}
	for _, c := range importacao.Conectores {
		d.Conectores = append(d.Conectores, views.ImportacaoConectorOpcao{Codigo: c.Codigo, Rotulo: c.Rotulo, Descr: c.Descr, Fonte: c.Fonte, Marcado: c.Codigo == conector})
	}
	for _, t := range importacao.Tipos {
		if !pr.Pode(permissaoDoTipo(t.Codigo)) {
			continue
		}
		d.Tipos = append(d.Tipos, views.ImportacaoTipoOpcao{Codigo: t.Codigo, Rotulo: t.Rotulo, Descr: t.Descr, Marcado: t.Codigo == tipo})
		for _, c := range t.Colunas {
			d.Colunas[t.Codigo] = append(d.Colunas[t.Codigo], views.ImportacaoColunaAjuda{Nome: c.Nome, Rotulo: c.Rotulo, Obrigatoria: c.Obrigatoria, Ajuda: c.Ajuda})
		}
	}
	if q != nil {
		if ms, err := q.ListTodosMapeamentosDeImportacao(r.Context()); err == nil {
			for _, m := range ms {
				d.Mapeamentos = append(d.Mapeamentos, views.ImportacaoMapeamentoOpcao{ID: m.ID, Rotulo: m.Nome + " (" + m.Conector + " · " + m.Tipo + ")"})
			}
		}
	}
	return d
}

func (h *handlers) importacaoNova(w http.ResponseWriter, r *http.Request) {
	if h.papelRestrito(r) {
		renderStatus(w, r, http.StatusForbidden, views.SemPermissao(h.layout(r, "Sem permissão", "")))
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	render(w, r, views.ImportacaoNova(h.layout(r, "Nova importação", "/importacoes"), h.dadosDoFormularioDeImportacao(r, q, "planilha", "", false, false, "", "", "")))
}

func (h *handlers) formularioDeImportacaoComErro(w http.ResponseWriter, r *http.Request, conector, tipo string, atualizar, revisar bool, urlApi, caminho, msg string) {
	q, _, _ := h.cliente(r)
	renderStatus(w, r, http.StatusBadRequest, views.ImportacaoNova(h.layout(r, "Nova importação", "/importacoes"), h.dadosDoFormularioDeImportacao(r, q, conector, tipo, atualizar, revisar, urlApi, caminho, msg)))
}

// ------------------------------------------------------------------- Enviar

func (h *handlers) importacaoEnviar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.papelRestrito(r) {
		renderStatus(w, r, http.StatusForbidden, views.SemPermissao(h.layout(r, "Sem permissão", "")))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, importacao.MaxBytes+(2<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		h.formularioDeImportacaoComErro(w, r, "", "", false, false, "", "", fmt.Sprintf("Não consegui receber o arquivo (o limite é %d MB).", importacao.MaxBytes>>20))
		return
	}
	v := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	conectorCod, tipoCod := v("conector"), v("tipo")
	if conectorCod == "" {
		conectorCod = "planilha"
	}
	atualizar, revisar := v("atualizar") == "1", v("revisar") == "1"
	urlApi, caminho := v("url"), v("caminho")
	erro := func(msg string) {
		h.formularioDeImportacaoComErro(w, r, conectorCod, tipoCod, atualizar, revisar, urlApi, caminho, msg)
	}

	conector, ok := importacao.ConectorPorCodigo(conectorCod)
	if !ok {
		conectorCod = "planilha"
		erro("Escolha de onde vêm os dados.")
		return
	}
	tipo, ok := importacao.TipoPorCodigo(tipoCod)
	if !ok || !h.principal(r).Pode(permissaoDoTipo(tipoCod)) {
		erro("Escolha o que importar.")
		return
	}
	var (
		plan  importacao.Planilha
		nome  string
		info  string
		dados []byte
		err   error
	)
	if conector.Fonte == "api" {
		cfg := importacao.ConfigREST{URL: urlApi, Token: v("token"), Usuario: v("usuario"), Senha: r.PostFormValue("senha"), Caminho: caminho}
		cctx, cancel := context.WithTimeout(ctx, 40*time.Second)
		defer cancel()
		if dados, err = importacao.BuscarJSON(cctx, cfg, h.importPermitirRedePrivada); err != nil {
			erro("Não foi possível buscar os dados da API: " + err.Error() + ".")
			return
		}
		if plan, err = importacao.PlanilhaDeJSON(dados, caminho); err != nil {
			erro("Dados da API recusados: " + err.Error() + ".")
			return
		}
		if u, e := url.Parse(cfg.URL); e == nil {
			info = u.Host // só o servidor: o caminho e os parâmetros do endereço podem ter segredos
		}
		nome = "API " + info
	} else {
		arq, cab, e := r.FormFile("arquivo")
		if e != nil {
			erro("Escolha o arquivo da planilha.")
			return
		}
		defer arq.Close()
		if dados, err = io.ReadAll(io.LimitReader(arq, importacao.MaxBytes+1)); err != nil {
			erro("Não consegui ler o arquivo.")
			return
		}
		if plan, err = importacao.Ler(cab.Filename, dados); err != nil {
			erro("Planilha recusada: " + err.Error() + ".")
			return
		}
		nome = nomeSeguroDoArquivo(cab.Filename)
	}

	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	var explicito map[string]string
	if idM, _ := strconv.ParseInt(v("mapeamento_id"), 10, 64); idM != 0 {
		m, e := q.GetMapeamentoDeImportacao(ctx, idM)
		if e != nil || m.Conector != conectorCod || m.Tipo != tipoCod {
			erro("O mapeamento escolhido não serve para esta origem e este tipo de dado.")
			return
		}
		_ = json.Unmarshal(m.Mapa, &explicito)
	}
	mapa := tipo.MapearComo(plan.Cabecalho, explicito, conector.Candidatos[tipoCod])
	soma := sha256.Sum256(dados)

	if len(mapa.Faltando) > 0 || revisar {
		a := atorDaSessao(r)
		imp, err := q.CreateImportacaoParaMapear(ctx, sqlccliente.CreateImportacaoParaMapearParams{
			Tipo: tipoCod, Conector: conectorCod, ArquivoNome: nome, ArquivoHash: soma[:], AtualizarExistentes: atualizar, TotalLinhas: int32(len(plan.Linhas)),
			ResponsavelID: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, ResponsavelNome: a.Nome, Cabecalho: emJSON(plan.Cabecalho),
			Arquivo: guardarPlanilha(plan), OrigemInfo: info,
		})
		if err != nil {
			h.falhaInterna(w, r, "guardar a importação", err)
			return
		}
		url := fmt.Sprintf("/importacoes/%d/mapear", imp.ID)
		if explicito != nil {
			url += "?mapeamento=" + v("mapeamento_id")
		}
		http.Redirect(w, r, url, http.StatusSeeOther)
		return
	}
	imp, err := h.gravarAnalise(ctx, r, pool, q, paramsAnalise{
		Tipo: tipoCod, Plan: plan, Mapa: mapa, Atualizar: atualizar, Conector: conectorCod, Info: info, Nome: nome, Hash: soma[:],
	})
	if err != nil {
		h.falhaInterna(w, r, "analisar", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/importacoes/%d", imp.ID), http.StatusSeeOther)
}

// ------------------------------------------------------------------- Análise e gravação (usada pela tela, pelo mapeamento e pela API)

type paramsAnalise struct {
	Tipo        string
	Plan        importacao.Planilha
	Mapa        importacao.Mapeamento
	Atualizar   bool
	Conector    string
	Info        string
	Nome        string
	Hash        []byte
	ExistenteID int64 // importação que estava em "mapeando" (0 = criar uma nova)
}

func (h *handlers) analisarPlanilha(ctx context.Context, r *http.Request, q *sqlccliente.Queries, tipo string, p importacao.Planilha, m importacao.Mapeamento, atualizar bool) ([]linhaAnalisada, error) {
	switch tipo {
	case "ciclos":
		return h.analisarCiclos(ctx, q, p, m, atualizar)
	case "visitas":
		return h.analisarVisitas(ctx, r, q, p, m)
	case "amostras":
		return h.analisarAmostras(ctx, r, q, p, m)
	case "investigacoes":
		return h.analisarInvestigacoes(ctx, r, q, p, m)
	case "bairros":
		return h.analisarBairros(ctx, q, p, m, atualizar)
	case "quarteiroes":
		return h.analisarQuarteiroes(ctx, q, p, m, atualizar)
	case "locais":
		return h.analisarLocais(ctx, q, p, m, atualizar)
	}
	return nil, fmt.Errorf("tipo de importação desconhecido: %s", tipo)
}

// gravarAnalise analisa a planilha (sem gravar dados) e guarda a importação "analisada" com a prévia linha a linha.
func (h *handlers) gravarAnalise(ctx context.Context, r *http.Request, pool *pgxpool.Pool, q *sqlccliente.Queries, p paramsAnalise) (sqlccliente.Importaco, error) {
	linhas, err := h.analisarPlanilha(ctx, r, q, p.Tipo, p.Plan, p.Mapa, p.Atualizar)
	if err != nil {
		return sqlccliente.Importaco{}, err
	}
	var criar, atu, ign, erros int32
	for _, l := range linhas {
		switch l.Acao {
		case acaoCriar:
			criar++
		case acaoAtualizar:
			atu++
		case acaoIgnorar:
			ign++
		default:
			erros++
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return sqlccliente.Importaco{}, err
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	ignoradas := strings.Join(p.Mapa.Ignoradas, ", ")
	var imp sqlccliente.Importaco
	if p.ExistenteID != 0 {
		imp, err = qt.FinalizarAnaliseDaImportacao(ctx, sqlccliente.FinalizarAnaliseDaImportacaoParams{
			ID: p.ExistenteID, TotalLinhas: int32(len(linhas)), PrevCriar: criar, PrevAtualizar: atu, PrevIgnorar: ign, PrevErro: erros, ColunasIgnoradas: ignoradas,
		})
	} else {
		a := atorDaSessao(r)
		imp, err = qt.CreateImportacao(ctx, sqlccliente.CreateImportacaoParams{
			Tipo: p.Tipo, ArquivoNome: p.Nome, ArquivoHash: p.Hash, AtualizarExistentes: p.Atualizar, TotalLinhas: int32(len(linhas)),
			PrevCriar: criar, PrevAtualizar: atu, PrevIgnorar: ign, PrevErro: erros, ColunasIgnoradas: ignoradas,
			ResponsavelID: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, ResponsavelNome: a.Nome, Cabecalho: emJSON(p.Plan.Cabecalho),
		})
		if err == nil && (p.Conector != "planilha" || p.Info != "") {
			_, err = tx.Exec(ctx, `UPDATE importacoes SET conector = $2, origem_info = $3 WHERE id = $1`, imp.ID, p.Conector, p.Info)
			imp.Conector, imp.OrigemInfo = p.Conector, p.Info
		}
	}
	if err != nil {
		return sqlccliente.Importaco{}, err
	}
	for _, l := range linhas {
		if err := qt.InsertImportacaoLinha(ctx, sqlccliente.InsertImportacaoLinhaParams{
			ImportacaoID: imp.ID, Linha: int32(l.N), Acao: l.Acao, Mensagem: l.Msg, AlvoID: l.AlvoID, Dados: l.Dados, Antes: l.Antes, Original: emJSON(l.Original),
		}); err != nil {
			return sqlccliente.Importaco{}, err
		}
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{
		Acao: "importacao.analisar", Entidade: "importacao", EntidadeID: strconv.FormatInt(imp.ID, 10), Rotulo: imp.ArquivoNome,
		Detalhes: map[string]any{
			"tipo": p.Tipo, "linhas": len(linhas), "a_criar": criar, "a_atualizar": atu, "a_ignorar": ign, "com_erro": erros,
			"atualizar_existentes": p.Atualizar, "origem": p.Conector,
		},
	})
	if err := tx.Commit(ctx); err != nil {
		return sqlccliente.Importaco{}, err
	}
	return imp, nil
}

// ------------------------------------------------------------------- Mapear colunas

func (h *handlers) importacaoParaMapear(w http.ResponseWriter, r *http.Request) (sqlccliente.Importaco, *sqlccliente.Queries, importacao.Planilha, bool) {
	imp, q, _, ok := h.importacaoDaURL(w, r)
	if !ok {
		return imp, nil, importacao.Planilha{}, false
	}
	if imp.Status != "mapeando" || len(imp.Arquivo) == 0 {
		http.Redirect(w, r, fmt.Sprintf("/importacoes/%d", imp.ID), http.StatusSeeOther)
		return imp, nil, importacao.Planilha{}, false
	}
	plan, err := lerPlanilhaGuardada(imp.Arquivo)
	if err != nil {
		h.falhaInterna(w, r, "ler a planilha guardada", err)
		return imp, nil, plan, false
	}
	return imp, q, plan, true
}

func (h *handlers) dadosDaTelaDeMapeamento(r *http.Request, q *sqlccliente.Queries, imp sqlccliente.Importaco, plan importacao.Planilha, escolhidos map[string]string, usarSalvo bool, salvarNome, erro string) views.ImportacaoMapearData {
	tipo, _ := importacao.TipoPorCodigo(imp.Tipo)
	conector, _ := importacao.ConectorPorCodigo(imp.Conector)
	if escolhidos == nil {
		var explicito map[string]string
		if idM, _ := strconv.ParseInt(r.URL.Query().Get("mapeamento"), 10, 64); usarSalvo && idM != 0 {
			if m, err := q.GetMapeamentoDeImportacao(r.Context(), idM); err == nil && m.Conector == imp.Conector && m.Tipo == imp.Tipo {
				_ = json.Unmarshal(m.Mapa, &explicito)
			}
		}
		escolhidos = tipo.MapearComo(plan.Cabecalho, explicito, conector.Candidatos[imp.Tipo]).MapaDe(plan.Cabecalho)
	}
	d := views.ImportacaoMapearData{ID: imp.ID, TipoRotulo: tipo.Rotulo, Conector: imp.Conector, Arquivo: imp.ArquivoNome, Cabecalhos: plan.Cabecalho, SalvarNome: salvarNome, Erro: erro, CSRF: csrfToken(r.Context())}
	for _, c := range tipo.Colunas {
		d.Linhas = append(d.Linhas, views.ImportacaoMapaLinha{Nome: c.Nome, Rotulo: c.Rotulo, Obrigatoria: c.Obrigatoria, Ajuda: c.Ajuda, Escolhido: escolhidos[c.Nome]})
	}
	for i, l := range plan.Linhas {
		if i == 3 {
			break
		}
		d.Amostra = append(d.Amostra, l.Celulas)
	}
	if ms, err := q.ListMapeamentosDeImportacao(r.Context(), sqlccliente.ListMapeamentosDeImportacaoParams{Conector: imp.Conector, Tipo: imp.Tipo}); err == nil {
		for _, m := range ms {
			d.Mapeamentos = append(d.Mapeamentos, views.ImportacaoMapeamentoOpcao{ID: m.ID, Rotulo: m.Nome})
		}
	}
	return d
}

func (h *handlers) importacaoMapearForm(w http.ResponseWriter, r *http.Request) {
	imp, q, plan, ok := h.importacaoParaMapear(w, r)
	if !ok {
		return
	}
	render(w, r, views.ImportacaoMapear(h.layout(r, "Ligar as colunas", "/importacoes"), h.dadosDaTelaDeMapeamento(r, q, imp, plan, nil, true, "", "")))
}

func (h *handlers) importacaoMapearSalvar(w http.ResponseWriter, r *http.Request) {
	imp, q, plan, ok := h.importacaoParaMapear(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	tipo, _ := importacao.TipoPorCodigo(imp.Tipo)
	validos := map[string]bool{}
	for _, c := range plan.Cabecalho {
		validos[c] = true
	}
	explicito := map[string]string{}
	escolhidos := map[string]string{}
	for _, c := range tipo.Colunas {
		h := r.PostFormValue("col_" + c.Nome)
		if !validos[h] {
			h = ""
		}
		explicito[c.Nome] = h
		if h != "" {
			escolhidos[c.Nome] = h
		}
	}
	salvarNome := strings.TrimSpace(r.PostFormValue("salvar_nome"))
	falha := func(msg string) {
		renderStatus(w, r, http.StatusBadRequest, views.ImportacaoMapear(h.layout(r, "Ligar as colunas", "/importacoes"), h.dadosDaTelaDeMapeamento(r, q, imp, plan, escolhidos, false, salvarNome, msg)))
	}
	mapa := tipo.MapearComo(plan.Cabecalho, explicito, nil)
	if len(mapa.Faltando) > 0 {
		falha("Ligue os campos obrigatórios: " + strings.Join(mapa.Faltando, ", ") + ".")
		return
	}
	if salvarNome != "" {
		if n := len([]rune(salvarNome)); n < 2 || n > 100 {
			falha("O nome do mapeamento deve ter de 2 a 100 caracteres.")
			return
		}
		if _, err := q.CreateMapeamentoDeImportacao(ctx, sqlccliente.CreateMapeamentoDeImportacaoParams{
			Nome: salvarNome, Conector: imp.Conector, Tipo: imp.Tipo, Mapa: emJSON(escolhidos), CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
		}); violacaoUnica(err) {
			falha("Já existe um mapeamento com esse nome para esta origem e este tipo de dado.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "salvar o mapeamento", err)
			return
		}
		h.auditar(r, audit.Evento{
			Acao: "importacao.mapeamento_salvar", Entidade: "mapeamento", Rotulo: salvarNome,
			Detalhes: map[string]any{"origem": imp.Conector, "tipo": imp.Tipo, "colunas": len(escolhidos)},
		})
	}
	_, pool, _ := h.cliente(r)
	if _, err := h.gravarAnalise(ctx, r, pool, q, paramsAnalise{
		Tipo: imp.Tipo, Plan: plan, Mapa: mapa, Atualizar: imp.AtualizarExistentes, Conector: imp.Conector, Info: imp.OrigemInfo, Nome: imp.ArquivoNome, ExistenteID: imp.ID,
	}); err != nil {
		h.falhaInterna(w, r, "analisar", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/importacoes/%d", imp.ID), http.StatusSeeOther)
}

// ------------------------------------------------------------------- Mapeamentos salvos

func (h *handlers) importacaoMapeamentos(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	ms, err := q.ListTodosMapeamentosDeImportacao(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar mapeamentos", err)
		return
	}
	loc := h.fuso(r)
	d := views.ImportacaoMapeamentosData{CSRF: csrfToken(r.Context())}
	for _, m := range ms {
		var mapa map[string]string
		_ = json.Unmarshal(m.Mapa, &mapa)
		d.Linhas = append(d.Linhas, views.ImportacaoMapeamentoLinha{ID: m.ID, Nome: m.Nome, Conector: m.Conector, Tipo: m.Tipo, Colunas: len(mapa), Quando: formatarData(loc, m.CriadoEm.Time)})
	}
	lay := h.layout(r, "Mapeamentos de colunas", "/importacoes")
	d.Linhas = paginar(r, &lay, d.Linhas)
	render(w, r, views.ImportacaoMapeamentos(lay, d))
}

func (h *handlers) importacaoMapeamentoExcluir(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.excluirRegistro(w, r, "mapeamento", id, "/importacoes/mapeamentos?aviso=excluido", "/importacoes/mapeamentos")
}

var _ = errors.New
var _ = pgx.ErrNoRows

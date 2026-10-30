package server

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// escopoDeItens junta as três restrições que valem para os locais de um grupo.
type escopoDeItens struct {
	somente        pgtype.Int8
	soPE           bool
	verDesativados bool
}

func (h *handlers) escopoDosItens(r *http.Request) escopoDeItens {
	s, pe := h.escopoLocais(r)
	return escopoDeItens{somente: s, soPE: pe, verDesativados: h.principal(r).Pode("local_ver_desativados")}
}

// grupoDaURL carrega o grupo do endereço; responde 404 quando não existe.
func (h *handlers) grupoDaURL(w http.ResponseWriter, r *http.Request) (sqlccliente.LocalGrupo, *sqlccliente.Queries, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.LocalGrupo{}, nil, false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return sqlccliente.LocalGrupo{}, nil, false
	}
	g, err := q.GetGrupo(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return g, nil, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar grupo", err)
		return g, nil, false
	}
	return g, q, true
}

// ------------------------------------------------------------------- Lista

func (h *handlers) gruposLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	busca := strings.TrimSpace(r.URL.Query().Get("busca"))
	e := h.escopoDosItens(r)
	rows, err := q.ListGrupos(r.Context(), sqlccliente.ListGruposParams{Busca: busca, SomenteUsuario: e.somente, SomentePe: e.soPE, VerDesativados: e.verDesativados})
	if err != nil {
		h.falhaInterna(w, r, "listar grupos", err)
		return
	}
	linhas := make([]views.GrupoLinha, len(rows))
	for i, g := range rows {
		linhas[i] = views.GrupoLinha{ID: g.ID, Nome: g.Nome, Descricao: g.Descricao, Total: int(g.TotalLocais), Criado: formatarData(h.fuso(r), g.CriadoEm.Time)}
	}
	lay := h.layout(r, "Grupos de locais", "/grupos")
	linhas = paginar(r, &lay, linhas)
	render(w, r, views.Grupos(lay, views.GruposData{
		Busca: busca, Linhas: linhas, PodeGerenciar: h.principal(r).Pode("local_grupo_gerenciar"),
		Aviso: map[string]string{"excluido": "Grupo excluído."}[r.URL.Query().Get("ok")],
	}))
}

// ------------------------------------------------------------------- Ficha

func (h *handlers) montarFormGrupo(ctx context.Context, r *http.Request, q *sqlccliente.Queries, g sqlccliente.LocalGrupo, nome, descricao, aviso, erro string) (views.GrupoFormData, error) {
	d := views.GrupoFormData{
		ID: g.ID, Nome: nome, Descricao: descricao, PodeGerenciar: h.principal(r).Pode("local_grupo_gerenciar"),
		Aviso: aviso, Erro: erro, CSRF: csrfToken(ctx), Pag: paginaDe(r),
	}
	if g.ID == 0 {
		return d, nil
	}
	e := h.escopoDosItens(r)
	if n, err := strconv.Atoi(r.URL.Query().Get("pagina")); err == nil && n > 1 {
		d.Pag.Page = n
	}
	total, err := q.CountLocaisDoGrupo(ctx, sqlccliente.CountLocaisDoGrupoParams{GrupoID: g.ID, SomenteUsuario: e.somente, SomentePe: e.soPE, VerDesativados: e.verDesativados})
	if err != nil {
		return d, err
	}
	d.Pag.Total = int(total)
	if d.Pag.Page > d.Pag.Pages() {
		d.Pag.Page = d.Pag.Pages()
	}
	rows, err := q.ListLocaisDoGrupo(ctx, sqlccliente.ListLocaisDoGrupoParams{
		GrupoID: g.ID, SomenteUsuario: e.somente, SomentePe: e.soPE, VerDesativados: e.verDesativados,
		Limite: int32(d.Pag.PerPage), Deslocamento: int32((d.Pag.Page - 1) * d.Pag.PerPage),
	})
	if err != nil {
		return d, err
	}
	for _, l := range rows {
		d.Locais = append(d.Locais, views.GrupoLocalLinha{
			ID: l.ID, Endereco: enderecoCompleto(l.Endereco, l.Numero), Tipo: rotuloTipoLocal(l.Tipo), Bairro: l.BairroNome,
			Quarteirao: l.QuarteiraoNome, Lado: l.Lado, Ativo: l.Ativo,
		})
	}
	d.Query = url.Values{}
	if d.PodeGerenciar {
		somente, _ := h.escopoLocais(r)
		bairros, err := q.ListBairrosAtivosParaLocal(ctx, somente)
		if err != nil {
			return d, err
		}
		for _, b := range bairros {
			d.Bairros = append(d.Bairros, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome})
		}
		d.Tipos = opcoesTipo("")
	}
	return d, nil
}

func (h *handlers) grupoNovoForm(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d, err := h.montarFormGrupo(r.Context(), r, q, sqlccliente.LocalGrupo{}, "", "", "", "")
	if err != nil {
		h.falhaInterna(w, r, "montar formulário", err)
		return
	}
	render(w, r, views.GrupoForm(h.layout(r, "Novo grupo", "/grupos"), d))
}

func (h *handlers) grupoFicha(w http.ResponseWriter, r *http.Request) {
	g, q, ok := h.grupoDaURL(w, r)
	if !ok {
		return
	}
	aviso := map[string]string{
		"criado": "Grupo criado. Agora acrescente locais.", "salvo": "Grupo salvo.", "limpo": "Locais retirados do grupo.", "removido": "Local retirado do grupo.",
	}[r.URL.Query().Get("ok")]
	if r.URL.Query().Has("adicionados") {
		a, _ := strconv.Atoi(r.URL.Query().Get("adicionados"))
		e, _ := strconv.Atoi(r.URL.Query().Get("existiam"))
		aviso = fmt.Sprintf("%d local(is) acrescentado(s)", a)
		if e > 0 {
			aviso += fmt.Sprintf(" e %d já estava(m) no grupo", e)
		}
		aviso += "."
	}
	d, err := h.montarFormGrupo(r.Context(), r, q, g, g.Nome, g.Descricao, aviso, "")
	if err != nil {
		h.falhaInterna(w, r, "montar ficha", err)
		return
	}
	render(w, r, views.GrupoForm(h.layout(r, "Grupo de locais", "/grupos"), d))
}

// ------------------------------------------------------------------- Salvar, excluir, clonar

func (h *handlers) grupoSalvar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	var atual sqlccliente.LocalGrupo
	if chi.URLParam(r, "id") != "" {
		var ok bool
		if atual, q, ok = h.grupoDaURL(w, r); !ok {
			return
		}
	}
	nome, desc := strings.TrimSpace(r.PostFormValue("nome")), strings.TrimSpace(r.PostFormValue("descricao"))
	falha := func(msg string) {
		d, e := h.montarFormGrupo(ctx, r, q, atual, nome, desc, "", msg)
		if e != nil {
			h.falhaInterna(w, r, "montar formulário", e)
			return
		}
		renderStatus(w, r, http.StatusBadRequest, views.GrupoForm(h.layout(r, "Grupo de locais", "/grupos"), d))
	}
	switch {
	case len([]rune(nome)) < 2 || len([]rune(nome)) > 120:
		falha("Informe o nome do grupo (de 2 a 120 caracteres).")
		return
	case len([]rune(desc)) > 300:
		falha("A descrição pode ter no máximo 300 caracteres.")
		return
	}

	if atual.ID == 0 {
		g, err := q.CreateGrupo(ctx, sqlccliente.CreateGrupoParams{Nome: nome, Descricao: desc, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}})
		if violacaoUnica(err) {
			falha("Já existe um grupo com esse nome.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "criar grupo", err)
			return
		}
		h.auditar(r, audit.Evento{Acao: "grupo.criar", Entidade: "grupo", EntidadeID: s64txt(g.ID), Rotulo: g.Nome, Depois: map[string]any{"nome": g.Nome, "descricao": g.Descricao}})
		http.Redirect(w, r, fmt.Sprintf("/grupos/%d?ok=criado", g.ID), http.StatusSeeOther)
		return
	}
	g, err := q.UpdateGrupo(ctx, sqlccliente.UpdateGrupoParams{ID: atual.ID, Nome: nome, Descricao: desc})
	if violacaoUnica(err) {
		falha("Já existe um grupo com esse nome.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "salvar grupo", err)
		return
	}
	a, dep := audit.Diferenca(map[string]any{"nome": atual.Nome, "descricao": atual.Descricao}, map[string]any{"nome": g.Nome, "descricao": g.Descricao})
	if len(a)+len(dep) == 0 {
		naoAuditar(r)
	} else {
		h.auditar(r, audit.Evento{Acao: "grupo.editar", Entidade: "grupo", EntidadeID: s64txt(g.ID), Rotulo: g.Nome, Antes: a, Depois: dep})
	}
	http.Redirect(w, r, fmt.Sprintf("/grupos/%d?ok=salvo", g.ID), http.StatusSeeOther)
}

func s64txt(n int64) string { return strconv.FormatInt(n, 10) }

func (h *handlers) grupoExcluir(w http.ResponseWriter, r *http.Request) {
	g, _, ok := h.grupoDaURL(w, r)
	if !ok {
		return
	}
	// exclusão lógica: o grupo vai para a Lixeira (os locais continuam cadastrados e podem voltar ao grupo restaurado)
	h.excluirRegistro(w, r, "grupo", g.ID, "/grupos?aviso=excluido", "/grupos/"+s64txt(g.ID))
}

// grupoClonar cria "Cópia de X" com os locais que a pessoa enxerga.
func (h *handlers) grupoClonar(w http.ResponseWriter, r *http.Request) {
	g, _, ok := h.grupoDaURL(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	_, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	var copia sqlccliente.LocalGrupo
	for i := 1; ; i++ { // "Cópia de X", "Cópia de X (2)", ...
		nome := "Cópia de " + g.Nome
		if i > 1 {
			nome += fmt.Sprintf(" (%d)", i)
		}
		if len([]rune(nome)) > 120 {
			nome = string([]rune(nome)[:120])
		}
		if existe, err := qt.NomeDeGrupoExiste(ctx, nome); err != nil {
			h.falhaInterna(w, r, "conferir nome", err)
			return
		} else if existe {
			continue
		}
		if copia, err = qt.CreateGrupo(ctx, sqlccliente.CreateGrupoParams{Nome: nome, Descricao: g.Descricao, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}}); err != nil {
			h.falhaInterna(w, r, "criar cópia", err)
			return
		}
		break
	}
	e := h.escopoDosItens(r)
	n, err := qt.CopyLocaisDoGrupo(ctx, sqlccliente.CopyLocaisDoGrupoParams{Destino: copia.ID, Origem: g.ID, SomenteUsuario: e.somente, SomentePe: e.soPE, VerDesativados: e.verDesativados})
	if err != nil {
		h.falhaInterna(w, r, "copiar locais", err)
		return
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{
		Acao: "grupo.clonar", Entidade: "grupo", EntidadeID: s64txt(copia.ID), Rotulo: copia.Nome,
		Detalhes: map[string]any{"origem": g.Nome, "locais": n},
	})
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar cópia", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/grupos/%d?ok=salvo", copia.ID), http.StatusSeeOther)
}

// ------------------------------------------------------------------- Locais do grupo

// grupoAdicionar acrescenta ao grupo todos os locais ativos de um bairro (ou de um quarteirão dele).
func (h *handlers) grupoAdicionar(w http.ResponseWriter, r *http.Request) {
	g, q, ok := h.grupoDaURL(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	falha := func(msg string) {
		d, e := h.montarFormGrupo(ctx, r, q, g, g.Nome, g.Descricao, "", msg)
		if e != nil {
			h.falhaInterna(w, r, "montar ficha", e)
			return
		}
		renderStatus(w, r, http.StatusBadRequest, views.GrupoForm(h.layout(r, "Grupo de locais", "/grupos"), d))
	}
	bairroID, _ := strconv.ParseInt(r.PostFormValue("bairro_id"), 10, 64)
	b, err := q.GetBairro(ctx, bairroID)
	if errors.Is(err, pgx.ErrNoRows) {
		falha("Escolha o bairro.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar bairro", err)
		return
	}
	if visivel, err := h.bairroVisivelParaLocal(ctx, r, q, b.ID); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return
	} else if !visivel {
		falha("Escolha um bairro em que você é responsável.")
		return
	}
	quart := pgtype.Int8{}
	quartNome := "todos"
	if qid, _ := strconv.ParseInt(r.PostFormValue("quarteirao_id"), 10, 64); qid != 0 {
		qt, err := q.GetQuarteirao(ctx, qid)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && qt.BairroID != b.ID) {
			falha("Escolha um quarteirão do bairro selecionado.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar quarteirão", err)
			return
		}
		if ok, err := h.quarteiraoVisivelParaLocal(ctx, r, q, qt.ID); err != nil {
			h.falhaInterna(w, r, "escopo do quarteirão", err)
			return
		} else if !ok {
			falha("Escolha um quarteirão ao qual você tem acesso.")
			return
		}
		quart, quartNome = pgtype.Int8{Int64: qt.ID, Valid: true}, qt.Nome
	}
	tipo := r.PostFormValue("tipo")
	if tipo != "" && rotuloTipoLocal(tipo) == tipo {
		falha("Tipo de local inválido.")
		return
	}
	e := h.escopoDosItens(r)

	_, pool, _ := h.cliente(r)
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	n, err := qt.AddLocaisAoGrupoPorFiltro(ctx, sqlccliente.AddLocaisAoGrupoPorFiltroParams{
		GrupoID: g.ID, BairroID: b.ID, QuarteiraoID: quart, Tipo: tipo, SomenteUsuario: e.somente, SomentePe: e.soPE,
	})
	if err != nil {
		h.falhaInterna(w, r, "acrescentar locais", err)
		return
	}
	if n > 0 {
		h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{
			Acao: "grupo.locais_adicionar", Entidade: "grupo", EntidadeID: s64txt(g.ID), Rotulo: g.Nome,
			Detalhes: map[string]any{"acrescentados": n, "bairro": b.Nome, "quarteirao": quartNome, "tipo": map[bool]string{true: "todos", false: rotuloTipoLocal(tipo)}[tipo == ""]},
		})
	} else {
		naoAuditar(r)
	}
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/grupos/%d?adicionados=%d", g.ID, n), http.StatusSeeOther)
}

func (h *handlers) grupoRemoverLocal(w http.ResponseWriter, r *http.Request) {
	g, q, ok := h.grupoDaURL(w, r)
	if !ok {
		return
	}
	lid, err := strconv.ParseInt(chi.URLParam(r, "lid"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	l, err := q.GetLocal(r.Context(), lid)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar local", err)
		return
	}
	if visivel, err := h.localVisivel(r.Context(), r, q, l); err != nil {
		h.falhaInterna(w, r, "escopo do local", err)
		return
	} else if !visivel {
		http.NotFound(w, r)
		return
	}
	n, err := q.RemoveLocalDoGrupo(r.Context(), sqlccliente.RemoveLocalDoGrupoParams{GrupoID: g.ID, LocalID: l.ID})
	if err != nil {
		h.falhaInterna(w, r, "retirar local", err)
		return
	}
	if n > 0 {
		h.auditar(r, audit.Evento{
			Acao: "grupo.local_remover", Entidade: "grupo", EntidadeID: s64txt(g.ID), Rotulo: g.Nome,
			Detalhes: map[string]any{"local": rotuloDoLocal(l)},
		})
	} else {
		naoAuditar(r)
	}
	http.Redirect(w, r, fmt.Sprintf("/grupos/%d?ok=removido", g.ID), http.StatusSeeOther)
}

func (h *handlers) grupoLimpar(w http.ResponseWriter, r *http.Request) {
	g, q, ok := h.grupoDaURL(w, r)
	if !ok {
		return
	}
	e := h.escopoDosItens(r)
	n, err := q.ClearLocaisDoGrupo(r.Context(), sqlccliente.ClearLocaisDoGrupoParams{GrupoID: g.ID, SomenteUsuario: e.somente, SomentePe: e.soPE, VerDesativados: e.verDesativados})
	if err != nil {
		h.falhaInterna(w, r, "esvaziar grupo", err)
		return
	}
	if n > 0 {
		h.auditar(r, audit.Evento{Acao: "grupo.limpar", Entidade: "grupo", EntidadeID: s64txt(g.ID), Rotulo: g.Nome, Detalhes: map[string]any{"retirados": n}})
	} else {
		naoAuditar(r)
	}
	http.Redirect(w, r, fmt.Sprintf("/grupos/%d?ok=limpo", g.ID), http.StatusSeeOther)
}

// localNoGrupo acrescenta um único local (da ficha dele) a um grupo.
func (h *handlers) localNoGrupo(w http.ResponseWriter, r *http.Request) {
	l, q, ok := h.localDaURL(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	gid, _ := strconv.ParseInt(r.PostFormValue("grupo_id"), 10, 64)
	g, err := q.GetGrupo(r.Context(), gid)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Redirect(w, r, fmt.Sprintf("/locais/%d?ok=semgrupo", l.ID), http.StatusSeeOther)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar grupo", err)
		return
	}
	n, err := q.AddLocalAoGrupo(r.Context(), sqlccliente.AddLocalAoGrupoParams{GrupoID: g.ID, LocalID: l.ID})
	if err != nil {
		h.falhaInterna(w, r, "acrescentar ao grupo", err)
		return
	}
	if n > 0 {
		h.auditar(r, audit.Evento{
			Acao: "grupo.locais_adicionar", Entidade: "grupo", EntidadeID: s64txt(g.ID), Rotulo: g.Nome,
			Detalhes: map[string]any{"acrescentados": 1, "local": rotuloDoLocal(l)},
		})
	} else {
		naoAuditar(r)
	}
	http.Redirect(w, r, fmt.Sprintf("/locais/%d?ok=nogrupo", l.ID), http.StatusSeeOther)
}

// ------------------------------------------------------------------- Exportar

// textoSeguroCSV impede que uma planilha execute fórmulas escondidas em texto digitado por alguém.
func textoSeguroCSV(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func (h *handlers) grupoExportar(w http.ResponseWriter, r *http.Request) {
	g, q, ok := h.grupoDaURL(w, r)
	if !ok {
		return
	}
	e := h.escopoDosItens(r)
	rows, err := q.ListLocaisDoGrupo(r.Context(), sqlccliente.ListLocaisDoGrupoParams{
		GrupoID: g.ID, SomenteUsuario: e.somente, SomentePe: e.soPE, VerDesativados: e.verDesativados, Limite: 100000,
	})
	if err != nil {
		h.falhaInterna(w, r, "listar locais", err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="grupo-%d.csv"`, g.ID))
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write([]byte("\xEF\xBB\xBF")) // BOM: o Excel abre em UTF-8
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	_ = cw.Write([]string{"Bairro", "Quarteirão", "Endereço", "Número", "Lado", "Sequência", "Complemento", "Tipo", "Situação", "Latitude", "Longitude", "Chave"})
	for _, l := range rows {
		situ := "Ativo"
		if !l.Ativo {
			situ = "Inativo"
		}
		_ = cw.Write([]string{
			textoSeguroCSV(l.BairroNome), textoSeguroCSV(l.QuarteiraoNome), textoSeguroCSV(l.Endereco), textoSeguroCSV(l.Numero), textoSeguroCSV(l.Lado),
			textoInt(l.Sequencia), textoInt(l.Complemento), rotuloTipoLocal(l.Tipo), situ,
			textoFloat(l.Latitude), textoFloat(l.Longitude), l.QrChave,
		})
	}
	cw.Flush()
	// dados saindo do sistema ficam registrados (sem dados pessoais: o arquivo só tem endereços)
	h.auditar(r, audit.Evento{Acao: "grupo.exportar", Entidade: "grupo", EntidadeID: s64txt(g.ID), Rotulo: g.Nome, Detalhes: map[string]any{"locais": len(rows)}})
}

// grupoQuarteiroes devolve as opções de quarteirão para o formulário "acrescentar locais" do grupo.
func (h *handlers) grupoQuarteiroes(w http.ResponseWriter, r *http.Request) {
	bid, err := strconv.ParseInt(r.URL.Query().Get("bairro_id"), 10, 64)
	if err != nil || bid <= 0 {
		render(w, r, views.OpcoesDeQuarteiraoTodos(nil))
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if ok, err := h.bairroVisivelParaLocal(r.Context(), r, q, bid); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return
	} else if !ok {
		http.NotFound(w, r)
		return
	}
	somente, _ := h.escopoLocais(r)
	qs, err := q.ListQuarteiroesAtivosDoBairro(r.Context(), sqlccliente.ListQuarteiroesAtivosDoBairroParams{BairroID: bid, SomenteUsuario: somente})
	if err != nil {
		h.falhaInterna(w, r, "listar quarteirões", err)
		return
	}
	ops := make([]views.CidadeOpcao, len(qs))
	for i, x := range qs {
		ops[i] = views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome}
	}
	render(w, r, views.OpcoesDeQuarteiraoTodos(ops))
}

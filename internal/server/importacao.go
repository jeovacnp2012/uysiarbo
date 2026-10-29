package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/importacao"
	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

var rotulosDeStatusImp = map[string]string{
	"analisada": "Aguardando confirmação", "executando": "Importando", "concluida": "Concluída", "interrompida": "Interrompida",
	"cancelada": "Descartada", "desfeita": "Desfeita", "erro": "Falhou",
}

// permissaoDoTipo diz qual permissão de edição o tipo de dado exige (além de conf_importacao).
func permissaoDoTipo(tipo string) string {
	if tipo == "locais" {
		return "local_adicionar"
	}
	return "bairro_adicionar"
}

// papelRestrito: quem só enxerga parte dos dados não importa (a importação mexe no cliente inteiro).
func (h *handlers) papelRestrito(r *http.Request) bool {
	pr := h.principal(r)
	return pr.Restrito("bairro_meubairro") || pr.Restrito("local_meubairro") || pr.Restrito("local_pe")
}

func (h *handlers) importacaoDaURL(w http.ResponseWriter, r *http.Request) (sqlccliente.Importaco, *sqlccliente.Queries, string, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return sqlccliente.Importaco{}, nil, "", false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return sqlccliente.Importaco{}, nil, "", false
	}
	c, _ := tenant.FromContext(r.Context())
	imp, err := q.GetImportacao(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return imp, nil, "", false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar importação", err)
		return imp, nil, "", false
	}
	// depois de um reinício do servidor, uma importação "executando" sem motor rodando está, na verdade, interrompida
	if imp.Status == "executando" && !h.imp.esta(chaveImp(c.SchemaNome, imp.ID)) {
		_ = q.SetImportacaoStatus(r.Context(), sqlccliente.SetImportacaoStatusParams{ID: imp.ID, Status: "interrompida", Mensagem: "Interrompida (o servidor reiniciou): use Retomar para continuar de onde parou."})
		imp.Status, imp.Mensagem = "interrompida", "Interrompida (o servidor reiniciou): use Retomar para continuar de onde parou."
	}
	return imp, q, c.SchemaNome, true
}

// ------------------------------------------------------------------- Lista e formulário

func (h *handlers) importacoesLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListImportacoes(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar importações", err)
		return
	}
	loc := h.fuso(r)
	linhas := make([]views.ImportacaoLinhaResumo, len(rows))
	for i, x := range rows {
		tipo := x.Tipo
		if t, ok := importacao.TipoPorCodigo(x.Tipo); ok {
			tipo = t.Rotulo
		}
		resumo := ""
		switch x.Status {
		case "analisada":
			resumo = fmt.Sprintf("prévia: %d a criar, %d a atualizar, %d com erro", x.PrevCriar, x.PrevAtualizar, x.PrevErro)
		case "executando", "interrompida":
			resumo = fmt.Sprintf("%d de %d linhas", x.Processadas, x.TotalLinhas)
		default:
			resumo = fmt.Sprintf("%d criado(s), %d atualizado(s), %d ignorado(s), %d erro(s)", x.Criados, x.Atualizados, x.Ignorados, x.Erros)
		}
		linhas[i] = views.ImportacaoLinhaResumo{
			ID: x.ID, Tipo: tipo, Arquivo: x.ArquivoNome, Status: x.Status, StatusRotulo: rotulosDeStatusImp[x.Status],
			Quando: formatarData(loc, x.CriadoEm.Time), Quem: x.ResponsavelNome, Total: int(x.TotalLinhas), Resumo: resumo,
		}
	}
	render(w, r, views.ImportacoesLista(h.layout(r, "Importação de dados", "/importacoes"), views.ImportacoesListaData{Linhas: linhas}))
}

func (h *handlers) dadosDoFormularioDeImportacao(r *http.Request, tipo string, atualizar bool, erro string) views.ImportacaoNovaData {
	pr := h.principal(r)
	d := views.ImportacaoNovaData{
		Atualizar: atualizar, Colunas: map[string][]views.ImportacaoColunaAjuda{}, MaxMB: importacao.MaxBytes >> 20, MaxLinhas: importacao.MaxLinhas,
		Erro: erro, CSRF: csrfToken(r.Context()),
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
	return d
}

func (h *handlers) importacaoNova(w http.ResponseWriter, r *http.Request) {
	if h.papelRestrito(r) {
		renderStatus(w, r, http.StatusForbidden, views.SemPermissao(h.layout(r, "Sem permissão", "")))
		return
	}
	render(w, r, views.ImportacaoNova(h.layout(r, "Nova importação", "/importacoes"), h.dadosDoFormularioDeImportacao(r, "", false, "")))
}

// importacaoModelo baixa o arquivo de exemplo (CSV ou Excel) de um tipo.
func (h *handlers) importacaoModelo(w http.ResponseWriter, r *http.Request) {
	nome := chi.URLParam(r, "arquivo")
	codigo, ext, _ := strings.Cut(nome, ".")
	t, ok := importacao.TipoPorCodigo(codigo)
	if !ok || (ext != "csv" && ext != "xlsx") {
		http.NotFound(w, r)
		return
	}
	var dados []byte
	var tipoMIME string
	if ext == "csv" {
		dados, tipoMIME = t.ModeloCSV(), "text/csv; charset=utf-8"
	} else {
		var err error
		if dados, err = t.ModeloXLSX(); err != nil {
			h.falhaInterna(w, r, "gerar modelo", err)
			return
		}
		tipoMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	w.Header().Set("Content-Type", tipoMIME)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="modelo-%s.%s"`, codigo, ext))
	_, _ = w.Write(dados)
}

// ------------------------------------------------------------------- Enviar e analisar

func (h *handlers) importacaoEnviar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.papelRestrito(r) {
		renderStatus(w, r, http.StatusForbidden, views.SemPermissao(h.layout(r, "Sem permissão", "")))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, importacao.MaxBytes+(2<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		h.formularioDeImportacaoComErro(w, r, "", false, fmt.Sprintf("Não consegui receber o arquivo (o limite é %d MB).", importacao.MaxBytes>>20))
		return
	}
	tipoCod, atualizar := r.PostFormValue("tipo"), r.PostFormValue("atualizar") == "1"
	tipo, ok := importacao.TipoPorCodigo(tipoCod)
	if !ok || !h.principal(r).Pode(permissaoDoTipo(tipoCod)) {
		h.formularioDeImportacaoComErro(w, r, "", atualizar, "Escolha o que importar.")
		return
	}
	arq, cab, err := r.FormFile("arquivo")
	if err != nil {
		h.formularioDeImportacaoComErro(w, r, tipoCod, atualizar, "Escolha o arquivo da planilha.")
		return
	}
	defer arq.Close()
	dados, err := io.ReadAll(io.LimitReader(arq, importacao.MaxBytes+1))
	if err != nil {
		h.formularioDeImportacaoComErro(w, r, tipoCod, atualizar, "Não consegui ler o arquivo.")
		return
	}
	plan, err := importacao.Ler(cab.Filename, dados)
	if err != nil {
		h.formularioDeImportacaoComErro(w, r, tipoCod, atualizar, "Planilha recusada: "+err.Error()+".")
		return
	}
	mapa := tipo.Mapear(plan.Cabecalho)
	if len(mapa.Faltando) > 0 {
		h.formularioDeImportacaoComErro(w, r, tipoCod, atualizar, "Faltam colunas obrigatórias no cabeçalho: "+strings.Join(mapa.Faltando, ", ")+". Baixe o modelo para conferir os nomes.")
		return
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	var linhas []linhaAnalisada
	switch tipoCod {
	case "bairros":
		linhas, err = h.analisarBairros(ctx, q, plan, mapa, atualizar)
	case "quarteiroes":
		linhas, err = h.analisarQuarteiroes(ctx, q, plan, mapa, atualizar)
	case "locais":
		linhas, err = h.analisarLocais(ctx, q, plan, mapa, atualizar)
	}
	if err != nil {
		h.falhaInterna(w, r, "analisar planilha", err)
		return
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
	soma := sha256.Sum256(dados)
	a := atorDaSessao(r)
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	imp, err := qt.CreateImportacao(ctx, sqlccliente.CreateImportacaoParams{
		Tipo: tipoCod, ArquivoNome: nomeSeguroDoArquivo(cab.Filename), ArquivoHash: soma[:], AtualizarExistentes: atualizar, TotalLinhas: int32(len(linhas)),
		PrevCriar: criar, PrevAtualizar: atu, PrevIgnorar: ign, PrevErro: erros, ColunasIgnoradas: strings.Join(mapa.Ignoradas, ", "),
		ResponsavelID: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, ResponsavelNome: a.Nome, Cabecalho: emJSON(plan.Cabecalho),
	})
	if err != nil {
		h.falhaInterna(w, r, "criar importação", err)
		return
	}
	for _, l := range linhas {
		if err := qt.InsertImportacaoLinha(ctx, sqlccliente.InsertImportacaoLinhaParams{
			ImportacaoID: imp.ID, Linha: int32(l.N), Acao: l.Acao, Mensagem: l.Msg, AlvoID: l.AlvoID, Dados: l.Dados, Antes: l.Antes, Original: emJSON(l.Original),
		}); err != nil {
			h.falhaInterna(w, r, "gravar linha da importação", err)
			return
		}
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{
		Acao: "importacao.analisar", Entidade: "importacao", EntidadeID: strconv.FormatInt(imp.ID, 10), Rotulo: imp.ArquivoNome,
		Detalhes: map[string]any{"tipo": tipoCod, "linhas": len(linhas), "a_criar": criar, "a_atualizar": atu, "a_ignorar": ign, "com_erro": erros, "atualizar_existentes": atualizar},
	})
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar análise", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/importacoes/%d", imp.ID), http.StatusSeeOther)
}

func nomeSeguroDoArquivo(n string) string {
	n = strings.Map(func(r rune) rune {
		if r < 32 || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, n)
	if len([]rune(n)) > 120 {
		n = string([]rune(n)[:120])
	}
	if n == "" {
		return "planilha"
	}
	return n
}

func (h *handlers) formularioDeImportacaoComErro(w http.ResponseWriter, r *http.Request, tipo string, atualizar bool, msg string) {
	renderStatus(w, r, http.StatusBadRequest, views.ImportacaoNova(h.layout(r, "Nova importação", "/importacoes"), h.dadosDoFormularioDeImportacao(r, tipo, atualizar, msg)))
}

// ------------------------------------------------------------------- Ficha, progresso e ações

func (h *handlers) fichaDaImportacao(r *http.Request, q *sqlccliente.Queries, imp sqlccliente.Importaco) (views.ImportacaoFichaData, error) {
	loc := h.fuso(r)
	rotulo := imp.Tipo
	if t, ok := importacao.TipoPorCodigo(imp.Tipo); ok {
		rotulo = t.Rotulo
	}
	d := views.ImportacaoFichaData{
		ID: imp.ID, Tipo: imp.Tipo, TipoRotulo: rotulo, Arquivo: imp.ArquivoNome, Status: imp.Status, StatusRotulo: rotulosDeStatusImp[imp.Status],
		Mensagem: imp.Mensagem, Quem: imp.ResponsavelNome, Criada: formatarData(loc, imp.CriadoEm.Time), ModoAtualizar: imp.AtualizarExistentes,
		ColunasIgnoradas: imp.ColunasIgnoradas, Total: int(imp.TotalLinhas), ParaCriar: int(imp.PrevCriar), ParaAtualizar: int(imp.PrevAtualizar),
		ParaIgnorar: int(imp.PrevIgnorar), ComErro: int(imp.PrevErro), Processadas: int(imp.Processadas), Criados: int(imp.Criados),
		Atualizados: int(imp.Atualizados), Ignorados: int(imp.Ignorados), Erros: int(imp.Erros), Desfeitos: int(imp.Desfeitos), CSRF: csrfToken(r.Context()),
		PodeExecutar: imp.Status == "analisada" && imp.PrevCriar+imp.PrevAtualizar > 0,
		PodeDesfazer: imp.Status == "concluida" && imp.Criados+imp.Atualizados > 0,
	}
	if imp.ConcluidoEm.Valid {
		d.Concluida = formatarData(loc, imp.ConcluidoEm.Time)
	}
	if imp.TotalLinhas > 0 {
		d.Percentual = int(imp.Processadas) * 100 / int(imp.TotalLinhas)
	}
	return d, nil
}

func (h *handlers) importacaoFicha(w http.ResponseWriter, r *http.Request) {
	imp, q, _, ok := h.importacaoDaURL(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	d, err := h.fichaDaImportacao(r, q, imp)
	if err != nil {
		h.falhaInterna(w, r, "montar ficha", err)
		return
	}
	d.Filtro = r.URL.Query().Get("acao")
	if !slicesContem([]string{"", "criar", "atualizar", "ignorar", "erro"}, d.Filtro) {
		d.Filtro = ""
	}
	d.Pag = views.Pagination{Page: 1, PerPage: 50}
	if n, err := strconv.Atoi(r.URL.Query().Get("pagina")); err == nil && n > 1 {
		d.Pag.Page = n
	}
	total, err := q.CountImportacaoLinhas(ctx, sqlccliente.CountImportacaoLinhasParams{ImportacaoID: imp.ID, Acao: d.Filtro})
	if err != nil {
		h.falhaInterna(w, r, "contar linhas", err)
		return
	}
	d.Pag.Total = int(total)
	if d.Pag.Page > d.Pag.Pages() {
		d.Pag.Page = d.Pag.Pages()
	}
	rows, err := q.ListImportacaoLinhas(ctx, sqlccliente.ListImportacaoLinhasParams{
		ImportacaoID: imp.ID, Acao: d.Filtro, Limite: int32(d.Pag.PerPage), Deslocamento: int32((d.Pag.Page - 1) * d.Pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar linhas", err)
		return
	}
	for _, x := range rows {
		var cel []string
		_ = json.Unmarshal(x.Original, &cel)
		resumo := strings.Join(primeirasCelulas(cel, 4), " · ")
		d.Linhas = append(d.Linhas, views.ImportacaoLinhaPrevia{N: int(x.Linha), Acao: x.Acao, Mensagem: x.Mensagem, Resumo: resumo, Resultado: x.Resultado.String, ResultadoMsg: x.ResultadoMsg})
	}
	d.Query = url.Values{}
	if d.Filtro != "" {
		d.Query.Set("acao", d.Filtro)
	}
	d.Aviso = map[string]string{"iniciada": "Importação iniciada: acompanhe o andamento abaixo.", "descartada": "", "interrompendo": "Interrompendo..."}[r.URL.Query().Get("ok")]
	render(w, r, views.ImportacaoFicha(h.layout(r, "Importação", "/importacoes"), d))
}

func slicesContem(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

func primeirasCelulas(c []string, n int) []string {
	var out []string
	for _, x := range c {
		if x == "" {
			continue
		}
		out = append(out, x)
		if len(out) == n {
			break
		}
	}
	return out
}

// importacaoProgresso devolve só a barra de andamento (o navegador pergunta de 2 em 2 segundos).
func (h *handlers) importacaoProgresso(w http.ResponseWriter, r *http.Request) {
	imp, q, _, ok := h.importacaoDaURL(w, r)
	if !ok {
		return
	}
	d, err := h.fichaDaImportacao(r, q, imp)
	if err != nil {
		h.falhaInterna(w, r, "montar progresso", err)
		return
	}
	if imp.Status != "executando" {
		w.Header().Set("HX-Refresh", "true") // terminou (ou parou): recarrega a página inteira com o resultado
	}
	render(w, r, views.ImportacaoProgresso(d))
}

func (h *handlers) origemDaRequisicao(r *http.Request) origemDaImportacao {
	return origemDaImportacao{Ator: atorDaSessao(r), IP: ipDe(r), UserAgent: r.UserAgent(), RequestID: r.Header.Get("X-Request-Id"), UsuarioID: meuID(r)}
}

func (h *handlers) importacaoExecutar(w http.ResponseWriter, r *http.Request) {
	h.comecarImportacao(w, r, "analisada")
}

func (h *handlers) importacaoRetomar(w http.ResponseWriter, r *http.Request) {
	h.comecarImportacao(w, r, "interrompida")
}

func (h *handlers) comecarImportacao(w http.ResponseWriter, r *http.Request, statusEsperado string) {
	imp, q, schema, ok := h.importacaoDaURL(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	_, pool, _ := h.cliente(r)
	falha := func(msg string) {
		d, _ := h.fichaDaImportacao(r, q, imp)
		d.Aviso = msg
		d.Pag = views.Pagination{Page: 1, PerPage: 50}
		renderStatus(w, r, http.StatusConflict, views.ImportacaoFicha(h.layout(r, "Importação", "/importacoes"), d))
	}
	if imp.Status != statusEsperado {
		falha("Esta importação não está em um estado que permita iniciar.")
		return
	}
	if n, err := q.ContarImportacoesEmExecucao(ctx); err != nil {
		h.falhaInterna(w, r, "contar execuções", err)
		return
	} else if n > 0 {
		falha("Já há uma importação em andamento neste ambiente. Espere ela terminar.")
		return
	}
	if linhas, _ := q.ContarImportacaoLinhasAFazer(ctx, imp.ID); linhas == 0 {
		falha("Não há linhas a gravar nesta importação.")
		return
	}
	if n, err := q.IniciarImportacao(ctx, imp.ID); err != nil {
		h.falhaInterna(w, r, "iniciar importação", err)
		return
	} else if n == 0 {
		falha("Esta importação não está em um estado que permita iniciar.")
		return
	}
	if !h.iniciarExecucao(pool, schema, imp.ID, h.origemDaRequisicao(r)) {
		_ = q.SetImportacaoStatus(ctx, sqlccliente.SetImportacaoStatusParams{ID: imp.ID, Status: statusEsperado, Mensagem: ""})
		falha("Esta importação já está rodando.")
		return
	}
	acao := "importacao.iniciar"
	if statusEsperado == "interrompida" {
		acao = "importacao.retomar"
	}
	h.auditar(r, audit.Evento{Acao: acao, Entidade: "importacao", EntidadeID: strconv.FormatInt(imp.ID, 10), Rotulo: imp.ArquivoNome,
		Detalhes: map[string]any{"tipo": imp.Tipo, "a_criar": imp.PrevCriar, "a_atualizar": imp.PrevAtualizar}})
	http.Redirect(w, r, fmt.Sprintf("/importacoes/%d?ok=iniciada", imp.ID), http.StatusSeeOther)
}

func (h *handlers) importacaoInterromper(w http.ResponseWriter, r *http.Request) {
	imp, _, schema, ok := h.importacaoDaURL(w, r)
	if !ok {
		return
	}
	if h.imp.interromper(chaveImp(schema, imp.ID)) {
		h.auditar(r, audit.Evento{Acao: "importacao.interromper", Entidade: "importacao", EntidadeID: strconv.FormatInt(imp.ID, 10), Rotulo: imp.ArquivoNome})
	} else {
		naoAuditar(r)
	}
	http.Redirect(w, r, fmt.Sprintf("/importacoes/%d?ok=interrompendo", imp.ID), http.StatusSeeOther)
}

func (h *handlers) importacaoCancelar(w http.ResponseWriter, r *http.Request) {
	imp, q, _, ok := h.importacaoDaURL(w, r)
	if !ok {
		return
	}
	if imp.Status == "analisada" {
		if err := q.SetImportacaoStatus(r.Context(), sqlccliente.SetImportacaoStatusParams{ID: imp.ID, Status: "cancelada", Mensagem: "Descartada antes de gravar: nada foi importado."}); err != nil {
			h.falhaInterna(w, r, "descartar importação", err)
			return
		}
		h.auditar(r, audit.Evento{Acao: "importacao.cancelar", Entidade: "importacao", EntidadeID: strconv.FormatInt(imp.ID, 10), Rotulo: imp.ArquivoNome})
	} else {
		naoAuditar(r)
	}
	http.Redirect(w, r, fmt.Sprintf("/importacoes/%d", imp.ID), http.StatusSeeOther)
}

func (h *handlers) importacaoDesfazer(w http.ResponseWriter, r *http.Request) {
	imp, q, _, ok := h.importacaoDaURL(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if imp.Status != "concluida" {
		naoAuditar(r)
		http.Redirect(w, r, fmt.Sprintf("/importacoes/%d", imp.ID), http.StatusSeeOther)
		return
	}
	_, pool, _ := h.cliente(r)
	desfeitos, nao, err := desfazerImportacao(context.WithoutCancel(ctx), pool, imp)
	if err != nil {
		h.falhaInterna(w, r, "desfazer importação", err)
		return
	}
	if nao == 0 {
		_ = q.DesfazerImportacaoConcluida(ctx, sqlccliente.DesfazerImportacaoConcluidaParams{ID: imp.ID, Desfeitos: int32(desfeitos), Mensagem: fmt.Sprintf("Desfeita: %d linha(s) revertida(s).", desfeitos)})
	} else {
		_ = q.SetImportacaoStatus(ctx, sqlccliente.SetImportacaoStatusParams{ID: imp.ID, Status: "concluida", Mensagem: fmt.Sprintf("Desfeita em parte: %d revertida(s) e %d mantida(s) porque já estão em uso (veja o relatório).", desfeitos, nao)})
	}
	h.atualizarContadores(ctx, q, imp.ID)
	h.auditar(r, audit.Evento{
		Acao: "importacao.desfazer", Entidade: "importacao", EntidadeID: strconv.FormatInt(imp.ID, 10), Rotulo: imp.ArquivoNome,
		Detalhes: map[string]any{"desfeitos": desfeitos, "mantidos": nao},
	})
	http.Redirect(w, r, fmt.Sprintf("/importacoes/%d", imp.ID), http.StatusSeeOther)
}

// ------------------------------------------------------------------- Relatórios

func (h *handlers) importacaoRelatorio(w http.ResponseWriter, r *http.Request) {
	h.relatorioDaImportacao(w, r, false)
}

func (h *handlers) importacaoErros(w http.ResponseWriter, r *http.Request) {
	h.relatorioDaImportacao(w, r, true)
}

func (h *handlers) relatorioDaImportacao(w http.ResponseWriter, r *http.Request, soErros bool) {
	imp, q, _, ok := h.importacaoDaURL(w, r)
	if !ok {
		return
	}
	rows, err := q.ListImportacaoLinhasRelatorio(r.Context(), sqlccliente.ListImportacaoLinhasRelatorioParams{ImportacaoID: imp.ID, SoErros: soErros})
	if err != nil {
		h.falhaInterna(w, r, "relatório da importação", err)
		return
	}
	var cab []string
	_ = json.Unmarshal(imp.Cabecalho, &cab)
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	cw := csv.NewWriter(&buf)
	cw.Comma = ';'
	nome := "relatorio"
	if soErros {
		// as linhas com erro, no formato da planilha enviada, com o motivo na última coluna: corrija e envie de novo
		nome = "linhas-com-erro"
		_ = cw.Write(append(append([]string{}, cab...), "motivo_do_erro"))
	} else {
		_ = cw.Write(append([]string{"linha", "analise", "resultado", "mensagem", "registro"}, cab...))
	}
	for _, x := range rows {
		var cel []string
		_ = json.Unmarshal(x.Original, &cel)
		seguras := make([]string, len(cel))
		for i, c := range cel {
			seguras[i] = textoSeguroCSV(c)
		}
		msg := x.Mensagem
		if x.ResultadoMsg != "" && x.Resultado.String == "erro" {
			msg = x.ResultadoMsg
		}
		if soErros {
			for len(seguras) < len(cab) {
				seguras = append(seguras, "")
			}
			_ = cw.Write(append(seguras[:len(cab)], textoSeguroCSV(msg)))
			continue
		}
		reg := ""
		if x.RegistroID.Valid {
			reg = strconv.FormatInt(x.RegistroID.Int64, 10)
		}
		if x.ResultadoMsg != "" {
			msg = strings.TrimSpace(msg + " " + x.ResultadoMsg)
		}
		_ = cw.Write(append([]string{strconv.Itoa(int(x.Linha)), x.Acao, x.Resultado.String, textoSeguroCSV(msg), reg}, seguras...))
	}
	cw.Flush()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="importacao-%d-%s.csv"`, imp.ID, nome))
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(buf.Bytes())
}

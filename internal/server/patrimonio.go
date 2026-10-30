package server

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Patrimônio (11.3): bens permanentes com responsável, manutenção, baixa e histórico de eventos.

var (
	rotulosStatusPatrimonio = []views.CidadeOpcao{
		{Valor: "ativo", Rotulo: "Ativo"},
		{Valor: "manutencao", Rotulo: "Em manutenção"},
		{Valor: "baixado", Rotulo: "Baixado"},
	}
	rotulosEventoPatrimonio = []views.CidadeOpcao{
		{Valor: "criacao", Rotulo: "Cadastro"},
		{Valor: "atribuicao", Rotulo: "Atribuição"},
		{Valor: "manutencao", Rotulo: "Manutenção"},
		{Valor: "retorno", Rotulo: "Retorno da manutenção"},
		{Valor: "anotacao", Rotulo: "Anotação"},
		{Valor: "baixa", Rotulo: "Baixa"},
		{Valor: "reativacao", Rotulo: "Reativação"},
	}
)

func patrimonioCaminho(id int64) string { return "/estoque/patrimonio/" + strconv.FormatInt(id, 10) }

func (h *handlers) patrimonios(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.PatrimonioFiltro{Busca: strings.TrimSpace(qs.Get("busca")), Status: qs.Get("status"), Responsavel: qs.Get("responsavel")}
	if !validoEm(rotulosStatusPatrimonio, f.Status) {
		f.Status = ""
	}
	ordem, dir := ordemDeEstoque(qs.Get("ordem"), qs.Get("dir"), "id", "nome")
	pag := paginaDe(r)
	total, err := q.CountPatrimonios(ctx, sqlccliente.CountPatrimoniosParams{Busca: f.Busca, Status: f.Status, ResponsavelID: idOpcional(f.Responsavel)})
	if err != nil {
		h.falhaInterna(w, r, "contar patrimônios", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListPatrimonios(ctx, sqlccliente.ListPatrimoniosParams{
		Busca: f.Busca, Status: f.Status, ResponsavelID: idOpcional(f.Responsavel), Ordem: ordem, Dir: dir,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar patrimônios", err)
		return
	}
	d := views.PatrimoniosData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNovo: h.principal(r).Pode("patrimonio_novo")}
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.PatrimonioLinha{
			IDTexto: strconv.FormatInt(x.ID, 10), Numero: x.NumeroPatrimonio, Serie: x.NumeroSerie, Nome: x.Nome, Responsavel: x.Responsavel,
			Entrada: dataBR(x.DataEntrada), Status: rotuloDe(rotulosStatusPatrimonio, x.Status),
		})
	}
	if d.Responsaveis, err = h.opcoesDeAgentes(r, q, idOpcional(f.Responsavel).Int64); err != nil {
		h.falhaInterna(w, r, "listar responsáveis", err)
		return
	}
	d.Status = marcarOpcao(rotulosStatusPatrimonio, f.Status)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "status": f.Status, "responsavel": f.Responsavel} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Patrimonios(h.layout(r, "Patrimônio", "/estoque/patrimonio"), d))
}

func (h *handlers) opcoesDeLotesPatrimonio(r *http.Request, q *sqlccliente.Queries, escolhido int64) ([]views.CidadeOpcao, error) {
	rows, err := q.ListEstoqueLotesParaPatrimonio(r.Context())
	if err != nil {
		return nil, err
	}
	out := make([]views.CidadeOpcao, len(rows))
	for i, x := range rows {
		out[i] = views.CidadeOpcao{ID: x.ID, Rotulo: x.Rotulo, Selecionada: x.ID == escolhido}
	}
	return out, nil
}

func (h *handlers) patrimonioNovo(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := views.PatrimonioForm{PodeEditar: true, CSRF: csrfToken(r.Context()), Entrada: h.hojeISO(r)}
	if d.Lotes, err = h.opcoesDeLotesPatrimonio(r, q, 0); err == nil {
		d.Responsaveis, err = h.opcoesDeAgentes(r, q, 0)
	}
	if err != nil {
		h.falhaInterna(w, r, "montar formulário de patrimônio", err)
		return
	}
	render(w, r, views.PatrimonioFicha(h.layout(r, "Novo bem patrimonial", "/estoque/patrimonio"), d))
}

type patrimonioLido struct {
	form                views.PatrimonioForm
	lote, responsavel   pgtype.Int8
	entrada             pgtype.Date
	numero, serie, nome string
	observacao          string
}

func lerPatrimonio(r *http.Request) (patrimonioLido, string) {
	p := patrimonioLido{
		numero: strings.TrimSpace(r.PostFormValue("numero")), serie: strings.TrimSpace(r.PostFormValue("serie")),
		nome: strings.TrimSpace(r.PostFormValue("nome")), observacao: strings.TrimSpace(r.PostFormValue("observacao")),
		lote: idOpcional(r.PostFormValue("lote")), responsavel: idOpcional(r.PostFormValue("responsavel")),
	}
	p.form = views.PatrimonioForm{Numero: p.numero, Serie: p.serie, Nome: p.nome, Entrada: r.PostFormValue("entrada"), Observacao: p.observacao, PodeEditar: true}
	var erro string
	if p.entrada, erro = dataOpcionalTexto(p.form.Entrada, "Data de entrada"); erro != "" {
		return p, erro
	}
	switch {
	case p.numero == "" || len([]rune(p.numero)) > 40:
		return p, "Informe o número de patrimônio (até 40 caracteres)."
	case p.nome == "" || len([]rune(p.nome)) > 150:
		return p, "Informe o nome do bem (até 150 caracteres)."
	case len([]rune(p.serie)) > 60:
		return p, "O número de série aceita até 60 caracteres."
	case len([]rune(p.observacao)) > 1000:
		return p, "A observação aceita até 1000 caracteres."
	}
	return p, ""
}

// registrarEvento grava o evento do bem (imutável).
func (h *handlers) registrarEventoPatrimonio(r *http.Request, q *sqlccliente.Queries, id int64, tipo, descricao string) error {
	return q.CreatePatrimonioEvento(r.Context(), sqlccliente.CreatePatrimonioEventoParams{PatrimonioID: id, Tipo: tipo, Descricao: descricao, UsuarioID: idOuNulo(meuID(r))})
}

func (h *handlers) patrimonioCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	p, erro := lerPatrimonio(r)
	if erro == "" && p.lote.Valid {
		if _, e := q.GetEstoqueLote(ctx, p.lote.Int64); e != nil {
			erro = "Escolha um lote válido."
		}
	}
	if erro == "" && p.responsavel.Valid {
		if _, e := q.GetUsuarioByID(ctx, p.responsavel.Int64); e != nil {
			erro = "Escolha um responsável válido."
		}
	}
	if erro == "" {
		tx, e := pool.Begin(ctx)
		if e != nil {
			h.falhaInterna(w, r, "iniciar cadastro de patrimônio", e)
			return
		}
		defer tx.Rollback(ctx)
		qt := q.WithTx(tx)
		x, e := qt.CreatePatrimonio(ctx, sqlccliente.CreatePatrimonioParams{
			LoteID: p.lote, NumeroPatrimonio: p.numero, NumeroSerie: p.serie, Nome: p.nome, ResponsavelID: p.responsavel, DataEntrada: p.entrada, Observacao: p.observacao,
		})
		if e == nil {
			e = h.registrarEventoPatrimonio(r, qt, x.ID, "criacao", "Bem cadastrado")
		}
		if e == nil {
			h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "patrimonio.criar", Entidade: "patrimonio", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: x.NumeroPatrimonio + " — " + x.Nome,
				Depois: map[string]any{"numero": x.NumeroPatrimonio, "nome": x.Nome, "serie": x.NumeroSerie}})
			if e = tx.Commit(ctx); e == nil {
				http.Redirect(w, r, patrimonioCaminho(x.ID)+"?aviso=criado", http.StatusSeeOther)
				return
			}
		}
		if !violacaoUnica(e) {
			h.falhaInterna(w, r, "criar patrimônio", e)
			return
		}
		erro = "Já existe um bem com esse número de patrimônio."
	}
	d := p.form
	d.CSRF, d.Erro = csrfToken(ctx), erro
	if d.Lotes, err = h.opcoesDeLotesPatrimonio(r, q, p.lote.Int64); err == nil {
		d.Responsaveis, err = h.opcoesDeAgentes(r, q, p.responsavel.Int64)
	}
	if err != nil {
		h.falhaInterna(w, r, "montar formulário de patrimônio", err)
		return
	}
	renderStatus(w, r, http.StatusBadRequest, views.PatrimonioFicha(h.layout(r, "Novo bem patrimonial", "/estoque/patrimonio"), d))
}

func (h *handlers) carregarPatrimonio(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.Patrimonio, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.Patrimonio{}, false
	}
	x, err := q.GetPatrimonio(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return x, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar patrimônio", err)
		return x, false
	}
	return x, true
}

func (h *handlers) renderPatrimonio(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, x sqlccliente.Patrimonio, enviado *patrimonioLido, erro string) {
	ctx := r.Context()
	pr := h.principal(r)
	d := views.PatrimonioForm{
		ID: x.ID, Numero: x.NumeroPatrimonio, Serie: x.NumeroSerie, Nome: x.Nome, Entrada: dataISO(x.DataEntrada), Observacao: x.Observacao,
		Status: x.Status, StatusRotulo: rotuloDe(rotulosStatusPatrimonio, x.Status), Responsavel: "—",
		PodeEditar: pr.Pode("patrimonio_editar") && x.Status != "baixado", PodeAtribuir: pr.Pode("patrimonio_atribuir"), PodeManutencao: pr.Pode("patrimonio_manutencao"),
		PodeAnotar: pr.Pode("patrimonio_anotacao"), PodeBaixa: pr.Pode("patrimonio_baixa"), PodeReativar: pr.Pode("patrimonio_reativar"), PodeExcluir: pr.Pode("patrimonio_excluir"),
		CSRF: csrfToken(ctx), Erro: erro,
	}
	if enviado != nil {
		d.Numero, d.Serie, d.Nome, d.Entrada, d.Observacao = enviado.numero, enviado.serie, enviado.nome, enviado.form.Entrada, enviado.observacao
	}
	if x.ResponsavelID.Valid {
		if u, err := q.GetUsuarioByID(ctx, x.ResponsavelID.Int64); err == nil {
			d.Responsavel = u.Nome
		}
	}
	switch r.URL.Query().Get("aviso") {
	case "criado":
		d.Aviso = "Bem cadastrado."
	case "salvo":
		d.Aviso = "Alterações salvas."
	case "evento":
		d.Aviso = "Registrado no histórico."
	}
	var err error
	if d.Lotes, err = h.opcoesDeLotesPatrimonio(r, q, x.LoteID.Int64); err == nil {
		d.Responsaveis, err = h.opcoesDeAgentes(r, q, 0)
	}
	if err != nil {
		h.falhaInterna(w, r, "montar ficha do patrimônio", err)
		return
	}
	evs, err := q.ListPatrimonioEventos(ctx, x.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar eventos do patrimônio", err)
		return
	}
	for _, e := range evs {
		d.Eventos = append(d.Eventos, views.PatrimonioEvento{
			Quando: e.CriadoEm.Time.In(h.fuso(r)).Format("02/01/2006 15:04"), Tipo: rotuloDe(rotulosEventoPatrimonio, e.Tipo), Descricao: e.Descricao, Por: e.Por,
		})
	}
	status := http.StatusOK
	if erro != "" {
		status = http.StatusBadRequest
	}
	renderStatus(w, r, status, views.PatrimonioFicha(h.layout(r, x.Nome, "/estoque/patrimonio"), d))
}

func (h *handlers) patrimonioFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if x, ok := h.carregarPatrimonio(w, r, q); ok {
		h.renderPatrimonio(w, r, q, x, nil, "")
	}
}

func (h *handlers) patrimonioSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	x, ok := h.carregarPatrimonio(w, r, q)
	if !ok {
		return
	}
	if x.Status == "baixado" {
		h.renderPatrimonio(w, r, q, x, nil, "Bem baixado não pode ser alterado; reative-o antes.")
		return
	}
	p, erro := lerPatrimonio(r)
	if erro == "" && p.lote.Valid {
		if _, e := q.GetEstoqueLote(ctx, p.lote.Int64); e != nil {
			erro = "Escolha um lote válido."
		}
	}
	if erro != "" {
		h.renderPatrimonio(w, r, q, x, &p, erro)
		return
	}
	if _, err = q.UpdatePatrimonioDados(ctx, sqlccliente.UpdatePatrimonioDadosParams{
		ID: x.ID, LoteID: p.lote, NumeroPatrimonio: p.numero, NumeroSerie: p.serie, Nome: p.nome, DataEntrada: p.entrada, Observacao: p.observacao,
	}); err != nil {
		if violacaoUnica(err) {
			h.renderPatrimonio(w, r, q, x, &p, "Já existe um bem com esse número de patrimônio.")
			return
		}
		h.falhaInterna(w, r, "salvar patrimônio", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "patrimonio.editar", Entidade: "patrimonio", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: p.numero + " — " + p.nome,
		Antes:  map[string]any{"numero": x.NumeroPatrimonio, "nome": x.Nome, "serie": x.NumeroSerie, "entrada": dataBR(x.DataEntrada)},
		Depois: map[string]any{"numero": p.numero, "nome": p.nome, "serie": p.serie, "entrada": dataBR(p.entrada)}})
	http.Redirect(w, r, patrimonioCaminho(x.ID)+"?aviso=salvo", http.StatusSeeOther)
}

// patrimonioEvento é o molde das ações do bem: atribuir, manutenção, retorno, anotação, baixa e reativação.
func (h *handlers) patrimonioEvento(acao string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Requisição inválida.", http.StatusBadRequest)
			return
		}
		ctx := r.Context()
		q, pool, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		x, ok := h.carregarPatrimonio(w, r, q)
		if !ok {
			return
		}
		desc := strings.TrimSpace(r.PostFormValue("descricao"))
		var tipo, novo, exige string // exige: situação em que a ação vale
		var erro string
		switch acao {
		case "atribuir":
			tipo, exige = "atribuicao", "ativo"
		case "manutencao":
			tipo, novo, exige = "manutencao", "manutencao", "ativo"
		case "retorno":
			tipo, novo, exige = "retorno", "ativo", "manutencao"
		case "anotacao":
			tipo = "anotacao"
		case "baixa":
			tipo, novo = "baixa", "baixado"
		case "reativar":
			tipo, novo, exige = "reativacao", "ativo", "baixado"
		}
		resp := idOpcional(r.PostFormValue("responsavel"))
		switch {
		case exige != "" && x.Status != exige:
			erro = "Essa ação não vale para um bem " + strings.ToLower(rotuloDe(rotulosStatusPatrimonio, x.Status)) + "."
		case acao == "baixa" && x.Status == "baixado":
			erro = "O bem já está baixado."
		case (acao == "anotacao" || acao == "baixa" || acao == "manutencao") && desc == "":
			erro = "Informe a descrição."
		case len([]rune(desc)) > 500:
			erro = "A descrição aceita até 500 caracteres."
		case acao == "atribuir" && !resp.Valid:
			erro = "Escolha o novo responsável."
		}
		var nome string
		if erro == "" && acao == "atribuir" {
			u, e := q.GetUsuarioByID(ctx, resp.Int64)
			if e != nil {
				erro = "Escolha um responsável válido."
			}
			nome = u.Nome
		}
		if erro != "" {
			h.renderPatrimonio(w, r, q, x, nil, erro)
			return
		}
		texto := desc
		if acao == "atribuir" {
			texto = "Atribuído a " + nome
			if desc != "" {
				texto += " — " + desc
			}
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			h.falhaInterna(w, r, "iniciar evento do patrimônio", err)
			return
		}
		defer tx.Rollback(ctx)
		qt := q.WithTx(tx)
		if _, err = qt.GetPatrimonioParaAtualizar(ctx, x.ID); err == nil {
			switch {
			case acao == "atribuir":
				err = qt.SetPatrimonioResponsavel(ctx, sqlccliente.SetPatrimonioResponsavelParams{ID: x.ID, ResponsavelID: resp})
			case novo != "":
				err = qt.SetPatrimonioStatus(ctx, sqlccliente.SetPatrimonioStatusParams{ID: x.ID, Status: novo})
			}
		}
		if err == nil {
			err = h.registrarEventoPatrimonio(r, qt, x.ID, tipo, texto)
		}
		if err != nil {
			h.falhaInterna(w, r, "registrar evento do patrimônio", err)
			return
		}
		h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "patrimonio." + acao, Entidade: "patrimonio", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: x.NumeroPatrimonio + " — " + x.Nome,
			Antes: map[string]any{"status": x.Status}, Detalhes: map[string]any{"descricao": texto}})
		if err = tx.Commit(ctx); err != nil {
			h.falhaInterna(w, r, "gravar evento do patrimônio", err)
			return
		}
		http.Redirect(w, r, patrimonioCaminho(x.ID)+"?aviso=evento", http.StatusSeeOther)
	}
}

func (h *handlers) patrimonioExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if x, ok := h.carregarPatrimonio(w, r, q); ok {
		h.excluirRegistro(w, r, "patrimonio", x.ID, "/estoque/patrimonio?aviso=excluido", patrimonioCaminho(x.ID))
	}
}

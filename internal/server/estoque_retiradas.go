package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Estoque (11.2): retiradas de campo e permanentes, e o balanço físico.

var (
	rotulosTipoRetirada = []views.CidadeOpcao{
		{Valor: "campo", Rotulo: "Campo"},
		{Valor: "permanente", Rotulo: "Permanente"},
	}
	rotulosStatusRetirada = []views.CidadeOpcao{
		{Valor: "aberta", Rotulo: "Aberta"},
		{Valor: "finalizada", Rotulo: "Finalizada"},
		{Valor: "cancelada", Rotulo: "Cancelada"},
	}
)

func estoqueRetiradaCaminho(id int64) string { return "/estoque/retirada/" + strconv.FormatInt(id, 10) }

// moverSaldo trava o lote, aplica a variação (se houver) e registra a movimentação. Devolve o saldo final.
// delta muda o saldo; para uso e perda já baixados na retirada, passe delta = 0 e a quantidade em info.
func (h *handlers) moverSaldo(ctx context.Context, r *http.Request, qt *sqlccliente.Queries, loteID int64, tipo string, delta, info float64, desc string, retirada int64) (float64, error) {
	return h.moverSaldoDe(ctx, r, qt, loteID, tipo, delta, info, desc, "retirada", retirada)
}

// moverSaldoDe é o moverSaldo com a origem da movimentação (retirada, diária...).
func (h *handlers) moverSaldoDe(ctx context.Context, r *http.Request, qt *sqlccliente.Queries, loteID int64, tipo string, delta, info float64, desc, origem string, retirada int64) (float64, error) {
	l, err := qt.GetEstoqueLoteParaAjuste(ctx, loteID)
	if err != nil {
		return 0, err
	}
	saldo := math.Round((qtdFloat(l.Saldo)+delta)*1000) / 1000
	if saldo < 0 {
		return 0, errors.New("O saldo do lote " + l.NumeroLote + " (" + qtdTexto(l.Saldo) + ") não cobre essa saída.")
	}
	if delta != 0 {
		if err := qt.SetEstoqueLoteSaldo(ctx, sqlccliente.SetEstoqueLoteSaldoParams{ID: loteID, Saldo: numericoDe(saldo, 3)}); err != nil {
			return 0, err
		}
	}
	qtd := delta
	if delta == 0 {
		qtd = info
	}
	return saldo, qt.CreateEstoqueMovimentacao(ctx, sqlccliente.CreateEstoqueMovimentacaoParams{
		LoteID: loteID, Tipo: tipo, Quantidade: numericoDe(qtd, 3), SaldoApos: numericoDe(saldo, 3),
		UsuarioID: idOuNulo(meuID(r)), Descricao: desc, Origem: origem, RetiradaID: idOuNulo(retirada),
	})
}

// ------------------------------------------------------------------ lista

func (h *handlers) estoqueRetiradas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.EstoqueRetiradaFiltro{Status: qs.Get("status"), Tipo: qs.Get("tipo"), Responsavel: qs.Get("responsavel")}
	if !validoEm(rotulosStatusRetirada, f.Status) {
		f.Status = ""
	}
	if !validoEm(rotulosTipoRetirada, f.Tipo) {
		f.Tipo = ""
	}
	ordem, dir := ordemDeEstoque(qs.Get("ordem"), qs.Get("dir"), "id", "saida")
	pag := paginaDe(r)
	total, err := q.CountEstoqueRetiradas(ctx, sqlccliente.CountEstoqueRetiradasParams{Status: f.Status, Tipo: f.Tipo, UsuarioID: idOpcional(f.Responsavel)})
	if err != nil {
		h.falhaInterna(w, r, "contar retiradas", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListEstoqueRetiradas(ctx, sqlccliente.ListEstoqueRetiradasParams{
		Status: f.Status, Tipo: f.Tipo, UsuarioID: idOpcional(f.Responsavel), Ordem: ordem, Dir: dir,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar retiradas", err)
		return
	}
	fuso := h.fuso(r)
	d := views.EstoqueRetiradasData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: h.principal(r).Pode("estoque_retirada_novo")}
	for _, x := range rows {
		ret := ""
		if x.DataRetornoReal.Valid {
			ret = x.DataRetornoReal.Time.In(fuso).Format("02/01/2006 15:04")
		}
		d.Linhas = append(d.Linhas, views.EstoqueRetiradaLinha{
			IDTexto: strconv.FormatInt(x.ID, 10), Tipo: rotuloDe(rotulosTipoRetirada, x.Tipo), Responsavel: x.Responsavel,
			Saida: x.DataSaida.Time.In(fuso).Format("02/01/2006 15:04"), Previsto: dataBR(x.DataRetornoPrevista), Retorno: ret,
			Status: rotuloDe(rotulosStatusRetirada, x.Status), Itens: int(x.Itens),
		})
	}
	if d.Responsaveis, err = h.opcoesDeAgentes(r, q, idOpcional(f.Responsavel).Int64); err != nil {
		h.falhaInterna(w, r, "listar responsáveis", err)
		return
	}
	d.Status, d.Tipos = marcarOpcao(rotulosStatusRetirada, f.Status), marcarOpcao(rotulosTipoRetirada, f.Tipo)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"status": f.Status, "tipo": f.Tipo, "responsavel": f.Responsavel} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.EstoqueRetiradas(h.layout(r, "Retiradas", "/estoque/retirada"), d))
}

// ------------------------------------------------------------------ criar e editar

func (h *handlers) estoqueRetiradaNova(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	resp, err := h.opcoesDeAgentes(r, q, meuID(r))
	if err != nil {
		h.falhaInterna(w, r, "listar responsáveis", err)
		return
	}
	d := views.EstoqueRetiradaForm{
		Tipos: marcarOpcao(rotulosTipoRetirada, "campo"), Responsaveis: resp, PodeEditar: true,
		Saida: time.Now().In(h.fuso(r)).Format(formatoHoraLocal), CSRF: csrfToken(r.Context()),
	}
	render(w, r, views.EstoqueRetiradaFicha(h.layout(r, "Nova retirada", "/estoque/retirada"), d))
}

func (h *handlers) estoqueRetiradaCriar(w http.ResponseWriter, r *http.Request) {
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
	tipo, obs := r.PostFormValue("tipo"), strings.TrimSpace(r.PostFormValue("observacao"))
	resp := idOpcional(r.PostFormValue("responsavel"))
	previsto, erro := dataOpcionalTexto(r.PostFormValue("previsto"), "Retorno previsto")
	saida := time.Now()
	if erro == "" && strings.TrimSpace(r.PostFormValue("saida")) != "" {
		t, e := time.ParseInLocation(formatoHoraLocal, r.PostFormValue("saida"), h.fuso(r))
		if e != nil {
			erro = "Saída: data e hora inválidas."
		} else {
			saida = t
		}
	}
	switch {
	case erro != "":
	case !validoEm(rotulosTipoRetirada, tipo):
		erro = "Escolha o tipo da retirada."
	case !resp.Valid:
		erro = "Escolha o responsável."
	case len([]rune(obs)) > 1000:
		erro = "A observação aceita até 1000 caracteres."
	}
	if erro == "" {
		if _, e := q.GetUsuarioByID(ctx, resp.Int64); e != nil {
			erro = "Escolha um responsável válido."
		}
	}
	if erro == "" {
		x, err := q.CreateEstoqueRetirada(ctx, sqlccliente.CreateEstoqueRetiradaParams{
			Tipo: tipo, UsuarioID: resp.Int64, DataSaida: pgtype.Timestamptz{Time: saida, Valid: true}, DataRetornoPrevista: previsto,
			Observacao: obs, CriadoPor: idOuNulo(meuID(r)),
		})
		if err != nil {
			h.falhaInterna(w, r, "criar retirada", err)
			return
		}
		h.auditar(r, audit.Evento{Acao: "estoque_retirada.criar", Entidade: "estoque_retirada", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: "Retirada " + strconv.FormatInt(x.ID, 10),
			Depois: map[string]any{"tipo": tipo, "responsavel_id": resp.Int64, "retorno_previsto": dataBR(previsto)}})
		http.Redirect(w, r, estoqueRetiradaCaminho(x.ID)+"?aviso=criada", http.StatusSeeOther)
		return
	}
	rs, err := h.opcoesDeAgentes(r, q, resp.Int64)
	if err != nil {
		h.falhaInterna(w, r, "listar responsáveis", err)
		return
	}
	d := views.EstoqueRetiradaForm{
		Tipos: marcarOpcao(rotulosTipoRetirada, tipo), Responsaveis: rs, PodeEditar: true, Saida: r.PostFormValue("saida"),
		Previsto: r.PostFormValue("previsto"), Observacao: obs, CSRF: csrfToken(ctx), Erro: erro,
	}
	renderStatus(w, r, http.StatusBadRequest, views.EstoqueRetiradaFicha(h.layout(r, "Nova retirada", "/estoque/retirada"), d))
}

func (h *handlers) carregarEstoqueRetirada(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.EstoqueRetirada, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.EstoqueRetirada{}, false
	}
	x, err := q.GetEstoqueRetirada(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return x, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar retirada", err)
		return x, false
	}
	return x, true
}

func (h *handlers) renderEstoqueRetirada(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, x sqlccliente.EstoqueRetirada, erro string) {
	ctx := r.Context()
	fuso := h.fuso(r)
	pr := h.principal(r)
	aberta := x.Status == "aberta"
	d := views.EstoqueRetiradaForm{
		ID: x.ID, Tipo: x.Tipo, TipoRotulo: rotuloDe(rotulosTipoRetirada, x.Tipo), Status: x.Status, StatusRotulo: rotuloDe(rotulosStatusRetirada, x.Status),
		Saida: x.DataSaida.Time.In(fuso).Format("02/01/2006 15:04"), Previsto: dataISO(x.DataRetornoPrevista), Observacao: x.Observacao,
		PodeEditar: aberta && pr.Pode("estoque_retirada_novo"), PodeFinalizar: pr.Pode("estoque_retirada_finalizar"), PodeCancelar: pr.Pode("estoque_retirada_cancelar"),
		CSRF: csrfToken(ctx), Erro: erro,
	}
	if x.DataRetornoReal.Valid {
		d.Retorno = x.DataRetornoReal.Time.In(fuso).Format("02/01/2006 15:04")
	}
	switch r.URL.Query().Get("aviso") {
	case "criada":
		d.Aviso = "Retirada criada. Adicione os itens."
	case "salvo":
		d.Aviso = "Alterações salvas."
	case "item":
		d.Aviso = "Item adicionado; o saldo do lote foi reduzido."
	case "removido":
		d.Aviso = "Item removido; a quantidade voltou ao lote."
	case "finalizada":
		d.Aviso = "Retirada finalizada."
	case "cancelada":
		d.Aviso = "Retirada cancelada; os itens voltaram ao estoque."
	}
	var err error
	if d.Responsaveis, err = h.opcoesDeAgentes(r, q, x.UsuarioID); err != nil {
		h.falhaInterna(w, r, "listar responsáveis", err)
		return
	}
	itens, err := q.ListEstoqueRetiradaItens(ctx, x.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar itens da retirada", err)
		return
	}
	for _, i := range itens {
		d.Itens = append(d.Itens, views.EstoqueRetiradaItem{
			ID: strconv.FormatInt(i.ID, 10), Produto: i.Produto, Lote: i.NumeroLote, Medida: i.UnidadeMedida,
			Retirado: qtdTexto(i.Retirado), Usado: qtdTexto(i.Usado), Devolvido: qtdTexto(i.Devolvido), Perda: qtdTexto(i.Perda),
		})
	}
	if aberta && d.PodeEditar {
		lotes, err := q.ListEstoqueLotesDisponiveis(ctx, x.Tipo)
		if err != nil {
			h.falhaInterna(w, r, "listar lotes disponíveis", err)
			return
		}
		for _, l := range lotes {
			d.Lotes = append(d.Lotes, views.CidadeOpcao{ID: l.ID, Rotulo: l.Rotulo})
		}
	}
	status := http.StatusOK
	if erro != "" {
		status = http.StatusBadRequest
	}
	renderStatus(w, r, status, views.EstoqueRetiradaFicha(h.layout(r, "Retirada "+strconv.FormatInt(x.ID, 10), "/estoque/retirada"), d))
}

func (h *handlers) estoqueRetiradaFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if x, ok := h.carregarEstoqueRetirada(w, r, q); ok {
		h.renderEstoqueRetirada(w, r, q, x, "")
	}
}

func (h *handlers) estoqueRetiradaSalvar(w http.ResponseWriter, r *http.Request) {
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
	x, ok := h.carregarEstoqueRetirada(w, r, q)
	if !ok {
		return
	}
	if x.Status != "aberta" {
		h.renderEstoqueRetirada(w, r, q, x, "Só retiradas abertas podem ser alteradas.")
		return
	}
	resp := idOpcional(r.PostFormValue("responsavel"))
	obs := strings.TrimSpace(r.PostFormValue("observacao"))
	previsto, erro := dataOpcionalTexto(r.PostFormValue("previsto"), "Retorno previsto")
	if erro == "" && !resp.Valid {
		erro = "Escolha o responsável."
	}
	if erro == "" && len([]rune(obs)) > 1000 {
		erro = "A observação aceita até 1000 caracteres."
	}
	if erro == "" {
		if _, e := q.GetUsuarioByID(ctx, resp.Int64); e != nil {
			erro = "Escolha um responsável válido."
		}
	}
	if erro != "" {
		h.renderEstoqueRetirada(w, r, q, x, erro)
		return
	}
	if _, err = q.UpdateEstoqueRetiradaDados(ctx, sqlccliente.UpdateEstoqueRetiradaDadosParams{ID: x.ID, UsuarioID: resp.Int64, DataRetornoPrevista: previsto, Observacao: obs}); err != nil {
		h.falhaInterna(w, r, "salvar retirada", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "estoque_retirada.editar", Entidade: "estoque_retirada", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: "Retirada " + strconv.FormatInt(x.ID, 10),
		Antes:  map[string]any{"responsavel_id": x.UsuarioID, "retorno_previsto": dataBR(x.DataRetornoPrevista)},
		Depois: map[string]any{"responsavel_id": resp.Int64, "retorno_previsto": dataBR(previsto)}})
	http.Redirect(w, r, estoqueRetiradaCaminho(x.ID)+"?aviso=salvo", http.StatusSeeOther)
}

// ------------------------------------------------------------------ itens

func (h *handlers) estoqueRetiradaItemAdicionar(w http.ResponseWriter, r *http.Request) {
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
	x, ok := h.carregarEstoqueRetirada(w, r, q)
	if !ok {
		return
	}
	lote := idOpcional(r.PostFormValue("lote"))
	qtd, erro := lerQuantidade(r.PostFormValue("quantidade"), "Quantidade", false)
	if erro == "" && !lote.Valid {
		erro = "Escolha o produto e o lote."
	}
	var l sqlccliente.EstoqueLote
	var p sqlccliente.EstoqueProduto
	if erro == "" {
		var e error
		if l, e = q.GetEstoqueLote(ctx, lote.Int64); e != nil {
			erro = "Escolha um lote válido."
		} else if p, e = q.GetEstoqueProduto(ctx, l.ProdutoID); e != nil || !p.Ativo {
			erro = "O produto desse lote não está ativo."
		} else if (x.Tipo == "campo" && p.SaidaPermitida == "permanente") || (x.Tipo == "permanente" && p.SaidaPermitida == "campo") {
			erro = "Esse produto não pode sair em retirada " + strings.ToLower(rotuloDe(rotulosTipoRetirada, x.Tipo)) + "."
		}
	}
	if erro != "" {
		h.renderEstoqueRetirada(w, r, q, x, erro)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar item da retirada", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := q.WithTx(tx)
	atual, err := qt.GetEstoqueRetiradaParaAtualizar(ctx, x.ID)
	if err != nil || atual.Status != "aberta" {
		h.renderEstoqueRetirada(w, r, q, x, "Só retiradas abertas aceitam itens.")
		return
	}
	if _, err = qt.CreateEstoqueRetiradaItem(ctx, sqlccliente.CreateEstoqueRetiradaItemParams{RetiradaID: x.ID, LoteID: l.ID, Retirado: numericoDe(qtd, 3)}); err != nil {
		if violacaoUnica(err) {
			h.renderEstoqueRetirada(w, r, q, x, "Esse lote já está na retirada. Remova o item e adicione de novo com a quantidade certa.")
			return
		}
		h.falhaInterna(w, r, "adicionar item da retirada", err)
		return
	}
	if _, err = h.moverSaldo(ctx, r, qt, l.ID, "retirada", -qtd, 0, "Retirada "+strconv.FormatInt(x.ID, 10), x.ID); err != nil {
		if strings.HasPrefix(err.Error(), "O saldo") {
			h.renderEstoqueRetirada(w, r, q, x, err.Error())
			return
		}
		h.falhaInterna(w, r, "baixar saldo da retirada", err)
		return
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "estoque_retirada.item", Entidade: "estoque_retirada", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: "Retirada " + strconv.FormatInt(x.ID, 10),
		Detalhes: map[string]any{"acao": "adicionar", "produto": p.Nome, "lote": l.NumeroLote, "quantidade": qtdTexto(numericoDe(qtd, 3))}})
	if err = tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "gravar item da retirada", err)
		return
	}
	http.Redirect(w, r, estoqueRetiradaCaminho(x.ID)+"?aviso=item", http.StatusSeeOther)
}

func (h *handlers) estoqueRetiradaItemRemover(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	x, ok := h.carregarEstoqueRetirada(w, r, q)
	if !ok {
		return
	}
	iid := idDaChave(r, "iid")
	if x.Status != "aberta" {
		h.renderEstoqueRetirada(w, r, q, x, "Só retiradas abertas aceitam alterações nos itens.")
		return
	}
	it, err := q.GetEstoqueRetiradaItem(ctx, sqlccliente.GetEstoqueRetiradaItemParams{ID: iid, RetiradaID: x.ID})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar remoção de item", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := q.WithTx(tx)
	if _, err = qt.GetEstoqueRetiradaParaAtualizar(ctx, x.ID); err != nil {
		h.falhaInterna(w, r, "travar retirada", err)
		return
	}
	if err = qt.DeleteEstoqueRetiradaItem(ctx, it.ID); err == nil {
		_, err = h.moverSaldo(ctx, r, qt, it.LoteID, "devolucao", qtdFloat(it.Retirado), 0, "Item removido da retirada "+strconv.FormatInt(x.ID, 10), x.ID)
	}
	if err != nil {
		h.falhaInterna(w, r, "remover item da retirada", err)
		return
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "estoque_retirada.item", Entidade: "estoque_retirada", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: "Retirada " + strconv.FormatInt(x.ID, 10),
		Detalhes: map[string]any{"acao": "remover", "lote_id": it.LoteID, "quantidade": qtdTexto(it.Retirado)}})
	if err = tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "gravar remoção de item", err)
		return
	}
	http.Redirect(w, r, estoqueRetiradaCaminho(x.ID)+"?aviso=removido", http.StatusSeeOther)
}

func idDaChave(r *http.Request, nome string) int64 {
	n, _ := strconv.ParseInt(chi.URLParam(r, nome), 10, 64)
	return n
}

// ------------------------------------------------------------------ finalizar e cancelar

func (h *handlers) estoqueRetiradaFinalizar(w http.ResponseWriter, r *http.Request) {
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
	x, ok := h.carregarEstoqueRetirada(w, r, q)
	if !ok {
		return
	}
	itens, err := q.ListEstoqueRetiradaItens(ctx, x.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar itens da retirada", err)
		return
	}
	type final struct{ usado, devolvido, perda float64 }
	finais := make([]final, len(itens))
	var erro string
	if x.Status != "aberta" {
		erro = "Essa retirada já foi encerrada."
	} else if len(itens) == 0 {
		erro = "Adicione ao menos um item antes de finalizar."
	}
	for n, i := range itens {
		if erro != "" {
			break
		}
		id := strconv.FormatInt(i.ID, 10)
		nome := i.Produto + " (lote " + i.NumeroLote + ")"
		var f final
		if x.Tipo == "campo" {
			if f.usado, erro = lerQuantidade(r.PostFormValue("usado_"+id), nome+": usado", true); erro != "" {
				break
			}
		}
		if f.devolvido, erro = lerQuantidade(r.PostFormValue("devolvido_"+id), nome+": devolvido", true); erro != "" {
			break
		}
		if f.perda, erro = lerQuantidade(r.PostFormValue("perda_"+id), nome+": perda", true); erro != "" {
			break
		}
		if math.Abs(f.usado+f.devolvido+f.perda-qtdFloat(i.Retirado)) > 0.0005 {
			erro = fmt.Sprintf("%s: usado + devolvido + perda deve ser %s.", nome, qtdTexto(i.Retirado))
			break
		}
		finais[n] = f
	}
	if erro != "" {
		h.renderEstoqueRetirada(w, r, q, x, erro)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar finalização", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := q.WithTx(tx)
	atual, err := qt.GetEstoqueRetiradaParaAtualizar(ctx, x.ID)
	if err != nil || atual.Status != "aberta" {
		h.renderEstoqueRetirada(w, r, q, x, "Essa retirada já foi encerrada.")
		return
	}
	rot := strconv.FormatInt(x.ID, 10)
	for n, i := range itens {
		f := finais[n]
		if err = qt.SetEstoqueRetiradaItemFinal(ctx, sqlccliente.SetEstoqueRetiradaItemFinalParams{ID: i.ID, Usado: numericoDe(f.usado, 3), Devolvido: numericoDe(f.devolvido, 3), Perda: numericoDe(f.perda, 3)}); err != nil {
			break
		}
		if f.devolvido > 0 {
			if _, err = h.moverSaldo(ctx, r, qt, i.LoteID, "devolucao", f.devolvido, 0, "Devolução da retirada "+rot, x.ID); err != nil {
				break
			}
		}
		if f.usado > 0 {
			if _, err = h.moverSaldo(ctx, r, qt, i.LoteID, "uso", 0, f.usado, "Uso em campo (já baixado na retirada "+rot+")", x.ID); err != nil {
				break
			}
		}
		if f.perda > 0 {
			if _, err = h.moverSaldo(ctx, r, qt, i.LoteID, "perda", 0, f.perda, "Perda (já baixada na retirada "+rot+")", x.ID); err != nil {
				break
			}
		}
	}
	if err == nil {
		err = qt.SetEstoqueRetiradaStatus(ctx, sqlccliente.SetEstoqueRetiradaStatusParams{ID: x.ID, Status: "finalizada", DataRetornoReal: pgtype.Timestamptz{Time: time.Now(), Valid: true}})
	}
	if err != nil {
		h.falhaInterna(w, r, "finalizar retirada", err)
		return
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "estoque_retirada.finalizar", Entidade: "estoque_retirada", EntidadeID: rot, Rotulo: "Retirada " + rot,
		Antes: map[string]any{"status": "aberta"}, Depois: map[string]any{"status": "finalizada", "itens": len(itens)}})
	if err = tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "gravar finalização", err)
		return
	}
	http.Redirect(w, r, estoqueRetiradaCaminho(x.ID)+"?aviso=finalizada", http.StatusSeeOther)
}

func (h *handlers) estoqueRetiradaCancelar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	x, ok := h.carregarEstoqueRetirada(w, r, q)
	if !ok {
		return
	}
	if x.Status != "aberta" {
		h.renderEstoqueRetirada(w, r, q, x, "Só retiradas abertas podem ser canceladas.")
		return
	}
	itens, err := q.ListEstoqueRetiradaItens(ctx, x.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar itens da retirada", err)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar cancelamento", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := q.WithTx(tx)
	atual, err := qt.GetEstoqueRetiradaParaAtualizar(ctx, x.ID)
	if err != nil || atual.Status != "aberta" {
		h.renderEstoqueRetirada(w, r, q, x, "Só retiradas abertas podem ser canceladas.")
		return
	}
	rot := strconv.FormatInt(x.ID, 10)
	for _, i := range itens {
		if _, err = h.moverSaldo(ctx, r, qt, i.LoteID, "devolucao", qtdFloat(i.Retirado), 0, "Cancelamento da retirada "+rot, x.ID); err != nil {
			h.falhaInterna(w, r, "devolver itens ao estoque", err)
			return
		}
	}
	if err = qt.SetEstoqueRetiradaStatus(ctx, sqlccliente.SetEstoqueRetiradaStatusParams{ID: x.ID, Status: "cancelada"}); err != nil {
		h.falhaInterna(w, r, "cancelar retirada", err)
		return
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "estoque_retirada.cancelar", Entidade: "estoque_retirada", EntidadeID: rot, Rotulo: "Retirada " + rot,
		Antes: map[string]any{"status": "aberta"}, Depois: map[string]any{"status": "cancelada", "itens": len(itens)}})
	if err = tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "gravar cancelamento", err)
		return
	}
	http.Redirect(w, r, estoqueRetiradaCaminho(x.ID)+"?aviso=cancelada", http.StatusSeeOther)
}

func (h *handlers) estoqueRetiradaExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if x, ok := h.carregarEstoqueRetirada(w, r, q); ok {
		h.excluirRegistro(w, r, "estoque_retirada", x.ID, "/estoque/retirada?aviso=excluido", estoqueRetiradaCaminho(x.ID))
	}
}

// ------------------------------------------------------------------ balanço físico

func (h *handlers) estoqueBalanco(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	unidade, tipo := qs.Get("unidade"), qs.Get("tipo")
	if !validoEm(rotulosTipoProduto, tipo) {
		tipo = ""
	}
	pag := paginaDe(r)
	total, err := q.CountEstoqueBalanco(ctx, sqlccliente.CountEstoqueBalancoParams{UnidadeID: idOpcional(unidade), Tipo: tipo})
	if err != nil {
		h.falhaInterna(w, r, "contar balanço", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	hoje, _ := time.Parse(formatoDataISO, h.hojeISO(r))
	rows, err := q.ListEstoqueBalanco(ctx, sqlccliente.ListEstoqueBalancoParams{
		UnidadeID: idOpcional(unidade), Tipo: tipo, Hoje: pgtype.Date{Time: hoje, Valid: true},
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar balanço", err)
		return
	}
	d := views.EstoqueBalancoData{Unidade: unidade, Tipo: tipo, Pag: pag}
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.EstoqueBalancoLinha{
			Unidade: x.Unidade, Produto: x.Nome, Tipo: rotuloDe(rotulosTipoProduto, x.Tipo), Medida: x.UnidadeMedida,
			EmEstoque: qtdTexto(x.EmEstoque), EmRetirada: qtdTexto(x.EmRetirada),
			Total: qtdTexto(numericoDe(qtdFloat(x.EmEstoque)+qtdFloat(x.EmRetirada), 3)), Vencido: qtdTexto(x.Vencido), ProximoVencimento: dataBR(x.ProximoVencimento),
		})
	}
	if d.Unidades, err = h.opcoesDeUnidadesEstoque(r, q, idOpcional(unidade).Int64); err != nil {
		h.falhaInterna(w, r, "listar unidades", err)
		return
	}
	d.Tipos = marcarOpcao(rotulosTipoProduto, tipo)
	d.Query = url.Values{}
	for k, v := range map[string]string{"unidade": unidade, "tipo": tipo} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.EstoqueBalanco(h.layout(r, "Balanço Físico", "/estoque/balanco"), d))
}

package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Estoque (11.4): a diária da campanha escolhe os lotes de vacina e seringa e, ao finalizar, baixa o consumo.
//   - seringas: usadas + perdidas + com defeito (fim do trabalho), em unidades;
//   - vacina: frascos vazios (fim do trabalho). Frasco cheio ou aberto que sobra continua no estoque.
// Reabrir a diária estorna a baixa.

// campanhaEstoqueObrigatorio diz se o parâmetro "uso obrigatório do estoque" está ligado.
func (h *handlers) campanhaEstoqueObrigatorio(r *http.Request, q *sqlccliente.Queries) bool {
	for _, p := range parametros {
		if p.Chave == "campanha_estoque" {
			return h.valorDoParametro(r, q, p) == "1"
		}
	}
	return false
}

// opcoesDeLotesDaDiaria lista os lotes de vacina ou seringa com saldo (e o já escolhido, mesmo sem saldo).
func (h *handlers) opcoesDeLotesDaDiaria(r *http.Request, q *sqlccliente.Queries, categoria string, atual pgtype.Int8) ([]views.CidadeOpcao, error) {
	rows, err := q.ListEstoqueLotesDaDiaria(r.Context(), sqlccliente.ListEstoqueLotesDaDiariaParams{Categoria: categoria, Atual: atual})
	if err != nil {
		return nil, err
	}
	out := make([]views.CidadeOpcao, len(rows))
	for i, x := range rows {
		out[i] = views.CidadeOpcao{ID: x.ID, Rotulo: x.Rotulo, Selecionada: atual.Valid && atual.Int64 == x.ID}
	}
	return out, nil
}

// diariaEstoque grava os lotes escolhidos para a diária.
func (h *handlers) diariaEstoque(w http.ResponseWriter, r *http.Request) {
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
	n, ok := h.carregarDiaria(w, r, q)
	if !ok {
		return
	}
	if n.Status != "aguardando" && n.Status != "executando" {
		h.renderDiaria(w, r, q, n, "Diária fechada: reabra para trocar os lotes.")
		return
	}
	vac, ser := idOpcional(r.PostFormValue("vacina_lote")), idOpcional(r.PostFormValue("seringa_lote"))
	if vac.Valid {
		if _, e := q.GetEstoqueLoteDaCategoria(ctx, sqlccliente.GetEstoqueLoteDaCategoriaParams{ID: vac.Int64, Categoria: "vacina"}); e != nil {
			h.renderDiaria(w, r, q, n, "Escolha um lote de vacina válido.")
			return
		}
	}
	if ser.Valid {
		if _, e := q.GetEstoqueLoteDaCategoria(ctx, sqlccliente.GetEstoqueLoteDaCategoriaParams{ID: ser.Int64, Categoria: "seringa"}); e != nil {
			h.renderDiaria(w, r, q, n, "Escolha um lote de seringa válido.")
			return
		}
	}
	if _, err = q.SetDiariaLotes(ctx, sqlccliente.SetDiariaLotesParams{ID: n.ID, VacinaLoteID: vac, SeringaLoteID: ser}); err != nil {
		h.falhaInterna(w, r, "salvar lotes da diária", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "diaria.estoque", Entidade: "diaria", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDiaria(n),
		Antes:  map[string]any{"vacina_lote": n.VacinaLoteID.Int64, "seringa_lote": n.SeringaLoteID.Int64},
		Depois: map[string]any{"vacina_lote": vac.Int64, "seringa_lote": ser.Int64}})
	http.Redirect(w, r, diariaCaminho(n.ID)+"?aviso=salva", http.StatusSeeOther)
}

func (h *handlers) diariaFinalizar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDiaria(w, r, q)
	if !ok {
		return
	}
	if n.Status != "executando" {
		h.renderDiaria(w, r, q, n, "Só uma diária em execução pode ser finalizada.")
		return
	}
	if h.campanhaEstoqueObrigatorio(r, q) && (!n.VacinaLoteID.Valid || !n.SeringaLoteID.Valid) {
		h.renderDiaria(w, r, q, n, "O uso do estoque é obrigatório na campanha: escolha os lotes de vacina e de seringa antes de finalizar.")
		return
	}
	v := valoresDeDiaria(n)
	type baixa struct {
		lote       int64
		categoria  string
		quantidade float64
		rotulo     string
	}
	var baixas []baixa
	if n.SeringaLoteID.Valid {
		if qtd := float64(v["fim_seringas_usadas"] + v["fim_seringas_perdidas"] + v["fim_seringas_defeito"]); qtd > 0 {
			baixas = append(baixas, baixa{n.SeringaLoteID.Int64, "seringa", qtd, "Seringas da diária"})
		}
	}
	if n.VacinaLoteID.Valid {
		if qtd := float64(v["fim_frascos_vazios"]); qtd > 0 {
			baixas = append(baixas, baixa{n.VacinaLoteID.Int64, "vacina", qtd, "Frascos de vacina da diária"})
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar finalização da diária", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := q.WithTx(tx)
	rot := strconv.FormatInt(n.ID, 10)
	for _, b := range baixas {
		if _, err = h.moverSaldoDe(ctx, r, qt, b.lote, "uso", -b.quantidade, 0, fmt.Sprintf("%s %s (campanha antirrábica)", b.rotulo, rot), "diaria", 0); err != nil {
			if len(err.Error()) > 8 && err.Error()[:8] == "O saldo " {
				h.renderDiaria(w, r, q, n, err.Error())
				return
			}
			h.falhaInterna(w, r, "baixar estoque da diária", err)
			return
		}
		if err = qt.CreateDiariaBaixa(ctx, sqlccliente.CreateDiariaBaixaParams{DiariaID: n.ID, LoteID: b.lote, Categoria: b.categoria, Quantidade: numericoDe(b.quantidade, 3)}); err != nil {
			h.falhaInterna(w, r, "registrar baixa da diária", err)
			return
		}
	}
	if _, err = qt.SetDiariaStatus(ctx, sqlccliente.SetDiariaStatusParams{ID: n.ID, Status: "finalizado"}); err != nil {
		h.falhaInterna(w, r, "finalizar diária", err)
		return
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "diaria.finalizar", Entidade: "diaria", EntidadeID: rot, Rotulo: rotuloDiaria(n),
		Antes: map[string]any{"status": n.Status}, Depois: map[string]any{"status": "finalizado", "baixas_de_estoque": len(baixas)}})
	if err = tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "gravar finalização da diária", err)
		return
	}
	http.Redirect(w, r, diariaCaminho(n.ID)+"?aviso=salva", http.StatusSeeOther)
}

func (h *handlers) diariaReabrir(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDiaria(w, r, q)
	if !ok {
		return
	}
	if n.Status != "finalizado" {
		h.renderDiaria(w, r, q, n, "Só uma diária finalizada pode ser reaberta.")
		return
	}
	baixas, err := q.ListDiariaBaixas(ctx, n.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar baixas da diária", err)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar reabertura da diária", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := q.WithTx(tx)
	rot := strconv.FormatInt(n.ID, 10)
	for _, b := range baixas {
		if _, err = h.moverSaldoDe(ctx, r, qt, b.LoteID, "devolucao", qtdFloat(b.Quantidade), 0, "Estorno: diária "+rot+" reaberta", "diaria", 0); err != nil {
			h.falhaInterna(w, r, "estornar estoque da diária", err)
			return
		}
	}
	if err = qt.DeleteDiariaBaixas(ctx, n.ID); err == nil {
		_, err = qt.SetDiariaStatus(ctx, sqlccliente.SetDiariaStatusParams{ID: n.ID, Status: "executando"})
	}
	if err != nil {
		h.falhaInterna(w, r, "reabrir diária", err)
		return
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "diaria.reabrir", Entidade: "diaria", EntidadeID: rot, Rotulo: rotuloDiaria(n),
		Antes: map[string]any{"status": n.Status}, Depois: map[string]any{"status": "executando", "estornos_de_estoque": len(baixas)}})
	if err = tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "gravar reabertura da diária", err)
		return
	}
	http.Redirect(w, r, diariaCaminho(n.ID)+"?aviso=salva", http.StatusSeeOther)
}

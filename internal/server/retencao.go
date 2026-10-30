package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// regraRetencao diz de que tabela, e com que critério, os dados pessoais de casos encerrados vencem.
type regraRetencao struct {
	Chave, Tabela, Rotulo string
	Pendente              string // ainda tem dado pessoal a apagar (evita contar de novo o que já foi anonimizado)
}

var regrasRetencao = []regraRetencao{
	{"lgpd_ret_denuncias", "denuncias", "Denúncias", "(cidadao_nome <> '' OR cidadao_telefone <> '' OR cidadao_cpf <> '')"},
	{"lgpd_ret_investigacoes", "investigacoes", "Investigações", "(paciente <> 'ANONIMIZADO' OR telefone <> '')"},
	{"lgpd_ret_baap", "baap_notificacoes", "Notificações BAAP", "(paciente <> 'ANONIMIZADO' OR nome_mae <> '' OR telefone <> '' OR telefone2 <> '' OR sinam <> '')"},
}

const maxMesesRetencao = 600

// intervaloDaRetencao é de quanto em quanto tempo o servidor aplica os prazos em todos os clientes.
const intervaloDaRetencao = 6 * time.Hour

func prazoDe(ctx context.Context, q *sqlccliente.Queries, chave string) int {
	info, err := q.GetClienteInfo(ctx, chave)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(info.Valor))
	if err != nil || n < 0 || n > maxMesesRetencao {
		return 0
	}
	return n
}

func idsVencidos(ctx context.Context, db consultadorLGPD, r regraRetencao, meses int) ([]int64, error) {
	rows, err := db.Query(ctx,
		"SELECT id FROM "+r.Tabela+" WHERE situacao IN ('encerrada', 'cancelada') AND COALESCE(encerrada_em, criado_em) < now() - make_interval(months => $1::int) AND "+r.Pendente,
		meses)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func alvoDaTabela(tabela string) (alvoLGPD, bool) {
	for _, a := range alvosLGPD {
		if a.Tabela == tabela {
			return a, true
		}
	}
	return alvoLGPD{}, false
}

// aplicarRetencao anonimiza, num cliente, os casos encerrados que passaram do prazo. Prazo 0 = não apaga.
// Devolve quantos registros mudaram em cada tabela; grava um evento de auditoria se algo mudou.
func aplicarRetencao(ctx context.Context, pool *pgxpool.Pool, q *sqlccliente.Queries, ator audit.Ator) (map[string]int, int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	feitos := map[string]int{}
	total := 0
	for _, r := range regrasRetencao {
		meses := prazoDe(ctx, q, r.Chave)
		if meses == 0 {
			continue
		}
		ids, err := idsVencidos(ctx, tx, r, meses)
		if err != nil {
			return nil, 0, err
		}
		if len(ids) == 0 {
			continue
		}
		alvo, ok := alvoDaTabela(r.Tabela)
		if !ok {
			continue
		}
		if _, err := tx.Exec(ctx, "UPDATE "+alvo.Tabela+" SET "+alvo.Limpar+" WHERE id = ANY($1)", ids); err != nil {
			return nil, 0, err
		}
		feitos[r.Tabela] = len(ids)
		total += len(ids)
	}
	if total > 0 {
		det := map[string]any{"total": total}
		for t, n := range feitos {
			det[t] = n
		}
		if err := (audit.Cliente{Q: q.WithTx(tx)}).Registrar(ctx, audit.Evento{
			Ator: ator, Acao: "lgpd.retencao", Entidade: "lgpd", Rotulo: "Retenção por prazo", Detalhes: det,
		}); err != nil {
			return nil, 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return feitos, total, nil
}

// aplicarRetencaoEmSegundoPlano aplica os prazos em todos os clientes ativos até o contexto acabar.
func (h *handlers) aplicarRetencaoEmSegundoPlano(ctx context.Context) {
	espera := time.NewTimer(2 * time.Minute)
	defer espera.Stop()
	t := time.NewTicker(intervaloDaRetencao)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-espera.C:
		case <-t.C:
		}
		h.retencaoDeTodos(ctx)
	}
}

func (h *handlers) retencaoDeTodos(ctx context.Context) {
	clientes, err := h.pub.ListClientes(ctx)
	if err != nil {
		h.log.Error("retenção LGPD: listar clientes", "erro", err)
		return
	}
	for _, c := range clientes {
		if c.Status == "cancelado" || c.Status == "provisionando" {
			continue
		}
		pool, err := h.pools.Get(ctx, c.SchemaNome)
		if err != nil {
			h.log.Error("retenção LGPD: banco do cliente", "cliente", c.Slug, "erro", err)
			continue
		}
		if _, _, err := aplicarRetencao(ctx, pool, sqlccliente.New(pool), audit.Ator{Tipo: audit.AtorSistema, Nome: "Retenção automática"}); err != nil {
			h.log.Error("retenção LGPD: aplicar", "cliente", c.Slug, "erro", err)
		}
	}
}

func (h *handlers) retencaoPagina(w http.ResponseWriter, r *http.Request) {
	h.renderRetencao(w, r, "", "", http.StatusOK)
}

func (h *handlers) renderRetencao(w http.ResponseWriter, r *http.Request, aviso, erro string, status int) {
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := views.RetencaoData{Aviso: aviso, Erro: erro, CSRF: csrfToken(r.Context())}
	for _, rg := range regrasRetencao {
		meses := prazoDe(r.Context(), q, rg.Chave)
		l := views.RetencaoLinha{Chave: rg.Chave, Rotulo: rg.Rotulo, Meses: meses}
		if meses > 0 {
			ids, err := idsVencidos(r.Context(), pool, rg, meses)
			if err != nil {
				h.falhaInterna(w, r, "contar vencidos", err)
				return
			}
			l.Vencidos = len(ids)
		}
		d.Linhas = append(d.Linhas, l)
	}
	lay := h.layout(r, "Retenção de dados (LGPD)", "/lgpd/retencao")
	w.WriteHeader(status)
	render(w, r, views.Retencao(lay, d))
}

func (h *handlers) retencaoSalvar(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	novos := map[string]int{}
	for _, rg := range regrasRetencao {
		txt := strings.TrimSpace(r.PostFormValue(rg.Chave))
		if txt == "" {
			txt = "0"
		}
		n, err := strconv.Atoi(txt)
		if err != nil || n < 0 || n > maxMesesRetencao {
			h.renderRetencao(w, r, "", "Informe os prazos em meses, de 0 a "+strconv.Itoa(maxMesesRetencao)+" (0 = não apagar).", http.StatusBadRequest)
			return
		}
		novos[rg.Chave] = n
	}
	antes, depois := map[string]any{}, map[string]any{}
	for _, rg := range regrasRetencao {
		a, n := prazoDe(r.Context(), q, rg.Chave), novos[rg.Chave]
		if a != n {
			antes[rg.Chave], depois[rg.Chave] = a, n
		}
		if err := q.SetClienteInfo(r.Context(), sqlccliente.SetClienteInfoParams{Chave: rg.Chave, Valor: strconv.Itoa(n)}); err != nil {
			h.falhaInterna(w, r, "gravar prazo", err)
			return
		}
	}
	if len(depois) > 0 {
		h.auditar(r, audit.Evento{Acao: "lgpd.retencao_prazos", Entidade: "lgpd", Rotulo: "Prazos de retenção", Antes: antes, Depois: depois})
	} else {
		naoAuditar(r)
	}
	h.renderRetencao(w, r, "Prazos salvos.", "", http.StatusOK)
}

func (h *handlers) retencaoExecutar(w http.ResponseWriter, r *http.Request) {
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	_, total, err := aplicarRetencao(r.Context(), pool, q, atorDaSessao(r))
	if err != nil {
		h.falhaInterna(w, r, "aplicar retenção", err)
		return
	}
	naoAuditar(r) // o evento já foi gravado junto com a mudança
	h.renderRetencao(w, r, "Pronto: "+strconv.Itoa(total)+" registro(s) anonimizado(s).", "", http.StatusOK)
}

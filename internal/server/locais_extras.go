package server

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/skip2/go-qrcode"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// localDaURL carrega o local do endereço e confere o escopo do papel; responde 404 quando não pode.
func (h *handlers) localDaURL(w http.ResponseWriter, r *http.Request) (sqlccliente.Locai, *sqlccliente.Queries, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.Locai{}, nil, false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return sqlccliente.Locai{}, nil, false
	}
	l, err := q.GetLocal(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return l, nil, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar local", err)
		return l, nil, false
	}
	if visivel, err := h.localVisivel(r.Context(), r, q, l); err != nil {
		h.falhaInterna(w, r, "escopo do local", err)
		return l, nil, false
	} else if !visivel {
		http.NotFound(w, r)
		return l, nil, false
	}
	return l, q, true
}

func rotuloDoLocal(l sqlccliente.Locai) string { return enderecoCompleto(l.Endereco, l.Numero) }

// localAtivo desativa ou reativa o local (nada é apagado; o histórico continua).
func (h *handlers) localAtivo(w http.ResponseWriter, r *http.Request) {
	l, q, ok := h.localDaURL(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	ativo := r.PostFormValue("ativo") == "1"
	resultado := map[bool]string{true: "reativado", false: "desativado"}[ativo]
	if l.Ativo == ativo {
		naoAuditar(r)
		http.Redirect(w, r, fmt.Sprintf("/locais/%d?ok=%s", l.ID, resultado), http.StatusSeeOther)
		return
	}
	if _, err := q.SetLocalAtivo(r.Context(), sqlccliente.SetLocalAtivoParams{ID: l.ID, Ativo: ativo}); err != nil {
		h.falhaInterna(w, r, "mudar situação do local", err)
		return
	}
	acao := "local.desativar"
	if ativo {
		acao = "local.reativar"
	}
	h.auditar(r, audit.Evento{
		Acao: acao, Entidade: "local", EntidadeID: strconv.FormatInt(l.ID, 10), Rotulo: rotuloDoLocal(l),
		Antes: map[string]any{"ativo": l.Ativo}, Depois: map[string]any{"ativo": ativo},
	})
	http.Redirect(w, r, fmt.Sprintf("/locais/%d?ok=%s", l.ID, resultado), http.StatusSeeOther)
}

// localNovaChave troca a chave do local (o QR Code impresso antes deixa de valer).
func (h *handlers) localNovaChave(w http.ResponseWriter, r *http.Request) {
	l, q, ok := h.localDaURL(w, r)
	if !ok {
		return
	}
	if _, err := q.NovaChaveDoLocal(r.Context(), l.ID); err != nil {
		h.falhaInterna(w, r, "gerar chave", err)
		return
	}
	// a chave em si não vai para o histórico: quem tem a auditoria não precisa conhecer o QR de cada imóvel
	h.auditar(r, audit.Evento{Acao: "local.chave_nova", Entidade: "local", EntidadeID: strconv.FormatInt(l.ID, 10), Rotulo: rotuloDoLocal(l)})
	http.Redirect(w, r, fmt.Sprintf("/locais/%d?ok=chave", l.ID), http.StatusSeeOther)
}

// localQR devolve o QR Code (PNG) da chave do local.
func (h *handlers) localQR(w http.ResponseWriter, r *http.Request) {
	l, _, ok := h.localDaURL(w, r)
	if !ok {
		return
	}
	png, err := qrcode.Encode(l.QrChave, qrcode.Medium, 320)
	if err != nil {
		h.falhaInterna(w, r, "gerar QR Code", err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(png)
}

// ------------------------------------------------------------------- Cadastro em lote

const maximoDoLote = 500

func (h *handlers) montarFormLote(r *http.Request, q *sqlccliente.Queries, c views.LoteCampos, erro string) (views.LoteFormData, error) {
	base, err := h.montarFormLocal(r.Context(), r, q, 0, views.LocalCampos{Tipo: c.Tipo, BairroID: c.BairroID, QuarteiraoID: c.QuarteiraoID}, "")
	if err != nil {
		return views.LoteFormData{}, err
	}
	return views.LoteFormData{
		Campos: c, Bairros: base.Bairros, Quarteiroes: base.Quarteiroes, Tipos: base.Tipos, Erro: erro, CSRF: csrfToken(r.Context()),
	}, nil
}

func (h *handlers) loteForm(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	form, err := h.montarFormLote(r, q, views.LoteCampos{Tipo: "residencial", Passo: "1"}, "")
	if err != nil {
		h.falhaInterna(w, r, "montar formulário", err)
		return
	}
	render(w, r, views.LoteForm(h.layout(r, "Cadastro em lote", "/locais"), form))
}

func (h *handlers) loteSalvar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	v := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	c := views.LoteCampos{
		Tipo: v("tipo"), BairroID: v("bairro_id"), QuarteiraoID: v("quarteirao_id"), Endereco: v("endereco"), Lado: v("lado"),
		De: v("numero_de"), Ate: v("numero_ate"), Passo: v("passo"), SequenciaInicial: v("sequencia_inicial"),
	}
	falha := func(msg string) {
		form, e := h.montarFormLote(r, q, c, msg)
		if e != nil {
			h.falhaInterna(w, r, "montar formulário", e)
			return
		}
		renderStatus(w, r, http.StatusBadRequest, views.LoteForm(h.layout(r, "Cadastro em lote", "/locais"), form))
	}

	if !slices.ContainsFunc(tiposDeLocal, func(t [2]string) bool { return t[0] == c.Tipo }) {
		falha("Escolha o tipo dos locais.")
		return
	}
	if _, soPE := h.escopoLocais(r); soPE && c.Tipo != "ponto_estrategico" {
		falha("O seu papel só cadastra pontos estratégicos.")
		return
	}
	if n := len([]rune(c.Endereco)); n < 2 || n > 200 {
		falha("Informe o endereço (rua ou avenida), de 2 a 200 caracteres.")
		return
	}
	if n := len([]rune(c.Lado)); n < 1 || n > 5 {
		falha("Informe o lado da quadra (até 5 caracteres, como Par, Ímpar, A).")
		return
	}
	de, e1 := strconv.Atoi(c.De)
	ate, e2 := strconv.Atoi(c.Ate)
	passo, e3 := strconv.Atoi(c.Passo)
	if e1 != nil || e2 != nil || de < 0 || ate < 0 || de > 99999 || ate > 99999 {
		falha("Informe o primeiro e o último número do trecho (de 0 a 99999).")
		return
	}
	if e3 != nil || (passo != 1 && passo != 2) {
		falha("Escolha o intervalo entre os números: de 1 em 1 ou de 2 em 2 (só pares ou só ímpares).")
		return
	}
	if ate < de {
		falha("O último número não pode ser menor que o primeiro.")
		return
	}
	total := (ate-de)/passo + 1
	if total > maximoDoLote {
		falha(fmt.Sprintf("Muitos locais de uma vez: o limite é %d por lote (este traria %d).", maximoDoLote, total))
		return
	}
	var seq0 pgtype.Int4
	if c.SequenciaInicial != "" {
		var msg string
		if seq0, msg = inteiroOpcional(c.SequenciaInicial, "Sequência inicial", 99999-maximoDoLote); msg != "" {
			falha(msg)
			return
		}
	}

	bairroID, _ := strconv.ParseInt(c.BairroID, 10, 64)
	quarteiraoID, _ := strconv.ParseInt(c.QuarteiraoID, 10, 64)
	b, err := q.GetBairro(ctx, bairroID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && b.Status != "ativo") {
		falha("Escolha um bairro ativo.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar bairro", err)
		return
	}
	if ok, err := h.bairroVisivelParaLocal(ctx, r, q, b.ID); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return
	} else if !ok {
		falha("Escolha um bairro em que você é responsável.")
		return
	}
	qt, err := q.GetQuarteirao(ctx, quarteiraoID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (qt.BairroID != b.ID || !qt.Ativo)) {
		falha("Escolha um quarteirão ativo do bairro selecionado.")
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

	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qx := sqlccliente.New(tx)
	criados := 0
	for i := 0; i < total; i++ {
		seq := pgtype.Int4{}
		if seq0.Valid {
			seq = pgtype.Int4{Int32: seq0.Int32 + int32(i), Valid: true}
		}
		n, err := qx.CreateLocalLote(ctx, sqlccliente.CreateLocalLoteParams{
			Tipo: c.Tipo, Endereco: c.Endereco, Numero: strconv.Itoa(de + i*passo), BairroID: b.ID, QuarteiraoID: qt.ID, Lado: c.Lado,
			Sequencia: seq, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
		})
		if err != nil {
			h.falhaInterna(w, r, "criar local do lote", err)
			return
		}
		criados += int(n)
	}
	if criados > 0 {
		h.auditarCom(audit.Cliente{Q: qx}, r, audit.Evento{
			Acao: "local.lote", Entidade: "bairro", EntidadeID: strconv.FormatInt(b.ID, 10), Rotulo: b.Nome,
			Detalhes: map[string]any{
				"criados": criados, "ja_existiam": total - criados, "tipo": rotuloTipoLocal(c.Tipo), "endereco": c.Endereco,
				"de": de, "ate": ate, "passo": passo, "quarteirao": qt.Nome, "lado": c.Lado,
			},
		})
	} else {
		naoAuditar(r)
	}
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar lote", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/locais?bairro=%d&lote=%d-%d", b.ID, criados, total-criados), http.StatusSeeOther)
}

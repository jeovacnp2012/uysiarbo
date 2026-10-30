package server

import (
	"context"
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

// Auto de infração (9.4): nasce de uma notificação que não foi resolvida no prazo; depois é pago ou cancelado.
// Só existe um auto vivo (aberto ou pago) por notificação; cancelar o auto reabre a notificação.

var rotulosSituacaoAuto = []views.CidadeOpcao{
	{Valor: "aberto", Rotulo: "Aberto"},
	{Valor: "pago", Rotulo: "Pago"},
	{Valor: "cancelado", Rotulo: "Cancelado"},
}

var opcoesVencidos = []views.CidadeOpcao{{Valor: "sim", Rotulo: "Só os vencidos"}}

func init() {
	registrarTipoDeCaso(tipoDeCaso{
		Slug: "infracao", Rotulo: "Auto de infração",
		Ver: "notificacao_visualizar", Editar: "notificacao_criarauto",
		Existe: func(ctx context.Context, q *sqlccliente.Queries, id int64) bool {
			_, err := q.GetAuto(ctx, id)
			return err == nil
		},
		Caminho: func(id int64) string { return "/infracoes/" + strconv.FormatInt(id, 10) },
	})
}

func rotuloAuto(a sqlccliente.AutosInfracao) string { return "Auto de infração " + a.Numero }

func ordemDeAutos(ordem, dir string) (string, string) {
	switch ordem {
	case "numero", "data", "vencimento", "valor", "endereco", "situacao":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func (h *handlers) infracoesLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.AutosFiltro{
		Busca: strings.TrimSpace(qs.Get("busca")), Situacao: qs.Get("situacao"), Vencidos: qs.Get("vencidos"), Bairro: qs.Get("bairro"),
		De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate")),
	}
	if !validoEm(rotulosSituacaoAuto, f.Situacao) {
		f.Situacao = ""
	}
	if f.Vencidos != "sim" {
		f.Vencidos = ""
	}
	ordem, dir := ordemDeAutos(qs.Get("ordem"), qs.Get("dir"))
	somente, _ := h.escopoLocais(r)
	hoje := h.hojeNoFuso(r)
	arg := sqlccliente.ListAutosParams{
		Busca: f.Busca, Situacao: f.Situacao, Vencidos: f.Vencidos, Hoje: hoje, BairroID: idOpcional(f.Bairro),
		DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate), SomenteUsuario: somente.Valid, UsuarioID: somente.Int64,
		Ordem: ordem, Dir: dir,
	}
	pag := paginaDe(r)
	total, err := q.CountAutos(ctx, sqlccliente.CountAutosParams{
		Busca: arg.Busca, Situacao: arg.Situacao, Vencidos: arg.Vencidos, Hoje: hoje, BairroID: arg.BairroID, DataDe: arg.DataDe,
		DataAte: arg.DataAte, SomenteUsuario: arg.SomenteUsuario, UsuarioID: arg.UsuarioID,
	})
	if err != nil {
		h.falhaInterna(w, r, "contar autos", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListAutos(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar autos", err)
		return
	}
	d := views.AutosData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag}
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.AutoLinha{
			ID: x.ID, Numero: x.Numero, NotificacaoID: x.NotificacaoID, NotificacaoNumero: x.NotificacaoNumero, Endereco: x.Endereco, Bairro: x.Bairro,
			Valor: reais(x.ValorCentavos), Data: dataBR(x.DataAuto), Vencimento: dataBR(x.Vencimento),
			Vencido:  x.Situacao == "aberto" && x.Vencimento.Valid && x.Vencimento.Time.Before(hoje.Time),
			Situacao: x.Situacao, SituacaoRotulo: rotuloDe(rotulosSituacaoAuto, x.Situacao),
		})
	}
	if d.Bairros, err = h.opcoesDeBairros(r, q, arg.BairroID.Int64); err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	d.Situacoes, d.Vencidos = marcarOpcao(rotulosSituacaoAuto, f.Situacao), marcarOpcao(opcoesVencidos, f.Vencidos)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "situacao": f.Situacao, "vencidos": f.Vencidos, "bairro": f.Bairro, "de": f.De, "ate": f.Ate} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Autos(h.layout(r, "Autos de infração", "/infracoes"), d))
}

func (h *handlers) formularioDeAuto(r *http.Request, q *sqlccliente.Queries, d views.AutoFormData) (views.AutoFormData, error) {
	d.CSRF = csrfToken(r.Context())
	t, _ := strconv.ParseInt(d.Trecho, 10, 64)
	var err error
	d.Trechos, err = h.opcoesDeTrechos(r.Context(), q, t)
	return d, err
}

// lerDadosDoAuto lê e valida o formulário do auto; devolve a mensagem de erro, se houver.
func (h *handlers) lerDadosDoAuto(r *http.Request, q *sqlccliente.Queries, d views.AutoFormData) (views.AutoFormData, sqlccliente.CreateAutoParams, string) {
	t := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	d.Descricao, d.Valor, d.Data, d.Vencimento, d.Trecho = t("descricao"), t("valor"), t("data"), t("vencimento"), t("trecho")
	centavos, okValor := valorEmCentavos(d.Valor)
	p := sqlccliente.CreateAutoParams{
		Descricao: d.Descricao, ValorCentavos: centavos, DataAuto: dataOpcional(d.Data), Vencimento: dataOpcional(d.Vencimento), LeiTrechoID: idOpcional(d.Trecho),
	}
	msg := ""
	switch {
	case d.Descricao == "" || len([]rune(d.Descricao)) > 4000:
		msg = "Descreva a infração (até 4000 caracteres)."
	case !okValor:
		msg = "Valor inválido. Use o formato 1.234,56."
	case !p.DataAuto.Valid:
		msg = "Informe a data do auto."
	case d.Vencimento != "" && !p.Vencimento.Valid:
		msg = "Vencimento inválido."
	case p.Vencimento.Valid && p.Vencimento.Time.Before(p.DataAuto.Time):
		msg = "O vencimento não pode ser anterior à data do auto."
	case d.Trecho != "" && !p.LeiTrechoID.Valid:
		msg = "Enquadramento inválido."
	}
	if msg == "" && p.LeiTrechoID.Valid && h.rotuloDoTrecho(r.Context(), q, p.LeiTrechoID) == "" {
		msg = "O enquadramento escolhido não existe."
	}
	d, _ = h.formularioDeAuto(r, q, d)
	return d, p, msg
}

// notificacaoGerarAuto cria o auto de infração a partir da notificação aberta.
func (h *handlers) notificacaoGerarAuto(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarNotificacao(w, r, q)
	if !ok {
		return
	}
	if n.Situacao != "aberta" {
		voltarAoCaso(w, r, tiposDeCaso["notificacao"], n.ID, "Só uma notificação aberta gera auto de infração.")
		return
	}
	_, p, msg := h.lerDadosDoAuto(r, q, views.AutoFormData{NotificacaoID: n.ID})
	if msg != "" {
		voltarAoCaso(w, r, tiposDeCaso["notificacao"], n.ID, "Auto de infração: "+msg)
		return
	}
	if p.Numero, err = h.numeroDoAno(r, q, "infracao"); err != nil {
		h.falhaInterna(w, r, "numerar auto", err)
		return
	}
	p.NotificacaoID, p.LocalID = n.ID, n.LocalID
	p.CriadoPor = pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}
	a, err := q.CreateAuto(r.Context(), p)
	if err != nil {
		if violacaoUnica(err) {
			voltarAoCaso(w, r, tiposDeCaso["notificacao"], n.ID, "Esta notificação já tem um auto de infração.")
			return
		}
		h.falhaInterna(w, r, "criar auto", err)
		return
	}
	if _, err := q.SetNotificacaoSituacao(r.Context(), sqlccliente.SetNotificacaoSituacaoParams{ID: n.ID, Situacao: "autuada"}); err != nil {
		h.falhaInterna(w, r, "marcar notificação como autuada", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "notificacao", n.ID, "situacao", "Auto de infração "+a.Numero, "Aberta", "Autuada", 0)
	_ = h.casoEvento(r.Context(), q, r, "infracao", a.ID, "criacao", "Gerado a partir da notificação "+n.Numero, "", "Aberto", 0)
	h.auditar(r, audit.Evento{Acao: "infracao.criar", Entidade: "infracao", EntidadeID: strconv.FormatInt(a.ID, 10), Rotulo: rotuloAuto(a),
		Depois: map[string]any{"notificacao": n.ID, "valor_centavos": a.ValorCentavos, "vencimento": dataISO(a.Vencimento)}})
	http.Redirect(w, r, "/infracoes/"+strconv.FormatInt(a.ID, 10)+"?aviso=criado", http.StatusSeeOther)
}

func (h *handlers) carregarAuto(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.AutosInfracao, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.AutosInfracao{}, false
	}
	a, err := q.GetAuto(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return a, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar auto", err)
		return a, false
	}
	if l, err := q.GetLocal(r.Context(), a.LocalID); err == nil {
		if ok, err := h.localVisivel(r.Context(), r, q, l); err != nil {
			h.falhaInterna(w, r, "checar acesso ao local", err)
			return a, false
		} else if !ok {
			http.NotFound(w, r)
			return a, false
		}
	}
	return a, true
}

func avisoDoAuto(c string) string {
	switch c {
	case "criado":
		return "Auto de infração gerado a partir da notificação."
	case "dados":
		return "Dados salvos."
	case "situacao":
		return "Situação atualizada."
	}
	return ""
}

func (h *handlers) infracaoFicha(w http.ResponseWriter, r *http.Request) {
	h.mostrarAuto(w, r, nil)
}

func (h *handlers) mostrarAuto(w http.ResponseWriter, r *http.Request, dadosForm *views.AutoFormData) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	a, ok := h.carregarAuto(w, r, q)
	if !ok {
		return
	}
	pr := h.principal(r)
	hoje := h.hojeNoFuso(r)
	d := views.AutoFichaData{
		ID: a.ID, Numero: a.Numero, NotificacaoID: a.NotificacaoID, LocalID: a.LocalID, Descricao: a.Descricao,
		Trecho: h.rotuloDoTrecho(ctx, q, a.LeiTrechoID), Valor: reais(a.ValorCentavos), Data: dataBR(a.DataAuto), Vencimento: dataBR(a.Vencimento),
		Vencido: a.Situacao == "aberto" && a.Vencimento.Valid && a.Vencimento.Time.Before(hoje.Time), PagoEm: dataBR(a.PagoEm),
		Situacao: a.Situacao, SituacaoRotulo: rotuloDe(rotulosSituacaoAuto, a.Situacao),
		PodeEditar: pr.Pode("notificacao_criarauto") && a.Situacao == "aberto", CSRF: csrfToken(ctx), Aviso: avisoDoAuto(r.URL.Query().Get("aviso")),
	}
	if n, err := q.GetNotificacaoQualquer(ctx, a.NotificacaoID); err == nil {
		d.NotificacaoNumero = n.Numero
	}
	if l, err := q.GetLocalQualquer(ctx, a.LocalID); err == nil {
		d.LocalEndereco = enderecoCompleto(l.Endereco, l.Numero)
	}
	if dadosForm != nil {
		d.Dados = *dadosForm
	} else if d.PodeEditar {
		f := views.AutoFormData{ID: a.ID, NotificacaoID: a.NotificacaoID, Descricao: a.Descricao, Valor: centavosEmTexto(a.ValorCentavos),
			Data: dataISO(a.DataAuto), Vencimento: dataISO(a.Vencimento)}
		if a.LeiTrechoID.Valid {
			f.Trecho = strconv.FormatInt(a.LeiTrechoID.Int64, 10)
		}
		if d.Dados, err = h.formularioDeAuto(r, q, f); err != nil {
			h.falhaInterna(w, r, "montar formulário do auto", err)
			return
		}
	}
	if d.Caso, err = h.fichaDoCaso(r, q, tiposDeCaso["infracao"], a.ID, h.acoesDoAuto(r, a)); err != nil {
		h.falhaInterna(w, r, "montar andamento do auto", err)
		return
	}
	render(w, r, views.AutoFicha(h.layout(r, "Auto "+a.Numero, "/infracoes"), d))
}

func (h *handlers) acoesDoAuto(r *http.Request, a sqlccliente.AutosInfracao) []views.CasoAcao {
	pr := h.principal(r)
	base := "/infracoes/" + strconv.FormatInt(a.ID, 10)
	var acoes []views.CasoAcao
	if a.Situacao == "cancelado" && pr.Pode("infracao_cancelar") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Excluir", URL: base + "/excluir", Icone: "trash", Perigo: true, Confirmar: "Enviar este auto cancelado para a lixeira?"})
	}
	return acoes
}

func (h *handlers) infracaoSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	antes, ok := h.carregarAuto(w, r, q)
	if !ok {
		return
	}
	if antes.Situacao != "aberto" {
		http.Error(w, "Só um auto aberto pode ser editado.", http.StatusConflict)
		return
	}
	d, p, msg := h.lerDadosDoAuto(r, q, views.AutoFormData{ID: antes.ID, NotificacaoID: antes.NotificacaoID})
	if msg != "" {
		d.Erro = msg
		w.WriteHeader(http.StatusBadRequest)
		h.mostrarAuto(w, r, &d)
		return
	}
	novo, err := q.UpdateAutoDados(r.Context(), sqlccliente.UpdateAutoDadosParams{
		ID: antes.ID, LeiTrechoID: p.LeiTrechoID, Descricao: p.Descricao, ValorCentavos: p.ValorCentavos, DataAuto: p.DataAuto, Vencimento: p.Vencimento,
	})
	if err != nil {
		h.falhaInterna(w, r, "salvar auto", err)
		return
	}
	ev := audit.Evento{Acao: "infracao.editar", Entidade: "infracao", EntidadeID: strconv.FormatInt(antes.ID, 10), Rotulo: rotuloAuto(novo)}
	ev.Antes, ev.Depois = audit.Diferenca(
		map[string]any{"descricao": antes.Descricao, "valor_centavos": antes.ValorCentavos, "vencimento": dataISO(antes.Vencimento), "data": dataISO(antes.DataAuto), "trecho": antes.LeiTrechoID.Int64},
		map[string]any{"descricao": novo.Descricao, "valor_centavos": novo.ValorCentavos, "vencimento": dataISO(novo.Vencimento), "data": dataISO(novo.DataAuto), "trecho": novo.LeiTrechoID.Int64})
	h.auditar(r, ev)
	http.Redirect(w, r, "/infracoes/"+strconv.FormatInt(antes.ID, 10)+"?aviso=dados", http.StatusSeeOther)
}

func (h *handlers) infracaoPagar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	a, ok := h.carregarAuto(w, r, q)
	if !ok {
		return
	}
	if a.Situacao != "aberto" {
		voltarAoCaso(w, r, tiposDeCaso["infracao"], a.ID, "Só um auto aberto pode ser marcado como pago.")
		return
	}
	if _, err := q.SetAutoSituacao(r.Context(), sqlccliente.SetAutoSituacaoParams{ID: a.ID, Situacao: "pago", PagoEm: h.hojeNoFuso(r)}); err != nil {
		h.falhaInterna(w, r, "marcar auto como pago", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "infracao", a.ID, "situacao", strings.TrimSpace(r.PostFormValue("texto")), "Aberto", "Pago", 0)
	h.auditar(r, audit.Evento{Acao: "infracao.pagar", Entidade: "infracao", EntidadeID: strconv.FormatInt(a.ID, 10), Rotulo: rotuloAuto(a),
		Antes: map[string]any{"situacao": a.Situacao}, Depois: map[string]any{"situacao": "pago"}})
	http.Redirect(w, r, "/infracoes/"+strconv.FormatInt(a.ID, 10)+"?aviso=situacao", http.StatusSeeOther)
}

// infracaoCancelar cancela o auto (aberto ou pago) com motivo e reabre a notificação de origem.
func (h *handlers) infracaoCancelar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	a, ok := h.carregarAuto(w, r, q)
	if !ok {
		return
	}
	texto := strings.TrimSpace(r.PostFormValue("texto"))
	switch {
	case a.Situacao == "cancelado":
		voltarAoCaso(w, r, tiposDeCaso["infracao"], a.ID, "Este auto já está cancelado.")
		return
	case texto == "":
		voltarAoCaso(w, r, tiposDeCaso["infracao"], a.ID, "Informe o motivo para cancelar o auto.")
		return
	}
	if len([]rune(texto)) > 2000 {
		texto = string([]rune(texto)[:2000])
	}
	if _, err := q.SetAutoSituacao(r.Context(), sqlccliente.SetAutoSituacaoParams{ID: a.ID, Situacao: "cancelado"}); err != nil {
		h.falhaInterna(w, r, "cancelar auto", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "infracao", a.ID, "situacao", texto, rotuloDe(rotulosSituacaoAuto, a.Situacao), "Cancelado", 0)
	if n, err := q.GetNotificacao(r.Context(), a.NotificacaoID); err == nil && n.Situacao == "autuada" {
		if _, err := q.SetNotificacaoSituacao(r.Context(), sqlccliente.SetNotificacaoSituacaoParams{ID: n.ID, Situacao: "aberta"}); err == nil {
			_ = h.casoEvento(r.Context(), q, r, "notificacao", n.ID, "situacao", "Auto "+a.Numero+" cancelado", "Autuada", "Aberta", 0)
		}
	}
	h.auditar(r, audit.Evento{Acao: "infracao.cancelar", Entidade: "infracao", EntidadeID: strconv.FormatInt(a.ID, 10), Rotulo: rotuloAuto(a),
		Antes: map[string]any{"situacao": a.Situacao}, Depois: map[string]any{"situacao": "cancelado"}})
	http.Redirect(w, r, "/infracoes/"+strconv.FormatInt(a.ID, 10)+"?aviso=situacao", http.StatusSeeOther)
}

func (h *handlers) infracaoExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	a, ok := h.carregarAuto(w, r, q)
	if !ok {
		return
	}
	if a.Situacao != "cancelado" {
		voltarAoCaso(w, r, tiposDeCaso["infracao"], a.ID, "Cancele o auto antes de excluí-lo.")
		return
	}
	h.excluirRegistro(w, r, "infracao", a.ID, "/infracoes?aviso=excluido", "/infracoes/"+strconv.FormatInt(a.ID, 10))
}

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Notificação (9.4): a vigilância notifica o responsável do imóvel para sanar um problema até um prazo. Se não for
// resolvida, gera um auto de infração (infracoes.go). O andamento usa a ficha comum de casos (9.1).

var rotulosSituacaoNotificacao = []views.CidadeOpcao{
	{Valor: "aberta", Rotulo: "Aberta"},
	{Valor: "resolvida", Rotulo: "Resolvida"},
	{Valor: "autuada", Rotulo: "Autuada"},
	{Valor: "cancelada", Rotulo: "Cancelada"},
}

var opcoesVencidas = []views.CidadeOpcao{{Valor: "sim", Rotulo: "Só as vencidas"}}

func init() {
	registrarTipoDeCaso(tipoDeCaso{
		Slug: "notificacao", Rotulo: "Notificação",
		Ver: "notificacao_visualizar", Editar: "notificacao_adicionar",
		Existe: func(ctx context.Context, q *sqlccliente.Queries, id int64) bool {
			_, err := q.GetNotificacao(ctx, id)
			return err == nil
		},
		Caminho: func(id int64) string { return "/notificacoes/" + strconv.FormatInt(id, 10) },
	})
}

// numeroDoAno gera o próximo número (NT-AAAA0000001, INF-AAAA00001). O ano segue o fuso da empresa.
func (h *handlers) numeroDoAno(r *http.Request, q *sqlccliente.Queries, tipo string) (string, error) {
	ano := time.Now().In(h.fuso(r)).Year()
	n, err := q.ProximoNumero(r.Context(), sqlccliente.ProximoNumeroParams{Tipo: tipo, Ano: int32(ano)})
	if err != nil {
		return "", err
	}
	if tipo == "notificacao" {
		return fmt.Sprintf("NT-%04d%07d", ano, n), nil
	}
	return fmt.Sprintf("INF-%04d%05d", ano, n), nil
}

// hojeNoFuso é a data de hoje no fuso da empresa, como data do banco.
func (h *handlers) hojeNoFuso(r *http.Request) pgtype.Date {
	n := time.Now().In(h.fuso(r))
	return pgtype.Date{Time: time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

// reais formata centavos como "R$ 1.234,56".
func reais(c int64) string {
	inteiro := strconv.FormatInt(c/100, 10)
	var partes []string
	for len(inteiro) > 3 {
		partes = append([]string{inteiro[len(inteiro)-3:]}, partes...)
		inteiro = inteiro[:len(inteiro)-3]
	}
	partes = append([]string{inteiro}, partes...)
	return fmt.Sprintf("R$ %s,%02d", strings.Join(partes, "."), c%100)
}

// valorEmCentavos lê "1.234,56", "1234,5" ou "1234.56".
func valorEmCentavos(s string) (int64, bool) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "R$"))
	if s == "" {
		return 0, true
	}
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	}
	inteiro, frac, _ := strings.Cut(s, ".")
	if len(frac) > 2 || inteiro == "" && frac == "" {
		return 0, false
	}
	for len(frac) < 2 {
		frac += "0"
	}
	n, err := strconv.ParseInt(inteiro+frac, 10, 64)
	if err != nil || n < 0 || n > 100000000000 {
		return 0, false
	}
	return n, true
}

func centavosEmTexto(c int64) string {
	t := reais(c)
	return strings.TrimPrefix(t, "R$ ")
}

func (h *handlers) opcoesDeTrechos(ctx context.Context, q *sqlccliente.Queries, escolhido int64) ([]views.CidadeOpcao, error) {
	ts, err := q.ListTrechosParaEscolha(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]views.CidadeOpcao, 0, len(ts))
	for _, t := range ts {
		rot := "Lei " + t.LeiNumero + "/" + strconv.Itoa(int(t.LeiAno))
		if t.Artigo != "" {
			rot += " — " + t.Artigo
		}
		out = append(out, views.CidadeOpcao{ID: t.ID, Rotulo: rot, Selecionada: t.ID == escolhido})
	}
	return out, nil
}

func (h *handlers) rotuloDoTrecho(ctx context.Context, q *sqlccliente.Queries, id pgtype.Int8) string {
	if !id.Valid {
		return ""
	}
	ops, err := h.opcoesDeTrechos(ctx, q, id.Int64)
	if err != nil {
		return ""
	}
	for _, o := range ops {
		if o.ID == id.Int64 {
			return o.Rotulo
		}
	}
	return ""
}

func ordemDeNotificacoes(ordem, dir string) (string, string) {
	switch ordem {
	case "numero", "motivo", "data", "prazo", "endereco", "situacao":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func rotuloNotificacao(n sqlccliente.Notificaco) string { return "Notificação " + n.Numero }

func (h *handlers) notificacoesLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.NotificacoesFiltro{
		Busca: strings.TrimSpace(qs.Get("busca")), Situacao: qs.Get("situacao"), Vencidas: qs.Get("vencidas"), Bairro: qs.Get("bairro"),
		De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate")),
	}
	if !validoEm(rotulosSituacaoNotificacao, f.Situacao) {
		f.Situacao = ""
	}
	if f.Vencidas != "sim" {
		f.Vencidas = ""
	}
	ordem, dir := ordemDeNotificacoes(qs.Get("ordem"), qs.Get("dir"))
	somente, _ := h.escopoLocais(r)
	hoje := h.hojeNoFuso(r)
	arg := sqlccliente.ListNotificacoesParams{
		Busca: f.Busca, Situacao: f.Situacao, Vencidas: f.Vencidas, Hoje: hoje, BairroID: idOpcional(f.Bairro),
		DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate), SomenteUsuario: somente.Valid, UsuarioID: somente.Int64,
		Ordem: ordem, Dir: dir,
	}
	pag := paginaDe(r)
	total, err := q.CountNotificacoes(ctx, sqlccliente.CountNotificacoesParams{
		Busca: arg.Busca, Situacao: arg.Situacao, Vencidas: arg.Vencidas, Hoje: hoje, BairroID: arg.BairroID, DataDe: arg.DataDe,
		DataAte: arg.DataAte, SomenteUsuario: arg.SomenteUsuario, UsuarioID: arg.UsuarioID,
	})
	if err != nil {
		h.falhaInterna(w, r, "contar notificações", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListNotificacoes(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar notificações", err)
		return
	}
	d := views.NotificacoesData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: h.principal(r).Pode("notificacao_adicionar")}
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.NotificacaoLinha{
			ID: x.ID, Numero: x.Numero, Motivo: x.Motivo, Data: dataBR(x.DataNotificacao), Prazo: dataBR(x.Prazo),
			Vencida: x.Situacao == "aberta" && x.Prazo.Valid && x.Prazo.Time.Before(hoje.Time), Endereco: x.Endereco, Bairro: x.Bairro,
			Situacao: x.Situacao, SituacaoRotulo: rotuloDe(rotulosSituacaoNotificacao, x.Situacao), Auto: x.AutoNumero,
		})
	}
	if d.Bairros, err = h.opcoesDeBairros(r, q, arg.BairroID.Int64); err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	d.Situacoes, d.Vencidas = marcarOpcao(rotulosSituacaoNotificacao, f.Situacao), marcarOpcao(opcoesVencidas, f.Vencidas)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "situacao": f.Situacao, "vencidas": f.Vencidas, "bairro": f.Bairro, "de": f.De, "ate": f.Ate} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Notificacoes(h.layout(r, "Notificações", "/notificacoes"), d))
}

// notificacaoNova mostra a busca do local (sem ?local=) ou o formulário (com ?local=).
func (h *handlers) notificacaoNova(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if id := idOpcional(r.URL.Query().Get("local")); id.Valid {
		l, ok := h.localDaNotificacao(w, r, q, id.Int64)
		if !ok {
			return
		}
		f, err := h.formularioDeNotificacao(r, q, views.NotificacaoFormData{
			LocalID: l.ID, LocalEndereco: enderecoCompleto(l.Endereco, l.Numero), Data: h.hojeNoFuso(r).Time.Format("2006-01-02"),
		})
		if err != nil {
			h.falhaInterna(w, r, "montar formulário", err)
			return
		}
		render(w, r, views.NotificacaoNova(h.layout(r, "Nova notificação", "/notificacoes"), f))
		return
	}
	e := views.NotificacaoEscolherLocal{Busca: strings.TrimSpace(r.URL.Query().Get("busca_local"))}
	if e.Busca != "" {
		e.Buscou = true
		achados, err := q.BuscarLocaisParaVinculo(ctx, e.Busca)
		if err != nil {
			h.falhaInterna(w, r, "buscar locais", err)
			return
		}
		for _, l := range achados {
			e.Locais = append(e.Locais, views.LocalVinculo{ID: l.ID, Endereco: enderecoCompleto(l.Endereco, l.Numero), Bairro: l.BairroNome})
		}
	}
	render(w, r, views.NotificacaoEscolher(h.layout(r, "Nova notificação", "/notificacoes"), e))
}

func (h *handlers) localDaNotificacao(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, id int64) (sqlccliente.Locai, bool) {
	l, err := q.GetLocal(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return l, false
	}
	if ok, err := h.localVisivel(r.Context(), r, q, l); err != nil {
		h.falhaInterna(w, r, "checar acesso ao local", err)
		return l, false
	} else if !ok {
		http.NotFound(w, r)
		return l, false
	}
	return l, true
}

func (h *handlers) formularioDeNotificacao(r *http.Request, q *sqlccliente.Queries, d views.NotificacaoFormData) (views.NotificacaoFormData, error) {
	d.CSRF = csrfToken(r.Context())
	t, _ := strconv.ParseInt(d.Trecho, 10, 64)
	var err error
	d.Trechos, err = h.opcoesDeTrechos(r.Context(), q, t)
	return d, err
}

// lerDadosDaNotificacao lê e valida o formulário; devolve a mensagem de erro, se houver.
func (h *handlers) lerDadosDaNotificacao(r *http.Request, q *sqlccliente.Queries, d views.NotificacaoFormData) (views.NotificacaoFormData, sqlccliente.CreateNotificacaoParams, string) {
	t := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	d.Motivo, d.Descricao, d.Data, d.Prazo, d.Trecho, d.Visita = t("motivo"), t("descricao"), t("data"), t("prazo"), t("trecho"), t("visita")
	p := sqlccliente.CreateNotificacaoParams{
		LocalID: d.LocalID, Motivo: d.Motivo, Descricao: d.Descricao, DataNotificacao: dataOpcional(d.Data), Prazo: dataOpcional(d.Prazo),
		LeiTrechoID: idOpcional(d.Trecho), VisitaID: idOpcional(d.Visita),
	}
	msg := ""
	switch {
	case d.Motivo == "" || len([]rune(d.Motivo)) > 150:
		msg = "Informe o motivo (até 150 caracteres)."
	case len([]rune(d.Descricao)) > 4000:
		msg = "A descrição passa de 4000 caracteres."
	case !p.DataNotificacao.Valid:
		msg = "Informe a data da notificação."
	case d.Prazo != "" && !p.Prazo.Valid:
		msg = "Prazo inválido."
	case p.Prazo.Valid && p.Prazo.Time.Before(p.DataNotificacao.Time):
		msg = "O prazo não pode ser anterior à data da notificação."
	case d.Trecho != "" && !p.LeiTrechoID.Valid:
		msg = "Enquadramento inválido."
	case d.Visita != "" && !p.VisitaID.Valid:
		msg = "Número de visita inválido."
	}
	if msg == "" && p.LeiTrechoID.Valid {
		if h.rotuloDoTrecho(r.Context(), q, p.LeiTrechoID) == "" {
			msg = "O enquadramento escolhido não existe."
		}
	}
	if msg == "" && p.VisitaID.Valid {
		v, err := q.GetVisita(r.Context(), p.VisitaID.Int64)
		if err != nil || v.LocalID != d.LocalID {
			msg = "A visita não existe ou não é deste local."
		} else if ok, err := h.visitaVisivel(r.Context(), r, q, v); err != nil || !ok {
			msg = "A visita não existe ou não é deste local."
		}
	}
	d, _ = h.formularioDeNotificacao(r, q, d)
	return d, p, msg
}

func (h *handlers) notificacaoCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	localID, _ := strconv.ParseInt(r.PostFormValue("local"), 10, 64)
	l, ok := h.localDaNotificacao(w, r, q, localID)
	if !ok {
		return
	}
	d, p, msg := h.lerDadosDaNotificacao(r, q, views.NotificacaoFormData{LocalID: l.ID, LocalEndereco: enderecoCompleto(l.Endereco, l.Numero)})
	if msg != "" {
		d.Erro = msg
		renderStatus(w, r, http.StatusBadRequest, views.NotificacaoNova(h.layout(r, "Nova notificação", "/notificacoes"), d))
		return
	}
	if p.Numero, err = h.numeroDoAno(r, q, "notificacao"); err != nil {
		h.falhaInterna(w, r, "numerar notificação", err)
		return
	}
	p.CriadoPor = pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}
	n, err := q.CreateNotificacao(r.Context(), p)
	if err != nil {
		h.falhaInterna(w, r, "criar notificação", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "notificacao", n.ID, "criacao", "", "", "Aberta", 0)
	h.auditar(r, audit.Evento{Acao: "notificacao.criar", Entidade: "notificacao", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloNotificacao(n),
		Depois: map[string]any{"local": n.LocalID, "motivo": n.Motivo, "prazo": dataISO(n.Prazo)}})
	http.Redirect(w, r, "/notificacoes/"+strconv.FormatInt(n.ID, 10), http.StatusSeeOther)
}

func (h *handlers) carregarNotificacao(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.Notificaco, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.Notificaco{}, false
	}
	n, err := q.GetNotificacao(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return n, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar notificação", err)
		return n, false
	}
	if ok, err := h.notificacaoVisivel(r, q, n); err != nil {
		h.falhaInterna(w, r, "checar acesso à notificação", err)
		return n, false
	} else if !ok {
		http.NotFound(w, r)
		return n, false
	}
	return n, true
}

// notificacaoVisivel: quem só vê os próprios bairros só enxerga notificações de locais ao seu alcance.
func (h *handlers) notificacaoVisivel(r *http.Request, q *sqlccliente.Queries, n sqlccliente.Notificaco) (bool, error) {
	l, err := q.GetLocal(r.Context(), n.LocalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil // local na lixeira: a notificação continua acessível a quem tem a permissão
	} else if err != nil {
		return false, err
	}
	return h.localVisivel(r.Context(), r, q, l)
}

func (h *handlers) notificacaoFicha(w http.ResponseWriter, r *http.Request) {
	h.mostrarNotificacao(w, r, nil)
}

func avisoDaNotificacao(c string) string {
	switch c {
	case "dados":
		return "Dados salvos."
	case "situacao":
		return "Situação atualizada."
	case "auto":
		return "Auto de infração gerado."
	}
	return ""
}

func (h *handlers) mostrarNotificacao(w http.ResponseWriter, r *http.Request, dadosForm *views.NotificacaoFormData) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarNotificacao(w, r, q)
	if !ok {
		return
	}
	pr := h.principal(r)
	hoje := h.hojeNoFuso(r)
	aberta := n.Situacao == "aberta"
	d := views.NotificacaoFichaData{
		ID: n.ID, Numero: n.Numero, Motivo: n.Motivo, Descricao: n.Descricao, Data: dataBR(n.DataNotificacao), Prazo: dataBR(n.Prazo),
		Vencida: aberta && n.Prazo.Valid && n.Prazo.Time.Before(hoje.Time), Situacao: n.Situacao,
		SituacaoRotulo: rotuloDe(rotulosSituacaoNotificacao, n.Situacao), Fechada: n.Situacao == "resolvida" || n.Situacao == "cancelada",
		LocalID: n.LocalID, VisitaID: n.VisitaID.Int64, Trecho: h.rotuloDoTrecho(ctx, q, n.LeiTrechoID),
		PodeEditar: pr.Pode("notificacao_adicionar") && aberta, PodeAuto: pr.Pode("notificacao_criarauto") && aberta,
		CSRF: csrfToken(ctx), Aviso: avisoDaNotificacao(r.URL.Query().Get("aviso")),
	}
	if l, err := q.GetLocalQualquer(ctx, n.LocalID); err == nil {
		d.LocalEndereco = enderecoCompleto(l.Endereco, l.Numero)
		if b, err := q.GetBairro(ctx, l.BairroID); err == nil {
			d.Bairro = b.Nome
		}
	}
	autos, err := q.ListAutosDaNotificacao(ctx, n.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar autos", err)
		return
	}
	for _, a := range autos {
		d.Autos = append(d.Autos, views.AutoResumo{ID: a.ID, Numero: a.Numero, Situacao: a.Situacao, Rotulo: rotuloDe(rotulosSituacaoAuto, a.Situacao),
			Valor: reais(a.ValorCentavos), Data: dataBR(a.DataAuto)})
	}
	if dadosForm != nil {
		d.Dados = *dadosForm
	} else if d.PodeEditar {
		f := views.NotificacaoFormData{
			ID: n.ID, LocalID: n.LocalID, LocalEndereco: d.LocalEndereco, Motivo: n.Motivo, Descricao: n.Descricao, Data: dataISO(n.DataNotificacao),
			Prazo: dataISO(n.Prazo),
		}
		if n.LeiTrechoID.Valid {
			f.Trecho = strconv.FormatInt(n.LeiTrechoID.Int64, 10)
		}
		if n.VisitaID.Valid {
			f.Visita = strconv.FormatInt(n.VisitaID.Int64, 10)
		}
		if d.Dados, err = h.formularioDeNotificacao(r, q, f); err != nil {
			h.falhaInterna(w, r, "montar formulário", err)
			return
		}
	}
	if d.PodeAuto {
		f := views.AutoFormData{NotificacaoID: n.ID, Data: hoje.Time.Format("2006-01-02"), Descricao: n.Motivo}
		if n.LeiTrechoID.Valid {
			f.Trecho = strconv.FormatInt(n.LeiTrechoID.Int64, 10)
		}
		if d.AutoForm, err = h.formularioDeAuto(r, q, f); err != nil {
			h.falhaInterna(w, r, "montar formulário do auto", err)
			return
		}
	}
	if d.Caso, err = h.fichaDoCaso(r, q, tiposDeCaso["notificacao"], n.ID, h.acoesDaNotificacao(r, n)); err != nil {
		h.falhaInterna(w, r, "montar andamento da notificação", err)
		return
	}
	render(w, r, views.NotificacaoFicha(h.layout(r, "Notificação "+n.Numero, "/notificacoes"), d))
}

func (h *handlers) acoesDaNotificacao(r *http.Request, n sqlccliente.Notificaco) []views.CasoAcao {
	pr := h.principal(r)
	base := "/notificacoes/" + strconv.FormatInt(n.ID, 10)
	var acoes []views.CasoAcao
	switch n.Situacao {
	case "resolvida", "cancelada":
		if pr.Pode("notificacao_resolver") {
			acoes = append(acoes, views.CasoAcao{Rotulo: "Reabrir", URL: base + "/reabrir", Icone: "undo", Confirmar: "Reabrir esta notificação?"})
		}
	}
	if pr.Pode("notificacao_cancelar") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Excluir", URL: base + "/excluir", Icone: "trash", Perigo: true, Confirmar: "Enviar esta notificação para a lixeira?"})
	}
	return acoes
}

func (h *handlers) notificacaoSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	antes, ok := h.carregarNotificacao(w, r, q)
	if !ok {
		return
	}
	if antes.Situacao != "aberta" {
		http.Error(w, "Só uma notificação aberta pode ser editada.", http.StatusConflict)
		return
	}
	d, p, msg := h.lerDadosDaNotificacao(r, q, views.NotificacaoFormData{ID: antes.ID, LocalID: antes.LocalID})
	if msg != "" {
		d.Erro = msg
		w.WriteHeader(http.StatusBadRequest)
		h.mostrarNotificacao(w, r, &d)
		return
	}
	novo, err := q.UpdateNotificacaoDados(r.Context(), sqlccliente.UpdateNotificacaoDadosParams{
		ID: antes.ID, VisitaID: p.VisitaID, LeiTrechoID: p.LeiTrechoID, Motivo: p.Motivo, Descricao: p.Descricao,
		DataNotificacao: p.DataNotificacao, Prazo: p.Prazo,
	})
	if err != nil {
		h.falhaInterna(w, r, "salvar notificação", err)
		return
	}
	ev := audit.Evento{Acao: "notificacao.editar", Entidade: "notificacao", EntidadeID: strconv.FormatInt(antes.ID, 10), Rotulo: rotuloNotificacao(novo)}
	ev.Antes, ev.Depois = audit.Diferenca(
		map[string]any{"motivo": antes.Motivo, "descricao": antes.Descricao, "prazo": dataISO(antes.Prazo), "data": dataISO(antes.DataNotificacao), "trecho": antes.LeiTrechoID.Int64, "visita": antes.VisitaID.Int64},
		map[string]any{"motivo": novo.Motivo, "descricao": novo.Descricao, "prazo": dataISO(novo.Prazo), "data": dataISO(novo.DataNotificacao), "trecho": novo.LeiTrechoID.Int64, "visita": novo.VisitaID.Int64})
	h.auditar(r, ev)
	http.Redirect(w, r, "/notificacoes/"+strconv.FormatInt(antes.ID, 10)+"?aviso=dados", http.StatusSeeOther)
}

// mudarSituacaoDaNotificacao é o caminho comum das ações de andamento; regra devolve a situação nova ou o erro.
func (h *handlers) mudarSituacaoDaNotificacao(w http.ResponseWriter, r *http.Request, acao string, regra func(n sqlccliente.Notificaco) (string, string)) {
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
	sit, erro := regra(n)
	if erro != "" {
		voltarAoCaso(w, r, tiposDeCaso["notificacao"], n.ID, erro)
		return
	}
	if _, err := q.SetNotificacaoSituacao(r.Context(), sqlccliente.SetNotificacaoSituacaoParams{ID: n.ID, Situacao: sit}); err != nil {
		h.falhaInterna(w, r, "mudar situação da notificação", err)
		return
	}
	texto := strings.TrimSpace(r.PostFormValue("texto"))
	if len([]rune(texto)) > 2000 {
		texto = string([]rune(texto)[:2000])
	}
	_ = h.casoEvento(r.Context(), q, r, "notificacao", n.ID, "situacao", texto, rotuloDe(rotulosSituacaoNotificacao, n.Situacao), rotuloDe(rotulosSituacaoNotificacao, sit), 0)
	h.auditar(r, audit.Evento{Acao: "notificacao." + acao, Entidade: "notificacao", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloNotificacao(n),
		Antes: map[string]any{"situacao": n.Situacao}, Depois: map[string]any{"situacao": sit}})
	http.Redirect(w, r, "/notificacoes/"+strconv.FormatInt(n.ID, 10)+"?aviso=situacao", http.StatusSeeOther)
}

func (h *handlers) notificacaoResolver(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDaNotificacao(w, r, "resolver", func(n sqlccliente.Notificaco) (string, string) {
		if n.Situacao != "aberta" {
			return "", "Só uma notificação aberta pode ser resolvida."
		}
		return "resolvida", ""
	})
}

func (h *handlers) notificacaoCancelar(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDaNotificacao(w, r, "cancelar", func(n sqlccliente.Notificaco) (string, string) {
		if n.Situacao != "aberta" {
			return "", "Só uma notificação aberta pode ser cancelada."
		}
		if strings.TrimSpace(r.PostFormValue("texto")) == "" {
			return "", "Informe o motivo para cancelar a notificação."
		}
		return "cancelada", ""
	})
}

func (h *handlers) notificacaoReabrir(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDaNotificacao(w, r, "reabrir", func(n sqlccliente.Notificaco) (string, string) {
		if n.Situacao != "resolvida" && n.Situacao != "cancelada" {
			return "", "Só uma notificação resolvida ou cancelada pode ser reaberta."
		}
		return "aberta", ""
	})
}

func (h *handlers) notificacaoExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarNotificacao(w, r, q)
	if !ok {
		return
	}
	h.excluirRegistro(w, r, "notificacao", n.ID, "/notificacoes?aviso=excluido", "/notificacoes/"+strconv.FormatInt(n.ID, 10))
}

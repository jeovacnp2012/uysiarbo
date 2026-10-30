package server

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/comunicacao"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Comunicações (12.1): fila de mensagens de WhatsApp e e-mail com tentativas, reenvio e linha do tempo.
// O envio de verdade fica atrás de comunicacao.Provedor (12.2 liga os provedores reais).

var (
	rotulosCanalCom = []views.CidadeOpcao{
		{Valor: comunicacao.WhatsApp, Rotulo: "WhatsApp"},
		{Valor: comunicacao.Email, Rotulo: "E-mail"},
	}
	rotulosStatusCom = []views.CidadeOpcao{
		{Valor: "fila", Rotulo: "Na fila"},
		{Valor: "enviada", Rotulo: "Enviada"},
		{Valor: "entregue", Rotulo: "Entregue"},
		{Valor: "visualizada", Rotulo: "Visualizada"},
		{Valor: "falhou", Rotulo: "Falhou"},
		{Valor: "cancelada", Rotulo: "Cancelada"},
	}
	rotulosEventoCom = []views.CidadeOpcao{
		{Valor: "criada", Rotulo: "Criada"},
		{Valor: "tentativa", Rotulo: "Tentativa de envio"},
		{Valor: "enviada", Rotulo: "Enviada"},
		{Valor: "falha", Rotulo: "Falha"},
		{Valor: "entregue", Rotulo: "Entregue"},
		{Valor: "visualizada", Rotulo: "Visualizada"},
		{Valor: "reenvio", Rotulo: "Reenvio"},
		{Valor: "cancelada", Rotulo: "Cancelada"},
	}
)

func comunicacaoCaminho(id int64) string { return "/comunicacoes/" + strconv.FormatInt(id, 10) }

// normalizarDestinatario valida e padroniza o destino: telefone só com dígitos e DDI 55, ou e-mail.
func normalizarDestinatario(canal, bruto string) (string, string) {
	bruto = strings.TrimSpace(bruto)
	if canal == comunicacao.Email {
		a, err := mail.ParseAddress(bruto)
		if err != nil || a.Address != bruto || len(bruto) > 160 {
			return "", "Informe um e-mail válido."
		}
		return strings.ToLower(bruto), ""
	}
	var d strings.Builder
	for _, c := range bruto {
		if unicode.IsDigit(c) {
			d.WriteRune(c)
		}
	}
	n := d.String()
	if len(n) == 10 || len(n) == 11 {
		n = "55" + n
	}
	if len(n) < 12 || len(n) > 15 {
		return "", "Informe o telefone com DDD (e DDI, se não for do Brasil)."
	}
	return n, ""
}

// enfileirarComunicacao põe uma mensagem na fila (para outros módulos usarem: notificações, denúncias...).
func (h *handlers) enfileirarComunicacao(ctx context.Context, q *sqlccliente.Queries, p sqlccliente.CreateComunicacaoParams) (sqlccliente.Comunicaco, error) {
	c, err := q.CreateComunicacao(ctx, p)
	if err != nil {
		return c, err
	}
	err = q.CreateComunicacaoEvento(ctx, sqlccliente.CreateComunicacaoEventoParams{ComunicacaoID: c.ID, Tipo: "criada", Descricao: "Mensagem colocada na fila", UsuarioID: p.CriadoPor})
	return c, err
}

var errForaDaFila = errors.New("a mensagem não está na fila")

// tentarEnvio faz uma tentativa de envio da mensagem (trava a linha para não enviar em dobro).
// Devolve o status em que a mensagem ficou; comunicacao.ErrDesligado quando não há provedor.
func (h *handlers) tentarEnvio(ctx context.Context, pool *pgxpool.Pool, q *sqlccliente.Queries, id int64, por pgtype.Int8) (string, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	qt := q.WithTx(tx)
	c, err := qt.GetComunicacaoParaAtualizar(ctx, id)
	if err != nil {
		return "", err
	}
	if c.Status != "fila" {
		return c.Status, errForaDaFila
	}
	if !h.provedorCom.Atende(c.Canal) {
		return "fila", comunicacao.ErrDesligado
	}
	idExterno, err := h.provedorCom.Enviar(ctx, comunicacao.Mensagem{Canal: c.Canal, Destinatario: c.Destinatario, Nome: c.DestinatarioNome, Assunto: c.Assunto, Corpo: c.Corpo})
	if errors.Is(err, comunicacao.ErrDesligado) {
		return "fila", err
	}
	var status string
	if err != nil {
		texto := err.Error()
		if r := []rune(texto); len(r) > 400 {
			texto = string(r[:400])
		}
		status, err = qt.SetComunicacaoFalha(ctx, sqlccliente.SetComunicacaoFalhaParams{ID: id, Erro: texto, Provedor: h.provedorCom.Nome()})
		if err == nil {
			err = qt.CreateComunicacaoEvento(ctx, sqlccliente.CreateComunicacaoEventoParams{ComunicacaoID: id, Tipo: "falha", Descricao: "Tentativa " + strconv.Itoa(int(c.Tentativas)+1) + " falhou: " + texto, UsuarioID: por})
		}
		if err == nil && status == "falhou" {
			err = qt.CreateComunicacaoEvento(ctx, sqlccliente.CreateComunicacaoEventoParams{ComunicacaoID: id, Tipo: "falha", Descricao: "Tentativas esgotadas; use Reenviar para tentar de novo", UsuarioID: por})
		}
	} else {
		status = "enviada"
		err = qt.SetComunicacaoEnviada(ctx, sqlccliente.SetComunicacaoEnviadaParams{ID: id, Provedor: h.provedorCom.Nome(), IDProvedor: idExterno})
		if err == nil {
			err = qt.CreateComunicacaoEvento(ctx, sqlccliente.CreateComunicacaoEventoParams{ComunicacaoID: id, Tipo: "enviada", Descricao: "Entregue ao provedor (" + h.provedorCom.Nome() + ") na tentativa " + strconv.Itoa(int(c.Tentativas)+1), UsuarioID: por})
		}
	}
	if err != nil {
		return "", err
	}
	return status, tx.Commit(ctx)
}

func (h *handlers) comunicacoes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.ComunicacaoFiltro{Busca: strings.TrimSpace(qs.Get("busca")), Canal: qs.Get("canal"), Status: qs.Get("status")}
	if !validoEm(rotulosCanalCom, f.Canal) {
		f.Canal = ""
	}
	if !validoEm(rotulosStatusCom, f.Status) {
		f.Status = ""
	}
	ordem, dir := ordemDeEstoque(qs.Get("ordem"), qs.Get("dir"), "id")
	pag := paginaDe(r)
	total, err := q.CountComunicacoes(ctx, sqlccliente.CountComunicacoesParams{Busca: f.Busca, Canal: f.Canal, Status: f.Status})
	if err != nil {
		h.falhaInterna(w, r, "contar comunicações", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListComunicacoes(ctx, sqlccliente.ListComunicacoesParams{
		Busca: f.Busca, Canal: f.Canal, Status: f.Status, Ordem: ordem, Dir: dir,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar comunicações", err)
		return
	}
	emFila, err := q.CountComunicacoesEmFila(ctx)
	if err != nil {
		h.falhaInterna(w, r, "contar fila", err)
		return
	}
	pr := h.principal(r)
	d := views.ComunicacoesData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: pr.Pode("comunicacao_reenviar"), EmFila: int(emFila)}
	switch qs.Get("aviso") {
	case "processada":
		d.Aviso = "Fila processada."
	case "sem_provedor":
		d.Aviso = "Nenhum provedor de envio está configurado: as mensagens continuam na fila."
	}
	loc := h.fuso(r)
	for _, x := range rows {
		destino := x.Destinatario
		if x.DestinatarioNome != "" {
			destino = x.DestinatarioNome + " — " + x.Destinatario
		}
		resumo := x.Assunto
		if resumo == "" {
			resumo = x.Corpo
		}
		d.Linhas = append(d.Linhas, views.ComunicacaoLinha{
			IDTexto: strconv.FormatInt(x.ID, 10), Canal: rotuloDe(rotulosCanalCom, x.Canal), Destino: destino, Resumo: resumo,
			Status: rotuloDe(rotulosStatusCom, x.Status), Tentativas: strconv.Itoa(int(x.Tentativas)) + "/" + strconv.Itoa(int(x.MaxTentativas)),
			Criada: x.CriadoEm.Time.In(loc).Format("02/01/2006 15:04"),
		})
	}
	d.Canais, d.Statuses = marcarOpcao(rotulosCanalCom, f.Canal), marcarOpcao(rotulosStatusCom, f.Status)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "canal": f.Canal, "status": f.Status} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	d.CSRF = csrfToken(ctx)
	render(w, r, views.Comunicacoes(h.layout(r, "Comunicações", "/comunicacoes"), d))
}

func (h *handlers) comunicacaoNova(w http.ResponseWriter, r *http.Request) {
	d := views.ComunicacaoNova{Canal: comunicacao.WhatsApp, Canais: marcarOpcao(rotulosCanalCom, comunicacao.WhatsApp), CSRF: csrfToken(r.Context())}
	render(w, r, views.ComunicacaoNovaTela(h.layout(r, "Nova comunicação", "/comunicacoes"), d))
}

func (h *handlers) comunicacaoCriar(w http.ResponseWriter, r *http.Request) {
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
	d := views.ComunicacaoNova{
		Canal: r.PostFormValue("canal"), Destinatario: strings.TrimSpace(r.PostFormValue("destinatario")), Nome: strings.TrimSpace(r.PostFormValue("nome")),
		Assunto: strings.TrimSpace(r.PostFormValue("assunto")), Corpo: strings.TrimSpace(r.PostFormValue("corpo")), CSRF: csrfToken(ctx),
	}
	var destino string
	switch {
	case !validoEm(rotulosCanalCom, d.Canal):
		d.Erro = "Escolha o canal."
	case len([]rune(d.Nome)) > 150:
		d.Erro = "O nome aceita até 150 caracteres."
	case d.Canal == comunicacao.Email && (d.Assunto == "" || len([]rune(d.Assunto)) > 150):
		d.Erro = "Informe o assunto do e-mail (até 150 caracteres)."
	case d.Corpo == "" || len([]rune(d.Corpo)) > 2000:
		d.Erro = "Escreva a mensagem (até 2000 caracteres)."
	default:
		destino, d.Erro = normalizarDestinatario(d.Canal, d.Destinatario)
	}
	if d.Erro != "" {
		d.Canais = marcarOpcao(rotulosCanalCom, d.Canal)
		renderStatus(w, r, http.StatusBadRequest, views.ComunicacaoNovaTela(h.layout(r, "Nova comunicação", "/comunicacoes"), d))
		return
	}
	if d.Canal == comunicacao.WhatsApp {
		d.Assunto = ""
	}
	c, err := h.enfileirarComunicacao(ctx, q, sqlccliente.CreateComunicacaoParams{
		Canal: d.Canal, Destinatario: destino, DestinatarioNome: d.Nome, Assunto: d.Assunto, Corpo: d.Corpo, Origem: "manual", CriadoPor: idOuNulo(meuID(r)),
	})
	if err != nil {
		h.falhaInterna(w, r, "criar comunicação", err)
		return
	}
	// Dados pessoais (destino e texto) ficam fora da auditoria.
	h.auditar(r, audit.Evento{Acao: "comunicacao.criar", Entidade: "comunicacao", EntidadeID: strconv.FormatInt(c.ID, 10), Rotulo: "Comunicação " + strconv.FormatInt(c.ID, 10),
		Depois: map[string]any{"canal": c.Canal, "origem": c.Origem}})
	http.Redirect(w, r, comunicacaoCaminho(c.ID)+"?aviso=criada", http.StatusSeeOther)
}

func (h *handlers) carregarComunicacao(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.Comunicaco, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.Comunicaco{}, false
	}
	c, err := q.GetComunicacao(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return c, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar comunicação", err)
		return c, false
	}
	return c, true
}

func (h *handlers) renderComunicacao(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, c sqlccliente.Comunicaco, erro string) {
	ctx := r.Context()
	loc := h.fuso(r)
	quando := func(t pgtype.Timestamptz) string {
		if !t.Valid {
			return "—"
		}
		return t.Time.In(loc).Format("02/01/2006 15:04")
	}
	pode := h.principal(r).Pode("comunicacao_reenviar")
	d := views.ComunicacaoFicha{
		ID: c.ID, Canal: rotuloDe(rotulosCanalCom, c.Canal), Destinatario: c.Destinatario, Nome: c.DestinatarioNome, Assunto: c.Assunto, Corpo: c.Corpo,
		Status: c.Status, StatusRotulo: rotuloDe(rotulosStatusCom, c.Status), Tentativas: int(c.Tentativas), MaxTentativas: int(c.MaxTentativas),
		UltimoErro: c.UltimoErro, Provedor: c.Provedor, IDProvedor: c.IDProvedor, Origem: c.Origem,
		Criada: quando(pgtype.Timestamptz{Time: c.CriadoEm.Time, Valid: true}), Enviada: quando(c.EnviadaEm), Entregue: quando(c.EntregueEm), Visualizada: quando(c.VisualizadaEm),
		PodeEnviar: pode && c.Status == "fila", PodeCancelar: pode && (c.Status == "fila" || c.Status == "falhou"),
		PodeReenviar: pode && (c.Status == "falhou" || c.Status == "cancelada"), PodeStatus: pode && (c.Status == "enviada" || c.Status == "entregue"),
		PodeVisualizar: pode && (c.Status == "enviada" || c.Status == "entregue"), CSRF: csrfToken(ctx), Erro: erro,
	}
	if c.Status == "fila" {
		d.Proxima = quando(c.ProximaTentativa)
	}
	if c.CriadoPor.Valid {
		if u, err := q.GetUsuarioByID(ctx, c.CriadoPor.Int64); err == nil {
			d.CriadaPor = u.Nome
		}
	}
	switch r.URL.Query().Get("aviso") {
	case "criada":
		d.Aviso = "Mensagem colocada na fila."
	case "feito":
		d.Aviso = "Ação registrada."
	case "sem_provedor":
		d.Aviso = "Nenhum provedor de envio está configurado: a mensagem continua na fila."
	}
	evs, err := q.ListComunicacaoEventos(ctx, c.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar eventos da comunicação", err)
		return
	}
	for _, e := range evs {
		d.Eventos = append(d.Eventos, views.ComunicacaoEvento{Quando: e.CriadoEm.Time.In(loc).Format("02/01/2006 15:04:05"), Tipo: rotuloDe(rotulosEventoCom, e.Tipo), Descricao: e.Descricao, Por: e.Por})
	}
	status := http.StatusOK
	if erro != "" {
		status = http.StatusBadRequest
	}
	renderStatus(w, r, status, views.ComunicacaoFichaTela(h.layout(r, "Comunicação "+strconv.FormatInt(c.ID, 10), "/comunicacoes"), d))
}

func (h *handlers) comunicacaoFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if c, ok := h.carregarComunicacao(w, r, q); ok {
		h.renderComunicacao(w, r, q, c, "")
	}
}

// comunicacaoAcao é o molde das ações da ficha: enviar, reenviar, cancelar, entregue e visualizada.
func (h *handlers) comunicacaoAcao(acao string) http.HandlerFunc {
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
		c, ok := h.carregarComunicacao(w, r, q)
		if !ok {
			return
		}
		por := idOuNulo(meuID(r))
		var erro string
		switch acao {
		case "enviar":
			if c.Status != "fila" {
				erro = "Só uma mensagem na fila pode ser enviada."
			}
		case "reenviar":
			if c.Status != "falhou" && c.Status != "cancelada" {
				erro = "Só uma mensagem que falhou ou foi cancelada pode ser reenviada."
			}
		case "cancelar":
			if c.Status != "fila" && c.Status != "falhou" {
				erro = "Só uma mensagem na fila ou que falhou pode ser cancelada."
			}
		case "entregue", "visualizada":
			if c.Status != "enviada" && c.Status != "entregue" {
				erro = "Só uma mensagem já enviada pode receber esse status."
			}
		}
		if erro != "" {
			h.renderComunicacao(w, r, q, c, erro)
			return
		}
		aviso := "feito"
		switch acao {
		case "enviar", "reenviar":
			if acao == "reenviar" {
				if err = q.SetComunicacaoNaFila(ctx, c.ID); err == nil {
					err = q.CreateComunicacaoEvento(ctx, sqlccliente.CreateComunicacaoEventoParams{ComunicacaoID: c.ID, Tipo: "reenvio", Descricao: "Voltou para a fila", UsuarioID: por})
				}
				if err != nil {
					h.falhaInterna(w, r, "reenviar comunicação", err)
					return
				}
			}
			if _, err = h.tentarEnvio(ctx, pool, q, c.ID, por); errors.Is(err, comunicacao.ErrDesligado) {
				aviso, err = "sem_provedor", nil
			} else if err != nil {
				h.falhaInterna(w, r, "enviar comunicação", err)
				return
			}
		case "cancelar":
			if err = q.SetComunicacaoCancelada(ctx, c.ID); err == nil {
				err = q.CreateComunicacaoEvento(ctx, sqlccliente.CreateComunicacaoEventoParams{ComunicacaoID: c.ID, Tipo: "cancelada", Descricao: "Cancelada", UsuarioID: por})
			}
		case "entregue":
			if err = q.SetComunicacaoEntregue(ctx, c.ID); err == nil {
				err = q.CreateComunicacaoEvento(ctx, sqlccliente.CreateComunicacaoEventoParams{ComunicacaoID: c.ID, Tipo: "entregue", Descricao: "Marcada como entregue", UsuarioID: por})
			}
		case "visualizada":
			if err = q.SetComunicacaoVisualizada(ctx, c.ID); err == nil {
				err = q.CreateComunicacaoEvento(ctx, sqlccliente.CreateComunicacaoEventoParams{ComunicacaoID: c.ID, Tipo: "visualizada", Descricao: "Marcada como visualizada", UsuarioID: por})
			}
		}
		if err != nil {
			h.falhaInterna(w, r, "registrar ação da comunicação", err)
			return
		}
		h.auditar(r, audit.Evento{Acao: "comunicacao." + acao, Entidade: "comunicacao", EntidadeID: strconv.FormatInt(c.ID, 10), Rotulo: "Comunicação " + strconv.FormatInt(c.ID, 10),
			Antes: map[string]any{"status": c.Status}})
		http.Redirect(w, r, comunicacaoCaminho(c.ID)+"?aviso="+aviso, http.StatusSeeOther)
	}
}

// comunicacaoProcessarFila tenta enviar as mensagens da fila cuja hora já chegou (até 50 por vez).
func (h *handlers) comunicacaoProcessarFila(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	canais := h.canaisAtendidos()
	if len(canais) == 0 {
		http.Redirect(w, r, "/comunicacoes?aviso=sem_provedor", http.StatusSeeOther)
		return
	}
	ids, err := q.ListComunicacoesVencidas(ctx, sqlccliente.ListComunicacoesVencidasParams{Canais: canais, Limite: 50})
	if err != nil {
		h.falhaInterna(w, r, "listar fila", err)
		return
	}
	aviso, enviadas := "processada", 0
	for _, id := range ids {
		st, e := h.tentarEnvio(ctx, pool, q, id, idOuNulo(meuID(r)))
		if errors.Is(e, comunicacao.ErrDesligado) {
			aviso = "sem_provedor"
			break
		}
		if e != nil && !errors.Is(e, errForaDaFila) {
			h.falhaInterna(w, r, "processar fila", e)
			return
		}
		if st == "enviada" {
			enviadas++
		}
	}
	h.auditar(r, audit.Evento{Acao: "comunicacao.processar_fila", Entidade: "comunicacao", EntidadeID: "fila", Rotulo: "Fila de comunicações",
		Detalhes: map[string]any{"vencidas": len(ids), "enviadas": enviadas}})
	http.Redirect(w, r, "/comunicacoes?aviso="+aviso, http.StatusSeeOther)
}

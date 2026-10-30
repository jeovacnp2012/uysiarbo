package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Denúncia (9.3): pedido de um cidadão (ou anônimo) para a vigilância verificar um problema num endereço. O
// andamento (linha do tempo, comentários, encaminhamento e anexos) vem da ficha comum de casos (9.1).

var rotulosSituacaoDenuncia = []views.CidadeOpcao{
	{Valor: "aguardando", Rotulo: "Aguardando atendimento"},
	{Valor: "em_atendimento", Rotulo: "Em atendimento"},
	{Valor: "encerrada", Rotulo: "Encerrada"},
	{Valor: "cancelada", Rotulo: "Cancelada"},
}

var rotulosOrigemDenuncia = []views.CidadeOpcao{
	{Valor: "balcao", Rotulo: "Balcão"},
	{Valor: "telefone", Rotulo: "Telefone"},
	{Valor: "aplicativo", Rotulo: "Aplicativo"},
	{Valor: "site", Rotulo: "Site"},
	{Valor: "outro", Rotulo: "Outro"},
}

var tiposDeDenuncia = []views.CidadeOpcao{
	{Valor: "Foco de mosquito", Rotulo: "Foco de mosquito"},
	{Valor: "Água parada", Rotulo: "Água parada"},
	{Valor: "Piscina abandonada", Rotulo: "Piscina abandonada"},
	{Valor: "Terreno baldio", Rotulo: "Terreno baldio"},
	{Valor: "Lixo ou entulho", Rotulo: "Lixo ou entulho"},
	{Valor: "Animal", Rotulo: "Animal"},
	{Valor: "Outros", Rotulo: "Outros"},
}

var opcoesAnonima = []views.CidadeOpcao{{Valor: "sim", Rotulo: "Anônima"}, {Valor: "nao", Rotulo: "Identificada"}}

func init() {
	registrarTipoDeCaso(tipoDeCaso{
		Slug: "denuncia", Rotulo: "Denúncia",
		Ver: "denuncia_visualizar", Editar: "denuncia_anotacao", Encaminhar: "denuncia_encaminhar",
		Existe: func(ctx context.Context, q *sqlccliente.Queries, id int64) bool {
			_, err := q.GetDenuncia(ctx, id)
			return err == nil
		},
		Caminho: func(id int64) string { return "/denuncias/" + strconv.FormatInt(id, 10) },
	})
}

var reSoDigitos = regexp.MustCompile(`\D`)

func ordemDeDenuncias(ordem, dir string) (string, string) {
	switch ordem {
	case "id", "titulo", "tipo", "data", "endereco", "situacao":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func rotuloDenuncia(d sqlccliente.Denuncia) string {
	return "Denúncia " + strconv.FormatInt(d.ID, 10) + " — " + d.Titulo
}

func (h *handlers) opcoesDeBairros(r *http.Request, q *sqlccliente.Queries, escolhido int64) ([]views.CidadeOpcao, error) {
	somente, _ := h.escopoLocais(r)
	bs, err := q.ListBairrosAtivosParaLocal(r.Context(), somente)
	if err != nil {
		return nil, err
	}
	out := make([]views.CidadeOpcao, 0, len(bs))
	for _, b := range bs {
		out = append(out, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome, Selecionada: b.ID == escolhido})
	}
	return out, nil
}

func (h *handlers) denunciasLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.DenunciasFiltro{
		Busca: strings.TrimSpace(qs.Get("busca")), Tipo: qs.Get("tipo"), Situacao: qs.Get("situacao"), Origem: qs.Get("origem"),
		Anonima: simNaoDeAnonima(qs.Get("anonima")), Bairro: qs.Get("bairro"), De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate")),
	}
	if !validoEm(tiposDeDenuncia, f.Tipo) {
		f.Tipo = ""
	}
	if !validoEm(rotulosSituacaoDenuncia, f.Situacao) {
		f.Situacao = ""
	}
	if !validoEm(rotulosOrigemDenuncia, f.Origem) {
		f.Origem = ""
	}
	ordem, dir := ordemDeDenuncias(qs.Get("ordem"), qs.Get("dir"))
	arg := sqlccliente.ListDenunciasParams{
		Busca: f.Busca, Tipo: f.Tipo, Situacao: f.Situacao, Origem: f.Origem, Anonima: f.Anonima, BairroID: idOpcional(f.Bairro),
		DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate), Ordem: ordem, Dir: dir,
	}
	pag := paginaDe(r)
	total, err := q.CountDenuncias(ctx, sqlccliente.CountDenunciasParams{
		Busca: arg.Busca, Tipo: arg.Tipo, Situacao: arg.Situacao, Origem: arg.Origem, Anonima: arg.Anonima, BairroID: arg.BairroID,
		DataDe: arg.DataDe, DataAte: arg.DataAte,
	})
	if err != nil {
		h.falhaInterna(w, r, "contar denúncias", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListDenuncias(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar denúncias", err)
		return
	}
	d := views.DenunciasData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: h.principal(r).Pode("denuncia_nova")}
	for _, x := range rows {
		cidadao := x.CidadaoNome
		if x.Anonima {
			cidadao = "Anônima"
		}
		d.Linhas = append(d.Linhas, views.DenunciaLinha{
			ID: x.ID, IDTexto: strconv.FormatInt(x.ID, 10), Titulo: x.Titulo, Tipo: x.Tipo, Cidadao: cidadao, Data: dataBR(x.DataDenuncia),
			Endereco: x.Endereco, Bairro: x.Bairro, Origem: rotuloDe(rotulosOrigemDenuncia, x.Origem), Situacao: x.Situacao,
			SituacaoRotulo: rotuloDe(rotulosSituacaoDenuncia, x.Situacao), Responsavel: x.Responsavel,
		})
	}
	if d.Bairros, err = h.opcoesDeBairros(r, q, arg.BairroID.Int64); err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	d.Tipos, d.Situacoes, d.Origens, d.Anonimas = marcarOpcao(tiposDeDenuncia, f.Tipo), marcarOpcao(rotulosSituacaoDenuncia, f.Situacao),
		marcarOpcao(rotulosOrigemDenuncia, f.Origem), marcarOpcao(opcoesAnonima, f.Anonima)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "tipo": f.Tipo, "situacao": f.Situacao, "origem": f.Origem, "anonima": f.Anonima, "bairro": f.Bairro, "de": f.De, "ate": f.Ate} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Denuncias(h.layout(r, "Denúncias", "/denuncias"), d))
}

func simNaoDeAnonima(v string) string {
	if v == "sim" || v == "nao" {
		return v
	}
	return ""
}

func (h *handlers) denunciaNova(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d, err := h.formularioDeDenuncia(r, q, views.DenunciaFormData{Data: time.Now().In(h.fuso(r)).Format("2006-01-02")}, 0)
	if err != nil {
		h.falhaInterna(w, r, "montar formulário de denúncia", err)
		return
	}
	render(w, r, views.DenunciaNova(h.layout(r, "Nova denúncia", "/denuncias"), d))
}

// formularioDeDenuncia completa as opções (tipos, origens, bairros) e a lista de denúncias anteriores do telefone.
func (h *handlers) formularioDeDenuncia(r *http.Request, q *sqlccliente.Queries, d views.DenunciaFormData, bairro int64) (views.DenunciaFormData, error) {
	d.CSRF = csrfToken(r.Context())
	d.Tipos = marcarOpcao(tiposDeDenuncia, d.Tipo)
	d.Origens = marcarOpcao(rotulosOrigemDenuncia, d.Origem)
	var err error
	d.Bairros, err = h.opcoesDeBairros(r, q, bairro)
	if err != nil {
		return d, err
	}
	if d.Telefone != "" && !d.Anonima {
		ant, err := q.ListDenunciasDoTelefone(r.Context(), d.Telefone)
		if err != nil {
			return d, err
		}
		for _, a := range ant {
			if a.ID != d.ID {
				d.Anteriores = append(d.Anteriores, views.DenunciaAnterior{ID: a.ID, Titulo: a.Titulo, Data: dataBR(a.DataDenuncia), Situacao: rotuloDe(rotulosSituacaoDenuncia, a.Situacao)})
			}
		}
	}
	return d, nil
}

// lerDadosDaDenuncia lê e valida o formulário. Devolve a mensagem de erro, se houver.
func (h *handlers) lerDadosDaDenuncia(r *http.Request, q *sqlccliente.Queries, id int64) (views.DenunciaFormData, sqlccliente.CreateDenunciaParams, string) {
	t := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	d := views.DenunciaFormData{
		ID: id, Anonima: r.PostFormValue("anonima") == "1", Nome: t("nome"), Telefone: t("telefone"), CPF: reSoDigitos.ReplaceAllString(t("cpf"), ""),
		Titulo: t("titulo"), Descricao: t("descricao"), Data: t("data"), Endereco: t("endereco"),
		Tipo: t("tipo"), Origem: t("origem"),
	}
	bairro := idOpcional(t("bairro"))
	if d.Anonima {
		d.Nome, d.Telefone, d.CPF = "", "", ""
	}
	p := sqlccliente.CreateDenunciaParams{
		Anonima: d.Anonima, CidadaoNome: d.Nome, CidadaoTelefone: d.Telefone, CidadaoCpf: d.CPF, Tipo: d.Tipo, Titulo: d.Titulo, Descricao: d.Descricao,
		DataDenuncia: dataOpcional(d.Data), Origem: d.Origem, EnderecoTexto: d.Endereco, BairroID: bairro,
	}
	msg := ""
	switch {
	case !validoEm(tiposDeDenuncia, d.Tipo):
		msg = "Escolha o tipo da denúncia."
	case !validoEm(rotulosOrigemDenuncia, d.Origem):
		msg = "Escolha a origem da denúncia."
	case d.Titulo == "" || len([]rune(d.Titulo)) > 150:
		msg = "Informe o título (até 150 caracteres)."
	case len([]rune(d.Descricao)) > 4000:
		msg = "A descrição passa de 4000 caracteres."
	case !p.DataDenuncia.Valid:
		msg = "Informe a data da denúncia."
	case len([]rune(d.Nome)) > 150 || len([]rune(d.Telefone)) > 40:
		msg = "Nome ou telefone grande demais."
	case d.CPF != "" && !cpfValido(d.CPF):
		msg = "CPF inválido."
	case len([]rune(d.Endereco)) > 300:
		msg = "O endereço passa de 300 caracteres."
	case d.Endereco == "" && !bairro.Valid:
		msg = "Informe o endereço ou escolha o bairro."
	case bairro.Valid && !h.bairroEscolhivel(r, q, bairro.Int64):
		msg = "O bairro escolhido não existe."
	}
	d, _ = h.formularioDeDenuncia(r, q, d, bairro.Int64)
	return d, p, msg
}

func (h *handlers) bairroEscolhivel(r *http.Request, q *sqlccliente.Queries, id int64) bool {
	ok, err := h.bairroVisivel(r.Context(), r, q, id)
	return err == nil && ok
}

func (h *handlers) denunciaCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d, p, msg := h.lerDadosDaDenuncia(r, q, 0)
	if msg != "" {
		d.Erro = msg
		renderStatus(w, r, http.StatusBadRequest, views.DenunciaNova(h.layout(r, "Nova denúncia", "/denuncias"), d))
		return
	}
	p.CriadoPor = pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}
	n, err := q.CreateDenuncia(r.Context(), p)
	if err != nil {
		h.falhaInterna(w, r, "criar denúncia", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "denuncia", n.ID, "criacao", "", "", "Aguardando atendimento", 0)
	h.auditar(r, audit.Evento{Acao: "denuncia.criar", Entidade: "denuncia", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDenuncia(n),
		Depois: map[string]any{"tipo": n.Tipo, "anonima": n.Anonima, "origem": n.Origem}})
	http.Redirect(w, r, "/denuncias/"+strconv.FormatInt(n.ID, 10), http.StatusSeeOther)
}

func (h *handlers) carregarDenuncia(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.Denuncia, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.Denuncia{}, false
	}
	d, err := q.GetDenuncia(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return d, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar denúncia", err)
		return d, false
	}
	return d, true
}

func (h *handlers) denunciaFicha(w http.ResponseWriter, r *http.Request) {
	h.mostrarDenuncia(w, r, nil)
}

func (h *handlers) mostrarDenuncia(w http.ResponseWriter, r *http.Request, dadosForm *views.DenunciaFormData) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDenuncia(w, r, q)
	if !ok {
		return
	}
	pr := h.principal(r)
	fechada := n.Situacao == "encerrada" || n.Situacao == "cancelada"
	d := views.DenunciaFichaData{
		ID: n.ID, IDTexto: strconv.FormatInt(n.ID, 10), Titulo: n.Titulo, Tipo: n.Tipo, Origem: rotuloDe(rotulosOrigemDenuncia, n.Origem),
		Data: dataBR(n.DataDenuncia), Descricao: n.Descricao, Anonima: n.Anonima, Cidadao: n.CidadaoNome, Telefone: n.CidadaoTelefone,
		Endereco: n.EnderecoTexto, Situacao: n.Situacao, SituacaoRotulo: rotuloDe(rotulosSituacaoDenuncia, n.Situacao), Fechada: fechada,
		CSRF: csrfToken(ctx), PodeEditar: pr.Pode("denuncia_editar") && !fechada, PodeLocal: pr.Pode("denuncia_local") && !fechada,
		PodeVisita: pr.Pode("denuncia_visita") && !fechada, Aviso: avisoDaDenuncia(r.URL.Query().Get("aviso")), VisitaID: n.VisitaID.Int64,
	}
	if n.ResponsavelID.Valid {
		if u, err := q.GetUsuarioByID(ctx, n.ResponsavelID.Int64); err == nil {
			d.Responsavel = u.Nome
		}
	}
	if n.BairroID.Valid {
		if b, err := q.GetBairro(ctx, n.BairroID.Int64); err == nil {
			d.Bairro = b.Nome
		}
	}
	if n.LocalID.Valid {
		d.LocalID = n.LocalID.Int64
		if l, err := q.GetLocal(ctx, n.LocalID.Int64); err == nil {
			d.LocalEndereco = enderecoCompleto(l.Endereco, l.Numero)
		}
	}
	if dadosForm != nil {
		d.Dados = *dadosForm
	} else if d.PodeEditar {
		f := views.DenunciaFormData{
			ID: n.ID, Anonima: n.Anonima, Nome: n.CidadaoNome, Telefone: n.CidadaoTelefone, CPF: n.CidadaoCpf, Titulo: n.Titulo,
			Descricao: n.Descricao, Data: dataISO(n.DataDenuncia), Endereco: n.EnderecoTexto, Tipo: n.Tipo, Origem: n.Origem,
		}
		if d.Dados, err = h.formularioDeDenuncia(r, q, f, n.BairroID.Int64); err != nil {
			h.falhaInterna(w, r, "montar formulário de denúncia", err)
			return
		}
	}
	if d.PodeLocal {
		d.Busca = strings.TrimSpace(r.URL.Query().Get("busca_local"))
		if d.Busca != "" {
			d.Buscou = true
			achados, err := q.BuscarLocaisParaVinculo(ctx, d.Busca)
			if err != nil {
				h.falhaInterna(w, r, "buscar locais", err)
				return
			}
			for _, l := range achados {
				d.Locais = append(d.Locais, views.LocalVinculo{ID: l.ID, Endereco: enderecoCompleto(l.Endereco, l.Numero), Bairro: l.BairroNome})
			}
		}
	}
	if d.Caso, err = h.fichaDoCaso(r, q, tiposDeCaso["denuncia"], n.ID, h.acoesDaDenuncia(r, n)); err != nil {
		h.falhaInterna(w, r, "montar andamento da denúncia", err)
		return
	}
	render(w, r, views.DenunciaFicha(h.layout(r, "Denúncia "+d.IDTexto, "/denuncias"), d))
}

func avisoDaDenuncia(c string) string {
	switch c {
	case "local":
		return "Local atualizado."
	case "visita":
		return "Visita atualizada."
	case "dados":
		return "Dados salvos."
	case "situacao":
		return "Situação atualizada."
	}
	return ""
}

func (h *handlers) acoesDaDenuncia(r *http.Request, n sqlccliente.Denuncia) []views.CasoAcao {
	pr := h.principal(r)
	base := "/denuncias/" + strconv.FormatInt(n.ID, 10)
	var acoes []views.CasoAcao
	switch n.Situacao {
	case "aguardando":
		if pr.Pode("denuncia_atender") {
			acoes = append(acoes, views.CasoAcao{Rotulo: "Assumir atendimento", URL: base + "/atender", Icone: "check"})
		}
	case "encerrada", "cancelada":
		if pr.Pode("denuncia_atender") {
			acoes = append(acoes, views.CasoAcao{Rotulo: "Reabrir", URL: base + "/reabrir", Icone: "undo", Confirmar: "Reabrir esta denúncia?"})
		}
	}
	if n.Situacao == "em_atendimento" && pr.Pode("denuncia_encerrar") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Encerrar", URL: base + "/encerrar", Icone: "lock", Confirmar: "Encerrar esta denúncia?"})
	}
	if (n.Situacao == "aguardando" || n.Situacao == "em_atendimento") && pr.Pode("denuncia_cancelar") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Cancelar", URL: base + "/cancelar", Icone: "x", Perigo: true, Confirmar: "Cancelar esta denúncia?"})
	}
	if pr.Pode("denuncia_cancelar") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Excluir", URL: base + "/excluir", Icone: "trash", Perigo: true, Confirmar: "Enviar esta denúncia para a lixeira?"})
	}
	return acoes
}

func (h *handlers) denunciaSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	antes, ok := h.carregarDenuncia(w, r, q)
	if !ok {
		return
	}
	if antes.Situacao == "encerrada" || antes.Situacao == "cancelada" {
		http.Error(w, "Denúncia fechada: reabra para editar.", http.StatusConflict)
		return
	}
	d, p, msg := h.lerDadosDaDenuncia(r, q, antes.ID)
	if msg != "" {
		d.Erro = msg
		w.WriteHeader(http.StatusBadRequest)
		h.mostrarDenuncia(w, r, &d)
		return
	}
	novo, err := q.UpdateDenunciaDados(r.Context(), sqlccliente.UpdateDenunciaDadosParams{
		ID: antes.ID, Anonima: p.Anonima, CidadaoNome: p.CidadaoNome, CidadaoTelefone: p.CidadaoTelefone, CidadaoCpf: p.CidadaoCpf,
		Tipo: p.Tipo, Titulo: p.Titulo, Descricao: p.Descricao, DataDenuncia: p.DataDenuncia, Origem: p.Origem, EnderecoTexto: p.EnderecoTexto,
		BairroID: p.BairroID,
	})
	if err != nil {
		h.falhaInterna(w, r, "salvar denúncia", err)
		return
	}
	// o CPF nunca vai à auditoria (só se mudou)
	ev := audit.Evento{Acao: "denuncia.editar", Entidade: "denuncia", EntidadeID: strconv.FormatInt(antes.ID, 10), Rotulo: rotuloDenuncia(novo)}
	ev.Antes, ev.Depois = audit.Diferenca(
		map[string]any{"tipo": antes.Tipo, "titulo": antes.Titulo, "anonima": antes.Anonima, "origem": antes.Origem, "endereco": antes.EnderecoTexto, "descricao": antes.Descricao, "cpf_informado": antes.CidadaoCpf != ""},
		map[string]any{"tipo": novo.Tipo, "titulo": novo.Titulo, "anonima": novo.Anonima, "origem": novo.Origem, "endereco": novo.EnderecoTexto, "descricao": novo.Descricao, "cpf_informado": novo.CidadaoCpf != ""})
	h.auditar(r, ev)
	http.Redirect(w, r, "/denuncias/"+strconv.FormatInt(antes.ID, 10)+"?aviso=dados", http.StatusSeeOther)
}

func (h *handlers) denunciaLocal(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDenuncia(w, r, q)
	if !ok {
		return
	}
	if n.Situacao == "encerrada" || n.Situacao == "cancelada" {
		http.Error(w, "Denúncia fechada: reabra para editar.", http.StatusConflict)
		return
	}
	alvo := idOpcional(r.PostFormValue("local"))
	texto := "Local desvinculado"
	if alvo.Valid {
		l, err := q.GetLocal(r.Context(), alvo.Int64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if ok, err := h.localVisivel(r.Context(), r, q, l); err != nil {
			h.falhaInterna(w, r, "checar acesso ao local", err)
			return
		} else if !ok {
			http.NotFound(w, r)
			return
		}
		texto = "Local vinculado: " + enderecoCompleto(l.Endereco, l.Numero)
	}
	if _, err := q.SetDenunciaLocal(r.Context(), sqlccliente.SetDenunciaLocalParams{ID: n.ID, LocalID: alvo}); err != nil {
		h.falhaInterna(w, r, "vincular local", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "denuncia", n.ID, "comentario", texto, "", "", 0)
	h.auditar(r, audit.Evento{Acao: "denuncia.local", Entidade: "denuncia", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDenuncia(n),
		Antes: map[string]any{"local": n.LocalID.Int64}, Depois: map[string]any{"local": alvo.Int64}})
	http.Redirect(w, r, "/denuncias/"+strconv.FormatInt(n.ID, 10)+"?aviso=local#local", http.StatusSeeOther)
}

// denunciaVisita liga (ou desliga) a visita feita por causa da denúncia.
func (h *handlers) denunciaVisita(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDenuncia(w, r, q)
	if !ok {
		return
	}
	if n.Situacao == "encerrada" || n.Situacao == "cancelada" {
		http.Error(w, "Denúncia fechada: reabra para editar.", http.StatusConflict)
		return
	}
	alvo := idOpcional(strings.TrimSpace(r.PostFormValue("visita")))
	texto := "Visita desvinculada"
	if alvo.Valid {
		v, err := q.GetVisita(r.Context(), alvo.Int64)
		if err != nil {
			http.Redirect(w, r, "/denuncias/"+strconv.FormatInt(n.ID, 10)+"?caso_erro="+url.QueryEscape("Visita não encontrada.")+"#linha-do-tempo", http.StatusSeeOther)
			return
		}
		if ok, err := h.visitaVisivel(r.Context(), r, q, v); err != nil || !ok {
			http.NotFound(w, r)
			return
		}
		texto = "Visita vinculada: nº " + strconv.FormatInt(v.ID, 10)
	}
	if _, err := q.SetDenunciaVisita(r.Context(), sqlccliente.SetDenunciaVisitaParams{ID: n.ID, VisitaID: alvo}); err != nil {
		h.falhaInterna(w, r, "vincular visita", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "denuncia", n.ID, "comentario", texto, "", "", 0)
	h.auditar(r, audit.Evento{Acao: "denuncia.visita", Entidade: "denuncia", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDenuncia(n),
		Antes: map[string]any{"visita": n.VisitaID.Int64}, Depois: map[string]any{"visita": alvo.Int64}})
	http.Redirect(w, r, "/denuncias/"+strconv.FormatInt(n.ID, 10)+"?aviso=visita", http.StatusSeeOther)
}

// mudarSituacaoDaDenuncia é o caminho comum das ações de andamento; regra devolve a situação nova e o responsável.
func (h *handlers) mudarSituacaoDaDenuncia(w http.ResponseWriter, r *http.Request, acao string, regra func(n sqlccliente.Denuncia) (string, pgtype.Int8, string)) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDenuncia(w, r, q)
	if !ok {
		return
	}
	sit, resp, erro := regra(n)
	if erro != "" {
		voltarAoCaso(w, r, tiposDeCaso["denuncia"], n.ID, erro)
		return
	}
	if _, err := q.SetDenunciaSituacao(r.Context(), sqlccliente.SetDenunciaSituacaoParams{ID: n.ID, Situacao: sit, ResponsavelID: resp}); err != nil {
		h.falhaInterna(w, r, "mudar situação da denúncia", err)
		return
	}
	texto := strings.TrimSpace(r.PostFormValue("texto"))
	if len([]rune(texto)) > 2000 {
		texto = string([]rune(texto)[:2000])
	}
	_ = h.casoEvento(r.Context(), q, r, "denuncia", n.ID, "situacao", texto, rotuloDe(rotulosSituacaoDenuncia, n.Situacao), rotuloDe(rotulosSituacaoDenuncia, sit), 0)
	h.auditar(r, audit.Evento{Acao: "denuncia." + acao, Entidade: "denuncia", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDenuncia(n),
		Antes: map[string]any{"situacao": n.Situacao}, Depois: map[string]any{"situacao": sit}})
	http.Redirect(w, r, "/denuncias/"+strconv.FormatInt(n.ID, 10)+"?aviso=situacao", http.StatusSeeOther)
}

func (h *handlers) denunciaAtender(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDaDenuncia(w, r, "atender", func(n sqlccliente.Denuncia) (string, pgtype.Int8, string) {
		if n.Situacao != "aguardando" {
			return "", pgtype.Int8{}, "Esta denúncia já está em atendimento ou fechada."
		}
		return "em_atendimento", pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, ""
	})
}

func (h *handlers) denunciaEncerrar(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDaDenuncia(w, r, "encerrar", func(n sqlccliente.Denuncia) (string, pgtype.Int8, string) {
		if n.Situacao != "em_atendimento" {
			return "", pgtype.Int8{}, "Só uma denúncia em atendimento pode ser encerrada."
		}
		return "encerrada", n.ResponsavelID, ""
	})
}

func (h *handlers) denunciaCancelar(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDaDenuncia(w, r, "cancelar", func(n sqlccliente.Denuncia) (string, pgtype.Int8, string) {
		if n.Situacao != "aguardando" && n.Situacao != "em_atendimento" {
			return "", pgtype.Int8{}, "Esta denúncia já está fechada."
		}
		if strings.TrimSpace(r.PostFormValue("texto")) == "" {
			return "", pgtype.Int8{}, "Informe o motivo para cancelar a denúncia."
		}
		return "cancelada", n.ResponsavelID, ""
	})
}

func (h *handlers) denunciaReabrir(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDaDenuncia(w, r, "reabrir", func(n sqlccliente.Denuncia) (string, pgtype.Int8, string) {
		if n.Situacao != "encerrada" && n.Situacao != "cancelada" {
			return "", pgtype.Int8{}, "Só uma denúncia fechada pode ser reaberta."
		}
		return "em_atendimento", pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, ""
	})
}

func (h *handlers) denunciaExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDenuncia(w, r, q)
	if !ok {
		return
	}
	h.excluirRegistro(w, r, "denuncia", n.ID, "/denuncias?aviso=excluido", "/denuncias/"+strconv.FormatInt(n.ID, 10))
}

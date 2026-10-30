package server

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/cifra"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Leishmaniose visceral canina (10.4): teste de um cachorro em um local (DPP, reteste e desfecho do animal).

var (
	rotulosResultadoLvc = []views.CidadeOpcao{
		{Valor: "aguardando", Rotulo: "Aguardando"},
		{Valor: "positivo", Rotulo: "Positivo"},
		{Valor: "negativo", Rotulo: "Negativo"},
	}
	rotulosSituacaoCachorro = []views.CidadeOpcao{
		{Valor: "acompanhamento", Rotulo: "Acompanhamento"},
		{Valor: "eutanasia", Rotulo: "Eutanásia"},
		{Valor: "obito", Rotulo: "Óbito"},
	}
	rotulosSexoLvc = []views.CidadeOpcao{
		{Valor: "femea", Rotulo: "Fêmea"},
		{Valor: "macho", Rotulo: "Macho"},
	}
)

func ordemDeLvc(ordem, dir string) (string, string) {
	switch ordem {
	case "id", "data":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func rotuloLvc(t sqlccliente.LvcTestis) string { return "Teste LVC " + strconv.FormatInt(t.ID, 10) }

func lvcCaminho(id int64) string { return "/zoonose/lvc/" + strconv.FormatInt(id, 10) }

func (h *handlers) lvcLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.LvcFiltro{Busca: strings.TrimSpace(qs.Get("busca")), Dpp: qs.Get("dpp"), Situacao: qs.Get("situacao"), Bairro: qs.Get("bairro"),
		De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate"))}
	if !validoEm(rotulosResultadoLvc, f.Dpp) {
		f.Dpp = ""
	}
	if !validoEm(rotulosSituacaoCachorro, f.Situacao) {
		f.Situacao = ""
	}
	ordem, dir := ordemDeLvc(qs.Get("ordem"), qs.Get("dir"))
	somente, _ := h.escopoLocais(r)
	if h.principal(r).Restrito("zoonose_lvc_meubairro") && !somente.Valid {
		somente = pgtype.Int8{Int64: meuID(r), Valid: true}
	}
	arg := sqlccliente.ListLvcParams{Busca: f.Busca, Dpp: f.Dpp, Situacao: f.Situacao, BairroID: idOpcional(f.Bairro), DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate),
		SomenteUsuario: somente.Valid, UsuarioID: somente.Int64, Ordem: ordem, Dir: dir}
	pag := paginaDe(r)
	total, err := q.CountLvc(ctx, sqlccliente.CountLvcParams{Busca: arg.Busca, Dpp: arg.Dpp, Situacao: arg.Situacao, BairroID: arg.BairroID,
		DataDe: arg.DataDe, DataAte: arg.DataAte, SomenteUsuario: arg.SomenteUsuario, UsuarioID: arg.UsuarioID})
	if err != nil {
		h.falhaInterna(w, r, "contar testes LVC", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListLvc(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar testes LVC", err)
		return
	}
	d := views.LvcData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNovo: h.principal(r).Pode("zoonose_lvc_novo")}
	for _, x := range rows {
		reteste := "Não"
		if x.PedidoReteste {
			reteste = rotuloDe(rotulosResultadoLvc, x.ResultadoReteste)
		}
		d.Linhas = append(d.Linhas, views.LvcLinha{ID: x.ID, IDTexto: strconv.FormatInt(x.ID, 10), Data: dataBR(x.Data), Endereco: x.Endereco,
			Proprietario: x.Proprietario, Cachorro: x.Nome, Dpp: rotuloDe(rotulosResultadoLvc, x.Dpp), Reteste: reteste,
			Situacao: rotuloDe(rotulosSituacaoCachorro, x.SituacaoCachorro), Por: x.Por})
	}
	if d.Bairros, err = h.opcoesDeBairros(r, q, arg.BairroID.Int64); err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	d.Dpps = marcarOpcao(rotulosResultadoLvc, f.Dpp)
	d.Situacoes = marcarOpcao(rotulosSituacaoCachorro, f.Situacao)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "dpp": f.Dpp, "situacao": f.Situacao, "bairro": f.Bairro, "de": f.De, "ate": f.Ate} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.LvcLista(h.layout(r, "Leishmaniose Visceral Canina", "/zoonose/lvc"), d))
}

func (h *handlers) lvcForm(r *http.Request, l sqlccliente.Locai) views.LvcFormData {
	return views.LvcFormData{
		LocalID: l.ID, LocalEndereco: enderecoCompleto(l.Endereco, l.Numero), CSRF: csrfToken(r.Context()),
		Data: h.hojeISO(r), Sexos: marcarOpcao(rotulosSexoLvc, ""), Dpps: marcarOpcao(rotulosResultadoLvc, "aguardando"),
		Retestes: marcarOpcao(rotulosResultadoLvc, "aguardando"), Situacoes: marcarOpcao(rotulosSituacaoCachorro, "acompanhamento"),
		PodeEditar: h.principal(r).Pode("zoonose_lvc_novo"),
	}
}

func (h *handlers) lvcNovo(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	id, ok := idDaURL(r)
	if !ok && r.URL.Query().Get("local") != "" {
		n, e := strconv.ParseInt(r.URL.Query().Get("local"), 10, 64)
		id, ok = n, e == nil
	}
	if ok {
		l, ok := h.droneLocal(w, r, q, id)
		if !ok {
			return
		}
		render(w, r, views.LvcNovo(h.layout(r, "Novo teste LVC", "/zoonose/lvc"), h.lvcForm(r, l)))
		return
	}
	e := views.CampanhaEscolherLocal{Busca: strings.TrimSpace(r.URL.Query().Get("busca_local"))}
	if e.Busca != "" {
		e.Buscou = true
		achados, err := q.BuscarLocaisParaVinculo(r.Context(), e.Busca)
		if err != nil {
			h.falhaInterna(w, r, "buscar locais", err)
			return
		}
		for _, l := range achados {
			e.Locais = append(e.Locais, views.LocalVinculo{ID: l.ID, Endereco: enderecoCompleto(l.Endereco, l.Numero), Bairro: l.BairroNome})
		}
	}
	render(w, r, views.LvcEscolher(h.layout(r, "Novo teste LVC", "/zoonose/lvc"), e))
}

// lerLvc valida o formulário e devolve os parâmetros de gravação.
func (h *handlers) lerLvc(r *http.Request, d *views.LvcFormData) (p sqlccliente.UpdateLvcParams) {
	f := func(n string) string { return strings.TrimSpace(r.PostFormValue(n)) }
	d.Data, d.Nome, d.Raca, d.Pelagem, d.Idade = f("data"), f("nome"), f("raca"), f("pelagem"), f("idade")
	d.Proprietario, d.Telefone, d.Observacao = f("proprietario"), f("proprietario_telefone"), f("observacao")
	d.Cpf = soDigitos(f("proprietario_cpf"))
	d.Reteste = r.PostFormValue("pedido_reteste") == "1"
	d.DataPedido, d.DataRecebido, d.DataMorte = f("data_pedido"), f("data_pedido_recebido"), f("data_morte")
	sexo, dpp, res, sit := f("sexo"), f("dpp"), f("resultado_reteste"), f("situacao_cachorro")
	d.Sexos, d.Dpps, d.Retestes, d.Situacoes = marcarOpcao(rotulosSexoLvc, sexo), marcarOpcao(rotulosResultadoLvc, dpp), marcarOpcao(rotulosResultadoLvc, res), marcarOpcao(rotulosSituacaoCachorro, sit)
	data := dataOpcional(d.Data)
	switch {
	case !data.Valid:
		d.Erro = "Informe a data do teste."
	case sexo != "" && !validoEm(rotulosSexoLvc, sexo):
		d.Erro = "Escolha o sexo do cachorro."
	case !validoEm(rotulosResultadoLvc, dpp) || !validoEm(rotulosResultadoLvc, res):
		d.Erro = "Resultado de teste inválido."
	case !validoEm(rotulosSituacaoCachorro, sit):
		d.Erro = "Escolha a situação do cachorro."
	case len(d.Cpf) != 0 && len(d.Cpf) != 11:
		d.Erro = "O CPF precisa ter 11 números."
	case len([]rune(d.Nome)) > 120 || len([]rune(d.Raca)) > 80 || len([]rune(d.Pelagem)) > 80 || len([]rune(d.Idade)) > 40 ||
		len([]rune(d.Proprietario)) > 160 || len([]rune(d.Telefone)) > 20 || len([]rune(d.Observacao)) > 2000:
		d.Erro = "Algum campo passa do tamanho permitido."
	case (f("data_pedido") != "" && !dataOpcional(d.DataPedido).Valid) || (d.DataRecebido != "" && !dataOpcional(d.DataRecebido).Valid) || (d.DataMorte != "" && !dataOpcional(d.DataMorte).Valid):
		d.Erro = "Alguma data está inválida."
	case sit == "acompanhamento" && d.DataMorte != "":
		d.Erro = "A data da morte só vale para eutanásia ou óbito."
	}
	p = sqlccliente.UpdateLvcParams{Data: data, Nome: d.Nome, Raca: d.Raca, Pelagem: d.Pelagem, Idade: d.Idade, Sexo: sexo, Proprietario: d.Proprietario,
		ProprietarioTelefone: d.Telefone, ProprietarioCpf: cifra.Cifrar(d.Cpf), Observacao: d.Observacao, Dpp: dpp, PedidoReteste: d.Reteste,
		ResultadoReteste: res, SituacaoCachorro: sit}
	if d.Reteste {
		p.DataPedido, p.DataPedidoRecebido = dataOpcional(d.DataPedido), dataOpcional(d.DataRecebido)
	} else {
		p.ResultadoReteste = "aguardando"
	}
	if sit != "acompanhamento" {
		p.DataMorte = dataOpcional(d.DataMorte)
	}
	return
}

func (h *handlers) lvcCriar(w http.ResponseWriter, r *http.Request) {
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
	l, ok := h.droneLocal(w, r, q, localID)
	if !ok {
		return
	}
	d := h.lvcForm(r, l)
	p := h.lerLvc(r, &d)
	if d.Erro != "" {
		renderStatus(w, r, http.StatusBadRequest, views.LvcNovo(h.layout(r, "Novo teste LVC", "/zoonose/lvc"), d))
		return
	}
	t, err := q.CreateLvc(r.Context(), sqlccliente.CreateLvcParams{LocalID: l.ID, Data: p.Data, Nome: p.Nome, Raca: p.Raca, Pelagem: p.Pelagem, Idade: p.Idade,
		Sexo: p.Sexo, Proprietario: p.Proprietario, ProprietarioTelefone: p.ProprietarioTelefone, ProprietarioCpf: p.ProprietarioCpf, Observacao: p.Observacao,
		Dpp: p.Dpp, PedidoReteste: p.PedidoReteste, DataPedido: p.DataPedido, DataPedidoRecebido: p.DataPedidoRecebido, ResultadoReteste: p.ResultadoReteste,
		SituacaoCachorro: p.SituacaoCachorro, DataMorte: p.DataMorte, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}})
	if err != nil {
		h.falhaInterna(w, r, "criar teste LVC", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "lvc.criar", Entidade: "lvc", EntidadeID: strconv.FormatInt(t.ID, 10), Rotulo: rotuloLvc(t),
		Depois: map[string]any{"local": t.LocalID, "dpp": t.Dpp, "situacao": t.SituacaoCachorro}})
	http.Redirect(w, r, lvcCaminho(t.ID)+"?aviso=criado", http.StatusSeeOther)
}

func (h *handlers) carregarLvc(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.LvcTestis, sqlccliente.Locai, bool) {
	var l sqlccliente.Locai
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.LvcTestis{}, l, false
	}
	t, err := q.GetLvc(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return t, l, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar teste LVC", err)
		return t, l, false
	}
	if l, err = q.GetLocalQualquer(r.Context(), t.LocalID); err != nil {
		h.falhaInterna(w, r, "buscar local", err)
		return t, l, false
	}
	if ok, err := h.localVisivel(r.Context(), r, q, l); err != nil {
		h.falhaInterna(w, r, "checar acesso ao local", err)
		return t, l, false
	} else if !ok && !l.DeletedAt.Valid {
		http.NotFound(w, r)
		return t, l, false
	}
	return t, l, true
}

func (h *handlers) lvcFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	t, l, ok := h.carregarLvc(w, r, q)
	if !ok {
		return
	}
	d := h.lvcForm(r, l)
	d.ID, d.Data, d.Nome, d.Raca, d.Pelagem, d.Idade = t.ID, dataISO(t.Data), t.Nome, t.Raca, t.Pelagem, t.Idade
	d.Proprietario, d.Telefone, d.Cpf, d.Observacao = t.Proprietario, t.ProprietarioTelefone, cifra.Decifrar(t.ProprietarioCpf), t.Observacao
	d.Reteste, d.DataPedido, d.DataRecebido, d.DataMorte = t.PedidoReteste, dataISO(t.DataPedido), dataISO(t.DataPedidoRecebido), dataISO(t.DataMorte)
	d.Sexos, d.Dpps, d.Retestes, d.Situacoes = marcarOpcao(rotulosSexoLvc, t.Sexo), marcarOpcao(rotulosResultadoLvc, t.Dpp), marcarOpcao(rotulosResultadoLvc, t.ResultadoReteste), marcarOpcao(rotulosSituacaoCachorro, t.SituacaoCachorro)
	d.PodeEditar = h.principal(r).Pode("zoonose_lvc_editar")
	switch r.URL.Query().Get("aviso") {
	case "criado":
		d.Aviso = "Teste registrado."
	case "salvo":
		d.Aviso = "Teste salvo."
	}
	render(w, r, views.LvcFicha(h.layout(r, "Teste LVC "+strconv.FormatInt(t.ID, 10), "/zoonose/lvc"), d))
}

func (h *handlers) lvcSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	t, l, ok := h.carregarLvc(w, r, q)
	if !ok {
		return
	}
	d := h.lvcForm(r, l)
	d.ID, d.PodeEditar = t.ID, true
	p := h.lerLvc(r, &d)
	if d.Erro != "" {
		renderStatus(w, r, http.StatusBadRequest, views.LvcFicha(h.layout(r, "Teste LVC "+strconv.FormatInt(t.ID, 10), "/zoonose/lvc"), d))
		return
	}
	p.ID = t.ID
	if _, err := q.UpdateLvc(r.Context(), p); err != nil {
		h.falhaInterna(w, r, "salvar teste LVC", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "lvc.editar", Entidade: "lvc", EntidadeID: strconv.FormatInt(t.ID, 10), Rotulo: rotuloLvc(t),
		Antes:  map[string]any{"dpp": t.Dpp, "reteste": t.ResultadoReteste, "situacao": t.SituacaoCachorro},
		Depois: map[string]any{"dpp": p.Dpp, "reteste": p.ResultadoReteste, "situacao": p.SituacaoCachorro}})
	http.Redirect(w, r, lvcCaminho(t.ID)+"?aviso=salvo", http.StatusSeeOther)
}

func (h *handlers) lvcExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	t, _, ok := h.carregarLvc(w, r, q)
	if !ok {
		return
	}
	h.excluirRegistro(w, r, "lvc", t.ID, "/zoonose/lvc?aviso=excluido", lvcCaminho(t.ID))
}

func soDigitos(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// hojeISO é o dia de hoje no fuso do cliente (aaaa-mm-dd).
func (h *handlers) hojeISO(r *http.Request) string {
	return time.Now().In(h.fuso(r)).Format("2006-01-02")
}

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
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Campanha antirrábica (10.2): atendimento de um local na campanha de vacinação, com os animais vacinados e não vacinados.
// O registro animal a animal com lote da vacina depende do estoque (Etapa 11); por enquanto só os totais.

var rotulosSituacaoCampanha = []views.CidadeOpcao{
	{Valor: "atendido", Rotulo: "Atendido"},
	{Valor: "recusa", Rotulo: "Recusa do Proprietário"},
	{Valor: "vacinado", Rotulo: "Animal já Vacinado"},
	{Valor: "fechada", Rotulo: "Porteira Fechada"},
	{Valor: "desabrigado", Rotulo: "Local Desabrigado"},
	{Valor: "responsavel_ausente", Rotulo: "Responsável Ausente"},
}

func ordemDeCampanhas(ordem, dir string) (string, string) {
	switch ordem {
	case "id", "data", "endereco":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func rotuloCampanha(c sqlccliente.Campanha) string {
	return "Campanha " + strconv.FormatInt(c.ID, 10)
}

func campanhaCaminho(id int64) string { return "/zoonose/campanha/" + strconv.FormatInt(id, 10) }

func (h *handlers) campanhasLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.CampanhasFiltro{
		Busca: strings.TrimSpace(qs.Get("busca")), Situacao: qs.Get("situacao"), Bairro: qs.Get("bairro"),
		De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate")),
	}
	if !validoEm(rotulosSituacaoCampanha, f.Situacao) {
		f.Situacao = ""
	}
	ordem, dir := ordemDeCampanhas(qs.Get("ordem"), qs.Get("dir"))
	somente, _ := h.escopoLocais(r)
	if h.principal(r).Restrito("campanha_meubairro") && !somente.Valid {
		somente = pgtype.Int8{Int64: meuID(r), Valid: true}
	}
	arg := sqlccliente.ListCampanhasParams{
		Busca: f.Busca, Situacao: f.Situacao, BairroID: idOpcional(f.Bairro), DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate),
		SomenteUsuario: somente.Valid, UsuarioID: somente.Int64, Ordem: ordem, Dir: dir,
	}
	pag := paginaDe(r)
	total, err := q.CountCampanhas(ctx, sqlccliente.CountCampanhasParams{
		Busca: arg.Busca, Situacao: arg.Situacao, BairroID: arg.BairroID, DataDe: arg.DataDe, DataAte: arg.DataAte,
		SomenteUsuario: arg.SomenteUsuario, UsuarioID: arg.UsuarioID,
	})
	if err != nil {
		h.falhaInterna(w, r, "contar campanhas", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListCampanhas(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar campanhas", err)
		return
	}
	loc := h.fuso(r)
	d := views.CampanhasData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: h.principal(r).Pode("campanha_adicionar")}
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.CampanhaLinha{
			ID: x.ID, IDTexto: strconv.FormatInt(x.ID, 10), Data: x.DataHora.Time.In(loc).Format("02/01/2006 15:04"),
			Endereco: x.Endereco, Bairro: x.Bairro, Situacao: rotuloDe(rotulosSituacaoCampanha, x.Situacao), Aplicador: x.Aplicador,
			Vacinados: int(x.VacCachorros + x.VacGatos), NaoVacinados: int(x.NaoCachorros + x.NaoGatos), Estoque: x.UsarEstoque,
		})
	}
	if d.Bairros, err = h.opcoesDeBairros(r, q, arg.BairroID.Int64); err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	d.Situacoes = marcarOpcao(rotulosSituacaoCampanha, f.Situacao)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "situacao": f.Situacao, "bairro": f.Bairro, "de": f.De, "ate": f.Ate} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Campanhas(h.layout(r, "Campanha antirrábica", "/zoonose/campanha"), d))
}

func (h *handlers) campanhaLocal(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, id int64) (sqlccliente.Locai, bool) {
	return h.droneLocal(w, r, q, id)
}

// campanhaForm monta os dados do formulário (novo ou edição) de um local.
func (h *handlers) campanhaForm(r *http.Request, q *sqlccliente.Queries, l sqlccliente.Locai) (views.CampanhaFormData, error) {
	d := views.CampanhaFormData{
		LocalID: l.ID, LocalEndereco: enderecoCompleto(l.Endereco, l.Numero), CSRF: csrfToken(r.Context()),
		Situacoes: marcarOpcao(rotulosSituacaoCampanha, "atendido"), Data: time.Now().In(h.fuso(r)).Format(formatoHoraLocal),
		Aplicador: strconv.FormatInt(meuID(r), 10), PodeEditar: h.principal(r).Pode("campanha_adicionar"),
		Vac: [2]string{"0", "0"}, Nao: [2]string{"0", "0"},
	}
	d.Diaria = strings.TrimSpace(r.FormValue("diaria"))
	diarias, err := h.opcoesDeDiarias(r, q, idOpcional(d.Diaria))
	if err != nil {
		return d, err
	}
	d.Diarias = diarias
	us, err := q.ListUsuariosAtivosParaResponsavel(r.Context())
	if err != nil {
		return d, err
	}
	for _, u := range us {
		d.Aplicadores = append(d.Aplicadores, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome, Selecionada: strconv.FormatInt(u.ID, 10) == d.Aplicador})
	}
	return d, nil
}

func (h *handlers) campanhaNova(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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
		l, ok := h.campanhaLocal(w, r, q, id)
		if !ok {
			return
		}
		d, err := h.campanhaForm(r, q, l)
		if err != nil {
			h.falhaInterna(w, r, "montar formulário da campanha", err)
			return
		}
		render(w, r, views.CampanhaNova(h.layout(r, "Nova campanha de vacinação", "/zoonose/campanha"), d))
		return
	}
	e := views.CampanhaEscolherLocal{Busca: strings.TrimSpace(r.URL.Query().Get("busca_local"))}
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
	render(w, r, views.CampanhaEscolher(h.layout(r, "Nova campanha de vacinação", "/zoonose/campanha"), e))
}

// lerCampanha valida o formulário; devolve os parâmetros comuns à criação e à edição.
func (h *handlers) lerCampanha(r *http.Request, d *views.CampanhaFormData) (data time.Time, sit string, vac, nao [2]int32, aplic, diaria pgtype.Int8) {
	loc := h.fuso(r)
	d.Data = strings.TrimSpace(r.PostFormValue("data"))
	sit = r.PostFormValue("situacao")
	d.Situacoes = marcarOpcao(rotulosSituacaoCampanha, sit)
	d.Descricao = strings.TrimSpace(r.PostFormValue("descricao"))
	d.Aplicador = strings.TrimSpace(r.PostFormValue("aplicador"))
	d.Diaria = strings.TrimSpace(r.PostFormValue("diaria"))
	d.Vac = [2]string{r.PostFormValue("vac_cachorros"), r.PostFormValue("vac_gatos")}
	d.Nao = [2]string{r.PostFormValue("nao_cachorros"), r.PostFormValue("nao_gatos")}
	var ok bool
	if data, ok = horaLocal(d.Data, loc); !ok {
		d.Erro = "Informe a data e a hora do atendimento."
	}
	if !validoEm(rotulosSituacaoCampanha, sit) {
		d.Erro = "Escolha a situação do atendimento."
	}
	for i, txt := range []string{d.Vac[0], d.Vac[1], d.Nao[0], d.Nao[1]} {
		v, err := strconv.Atoi(strings.TrimSpace(txt))
		if strings.TrimSpace(txt) == "" {
			v, err = 0, nil
		}
		if err != nil || v < 0 || v > 9999 {
			d.Erro = "As quantidades de animais precisam ser números de 0 a 9999."
			continue
		}
		if i < 2 {
			vac[i] = int32(v)
		} else {
			nao[i-2] = int32(v)
		}
	}
	if len([]rune(d.Descricao)) > 2000 {
		d.Erro = "A anotação passa de 2000 caracteres."
	}
	aplic = idOpcional(d.Aplicador)
	diaria = idOpcional(d.Diaria)
	return
}

func (h *handlers) campanhaCriar(w http.ResponseWriter, r *http.Request) {
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
	l, ok := h.campanhaLocal(w, r, q, localID)
	if !ok {
		return
	}
	d, err := h.campanhaForm(r, q, l)
	if err != nil {
		h.falhaInterna(w, r, "montar formulário da campanha", err)
		return
	}
	data, sit, vac, nao, aplic, diaria := h.lerCampanha(r, &d)
	if d.Erro != "" {
		renderStatus(w, r, http.StatusBadRequest, views.CampanhaNova(h.layout(r, "Nova campanha de vacinação", "/zoonose/campanha"), d))
		return
	}
	c, err := q.CreateCampanha(r.Context(), sqlccliente.CreateCampanhaParams{
		LocalID: l.ID, DataHora: pgtype.Timestamptz{Time: data, Valid: true}, Situacao: sit,
		VacCachorros: vac[0], VacGatos: vac[1], NaoCachorros: nao[0], NaoGatos: nao[1], Descricao: d.Descricao,
		AplicadorID: aplic, DiariaID: diaria, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
	})
	if err != nil {
		h.falhaInterna(w, r, "criar campanha", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "campanha.criar", Entidade: "campanha", EntidadeID: strconv.FormatInt(c.ID, 10), Rotulo: rotuloCampanha(c),
		Depois: map[string]any{"local": c.LocalID, "situacao": c.Situacao, "vacinados": c.VacCachorros + c.VacGatos, "nao_vacinados": c.NaoCachorros + c.NaoGatos}})
	http.Redirect(w, r, campanhaCaminho(c.ID)+"?aviso=criada", http.StatusSeeOther)
}

func (h *handlers) carregarCampanha(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.Campanha, sqlccliente.Locai, bool) {
	var l sqlccliente.Locai
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.Campanha{}, l, false
	}
	c, err := q.GetCampanha(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return c, l, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar campanha", err)
		return c, l, false
	}
	l, err = q.GetLocalQualquer(r.Context(), c.LocalID)
	if err != nil {
		h.falhaInterna(w, r, "buscar local", err)
		return c, l, false
	}
	if ok, err := h.localVisivel(r.Context(), r, q, l); err != nil {
		h.falhaInterna(w, r, "checar acesso ao local", err)
		return c, l, false
	} else if !ok && !l.DeletedAt.Valid {
		http.NotFound(w, r)
		return c, l, false
	}
	return c, l, true
}

func (h *handlers) campanhaFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	c, l, ok := h.carregarCampanha(w, r, q)
	if !ok {
		return
	}
	d, err := h.campanhaForm(r, q, l)
	if err != nil {
		h.falhaInterna(w, r, "montar formulário da campanha", err)
		return
	}
	d.ID = c.ID
	d.Data = c.DataHora.Time.In(h.fuso(r)).Format(formatoHoraLocal)
	d.Situacoes = marcarOpcao(rotulosSituacaoCampanha, c.Situacao)
	d.Descricao = c.Descricao
	d.Aplicador = strconv.FormatInt(c.AplicadorID.Int64, 10)
	if d.Diarias, err = h.opcoesDeDiarias(r, q, c.DiariaID); err != nil {
		h.falhaInterna(w, r, "listar diárias", err)
		return
	}
	d.Diaria = ""
	if c.DiariaID.Valid {
		d.Diaria = strconv.FormatInt(c.DiariaID.Int64, 10)
	}
	if !c.AplicadorID.Valid {
		d.Aplicador = ""
	}
	for i := range d.Aplicadores {
		d.Aplicadores[i].Selecionada = d.Aplicadores[i].ID == c.AplicadorID.Int64 && c.AplicadorID.Valid
	}
	d.Vac = [2]string{strconv.Itoa(int(c.VacCachorros)), strconv.Itoa(int(c.VacGatos))}
	d.Nao = [2]string{strconv.Itoa(int(c.NaoCachorros)), strconv.Itoa(int(c.NaoGatos))}
	d.PodeEditar = h.principal(r).Pode("campanha_editar")
	d.Estoque = c.UsarEstoque
	switch r.URL.Query().Get("aviso") {
	case "criada":
		d.Aviso = "Campanha registrada."
	case "salva":
		d.Aviso = "Campanha salva."
	}
	render(w, r, views.CampanhaFicha(h.layout(r, "Campanha "+strconv.FormatInt(c.ID, 10), "/zoonose/campanha"), d))
}

func (h *handlers) campanhaSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	c, l, ok := h.carregarCampanha(w, r, q)
	if !ok {
		return
	}
	d, err := h.campanhaForm(r, q, l)
	if err != nil {
		h.falhaInterna(w, r, "montar formulário da campanha", err)
		return
	}
	d.ID, d.PodeEditar, d.Estoque = c.ID, true, c.UsarEstoque
	data, sit, vac, nao, aplic, diaria := h.lerCampanha(r, &d)
	if d.Erro != "" {
		renderStatus(w, r, http.StatusBadRequest, views.CampanhaFicha(h.layout(r, "Campanha "+strconv.FormatInt(c.ID, 10), "/zoonose/campanha"), d))
		return
	}
	if _, err := q.UpdateCampanha(r.Context(), sqlccliente.UpdateCampanhaParams{
		ID: c.ID, DataHora: pgtype.Timestamptz{Time: data, Valid: true}, Situacao: sit, VacCachorros: vac[0], VacGatos: vac[1],
		NaoCachorros: nao[0], NaoGatos: nao[1], Descricao: d.Descricao, AplicadorID: aplic, DiariaID: diaria,
	}); err != nil {
		h.falhaInterna(w, r, "salvar campanha", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "campanha.editar", Entidade: "campanha", EntidadeID: strconv.FormatInt(c.ID, 10), Rotulo: rotuloCampanha(c),
		Antes:  map[string]any{"situacao": c.Situacao, "vacinados": c.VacCachorros + c.VacGatos, "nao_vacinados": c.NaoCachorros + c.NaoGatos},
		Depois: map[string]any{"situacao": sit, "vacinados": vac[0] + vac[1], "nao_vacinados": nao[0] + nao[1]}})
	http.Redirect(w, r, campanhaCaminho(c.ID)+"?aviso=salva", http.StatusSeeOther)
}

func (h *handlers) campanhaExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	c, _, ok := h.carregarCampanha(w, r, q)
	if !ok {
		return
	}
	h.excluirRegistro(w, r, "campanha", c.ID, "/zoonose/campanha?aviso=excluido", campanhaCaminho(c.ID))
}

// opcoesDeDiarias lista as diárias abertas (e a atual, mesmo fechada) para o atendimento escolher a sua.
func (h *handlers) opcoesDeDiarias(r *http.Request, q *sqlccliente.Queries, atual pgtype.Int8) ([]views.CidadeOpcao, error) {
	abertas, err := q.ListDiariasAbertas(r.Context())
	if err != nil {
		return nil, err
	}
	var out []views.CidadeOpcao
	achou := false
	for _, a := range abertas {
		achou = achou || (atual.Valid && a.ID == atual.Int64)
		out = append(out, views.CidadeOpcao{ID: a.ID, Rotulo: "Diária " + strconv.FormatInt(a.ID, 10) + " — " + dataBR(a.Data), Selecionada: atual.Valid && a.ID == atual.Int64})
	}
	if atual.Valid && !achou {
		if d, err := q.GetDiariaQualquer(r.Context(), atual.Int64); err == nil {
			out = append(out, views.CidadeOpcao{ID: d.ID, Rotulo: "Diária " + strconv.FormatInt(d.ID, 10) + " — " + dataBR(d.Data), Selecionada: true})
		}
	}
	return out, nil
}

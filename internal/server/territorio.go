package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

var ufsValidas = map[string]bool{
	"AC": true, "AL": true, "AP": true, "AM": true, "BA": true, "CE": true, "DF": true, "ES": true, "GO": true,
	"MA": true, "MT": true, "MS": true, "MG": true, "PA": true, "PB": true, "PR": true, "PE": true, "PI": true,
	"RJ": true, "RN": true, "RS": true, "RO": true, "RR": true, "SC": true, "SP": true, "SE": true, "TO": true,
}

// somenteUsuario devolve o usuário a quem os bairros ficam restritos (permissão "apenas atribuídos"),
// ou um valor inválido (sem restrição).
func (h *handlers) somenteUsuario(r *http.Request) pgtype.Int8 {
	if h.principal(r).Restrito("bairro_meubairro") {
		if id := meuID(r); id != 0 {
			return pgtype.Int8{Int64: id, Valid: true}
		}
		return pgtype.Int8{Int64: -1, Valid: true} // restrito, mas sem usuário do cliente: não vê nada
	}
	return pgtype.Int8{}
}

// bairroVisivel diz se o bairro está ao alcance de quem pede (respeita "apenas atribuídos").
func (h *handlers) bairroVisivel(ctx context.Context, r *http.Request, q *sqlccliente.Queries, id int64) (bool, error) {
	somente := h.somenteUsuario(r)
	if !somente.Valid {
		return true, nil
	}
	resp, err := q.ListResponsaveisDoBairro(ctx, id)
	if err != nil {
		return false, err
	}
	return slices.Contains(resp, somente.Int64), nil
}

// ------------------------------------------------------------------- Cidades

func (h *handlers) cidadesLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListCidades(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar cidades", err)
		return
	}
	linhas := make([]views.CidadeLinha, len(rows))
	for i, c := range rows {
		linhas[i] = views.CidadeLinha{ID: c.ID, Nome: c.Nome, UF: c.Uf, IBGE: c.Ibge, Ativo: c.Ativo, Bairros: int(c.TotalBairros)}
	}
	render(w, r, views.Cidades(h.layout(r, "Cidades", "/cidades"), linhas, h.principal(r).Pode("bairro_adicionar")))
}

func (h *handlers) cidadeNovaForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.CidadeForm(h.layout(r, "Nova cidade", "/cidades"), views.CidadeFormData{Ativo: true, PodeEditar: true, CSRF: csrfToken(r.Context())}))
}

func (h *handlers) cidadeEditarForm(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	c, err := q.GetCidade(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar cidade", err)
		return
	}
	render(w, r, views.CidadeForm(h.layout(r, "Cidade", "/cidades"), views.CidadeFormData{
		ID: c.ID, Nome: c.Nome, UF: c.Uf, IBGE: c.Ibge, Ativo: c.Ativo,
		PodeEditar: h.principal(r).Pode("bairro_adicionar"), CSRF: csrfToken(r.Context()),
	}))
}

var reIBGE = regexp.MustCompile(`^[0-9]{0,7}$`)

func (h *handlers) cidadeSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	var id int64
	if chi.URLParam(r, "id") != "" {
		var ok bool
		if id, ok = idDaURL(r); !ok {
			http.NotFound(w, r)
			return
		}
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	var antes sqlccliente.Cidade
	if id != 0 {
		if antes, err = q.GetCidade(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar cidade", err)
			return
		}
	}
	nome, uf := strings.TrimSpace(r.PostFormValue("nome")), strings.ToUpper(strings.TrimSpace(r.PostFormValue("uf")))
	ibge, ativo := strings.TrimSpace(r.PostFormValue("ativo")), r.PostFormValue("ativo") == "1"
	if id == 0 {
		ativo = true
	}
	ibge = strings.TrimSpace(r.PostFormValue("ibge"))
	falha := func(msg string) {
		renderStatus(w, r, http.StatusBadRequest, views.CidadeForm(h.layout(r, "Cidade", "/cidades"), views.CidadeFormData{
			ID: id, Nome: nome, UF: uf, IBGE: ibge, Ativo: ativo, PodeEditar: true, Erro: msg, CSRF: csrfToken(r.Context()),
		}))
	}
	switch {
	case len([]rune(nome)) < 2 || len([]rune(nome)) > 100:
		falha("Informe o nome da cidade (de 2 a 100 caracteres).")
		return
	case !ufsValidas[uf]:
		falha("Informe a sigla de um estado brasileiro, como MT.")
		return
	case !reIBGE.MatchString(ibge):
		falha("O código IBGE tem até 7 dígitos, só números.")
		return
	}

	tx, err := pool.Begin(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(r.Context())
	qt := sqlccliente.New(tx)
	ev := audit.Evento{Entidade: "cidade", Rotulo: nome}
	novo := map[string]any{"nome": nome, "uf": uf, "ibge": ibge, "ativo": ativo}
	if id == 0 {
		c, err := qt.CreateCidade(r.Context(), sqlccliente.CreateCidadeParams{Nome: nome, Uf: uf, Ibge: ibge})
		if violacaoUnica(err) {
			falha("Já existe uma cidade com esse nome nesse estado.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "criar cidade", err)
			return
		}
		ev.Acao, ev.EntidadeID, ev.Depois = "cidade.criar", strconv.FormatInt(c.ID, 10), novo
	} else {
		_, err := qt.UpdateCidade(r.Context(), sqlccliente.UpdateCidadeParams{ID: id, Nome: nome, Uf: uf, Ibge: ibge, Ativo: ativo})
		if violacaoUnica(err) {
			falha("Já existe uma cidade com esse nome nesse estado.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "salvar cidade", err)
			return
		}
		a, d := audit.Diferenca(map[string]any{"nome": antes.Nome, "uf": antes.Uf, "ibge": antes.Ibge, "ativo": antes.Ativo}, novo)
		if len(a)+len(d) == 0 {
			naoAuditar(r)
		} else {
			ev.Acao, ev.EntidadeID, ev.Antes, ev.Depois = "cidade.editar", strconv.FormatInt(id, 10), a, d
		}
	}
	if ev.Acao != "" {
		h.auditarCom(audit.Cliente{Q: qt}, r, ev)
	}
	if err := tx.Commit(r.Context()); err != nil {
		h.falhaInterna(w, r, "confirmar cidade", err)
		return
	}
	http.Redirect(w, r, "/cidades", http.StatusSeeOther)
}

// ------------------------------------------------------------------- Bairros

func opcoesDeCidade(cidades []sqlccliente.Cidade, escolhida int64) []views.CidadeOpcao {
	out := make([]views.CidadeOpcao, 0, len(cidades))
	for _, c := range cidades {
		out = append(out, views.CidadeOpcao{ID: c.ID, Rotulo: c.Nome + "/" + c.Uf, Selecionada: c.ID == escolhida})
	}
	return out
}

func (h *handlers) bairrosLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.BairrosFiltro{Busca: strings.TrimSpace(qs.Get("busca")), Cidade: qs.Get("cidade"), Status: qs.Get("status"), Categoria: qs.Get("categoria")}
	cidadeID := pgtype.Int8{}
	if n, err := strconv.ParseInt(f.Cidade, 10, 64); err == nil && n > 0 {
		cidadeID = pgtype.Int8{Int64: n, Valid: true}
	}
	somente := h.somenteUsuario(r)

	pag := views.Pagination{Page: 1, PerPage: 20}
	if n, err := strconv.Atoi(qs.Get("pagina")); err == nil && n > 1 {
		pag.Page = n
	}
	total, err := q.CountBairros(ctx, sqlccliente.CountBairrosParams{Busca: f.Busca, CidadeID: cidadeID, Status: f.Status, Categoria: f.Categoria, SomenteUsuario: somente})
	if err != nil {
		h.falhaInterna(w, r, "contar bairros", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListBairros(ctx, sqlccliente.ListBairrosParams{
		Busca: f.Busca, CidadeID: cidadeID, Status: f.Status, Categoria: f.Categoria, SomenteUsuario: somente,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	cidades, err := q.ListCidades(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar cidades", err)
		return
	}
	var cs []sqlccliente.Cidade
	for _, c := range cidades {
		cs = append(cs, sqlccliente.Cidade{ID: c.ID, Nome: c.Nome, Uf: c.Uf})
	}

	linhas := make([]views.BairroLinha, len(rows))
	for i, b := range rows {
		linhas[i] = views.BairroLinha{
			ID: b.ID, Nome: b.Nome, Cidade: b.CidadeNome + "/" + b.CidadeUf, Responsaveis: b.Responsaveis, Status: b.Status,
			Categoria: rotuloCategoria(b.Categoria), Classificacao: rotuloClassificacao(b.Classificacao), Mutirao: b.Mutirao, Quarteiroes: int(b.TotalQuarteiroes),
		}
	}
	query := url.Values{}
	for k, v := range map[string]string{"busca": f.Busca, "cidade": f.Cidade, "status": f.Status, "categoria": f.Categoria} {
		if v != "" {
			query.Set(k, v)
		}
	}
	render(w, r, views.Bairros(h.layout(r, "Bairros", "/bairros"), views.BairrosData{
		Filtro: f, Cidades: opcoesDeCidade(cs, cidadeID.Int64), Linhas: linhas, Pag: pag, Query: query,
		PodeEditar: h.principal(r).Pode("bairro_adicionar"), Restrito: somente.Valid,
	}))
}

func rotuloCategoria(s string) string {
	if s == "povoado" {
		return "povoado"
	}
	return "bairro"
}

func rotuloClassificacao(s string) string {
	if s == "rural" {
		return "rural"
	}
	return "urbano"
}

// dadosBairro é o que veio do formulário (ou do banco), antes de validar.
type dadosBairro struct {
	cidadeID                                 int64
	nome                                     string
	mutirao, producao                        bool
	status, classificacao, categoria         string
	ibge, zona                               string
	latitude, longitude, altitude, distancia string
	responsaveis                             []int64
}

func textoFloat(v pgtype.Float8) string {
	if !v.Valid {
		return ""
	}
	return strconv.FormatFloat(v.Float64, 'f', -1, 64)
}

func dadosDoBairro(b sqlccliente.Bairro, resp []int64) dadosBairro {
	return dadosBairro{
		cidadeID: b.CidadeID, nome: b.Nome, mutirao: b.Mutirao, producao: b.Producao, status: b.Status,
		classificacao: b.Classificacao, categoria: b.Categoria, ibge: b.Ibge, zona: b.Zona,
		latitude: textoFloat(b.Latitude), longitude: textoFloat(b.Longitude), altitude: textoFloat(b.Altitude),
		distancia: textoFloat(b.DistanciaCentro), responsaveis: resp,
	}
}

func lerBairro(r *http.Request) dadosBairro {
	cid, _ := strconv.ParseInt(r.PostFormValue("cidade_id"), 10, 64)
	d := dadosBairro{
		cidadeID: cid, nome: strings.TrimSpace(r.PostFormValue("nome")),
		mutirao: r.PostFormValue("mutirao") == "1", producao: r.PostFormValue("producao") == "1",
		status: r.PostFormValue("status"), classificacao: r.PostFormValue("classificacao"), categoria: r.PostFormValue("categoria"),
		ibge: strings.TrimSpace(r.PostFormValue("ibge")), zona: strings.TrimSpace(r.PostFormValue("zona")),
		latitude: strings.TrimSpace(r.PostFormValue("latitude")), longitude: strings.TrimSpace(r.PostFormValue("longitude")),
		altitude: strings.TrimSpace(r.PostFormValue("altitude")), distancia: strings.TrimSpace(r.PostFormValue("distancia_centro")),
	}
	for _, s := range r.PostForm["responsavel"] {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 && !slices.Contains(d.responsaveis, n) {
			d.responsaveis = append(d.responsaveis, n)
		}
	}
	return d
}

// numeroOpcional lê um número decimal opcional (aceita vírgula) dentro de uma faixa.
func numeroOpcional(s, nome string, min, max float64) (pgtype.Float8, string) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return pgtype.Float8{}, ""
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return pgtype.Float8{}, nome + " inválida: use um número, como -13.8247."
	}
	if v < min || v > max {
		return pgtype.Float8{}, fmt.Sprintf("%s fora do limite (de %g a %g).", nome, min, max)
	}
	return pgtype.Float8{Float64: v, Valid: true}, ""
}

// validarBairro confere os campos e devolve os parâmetros prontos para gravar, ou a mensagem de erro.
func validarBairro(d dadosBairro, cidadesAtivas []sqlccliente.Cidade, cidadeAtual int64) (sqlccliente.CreateBairroParams, string) {
	var p sqlccliente.CreateBairroParams
	switch {
	case len([]rune(d.nome)) < 2 || len([]rune(d.nome)) > 120:
		return p, "Informe o nome do bairro (de 2 a 120 caracteres)."
	case !slices.Contains([]string{"ativo", "extinto"}, d.status):
		return p, "Situação inválida."
	case !slices.Contains([]string{"urbano", "rural"}, d.classificacao):
		return p, "Classificação inválida."
	case !slices.Contains([]string{"bairro", "povoado"}, d.categoria):
		return p, "Categoria inválida."
	case len([]rune(d.zona)) > 30:
		return p, "A zona pode ter no máximo 30 caracteres."
	case len([]rune(d.ibge)) > 20:
		return p, "O código IBGE pode ter no máximo 20 caracteres."
	}
	cidadeOK := false
	for _, c := range cidadesAtivas {
		cidadeOK = cidadeOK || c.ID == d.cidadeID
	}
	if !cidadeOK && d.cidadeID != cidadeAtual { // uma cidade inativa só vale se o bairro já estava nela
		return p, "Escolha uma cidade ativa."
	}
	var msg string
	lat, msg := numeroOpcional(d.latitude, "Latitude", -90, 90)
	if msg != "" {
		return p, msg
	}
	lon, msg := numeroOpcional(d.longitude, "Longitude", -180, 180)
	if msg != "" {
		return p, msg
	}
	alt, msg := numeroOpcional(d.altitude, "Altitude", -500, 9000)
	if msg != "" {
		return p, msg
	}
	dist, msg := numeroOpcional(d.distancia, "Distância do centro", 0, 100000)
	if msg != "" {
		return p, msg
	}
	if lat.Valid != lon.Valid {
		return p, "Informe latitude e longitude juntas, ou deixe as duas em branco."
	}
	return sqlccliente.CreateBairroParams{
		CidadeID: d.cidadeID, Nome: d.nome, Mutirao: d.mutirao, Status: d.status, Ibge: d.ibge, Classificacao: d.classificacao,
		Zona: d.zona, Categoria: d.categoria, Latitude: lat, Longitude: lon, Altitude: alt, DistanciaCentro: dist, Producao: d.producao,
	}, ""
}

// montarFormBairro monta o formulário (do banco ou do que a pessoa digitou) com cidades, responsáveis e quarteirões.
func (h *handlers) montarFormBairro(ctx context.Context, r *http.Request, q *sqlccliente.Queries, id int64, d dadosBairro, aviso, erro string) (views.BairroFormData, error) {
	cidades, err := q.ListCidadesAtivas(ctx)
	if err != nil {
		return views.BairroFormData{}, err
	}
	usuarios, err := q.ListUsuariosAtivosParaResponsavel(ctx)
	if err != nil {
		return views.BairroFormData{}, err
	}
	resp := make([]views.ResponsavelOpcao, 0, len(usuarios))
	for _, u := range usuarios {
		resp = append(resp, views.ResponsavelOpcao{ID: u.ID, Nome: u.Nome, Email: u.Email, Marcado: slices.Contains(d.responsaveis, u.ID)})
	}
	form := views.BairroFormData{
		ID: id, Cidades: opcoesDeCidade(cidades, d.cidadeID), Nome: d.nome, Mutirao: d.mutirao, Producao: d.producao,
		Status: d.status, Classificacao: d.classificacao, Categoria: d.categoria, Ibge: d.ibge, Zona: d.zona,
		Latitude: d.latitude, Longitude: d.longitude, Altitude: d.altitude, Distancia: d.distancia, Responsaveis: resp,
		PodeEditar: h.principal(r).Pode("bairro_adicionar"), Aviso: aviso, Erro: erro, CSRF: csrfToken(ctx),
	}
	if id != 0 {
		qs, err := q.ListQuarteiroesDoBairro(ctx, id)
		if err != nil {
			return form, err
		}
		for _, x := range qs {
			form.Quarteiroes = append(form.Quarteiroes, views.QuarteiraoLinha{ID: x.ID, Nome: x.Nome, Ativo: x.Ativo})
		}
	}
	return form, nil
}

func (h *handlers) bairroNovoForm(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := dadosBairro{producao: true, status: "ativo", classificacao: "urbano", categoria: "bairro"}
	if mu := meuID(r); mu != 0 && h.somenteUsuario(r).Valid {
		d.responsaveis = []int64{mu} // quem só vê os próprios bairros começa como responsável
	}
	form, err := h.montarFormBairro(r.Context(), r, q, 0, d, "", "")
	if err != nil {
		h.falhaInterna(w, r, "montar formulário", err)
		return
	}
	render(w, r, views.BairroForm(h.layout(r, "Novo bairro", "/bairros"), form))
}

func (h *handlers) bairroEditarForm(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	b, err := q.GetBairro(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar bairro", err)
		return
	}
	if visivel, err := h.bairroVisivel(ctx, r, q, id); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return
	} else if !visivel {
		http.NotFound(w, r)
		return
	}
	resp, err := q.ListResponsaveisDoBairro(ctx, id)
	if err != nil {
		h.falhaInterna(w, r, "responsáveis", err)
		return
	}
	aviso := ""
	if r.URL.Query().Has("criados") {
		c, _ := strconv.Atoi(r.URL.Query().Get("criados"))
		e, _ := strconv.Atoi(r.URL.Query().Get("existiam"))
		aviso = fmt.Sprintf("%d quarteirão(ões) criado(s)", c)
		if e > 0 {
			aviso += fmt.Sprintf(" e %d já existia(m)", e)
		}
		aviso += "."
	}
	form, err := h.montarFormBairro(ctx, r, q, id, dadosDoBairro(b, resp), aviso, "")
	if err != nil {
		h.falhaInterna(w, r, "montar formulário", err)
		return
	}
	render(w, r, views.BairroForm(h.layout(r, "Bairro", "/bairros"), form))
}

func dadosAuditoriaBairro(p sqlccliente.CreateBairroParams, cidade string) map[string]any {
	f := func(v pgtype.Float8) any {
		if !v.Valid {
			return nil
		}
		return v.Float64
	}
	return map[string]any{
		"nome": p.Nome, "cidade": cidade, "mutirao": p.Mutirao, "status": p.Status, "ibge": p.Ibge, "classificacao": p.Classificacao,
		"zona": p.Zona, "categoria": p.Categoria, "latitude": f(p.Latitude), "longitude": f(p.Longitude), "altitude": f(p.Altitude),
		"distancia_centro": f(p.DistanciaCentro), "producao": p.Producao,
	}
}

func (h *handlers) bairroSalvar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	var id int64
	if chi.URLParam(r, "id") != "" {
		var ok bool
		if id, ok = idDaURL(r); !ok {
			http.NotFound(w, r)
			return
		}
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}

	var atual sqlccliente.Bairro
	var respAntes []int64
	if id != 0 {
		if atual, err = q.GetBairro(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar bairro", err)
			return
		}
		if visivel, err := h.bairroVisivel(ctx, r, q, id); err != nil {
			h.falhaInterna(w, r, "escopo do bairro", err)
			return
		} else if !visivel {
			http.NotFound(w, r)
			return
		}
		if respAntes, err = q.ListResponsaveisDoBairro(ctx, id); err != nil {
			h.falhaInterna(w, r, "responsáveis", err)
			return
		}
	}

	d := lerBairro(r)
	falha := func(msg string) {
		form, e := h.montarFormBairro(ctx, r, q, id, d, "", msg)
		if e != nil {
			h.falhaInterna(w, r, "montar formulário", e)
			return
		}
		renderStatus(w, r, http.StatusBadRequest, views.BairroForm(h.layout(r, "Bairro", "/bairros"), form))
	}

	cidades, err := q.ListCidadesAtivas(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar cidades", err)
		return
	}
	params, msg := validarBairro(d, cidades, atual.CidadeID)
	if msg != "" {
		falha(msg)
		return
	}
	usuarios, err := q.ListUsuariosAtivosParaResponsavel(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar usuários", err)
		return
	}
	nomes := map[int64]string{}
	for _, u := range usuarios {
		nomes[u.ID] = u.Nome
	}
	for _, rid := range d.responsaveis {
		// um responsável que já estava no bairro pode ter sido desativado depois: mantém
		if _, ativo := nomes[rid]; !ativo && !slices.Contains(respAntes, rid) {
			falha("Escolha apenas usuários ativos como responsáveis.")
			return
		}
	}

	var nomeCidade string
	if c, err := q.GetCidade(ctx, params.CidadeID); err == nil {
		nomeCidade = c.Nome + "/" + c.Uf
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)

	novoID := id
	if id == 0 {
		b, err := qt.CreateBairro(ctx, params)
		if violacaoUnica(err) {
			falha("Já existe um bairro com esse nome nessa cidade.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "criar bairro", err)
			return
		}
		novoID = b.ID
	} else if _, err := qt.UpdateBairro(ctx, sqlccliente.UpdateBairroParams{
		ID: id, CidadeID: params.CidadeID, Nome: params.Nome, Mutirao: params.Mutirao, Status: params.Status, Ibge: params.Ibge,
		Classificacao: params.Classificacao, Zona: params.Zona, Categoria: params.Categoria, Latitude: params.Latitude,
		Longitude: params.Longitude, Altitude: params.Altitude, DistanciaCentro: params.DistanciaCentro, Producao: params.Producao,
	}); violacaoUnica(err) {
		falha("Já existe um bairro com esse nome nessa cidade.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "salvar bairro", err)
		return
	}
	if err := qt.ClearResponsaveisDoBairro(ctx, novoID); err != nil {
		h.falhaInterna(w, r, "limpar responsáveis", err)
		return
	}
	if len(d.responsaveis) > 0 {
		if err := qt.AddResponsaveisAoBairro(ctx, sqlccliente.AddResponsaveisAoBairroParams{BairroID: novoID, Usuarios: d.responsaveis}); err != nil {
			h.falhaInterna(w, r, "gravar responsáveis", err)
			return
		}
	}

	// Auditoria na mesma transação: o que mudou nos campos e quem entrou ou saiu da responsabilidade.
	nomeDe := func(ids []int64) []string {
		out := make([]string, 0, len(ids))
		for _, x := range ids {
			if n, ok := nomes[x]; ok {
				out = append(out, n)
			} else {
				out = append(out, "usuário nº "+strconv.FormatInt(x, 10))
			}
		}
		slices.Sort(out)
		return out
	}
	var entrou, saiu []int64
	for _, x := range d.responsaveis {
		if !slices.Contains(respAntes, x) {
			entrou = append(entrou, x)
		}
	}
	for _, x := range respAntes {
		if !slices.Contains(d.responsaveis, x) {
			saiu = append(saiu, x)
		}
	}
	depois := dadosAuditoriaBairro(params, nomeCidade)
	ev := audit.Evento{
		Entidade: "bairro", EntidadeID: strconv.FormatInt(novoID, 10), Rotulo: params.Nome,
		Detalhes: map[string]any{"responsaveis_adicionados": nomeDe(entrou), "responsaveis_removidos": nomeDe(saiu)},
	}
	if id == 0 {
		ev.Acao, ev.Depois = "bairro.criar", depois
	} else {
		var cidadeAntes string
		if c, err := q.GetCidade(ctx, atual.CidadeID); err == nil {
			cidadeAntes = c.Nome + "/" + c.Uf
		}
		a, dep := audit.Diferenca(dadosAuditoriaBairro(sqlccliente.CreateBairroParams{
			CidadeID: atual.CidadeID, Nome: atual.Nome, Mutirao: atual.Mutirao, Status: atual.Status, Ibge: atual.Ibge,
			Classificacao: atual.Classificacao, Zona: atual.Zona, Categoria: atual.Categoria, Latitude: atual.Latitude,
			Longitude: atual.Longitude, Altitude: atual.Altitude, DistanciaCentro: atual.DistanciaCentro, Producao: atual.Producao,
		}, cidadeAntes), depois)
		if len(a)+len(dep)+len(entrou)+len(saiu) == 0 {
			naoAuditar(r)
		} else {
			ev.Acao, ev.Antes, ev.Depois = "bairro.editar", a, dep
		}
	}
	if ev.Acao != "" {
		h.auditarCom(audit.Cliente{Q: qt}, r, ev)
	}
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar bairro", err)
		return
	}
	if id == 0 {
		http.Redirect(w, r, "/bairros/"+strconv.FormatInt(novoID, 10)+"#quarteiroes", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/bairros", http.StatusSeeOther)
}

// ---------------------------------------------------------------- Quarteirões

var reFaixa = regexp.MustCompile(`^([0-9]+)\s*-\s*([0-9]+)$`)

// nomesDeQuarteirao interpreta o texto do cadastro em lote: uma linha por quarteirão, ou uma faixa
// numérica ("1-20"). Devolve os nomes sem repetição, na ordem em que apareceram.
func nomesDeQuarteirao(texto string) ([]string, string) {
	const maximo = 500
	var out []string
	vistos := map[string]bool{}
	add := func(n string) string {
		if vistos[strings.ToLower(n)] {
			return ""
		}
		vistos[strings.ToLower(n)] = true
		out = append(out, n)
		if len(out) > maximo {
			return fmt.Sprintf("Muitos quarteirões de uma vez: o limite é %d por vez.", maximo)
		}
		return ""
	}
	for _, linha := range strings.Split(strings.ReplaceAll(texto, "\r\n", "\n"), "\n") {
		linha = strings.TrimSpace(linha)
		if linha == "" {
			continue
		}
		if m := reFaixa.FindStringSubmatch(linha); m != nil {
			de, _ := strconv.Atoi(m[1])
			ate, _ := strconv.Atoi(m[2])
			if de > ate || ate-de >= maximo {
				return nil, fmt.Sprintf("Faixa inválida: %q (use do menor para o maior, até %d números).", linha, maximo)
			}
			for n := de; n <= ate; n++ {
				if msg := add(strconv.Itoa(n)); msg != "" {
					return nil, msg
				}
			}
			continue
		}
		if len([]rune(linha)) > 30 {
			return nil, fmt.Sprintf("O nome do quarteirão pode ter no máximo 30 caracteres: %q.", linha)
		}
		if msg := add(linha); msg != "" {
			return nil, msg
		}
	}
	if len(out) == 0 {
		return nil, "Informe pelo menos um quarteirão."
	}
	return out, ""
}

func (h *handlers) quarteiraoCriar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	b, err := q.GetBairro(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar bairro", err)
		return
	}
	if visivel, err := h.bairroVisivel(ctx, r, q, id); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return
	} else if !visivel {
		http.NotFound(w, r)
		return
	}

	nomes, msg := nomesDeQuarteirao(r.PostFormValue("nomes"))
	if msg != "" {
		resp, _ := q.ListResponsaveisDoBairro(ctx, id)
		form, e := h.montarFormBairro(ctx, r, q, id, dadosDoBairro(b, resp), "", msg)
		if e != nil {
			h.falhaInterna(w, r, "montar formulário", e)
			return
		}
		renderStatus(w, r, http.StatusBadRequest, views.BairroForm(h.layout(r, "Bairro", "/bairros"), form))
		return
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	criados := 0
	var novos []string
	for _, n := range nomes {
		c, err := qt.CreateQuarteirao(ctx, sqlccliente.CreateQuarteiraoParams{BairroID: id, Nome: n})
		if err != nil {
			h.falhaInterna(w, r, "criar quarteirão", err)
			return
		}
		if c > 0 {
			criados++
			novos = append(novos, n)
		}
	}
	if criados > 0 {
		amostra := novos
		if len(amostra) > 50 {
			amostra = amostra[:50]
		}
		h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{
			Acao: "quarteirao.criar", Entidade: "bairro", EntidadeID: strconv.FormatInt(id, 10), Rotulo: b.Nome,
			Detalhes: map[string]any{"criados": criados, "ja_existiam": len(nomes) - criados, "quarteiroes": amostra},
		})
	} else {
		naoAuditar(r)
	}
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar quarteirões", err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/bairros/%d?criados=%d&existiam=%d#quarteiroes", id, criados, len(nomes)-criados), http.StatusSeeOther)
}

func (h *handlers) quarteiraoAtivo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := idDaURL(r)
	qid, e := strconv.ParseInt(chi.URLParam(r, "qid"), 10, 64)
	if !ok || e != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	ativo := r.PostFormValue("ativo") == "1"
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if visivel, err := h.bairroVisivel(ctx, r, q, id); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return
	} else if !visivel {
		http.NotFound(w, r)
		return
	}
	quart, err := q.GetQuarteirao(ctx, qid)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && quart.BairroID != id) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar quarteirão", err)
		return
	}
	if quart.Ativo == ativo {
		naoAuditar(r)
		http.Redirect(w, r, fmt.Sprintf("/bairros/%d#quarteiroes", id), http.StatusSeeOther)
		return
	}
	if _, err := q.SetQuarteiraoAtivo(ctx, sqlccliente.SetQuarteiraoAtivoParams{ID: qid, BairroID: id, Ativo: ativo}); err != nil {
		h.falhaInterna(w, r, "mudar quarteirão", err)
		return
	}
	bairro, _ := q.GetBairro(ctx, id)
	h.auditar(r, audit.Evento{
		Acao: "quarteirao.status", Entidade: "quarteirao", EntidadeID: strconv.FormatInt(qid, 10), Rotulo: bairro.Nome + " · quarteirão " + quart.Nome,
		Antes: map[string]any{"ativo": quart.Ativo}, Depois: map[string]any{"ativo": ativo},
	})
	http.Redirect(w, r, fmt.Sprintf("/bairros/%d#quarteiroes", id), http.StatusSeeOther)
}

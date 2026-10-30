package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Banners (12.4): imagem de fundo (ou cor) com textos posicionados, montada no editor visual da tela.
// A definição dos textos é um JSON validado aqui; a posição é em % do banner e o tamanho em pixels do banner.

const (
	maxFundoBanner     = 2 << 20
	maxElementosBanner = 30
)

var (
	corHex          = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	fontesBanner    = map[string]bool{"sans": true, "serif": true, "mono": true}
	alinhamentosBan = map[string]bool{"left": true, "center": true, "right": true}
	rotulosSituBan  = []views.CidadeOpcao{{Valor: "ativo", Rotulo: "Ativo"}, {Valor: "inativo", Rotulo: "Inativo"}}
)

type bannerElemento struct {
	Texto   string  `json:"texto"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Fonte   string  `json:"fonte"`
	Tamanho int     `json:"tamanho"`
	Cor     string  `json:"cor"`
	Negrito bool    `json:"negrito"`
	Sombra  bool    `json:"sombra"`
	Alinhar string  `json:"alinhar"`
}

// lerElementosBanner valida a definição enviada pela tela e devolve o JSON normalizado.
func lerElementosBanner(bruto string) ([]byte, string) {
	bruto = strings.TrimSpace(bruto)
	if bruto == "" {
		bruto = "[]"
	}
	if len(bruto) > 40000 {
		return nil, "A definição do banner é grande demais."
	}
	var els []bannerElemento
	if err := json.Unmarshal([]byte(bruto), &els); err != nil {
		return nil, "A definição dos textos do banner é inválida."
	}
	out := make([]bannerElemento, 0, len(els))
	for _, e := range els {
		e.Texto = strings.TrimSpace(e.Texto)
		if e.Texto == "" {
			continue
		}
		switch {
		case len([]rune(e.Texto)) > 300:
			return nil, "Cada texto do banner aceita até 300 caracteres."
		case e.X < 0 || e.X > 100 || e.Y < 0 || e.Y > 100:
			return nil, "Um texto está fora do banner."
		case !fontesBanner[e.Fonte]:
			return nil, "Fonte inválida."
		case e.Tamanho < 8 || e.Tamanho > 600:
			return nil, "O tamanho do texto deve ficar entre 8 e 600."
		case !corHex.MatchString(e.Cor):
			return nil, "Cor do texto inválida."
		case !alinhamentosBan[e.Alinhar]:
			return nil, "Alinhamento inválido."
		}
		out = append(out, e)
	}
	if len(out) > maxElementosBanner {
		return nil, "Um banner aceita até " + strconv.Itoa(maxElementosBanner) + " textos."
	}
	b, _ := json.Marshal(out)
	return b, ""
}

func bannerCaminho(id int64) string { return "/banners/" + strconv.FormatInt(id, 10) }

func (h *handlers) banners(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.BannerFiltro{Busca: strings.TrimSpace(qs.Get("busca")), Situacao: qs.Get("situacao")}
	if !validoEm(rotulosSituBan, f.Situacao) {
		f.Situacao = ""
	}
	ordem, dir := ordemDeEstoque(qs.Get("ordem"), qs.Get("dir"), "id", "nome")
	pag := paginaDe(r)
	total, err := q.CountBanners(ctx, sqlccliente.CountBannersParams{Busca: f.Busca, Situacao: f.Situacao})
	if err != nil {
		h.falhaInterna(w, r, "contar banners", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListBanners(ctx, sqlccliente.ListBannersParams{
		Busca: f.Busca, Situacao: f.Situacao, Ordem: ordem, Dir: dir,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar banners", err)
		return
	}
	d := views.BannersData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNovo: h.principal(r).Pode("banner_adicionar"), Situacoes: marcarOpcao(rotulosSituBan, f.Situacao)}
	if r.URL.Query().Get("aviso") == "excluido" {
		d.Aviso = "Banner enviado para a lixeira."
	}
	loc := h.fuso(r)
	for _, x := range rows {
		fundo := "Cor lisa"
		if x.ComImagem {
			fundo = "Imagem"
		}
		d.Linhas = append(d.Linhas, views.BannerLinha{
			IDTexto: strconv.FormatInt(x.ID, 10), Nome: x.Nome, Tamanho: strconv.Itoa(int(x.Largura)) + " × " + strconv.Itoa(int(x.Altura)), Fundo: fundo,
			Situacao: rotuloDe(rotulosSituBan, map[bool]string{true: "ativo", false: "inativo"}[x.Ativo]), Atualizado: x.AtualizadoEm.Time.In(loc).Format("02/01/2006 15:04"),
		})
	}
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "situacao": f.Situacao} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Banners(h.layout(r, "Banners", "/banners"), d))
}

func (h *handlers) bannerNovo(w http.ResponseWriter, r *http.Request) {
	d := views.BannerForm{Largura: "1080", Altura: "1080", CorFundo: "#155a9e", Ativo: true, Definicao: "[]", PodeEditar: true, CSRF: csrfToken(r.Context())}
	render(w, r, views.BannerFicha(h.layout(r, "Novo banner", "/banners"), d))
}

type bannerLido struct {
	form                views.BannerForm
	largura, altura     int32
	definicao, fundo    []byte
	tipoFundo           string
	removerFundo, ativo bool
}

// lerBanner lê o formulário (multipart, com a imagem de fundo opcional).
func lerBanner(w http.ResponseWriter, r *http.Request) (bannerLido, string) {
	var b bannerLido
	r.Body = http.MaxBytesReader(w, r.Body, maxFundoBanner+1<<20)
	if err := r.ParseMultipartForm(4 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		b.form = views.BannerForm{PodeEditar: true, Definicao: "[]"}
		return b, "O envio é grande demais ou está incompleto (a imagem de fundo pode ter até 2 MB)."
	}
	b.ativo, b.removerFundo = r.PostFormValue("ativo") == "1", r.PostFormValue("remover_fundo") == "1"
	b.form = views.BannerForm{
		Nome: strings.TrimSpace(r.PostFormValue("nome")), Largura: strings.TrimSpace(r.PostFormValue("largura")), Altura: strings.TrimSpace(r.PostFormValue("altura")),
		CorFundo: strings.TrimSpace(r.PostFormValue("cor_fundo")), Ativo: b.ativo, Definicao: r.PostFormValue("definicao"), PodeEditar: true,
	}
	l, e1 := strconv.Atoi(b.form.Largura)
	a, e2 := strconv.Atoi(b.form.Altura)
	switch {
	case b.form.Nome == "" || len([]rune(b.form.Nome)) > 100:
		return b, "Informe o nome do banner (até 100 caracteres)."
	case e1 != nil || e2 != nil || l < 100 || l > 4000 || a < 100 || a > 4000:
		return b, "Largura e altura devem ficar entre 100 e 4000."
	case !corHex.MatchString(b.form.CorFundo):
		return b, "Escolha a cor de fundo."
	}
	b.largura, b.altura = int32(l), int32(a)
	var erro string
	if b.definicao, erro = lerElementosBanner(b.form.Definicao); erro != "" {
		return b, erro
	}
	b.form.Definicao = string(b.definicao)
	if r.MultipartForm == nil {
		return b, ""
	}
	if arqs := r.MultipartForm.File["fundo"]; len(arqs) > 0 {
		f, err := arqs[0].Open()
		if err != nil {
			return b, "Não foi possível ler a imagem de fundo."
		}
		defer f.Close()
		dados, err := io.ReadAll(io.LimitReader(f, maxFundoBanner+1))
		if err != nil || len(dados) == 0 {
			return b, "Não foi possível ler a imagem de fundo."
		}
		if len(dados) > maxFundoBanner {
			return b, "A imagem de fundo passa de 2 MB."
		}
		if tipo := http.DetectContentType(dados); tiposDeFoto[tipo] {
			b.fundo, b.tipoFundo = dados, tipo
		} else {
			return b, "A imagem de fundo deve ser JPEG, PNG ou WebP."
		}
	}
	return b, ""
}

func (h *handlers) bannerCriar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	b, erro := lerBanner(w, r)
	if erro == "" {
		tx, e := pool.Begin(ctx)
		if e != nil {
			h.falhaInterna(w, r, "iniciar cadastro de banner", e)
			return
		}
		defer tx.Rollback(ctx)
		qt := q.WithTx(tx)
		x, e := qt.CreateBanner(ctx, sqlccliente.CreateBannerParams{Nome: b.form.Nome, Largura: b.largura, Altura: b.altura, CorFundo: b.form.CorFundo, Definicao: b.definicao, Ativo: b.ativo, CriadoPor: idOuNulo(meuID(r))})
		if e == nil && b.fundo != nil {
			_, e = qt.SetBannerFundo(ctx, sqlccliente.SetBannerFundoParams{ID: x.ID, FundoTipo: b.tipoFundo, Fundo: b.fundo})
		}
		if e == nil {
			h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "banner.criar", Entidade: "banner", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: x.Nome,
				Depois: map[string]any{"nome": x.Nome, "largura": x.Largura, "altura": x.Altura, "com_imagem": b.fundo != nil}})
			if e = tx.Commit(ctx); e == nil {
				http.Redirect(w, r, bannerCaminho(x.ID)+"?aviso=criado", http.StatusSeeOther)
				return
			}
		}
		if !violacaoUnica(e) {
			h.falhaInterna(w, r, "criar banner", e)
			return
		}
		erro = "Já existe um banner com esse nome."
	}
	d := b.form
	d.CSRF, d.Erro = csrfToken(ctx), erro
	if d.Definicao == "" {
		d.Definicao = "[]"
	}
	renderStatus(w, r, http.StatusBadRequest, views.BannerFicha(h.layout(r, "Novo banner", "/banners"), d))
}

func (h *handlers) carregarBanner(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.GetBannerRow, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.GetBannerRow{}, false
	}
	x, err := q.GetBanner(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return x, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar banner", err)
		return x, false
	}
	return x, true
}

func (h *handlers) renderBanner(w http.ResponseWriter, r *http.Request, x sqlccliente.GetBannerRow, enviado *bannerLido, erro string) {
	pr := h.principal(r)
	d := views.BannerForm{
		ID: x.ID, Nome: x.Nome, Largura: strconv.Itoa(int(x.Largura)), Altura: strconv.Itoa(int(x.Altura)), CorFundo: x.CorFundo, Ativo: x.Ativo,
		Definicao: string(x.Definicao), TemFundo: x.FundoTipo != "", PodeEditar: pr.Pode("banner_editar"), PodeExcluir: pr.Pode("banner_excluir"),
		CSRF: csrfToken(r.Context()), Erro: erro,
	}
	if enviado != nil {
		d.Nome, d.Largura, d.Altura, d.CorFundo, d.Ativo = enviado.form.Nome, enviado.form.Largura, enviado.form.Altura, enviado.form.CorFundo, enviado.ativo
		if enviado.definicao != nil {
			d.Definicao = string(enviado.definicao)
		}
	}
	switch r.URL.Query().Get("aviso") {
	case "criado":
		d.Aviso = "Banner criado."
	case "salvo":
		d.Aviso = "Banner salvo."
	}
	status := http.StatusOK
	if erro != "" {
		status = http.StatusBadRequest
	}
	renderStatus(w, r, status, views.BannerFicha(h.layout(r, x.Nome, "/banners"), d))
}

func (h *handlers) bannerFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if x, ok := h.carregarBanner(w, r, q); ok {
		h.renderBanner(w, r, x, nil, "")
	}
}

func (h *handlers) bannerSalvar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	x, ok := h.carregarBanner(w, r, q)
	if !ok {
		return
	}
	b, erro := lerBanner(w, r)
	if erro != "" {
		h.renderBanner(w, r, x, &b, erro)
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar alteração de banner", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := q.WithTx(tx)
	_, err = qt.UpdateBanner(ctx, sqlccliente.UpdateBannerParams{ID: x.ID, Nome: b.form.Nome, Largura: b.largura, Altura: b.altura, CorFundo: b.form.CorFundo, Definicao: b.definicao, Ativo: b.ativo})
	if err == nil {
		switch {
		case b.fundo != nil:
			_, err = qt.SetBannerFundo(ctx, sqlccliente.SetBannerFundoParams{ID: x.ID, FundoTipo: b.tipoFundo, Fundo: b.fundo})
		case b.removerFundo:
			_, err = qt.ClearBannerFundo(ctx, x.ID)
		}
	}
	if err != nil {
		if violacaoUnica(err) {
			h.renderBanner(w, r, x, &b, "Já existe um banner com esse nome.")
			return
		}
		h.falhaInterna(w, r, "salvar banner", err)
		return
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{Acao: "banner.editar", Entidade: "banner", EntidadeID: strconv.FormatInt(x.ID, 10), Rotulo: b.form.Nome,
		Antes:  map[string]any{"nome": x.Nome, "largura": x.Largura, "altura": x.Altura, "ativo": x.Ativo},
		Depois: map[string]any{"nome": b.form.Nome, "largura": b.largura, "altura": b.altura, "ativo": b.ativo, "imagem_trocada": b.fundo != nil, "imagem_removida": b.removerFundo}})
	if err = tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "gravar banner", err)
		return
	}
	http.Redirect(w, r, bannerCaminho(x.ID)+"?aviso=salvo", http.StatusSeeOther)
}

func (h *handlers) bannerFundo(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := q.GetBannerFundo(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", f.FundoTipo)
	w.Header().Set("Content-Length", strconv.Itoa(len(f.Fundo)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "private, no-cache")
	_, _ = w.Write(f.Fundo)
}

func (h *handlers) bannerExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if x, ok := h.carregarBanner(w, r, q); ok {
		h.excluirRegistro(w, r, "banner", x.ID, "/banners?aviso=excluido", bannerCaminho(x.ID))
	}
}

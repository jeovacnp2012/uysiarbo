package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/guia"
	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

// ---------------------------------------------------------- versão atual (cache)

type versaoAtual struct {
	id   int64
	nome string
	ate  time.Time
}

// ultimaVersao devolve a versão mais recente das Novidades. Fica em memória por 30 segundos, porque
// toda página precisa dela (rodapé do menu e aviso de versão nova).
func (h *handlers) ultimaVersao(ctx context.Context) (int64, string) {
	if h.pub == nil {
		return 0, ""
	}
	h.vmu.Lock()
	defer h.vmu.Unlock()
	if time.Now().Before(h.vatual.ate) {
		return h.vatual.id, h.vatual.nome
	}
	v, err := h.pub.GetUltimaVersao(ctx)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.log.Error("última versão", "erro", err)
		}
		return h.vatual.id, h.vatual.nome
	}
	h.vatual = versaoAtual{id: v.ID, nome: v.Versao, ate: time.Now().Add(30 * time.Second)}
	return v.ID, v.Versao
}

func (h *handlers) limparVersao() {
	h.vmu.Lock()
	h.vatual.ate = time.Time{}
	h.vmu.Unlock()
}

// ----------------------------------------------------------------------- Guia

func (h *handlers) guiaVisiveis(r *http.Request) []guia.Artigo {
	pr := h.principal(r)
	return guia.Padrao.Visiveis(pr.Master, pr.Pode)
}

func (h *handlers) guiaLista(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var itens []views.GuiaItem
	for _, res := range guia.Buscar(h.guiaVisiveis(r), q) {
		itens = append(itens, views.GuiaItem{Slug: res.Artigo.Slug, Titulo: res.Artigo.Titulo, Resumo: res.Artigo.Resumo, Trecho: res.Trecho})
	}
	render(w, r, views.GuiaLista(h.layout(r, "Guia do sistema", "/guia"), views.GuiaListaData{Q: q, Itens: itens}))
}

func (h *handlers) guiaArtigo(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var outros []views.GuiaItem
	var achado *guia.Artigo
	for _, a := range h.guiaVisiveis(r) {
		outros = append(outros, views.GuiaItem{Slug: a.Slug, Titulo: a.Titulo})
		if a.Slug == slug {
			a := a
			achado = &a
		}
	}
	if achado == nil { // inexistente ou sem permissão: não revela a diferença
		http.NotFound(w, r)
		return
	}
	render(w, r, views.GuiaArtigo(h.layout(r, achado.Titulo, "/guia"), views.GuiaArtigoData{
		Slug: achado.Slug, Titulo: achado.Titulo, HTML: achado.HTML, Outros: outros,
	}))
}

// ------------------------------------------------------------------- Novidades

func (h *handlers) novidades(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vs, err := h.pub.ListVersoes(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar versões", err)
		return
	}
	itens, err := h.pub.ListVersaoItens(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar itens das versões", err)
		return
	}
	porVersao := map[int64][]sqlc.VersaoIten{}
	for _, it := range itens {
		porVersao[it.VersaoID] = append(porVersao[it.VersaoID], it)
	}

	a, _ := authFrom(ctx)
	visto := int64(0)
	if a.sess != nil {
		visto = a.sess.Conta.NovidadeVista
	}
	var lista []views.VersaoView
	for _, v := range vs {
		vv := views.VersaoView{Versao: v.Versao, Titulo: v.Titulo, Data: v.PublicadaEm.Time.Format("02/01/2006"), Nova: v.ID > visto}
		for _, it := range porVersao[v.ID] {
			switch it.Tipo {
			case "novo":
				vv.Novos = append(vv.Novos, it.Texto)
			case "melhorado":
				vv.Melhorados = append(vv.Melhorados, it.Texto)
			case "corrigido":
				vv.Corrigidos = append(vv.Corrigidos, it.Texto)
			}
		}
		lista = append(lista, vv)
	}
	// Quem abriu a página viu tudo: o aviso some (já nesta página).
	if a.sess != nil && len(vs) > 0 {
		if err := a.svc.MarcarNovidadeVista(ctx, a.sess, vs[0].ID); err != nil {
			h.log.Error("marcar novidade vista", "erro", err)
		}
	}
	_, ehCliente := tenant.FromContext(ctx)
	render(w, r, views.Novidades(h.layout(r, "Novidades", "/novidades"), views.NovidadesData{
		Versoes: lista, PodeRegistrar: !ehCliente && h.principal(r).Master && h.tenants != nil,
	}))
}

var reVersao = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]{3}$`)

// proximaVersao sugere o número seguinte ao da última versão (1.0.004 vira 1.0.005).
func proximaVersao(atual string) string {
	partes := strings.Split(atual, ".")
	if len(partes) != 3 {
		return "1.0.001"
	}
	n, err := strconv.Atoi(partes[2])
	if err != nil {
		return "1.0.001"
	}
	return fmt.Sprintf("%s.%s.%03d", partes[0], partes[1], n+1)
}

func (h *handlers) versaoNovaForm(w http.ResponseWriter, r *http.Request) {
	_, atual := h.ultimaVersao(r.Context())
	render(w, r, views.VersaoForm(h.layout(r, "Registrar versão", "/novidades"), views.VersaoFormData{
		Versao: proximaVersao(atual), Data: time.Now().In(fusoLocal).Format("2006-01-02"), CSRF: csrfToken(r.Context()),
	}))
}

// linhasDe separa um texto em frases (uma por linha), sem linhas vazias.
func linhasDe(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func (h *handlers) versaoCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	d := views.VersaoFormData{
		Versao: strings.TrimSpace(r.PostFormValue("versao")), Titulo: strings.TrimSpace(r.PostFormValue("titulo")),
		Data: r.PostFormValue("data"), Novo: r.PostFormValue("novo"), Melhorado: r.PostFormValue("melhorado"),
		Corrigido: r.PostFormValue("corrigido"), CSRF: csrfToken(r.Context()),
	}
	falha := func(msg string) {
		d.Erro = msg
		renderStatus(w, r, http.StatusBadRequest, views.VersaoForm(h.layout(r, "Registrar versão", "/novidades"), d))
	}
	data, err := time.Parse("2006-01-02", d.Data)
	tipos := []struct {
		tipo   string
		linhas []string
	}{{"novo", linhasDe(d.Novo)}, {"melhorado", linhasDe(d.Melhorado)}, {"corrigido", linhasDe(d.Corrigido)}}
	total := 0
	for _, t := range tipos {
		total += len(t.linhas)
		for _, l := range t.linhas {
			if len([]rune(l)) > 500 {
				falha("Cada frase pode ter no máximo 500 caracteres.")
				return
			}
		}
	}
	switch {
	case !reVersao.MatchString(d.Versao):
		falha("Número de versão inválido. Use o formato 1.0.005.")
		return
	case d.Titulo == "" || len([]rune(d.Titulo)) > 100:
		falha("Informe o título (até 100 caracteres).")
		return
	case err != nil:
		falha("Informe uma data válida.")
		return
	case total == 0:
		falha("Escreva pelo menos uma novidade (em Novo, Melhorado ou Corrigido).")
		return
	case total > 40:
		falha("Use no máximo 40 frases por versão.")
		return
	}

	a, _ := authFrom(r.Context())
	tx, err := h.tenants.Pool.Begin(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(r.Context())
	qt := sqlc.New(tx)
	v, err := qt.CreateVersao(r.Context(), sqlc.CreateVersaoParams{
		Versao: d.Versao, Titulo: d.Titulo, PublicadaEm: pgtype.Date{Time: data, Valid: true}, CriadoPor: a.sess.Conta.Nome,
	})
	if violacaoUnica(err) {
		falha("Já existe uma versão com esse número.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "criar versão", err)
		return
	}
	ordem := int32(0)
	for _, t := range tipos {
		for _, l := range t.linhas {
			ordem++
			if err := qt.AddVersaoItem(r.Context(), sqlc.AddVersaoItemParams{VersaoID: v.ID, Tipo: t.tipo, Texto: l, Ordem: ordem}); err != nil {
				h.falhaInterna(w, r, "gravar item da versão", err)
				return
			}
		}
	}
	h.auditarCom(audit.Plataforma{Q: qt}, r, audit.Evento{
		Acao: "versao.criar", Entidade: "versao", EntidadeID: v.Versao, Rotulo: v.Titulo,
		Depois:   map[string]any{"versao": v.Versao, "titulo": v.Titulo, "data": d.Data},
		Detalhes: map[string]any{"novo": len(tipos[0].linhas), "melhorado": len(tipos[1].linhas), "corrigido": len(tipos[2].linhas)},
	})
	if err := tx.Commit(r.Context()); err != nil {
		h.falhaInterna(w, r, "confirmar versão", err)
		return
	}
	h.limparVersao()
	http.Redirect(w, r, "/novidades", http.StatusSeeOther)
}

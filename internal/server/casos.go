package server

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Ficha de caso (9.1): o mesmo andamento (linha do tempo, comentários, encaminhamento e anexos) para todo tipo de
// caso. Um tipo novo (investigação, denúncia, notificação...) se registra com registrarTipoDeCaso, põe
// @views.FichaCaso(d) na própria ficha (d vem de h.fichaDoCaso) e registra suas mudanças de situação com
// h.casoEvento. As rotas /casos/{tipo}/{id}/... são genéricas e usam as permissões do tipo.

const (
	maxAnexoCaso      = 5 << 20
	maxAnexosPorEnvio = 5
	maxAnexosPorCaso  = 30
)

// tipoDeCaso descreve um tipo de caso para a ficha comum.
type tipoDeCaso struct {
	Slug       string                                                           // vai na URL: /casos/{slug}/{id}/...
	Rotulo     string                                                           // "Investigação", "Denúncia"...
	Ver        string                                                           // permissão para ver o andamento e baixar anexos
	Encaminhar string                                                           // permissão de encaminhar (vazia = a mesma de Editar)
	Editar     string                                                           // permissão para comentar, encaminhar e anexar
	Existe     func(ctx context.Context, q *sqlccliente.Queries, id int64) bool // o caso existe e a pessoa pode vê-lo
	Caminho    func(id int64) string                                            // endereço da ficha do caso (para voltar)
}

var tiposDeCaso = map[string]tipoDeCaso{}

func registrarTipoDeCaso(t tipoDeCaso) { tiposDeCaso[t.Slug] = t }

var tiposDeAnexo = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true, "application/pdf": true}

// casoEvento grava um fato na linha do tempo do caso (criação, mudança de situação...). Os tipos de caso o chamam
// quando alteram a situação: de e para são os rótulos legíveis das situações.
func (h *handlers) casoEvento(ctx context.Context, q *sqlccliente.Queries, r *http.Request, tipo string, id int64, evento, texto, de, para string, destino int64) error {
	_, err := q.CreateCasoEvento(ctx, sqlccliente.CreateCasoEventoParams{
		Tipo: tipo, CasoID: id, Evento: evento, Texto: texto,
		SituacaoDe: textoOpcional(de), SituacaoPara: textoOpcional(para),
		DestinoID: pgtype.Int8{Int64: destino, Valid: destino != 0},
		UsuarioID: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
	})
	return err
}

func textoOpcional(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// fichaDoCaso monta os dados do componente FichaCaso. acoes são os botões próprios do tipo (cada um aponta para
// uma rota do tipo que muda a situação e chama casoEvento).
func (h *handlers) fichaDoCaso(r *http.Request, q *sqlccliente.Queries, t tipoDeCaso, id int64, acoes []views.CasoAcao) (views.FichaCasoData, error) {
	ctx := r.Context()
	loc := h.fuso(r)
	d := views.FichaCasoData{
		Tipo: t.Slug, ID: id, CSRF: csrfToken(ctx), Acoes: acoes, PodeEditar: h.principal(r).Pode(t.Editar), PodeEncaminhar: h.principal(r).Pode(t.Editar) && (t.Encaminhar == "" || h.principal(r).Pode(t.Encaminhar)),
		Erro: r.URL.Query().Get("caso_erro"),
	}
	eventos, err := q.ListCasoEventos(ctx, sqlccliente.ListCasoEventosParams{Tipo: t.Slug, CasoID: id})
	if err != nil {
		return d, err
	}
	for _, e := range eventos {
		d.Eventos = append(d.Eventos, views.CasoEvento{
			Evento: e.Evento, Texto: e.Texto, Autor: e.Autor, Destino: e.Destino, De: e.SituacaoDe.String, Para: e.SituacaoPara.String,
			Quando: formatarData(loc, e.CriadoEm.Time),
		})
	}
	anexos, err := q.ListCasoAnexos(ctx, sqlccliente.ListCasoAnexosParams{Tipo: t.Slug, CasoID: id})
	if err != nil {
		return d, err
	}
	for _, a := range anexos {
		d.Anexos = append(d.Anexos, views.CasoAnexo{
			ID: a.ID, Nome: a.Nome, Tamanho: tamanhoLegivel(int(a.Tamanho)), Quando: formatarData(loc, a.CriadoEm.Time), Autor: a.Autor,
			Imagem: strings.HasPrefix(a.TipoMidia, "image/"),
		})
	}
	if resp, err := q.GetCasoResponsavel(ctx, sqlccliente.GetCasoResponsavelParams{Tipo: t.Slug, CasoID: id}); err == nil {
		d.Responsavel = resp.Nome
	}
	if d.PodeEditar {
		us, err := q.ListUsuariosAtivosParaResponsavel(ctx)
		if err != nil {
			return d, err
		}
		for _, u := range us {
			d.Usuarios = append(d.Usuarios, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome})
		}
	}
	return d, nil
}

// casoRota resolve o tipo e o caso da URL, confere a permissão do tipo e entrega o caso ao handler.
func (h *handlers) casoRota(editar bool, fn func(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, t tipoDeCaso, id int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, ok := tiposDeCaso[chi.URLParam(r, "tipo")]
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if !ok || err != nil {
			http.NotFound(w, r)
			return
		}
		perm := t.Ver
		if editar {
			perm = t.Editar
		}
		if !h.principal(r).Pode(perm) {
			http.Error(w, "Você não tem permissão para isto.", http.StatusForbidden)
			return
		}
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		if !t.Existe(r.Context(), q, id) {
			http.NotFound(w, r)
			return
		}
		fn(w, r, q, t, id)
	}
}

func voltarAoCaso(w http.ResponseWriter, r *http.Request, t tipoDeCaso, id int64, erro string) {
	destino := t.Caminho(id)
	if erro != "" {
		destino += "?caso_erro=" + url.QueryEscape(erro)
	}
	http.Redirect(w, r, destino+"#linha-do-tempo", http.StatusSeeOther)
}

func (h *handlers) casoComentar(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, t tipoDeCaso, id int64) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	texto := strings.TrimSpace(r.PostFormValue("texto"))
	if texto == "" || len([]rune(texto)) > 2000 {
		voltarAoCaso(w, r, t, id, "O comentário precisa ter de 1 a 2000 caracteres.")
		return
	}
	if err := h.casoEvento(r.Context(), q, r, t.Slug, id, "comentario", texto, "", "", 0); err != nil {
		h.falhaInterna(w, r, "gravar comentário do caso", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "caso.comentar", Entidade: t.Slug, EntidadeID: strconv.FormatInt(id, 10), Rotulo: t.Rotulo + " " + strconv.FormatInt(id, 10)})
	voltarAoCaso(w, r, t, id, "")
}

func (h *handlers) casoEncaminhar(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, t tipoDeCaso, id int64) {
	if t.Encaminhar != "" && !h.principal(r).Pode(t.Encaminhar) {
		http.Error(w, "Você não tem permissão para encaminhar.", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	destino, err := strconv.ParseInt(r.PostFormValue("destino"), 10, 64)
	texto := strings.TrimSpace(r.PostFormValue("texto"))
	if err != nil || len([]rune(texto)) > 2000 {
		voltarAoCaso(w, r, t, id, "Escolha para quem encaminhar.")
		return
	}
	u, err := q.GetUsuarioByID(r.Context(), destino)
	if err != nil || !u.Ativo {
		voltarAoCaso(w, r, t, id, "Essa pessoa não está disponível para receber o caso.")
		return
	}
	if err := h.casoEvento(r.Context(), q, r, t.Slug, id, "encaminhamento", texto, "", "", destino); err != nil {
		h.falhaInterna(w, r, "encaminhar caso", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "caso.encaminhar", Entidade: t.Slug, EntidadeID: strconv.FormatInt(id, 10), Rotulo: t.Rotulo + " " + strconv.FormatInt(id, 10),
		Depois: map[string]any{"para": u.Nome}})
	voltarAoCaso(w, r, t, id, "")
}

func (h *handlers) casoAnexar(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, t tipoDeCaso, id int64) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAnexosPorEnvio*maxAnexoCaso+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		voltarAoCaso(w, r, t, id, "O envio é grande demais ou está incompleto (cada arquivo pode ter até 5 MB).")
		return
	}
	arquivos := r.MultipartForm.File["arquivo"]
	if len(arquivos) == 0 || len(arquivos) > maxAnexosPorEnvio {
		voltarAoCaso(w, r, t, id, "Escolha de 1 a "+strconv.Itoa(maxAnexosPorEnvio)+" arquivos.")
		return
	}
	atuais, err := q.CountCasoAnexos(r.Context(), sqlccliente.CountCasoAnexosParams{Tipo: t.Slug, CasoID: id})
	if err != nil {
		h.falhaInterna(w, r, "contar anexos", err)
		return
	}
	if int(atuais)+len(arquivos) > maxAnexosPorCaso {
		voltarAoCaso(w, r, t, id, "Um caso pode ter no máximo "+strconv.Itoa(maxAnexosPorCaso)+" anexos.")
		return
	}
	type pronto struct {
		nome, tipo string
		dados      []byte
	}
	var prontos []pronto
	for _, fh := range arquivos {
		f, err := fh.Open()
		if err != nil {
			voltarAoCaso(w, r, t, id, "Não foi possível ler o arquivo "+fh.Filename+".")
			return
		}
		dados, err := io.ReadAll(io.LimitReader(f, maxAnexoCaso+1))
		f.Close()
		if err != nil || len(dados) == 0 {
			voltarAoCaso(w, r, t, id, "Não foi possível ler o arquivo "+fh.Filename+".")
			return
		}
		if len(dados) > maxAnexoCaso {
			voltarAoCaso(w, r, t, id, "O arquivo "+fh.Filename+" passa de 5 MB.")
			return
		}
		tipo := http.DetectContentType(dados)
		if !tiposDeAnexo[tipo] {
			voltarAoCaso(w, r, t, id, "O arquivo "+fh.Filename+" não é JPEG, PNG, WebP nem PDF.")
			return
		}
		nome := fh.Filename
		if i := strings.LastIndexAny(nome, `/\`); i >= 0 {
			nome = nome[i+1:]
		}
		if rn := []rune(nome); len(rn) > 120 {
			nome = string(rn[:120])
		}
		prontos = append(prontos, pronto{nome, tipo, dados})
	}
	var nomes []string
	for _, p := range prontos {
		if _, err := q.CreateCasoAnexo(r.Context(), sqlccliente.CreateCasoAnexoParams{
			Tipo: t.Slug, CasoID: id, Nome: p.nome, TipoMidia: p.tipo, Conteudo: p.dados, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
		}); err != nil {
			h.falhaInterna(w, r, "gravar anexo", err)
			return
		}
		nomes = append(nomes, p.nome)
	}
	_ = h.casoEvento(r.Context(), q, r, t.Slug, id, "anexo", strings.Join(nomes, ", "), "", "", 0)
	h.auditar(r, audit.Evento{Acao: "caso.anexar", Entidade: t.Slug, EntidadeID: strconv.FormatInt(id, 10), Rotulo: t.Rotulo + " " + strconv.FormatInt(id, 10),
		Depois: map[string]any{"anexos": len(prontos)}})
	voltarAoCaso(w, r, t, id, "")
}

func (h *handlers) casoAnexoVer(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, t tipoDeCaso, id int64) {
	aid, err := strconv.ParseInt(chi.URLParam(r, "aid"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a, err := q.GetCasoAnexo(r.Context(), sqlccliente.GetCasoAnexoParams{ID: aid, Tipo: t.Slug, CasoID: id})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", a.TipoMidia)
	w.Header().Set("Content-Length", strconv.Itoa(len(a.Conteudo)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(a.Conteudo)
}

func (h *handlers) casoAnexoExcluir(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, t tipoDeCaso, id int64) {
	aid, err := strconv.ParseInt(chi.URLParam(r, "aid"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	n, err := q.SoftDeleteCasoAnexo(r.Context(), sqlccliente.SoftDeleteCasoAnexoParams{
		ID: aid, Tipo: t.Slug, CasoID: id, Por: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
	})
	if err != nil {
		h.falhaInterna(w, r, "excluir anexo", err)
		return
	}
	if n == 0 {
		http.NotFound(w, r)
		return
	}
	h.auditar(r, audit.Evento{Acao: "caso.anexo_excluir", Entidade: t.Slug, EntidadeID: strconv.FormatInt(id, 10), Rotulo: t.Rotulo + " " + strconv.FormatInt(id, 10),
		Antes: map[string]any{"anexo": aid}})
	voltarAoCaso(w, r, t, id, "")
}

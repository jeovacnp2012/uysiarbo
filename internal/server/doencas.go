package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// ------------------------------------------------------------------- Doenças / agravos

func (h *handlers) opcoesDeEntrevista(r *http.Request, q *sqlccliente.Queries, escolhida int64) ([]views.CidadeOpcao, error) {
	gs, err := q.ListEntrevistaGrupos(r.Context())
	if err != nil {
		return nil, err
	}
	out := []views.CidadeOpcao{{Valor: "", Rotulo: "— Sem entrevista —", Selecionada: escolhida == 0}}
	for _, g := range gs {
		if !g.Ativo && g.ID != escolhida {
			continue
		}
		out = append(out, views.CidadeOpcao{ID: g.ID, Rotulo: g.Titulo, Selecionada: g.ID == escolhida})
	}
	return out, nil
}

func (h *handlers) doencasLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListDoencas(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar doenças", err)
		return
	}
	linhas := make([]views.DoencaLinha, len(rows))
	for i, d := range rows {
		linhas[i] = views.DoencaLinha{ID: d.ID, Nome: d.Nome, Entrevista: d.GrupoTitulo, Ativo: d.Ativo}
	}
	lay := h.layout(r, "Doenças e agravos", "/doencas")
	linhas = paginar(r, &lay, linhas)
	render(w, r, views.Doencas(lay, linhas))
}

func (h *handlers) doencaForm(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := views.DoencaFormData{Ativo: true, CSRF: csrfToken(r.Context())}
	if chi.URLParam(r, "id") != "" {
		id, ok := idDaURL(r)
		if !ok {
			http.NotFound(w, r)
			return
		}
		dc, err := q.GetDoenca(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar doença", err)
			return
		}
		d.ID, d.Nome, d.Ativo = dc.ID, dc.Nome, dc.Ativo
		d.Entrevistas, err = h.opcoesDeEntrevista(r, q, dc.GrupoID.Int64)
		if err != nil {
			h.falhaInterna(w, r, "listar entrevistas", err)
			return
		}
	} else if d.Entrevistas, err = h.opcoesDeEntrevista(r, q, 0); err != nil {
		h.falhaInterna(w, r, "listar entrevistas", err)
		return
	}
	render(w, r, views.DoencaForm(h.layout(r, "Doença / agravo", "/doencas"), d))
}

func (h *handlers) doencaSalvar(w http.ResponseWriter, r *http.Request) {
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
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	var antes sqlccliente.Doenca
	if id != 0 {
		if antes, err = q.GetDoenca(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar doença", err)
			return
		}
	}
	nome := strings.TrimSpace(r.PostFormValue("nome"))
	grupo := idOpcional(r.PostFormValue("entrevista"))
	ativo := r.PostFormValue("ativo") == "1" || id == 0
	falha := func(msg string) {
		ops, _ := h.opcoesDeEntrevista(r, q, grupo.Int64)
		renderStatus(w, r, http.StatusBadRequest, views.DoencaForm(h.layout(r, "Doença / agravo", "/doencas"),
			views.DoencaFormData{ID: id, Nome: nome, Ativo: ativo, Entrevistas: ops, Erro: msg, CSRF: csrfToken(r.Context())}))
	}
	if nome == "" || len([]rune(nome)) > 100 {
		falha("Informe o nome da doença (até 100 caracteres).")
		return
	}
	if grupo.Valid {
		if _, err := q.GetEntrevistaGrupo(r.Context(), grupo.Int64); err != nil {
			falha("A entrevista escolhida não existe.")
			return
		}
	}
	ev := audit.Evento{Entidade: "doenca", Rotulo: nome}
	if id == 0 {
		novo, err := q.CreateDoenca(r.Context(), sqlccliente.CreateDoencaParams{Nome: nome, GrupoID: grupo, Ativo: true})
		if violacaoUnica(err) {
			falha("Já existe uma doença com esse nome.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "criar doença", err)
			return
		}
		ev.Acao, ev.EntidadeID, ev.Depois = "doenca.criar", strconv.FormatInt(novo.ID, 10), map[string]any{"nome": nome}
	} else {
		if _, err := q.UpdateDoenca(r.Context(), sqlccliente.UpdateDoencaParams{ID: id, Nome: nome, GrupoID: grupo, Ativo: ativo}); violacaoUnica(err) {
			falha("Já existe uma doença com esse nome.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "salvar doença", err)
			return
		}
		ev.Acao, ev.EntidadeID = "doenca.editar", strconv.FormatInt(id, 10)
		ev.Antes, ev.Depois = audit.Diferenca(
			map[string]any{"nome": antes.Nome, "entrevista": antes.GrupoID.Int64, "ativo": antes.Ativo},
			map[string]any{"nome": nome, "entrevista": grupo.Int64, "ativo": ativo})
	}
	h.auditar(r, ev)
	http.Redirect(w, r, "/doencas", http.StatusSeeOther)
}

// ------------------------------------------------------------------- Entrevistas

func (h *handlers) entrevistasLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	rows, err := q.ListEntrevistaGrupos(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar entrevistas", err)
		return
	}
	linhas := make([]views.EntrevistaLinha, len(rows))
	for i, g := range rows {
		var campos []campoEntrevista
		_ = json.Unmarshal(g.Campos, &campos)
		linhas[i] = views.EntrevistaLinha{ID: g.ID, Codigo: g.Codigo, Titulo: g.Titulo, Ativo: g.Ativo, Perguntas: len(campos), Doencas: int(g.TotalDoencas)}
	}
	lay := h.layout(r, "Entrevistas", "/entrevistas")
	linhas = paginar(r, &lay, linhas)
	render(w, r, views.Entrevistas(lay, linhas))
}

func (h *handlers) entrevistaForm(w http.ResponseWriter, r *http.Request) {
	d := views.EntrevistaFormData{Ativo: true, CamposJSON: "[]", CSRF: csrfToken(r.Context())}
	if chi.URLParam(r, "id") != "" {
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
		g, err := q.GetEntrevistaGrupo(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar entrevista", err)
			return
		}
		d.ID, d.Codigo, d.Titulo, d.Ativo, d.CamposJSON = g.ID, g.Codigo, g.Titulo, g.Ativo, string(g.Campos)
	}
	render(w, r, views.EntrevistaForm(h.layout(r, "Entrevista", "/entrevistas"), d))
}

func (h *handlers) entrevistaSalvar(w http.ResponseWriter, r *http.Request) {
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
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	var antes sqlccliente.EntrevistaGrupo
	if id != 0 {
		if antes, err = q.GetEntrevistaGrupo(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar entrevista", err)
			return
		}
	}
	codigo := strings.TrimSpace(r.PostFormValue("codigo"))
	if id != 0 {
		codigo = antes.Codigo // o código não muda depois de criado
	}
	titulo := strings.TrimSpace(r.PostFormValue("titulo"))
	ativo := r.PostFormValue("ativo") == "1" || id == 0
	camposTxt := r.PostFormValue("campos")
	falha := func(msg string) {
		renderStatus(w, r, http.StatusBadRequest, views.EntrevistaForm(h.layout(r, "Entrevista", "/entrevistas"),
			views.EntrevistaFormData{ID: id, Codigo: codigo, Titulo: titulo, Ativo: ativo, CamposJSON: camposTxt, Erro: msg, CSRF: csrfToken(r.Context())}))
	}
	if titulo == "" || len([]rune(titulo)) > 120 {
		falha("Informe o título da entrevista (até 120 caracteres).")
		return
	}
	if id == 0 && !reNomeDeCampo.MatchString(codigo) {
		falha("O código precisa ter letras minúsculas, números e _ (começando por letra).")
		return
	}
	campos, msg := lerCamposDaConfiguracao(camposTxt)
	if msg != "" {
		falha(msg)
		return
	}
	json1, _ := json.Marshal(campos)
	if len(campos) == 0 {
		json1 = []byte("[]")
	}
	ev := audit.Evento{Entidade: "entrevista", Rotulo: titulo}
	if id == 0 {
		novo, err := q.CreateEntrevistaGrupo(r.Context(), sqlccliente.CreateEntrevistaGrupoParams{Codigo: codigo, Titulo: titulo, Ativo: true, Campos: json1})
		if violacaoUnica(err) {
			falha("Já existe uma entrevista com esse código.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "criar entrevista", err)
			return
		}
		ev.Acao, ev.EntidadeID, ev.Depois = "entrevista.criar", strconv.FormatInt(novo.ID, 10), map[string]any{"titulo": titulo, "perguntas": len(campos)}
		id = novo.ID
	} else {
		if _, err := q.UpdateEntrevistaGrupo(r.Context(), sqlccliente.UpdateEntrevistaGrupoParams{ID: id, Titulo: titulo, Ativo: ativo, Campos: json1}); err != nil {
			h.falhaInterna(w, r, "salvar entrevista", err)
			return
		}
		var ant []campoEntrevista
		_ = json.Unmarshal(antes.Campos, &ant)
		ev.Acao, ev.EntidadeID = "entrevista.editar", strconv.FormatInt(id, 10)
		ev.Antes, ev.Depois = audit.Diferenca(
			map[string]any{"titulo": antes.Titulo, "ativo": antes.Ativo, "perguntas": len(ant)},
			map[string]any{"titulo": titulo, "ativo": ativo, "perguntas": len(campos)})
	}
	h.auditar(r, ev)
	http.Redirect(w, r, "/entrevistas", http.StatusSeeOther)
}

var _ = pgtype.Int8{}

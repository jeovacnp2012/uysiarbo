package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// As ações "Excluir" de cada tela: conferem a permissão (na rota) e o escopo do papel, e entregam à regra
// comum de exclusion.go (excluirRegistro).

func (h *handlers) cidadeExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.excluirRegistro(w, r, "cidade", id, "/cidades?aviso=excluido", fmt.Sprintf("/cidades/%d", id))
}

func (h *handlers) bairroExcluir(w http.ResponseWriter, r *http.Request) {
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
	if visivel, err := h.bairroVisivel(r.Context(), r, q, id); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return
	} else if !visivel {
		http.NotFound(w, r)
		return
	}
	h.excluirRegistro(w, r, "bairro", id, "/bairros?aviso=excluido", fmt.Sprintf("/bairros/%d", id))
}

func (h *handlers) quarteiraoExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	qid, err := strconv.ParseInt(chi.URLParam(r, "qid"), 10, 64)
	if !ok || err != nil {
		http.NotFound(w, r)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if visivel, err := h.bairroVisivel(r.Context(), r, q, id); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return
	} else if !visivel {
		http.NotFound(w, r)
		return
	}
	quart, err := q.GetQuarteirao(r.Context(), qid)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && quart.BairroID != id) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar quarteirão", err)
		return
	}
	h.excluirRegistro(w, r, "quarteirao", qid, fmt.Sprintf("/bairros/%d?aviso=excluido#quarteiroes", id), fmt.Sprintf("/bairros/%d#quarteiroes", id))
}

func (h *handlers) localExcluir(w http.ResponseWriter, r *http.Request) {
	l, _, ok := h.localDaURL(w, r)
	if !ok {
		return
	}
	h.excluirRegistro(w, r, "local", l.ID, "/locais?aviso=excluido", fmt.Sprintf("/locais/%d", l.ID))
}

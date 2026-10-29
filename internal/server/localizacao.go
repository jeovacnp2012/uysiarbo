package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"uysiarbo/internal/geo"
	"uysiarbo/internal/geocode"
)

// semAcento põe em minúsculas e tira os acentos do português, para comparar nomes de bairro.
var semAcento = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e", "í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "ù", "u", "û", "u", "ü", "u", "ç", "c", "ñ", "n",
)

func chaveDeNome(s string) string {
	return strings.Join(strings.Fields(semAcento.Replace(strings.ToLower(s))), " ")
}

// respostaLocalizacao é o que os botões do formulário do local recebem (JSON).
type respostaLocalizacao struct {
	Encontrado bool     `json:"encontrado"`
	Desligado  bool     `json:"desligado,omitempty"`
	Mensagem   string   `json:"mensagem,omitempty"`
	Logradouro string   `json:"logradouro,omitempty"`
	Bairro     string   `json:"bairro,omitempty"`
	BairroID   int64    `json:"bairro_id,omitempty"` // bairro cadastrado que combina com o do CEP
	Cidade     string   `json:"cidade,omitempty"`
	UF         string   `json:"uf,omitempty"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
	Precisao   string   `json:"precisao,omitempty"`
	Dentro     *bool    `json:"dentro_do_bairro,omitempty"` // nil = o bairro não tem limite gravado
}

func (h *handlers) responderLocalizacao(w http.ResponseWriter, status int, v respostaLocalizacao) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// erroDeLocalizacao traduz o erro do serviço externo para a mensagem que a pessoa lê.
func (h *handlers) erroDeLocalizacao(w http.ResponseWriter, err error, naoAchou string) {
	switch {
	case errors.Is(err, geocode.ErrDesligado):
		h.responderLocalizacao(w, http.StatusOK, respostaLocalizacao{Desligado: true, Mensagem: "A localização automática está desligada neste servidor. Preencha à mão."})
	case errors.Is(err, geocode.ErrNaoEncontrado):
		h.responderLocalizacao(w, http.StatusOK, respostaLocalizacao{Mensagem: naoAchou})
	default:
		h.log.Warn("localização automática", "erro", err)
		h.responderLocalizacao(w, http.StatusServiceUnavailable, respostaLocalizacao{Mensagem: "O serviço de localização não respondeu agora. Tente de novo em instantes ou preencha à mão."})
	}
}

// localCEP busca o endereço de um CEP (ViaCEP) e sugere o bairro cadastrado que combina.
func (h *handlers) localCEP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	e, err := h.geocoder.PorCEP(ctx, r.URL.Query().Get("cep"))
	if err != nil {
		h.erroDeLocalizacao(w, err, "CEP não encontrado. Confira os números.")
		return
	}
	resp := respostaLocalizacao{Encontrado: true, Logradouro: e.Logradouro, Bairro: e.Bairro, Cidade: e.Cidade, UF: e.UF}
	if q, _, err := h.cliente(r); err == nil && e.Bairro != "" {
		somente, _ := h.escopoLocais(r)
		if bs, err := q.ListBairrosAtivosParaLocal(ctx, somente); err == nil {
			for _, b := range bs {
				if chaveDeNome(b.Nome) == chaveDeNome(e.Bairro) {
					resp.BairroID = b.ID
					break
				}
			}
		}
	}
	h.responderLocalizacao(w, http.StatusOK, resp)
}

// localGeocodificar acha latitude e longitude do endereço (Nominatim) e confere se cai dentro do bairro.
func (h *handlers) localGeocodificar(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	qs := r.URL.Query()
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	bairroID, _ := strconv.ParseInt(qs.Get("bairro_id"), 10, 64)
	b, err := q.GetBairro(ctx, bairroID)
	if err != nil {
		h.responderLocalizacao(w, http.StatusOK, respostaLocalizacao{Mensagem: "Escolha o bairro antes de localizar o endereço."})
		return
	}
	if ok, err := h.bairroVisivelParaLocal(ctx, r, q, b.ID); err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	cidade, err := q.GetCidade(ctx, b.CidadeID)
	if err != nil {
		h.falhaInterna(w, r, "buscar cidade", err)
		return
	}
	rua := strings.TrimSpace(qs.Get("endereco"))
	if rua == "" {
		h.responderLocalizacao(w, http.StatusOK, respostaLocalizacao{Mensagem: "Preencha a rua ou avenida antes de localizar."})
		return
	}
	p, err := h.geocoder.Localizar(ctx, geocode.Consulta{
		Logradouro: rua, Numero: strings.TrimSpace(qs.Get("numero")), Bairro: b.Nome, Cidade: cidade.Nome, UF: cidade.Uf,
	})
	if err != nil {
		h.erroDeLocalizacao(w, err, "Não achei esse endereço no mapa. Confira a rua e o número, ou preencha à mão.")
		return
	}
	resp := respostaLocalizacao{Encontrado: true, Latitude: &p.Latitude, Longitude: &p.Longitude, Precisao: p.Precisao, Bairro: b.Nome, Cidade: cidade.Nome, UF: cidade.Uf}
	if dentro, tem, err := geo.PontoNoBairro(ctx, pool, b.ID, p.Longitude, p.Latitude); err == nil && tem {
		resp.Dentro = &dentro
	}
	h.responderLocalizacao(w, http.StatusOK, resp)
}

// localizacaoLigada diz se os botões de CEP e de coordenadas devem aparecer.
func (h *handlers) localizacaoLigada() bool {
	_, desligada := h.geocoder.(geocode.Desligado)
	return !desligada
}

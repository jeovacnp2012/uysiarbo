package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// corpoAPI é o JSON de um pedido de gravação, campo a campo: assim dá para distinguir "não enviado"
// (mantém o valor atual), "null" (limpa) e um valor. Campos fora da lista permitida são recusados.
type corpoAPI map[string]json.RawMessage

func (h *handlers) lerCorpo(w http.ResponseWriter, r *http.Request, permitidos ...string) (corpoAPI, bool) {
	dados, err := readAllLimitado(w, r)
	if err != nil {
		h.apiErro(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Corpo grande demais (máximo 1 MB).")
		return nil, false
	}
	var c corpoAPI
	dec := json.NewDecoder(bytes.NewReader(dados))
	if err := dec.Decode(&c); err != nil || dec.More() || c == nil {
		h.apiErro(w, r, http.StatusBadRequest, "invalid_json", "O corpo precisa ser um objeto JSON válido.")
		return nil, false
	}
	for k := range c {
		if !slices.Contains(permitidos, k) {
			h.apiErro(w, r, http.StatusBadRequest, "invalid_json", fmt.Sprintf("Campo desconhecido: %q.", k))
			return nil, false
		}
	}
	return c, true
}

func readAllLimitado(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(http.MaxBytesReader(w, r.Body, maxCorpoAPI))
	return buf.Bytes(), err
}

func (c corpoAPI) tem(k string) bool { _, ok := c[k]; return ok }

func (c corpoAPI) nulo(k string) bool {
	v, ok := c[k]
	return ok && bytes.Equal(bytes.TrimSpace(v), []byte("null"))
}

// texto devolve o valor do campo como texto: string (sem espaços nas pontas) ou número (como veio).
// Ausente mantém o atual; null limpa.
func (c corpoAPI) texto(k, atual string) (string, error) {
	v, ok := c[k]
	if !ok {
		return atual, nil
	}
	if c.nulo(k) {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return strings.TrimSpace(s), nil
	}
	var n json.Number
	if err := json.Unmarshal(v, &n); err == nil {
		return n.String(), nil
	}
	return "", fmt.Errorf("%s: envie texto ou número", k)
}

func (c corpoAPI) logico(k string, atual bool) (bool, error) {
	v, ok := c[k]
	if !ok || c.nulo(k) {
		return atual, nil
	}
	var b bool
	if err := json.Unmarshal(v, &b); err != nil {
		return false, fmt.Errorf("%s: envie true ou false", k)
	}
	return b, nil
}

func (c corpoAPI) inteiro(k string, atual int64) (int64, error) {
	v, ok := c[k]
	if !ok || c.nulo(k) {
		return atual, nil
	}
	var n int64
	if err := json.Unmarshal(v, &n); err != nil {
		return 0, fmt.Errorf("%s: envie um número inteiro", k)
	}
	return n, nil
}

// idExterno lê o id_externo do corpo: "" se não veio. Só vale texto de 1 a 64 caracteres.
func (c corpoAPI) idExterno() (string, error) {
	s, err := c.texto("id_externo", "")
	if err != nil {
		return "", err
	}
	if c.tem("id_externo") && (len([]rune(s)) < 1 || len([]rune(s)) > 64) {
		return "", errors.New("id_externo: use de 1 a 64 caracteres")
	}
	return s, nil
}

// desdeDaURL lê ?updated_since=<RFC 3339>.
func desdeDaURL(r *http.Request) (pgtype.Timestamptz, bool) {
	v := r.URL.Query().Get("updated_since")
	if v == "" {
		return pgtype.Timestamptz{}, true
	}
	t, err := time.Parse(time.RFC3339, v)
	return pgtype.Timestamptz{Time: t, Valid: err == nil}, err == nil
}

// logicoDaURL lê ?ativo=true|false (ausente = sem filtro).
func logicoDaURL(r *http.Request, nome string) (pgtype.Bool, bool) {
	v := r.URL.Query().Get(nome)
	if v == "" {
		return pgtype.Bool{}, true
	}
	b, err := strconv.ParseBool(v)
	return pgtype.Bool{Bool: b, Valid: err == nil}, err == nil
}

// inteiroDaURL lê um id opcional da URL (0 = sem filtro).
func inteiroDaURL(r *http.Request, nome string) (pgtype.Int8, bool) {
	v := r.URL.Query().Get(nome)
	if v == "" {
		return pgtype.Int8{}, true
	}
	n, err := strconv.ParseInt(v, 10, 64)
	return pgtype.Int8{Int64: n, Valid: err == nil && n > 0}, err == nil && n > 0
}

func (h *handlers) apiParametroInvalido(w http.ResponseWriter, r *http.Request, nome string) {
	h.apiErro(w, r, http.StatusBadRequest, "invalid_parameter", "Parâmetro inválido: "+nome+".")
}

// optTexto devolve um texto opcional para o JSON (null quando não há).
func optTexto(t pgtype.Text) any {
	if !t.Valid {
		return nil
	}
	return t.String
}

func optFloat(f pgtype.Float8) any {
	if !f.Valid {
		return nil
	}
	return f.Float64
}

func optInt(i pgtype.Int4) any {
	if !i.Valid {
		return nil
	}
	return i.Int32
}

// optTempo devolve a data para o JSON (null quando não há).
func optTempo(t pgtype.Timestamptz) any {
	if !t.Valid {
		return nil
	}
	return t.Time
}

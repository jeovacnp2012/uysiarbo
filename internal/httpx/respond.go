// Package httpx reúne utilitários HTTP compartilhados: respostas JSON e erros padronizados.
package httpx

import (
	"encoding/json"
	"net/http"
)

// ErrorBody é o formato padrão de erro devolvido pela API.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	JSON(w, status, ErrorBody{Error: ErrorDetail{
		Code:      code,
		Message:   message,
		RequestID: r.Header.Get("X-Request-Id"),
	}})
}

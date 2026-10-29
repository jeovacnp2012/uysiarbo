package server

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var contratoOpenAPI []byte

// openAPI publica o contrato da API (público: não tem dado de ninguém).
func openAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(contratoOpenAPI)
}

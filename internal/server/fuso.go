package server

import (
	"net/http"
	"sync"
	"time"

	"uysiarbo/internal/tenant"
)

// fusoLocal é o fuso do console da plataforma (e o que vale quando não há cliente na requisição).
var fusoLocal = tenant.Local(tenant.FusoDoConsole)

var fusosCarregados sync.Map // nome -> *time.Location

// fuso devolve o fuso horário da requisição: o do cliente (empresa) do endereço, ou o do console.
// TODO o horário mostrado e todo "dia" calculado no sistema usa este fuso, nunca o do servidor.
func (h *handlers) fuso(r *http.Request) *time.Location {
	if c, ok := tenant.FromContext(r.Context()); ok {
		return fusoPorNome(c.Fuso)
	}
	return fusoLocal
}

func fusoPorNome(nome string) *time.Location {
	if l, ok := fusosCarregados.Load(nome); ok {
		return l.(*time.Location)
	}
	l := tenant.Local(nome)
	fusosCarregados.Store(nome, l)
	return l
}

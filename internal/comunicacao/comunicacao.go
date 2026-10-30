// Package comunicacao define o envio de mensagens (WhatsApp e e-mail) por trás de uma interface,
// para o provedor poder ser trocado sem mexer nas telas nem na fila.
package comunicacao

import (
	"context"
	"errors"
)

// Canais aceitos.
const (
	WhatsApp = "whatsapp"
	Email    = "email"
)

// ErrDesligado diz que nenhum provedor está configurado: a mensagem fica na fila, sem gastar tentativa.
var ErrDesligado = errors.New("nenhum provedor de envio está configurado")

// Mensagem é o que o provedor recebe.
type Mensagem struct {
	Canal        string
	Destinatario string // telefone só com dígitos (com DDI) ou e-mail
	Nome         string
	Assunto      string
	Corpo        string
}

// Provedor envia uma mensagem e devolve o identificador dela no provedor (para acompanhar entrega e leitura).
type Provedor interface {
	Nome() string
	Enviar(ctx context.Context, m Mensagem) (idExterno string, err error)
}

// Desligado não envia nada.
type Desligado struct{}

func (Desligado) Nome() string { return "desligado" }

func (Desligado) Enviar(context.Context, Mensagem) (string, error) { return "", ErrDesligado }

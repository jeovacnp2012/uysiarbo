package server

import (
	"context"
	"errors"
	"time"

	"uysiarbo/internal/comunicacao"
	"uysiarbo/internal/db/sqlccliente"
)

// intervaloDaFila é de quanto em quanto tempo o servidor procura mensagens vencidas na fila de cada cliente.
const intervaloDaFila = time.Minute

// canaisAtendidos lista os canais para os quais há provedor configurado.
func (h *handlers) canaisAtendidos() []string {
	var out []string
	for _, c := range rotulosCanalCom {
		if h.provedorCom.Atende(c.Valor) {
			out = append(out, c.Valor)
		}
	}
	return out
}

// processarFilasEmSegundoPlano envia sozinho as mensagens vencidas de todos os clientes até o contexto acabar.
// Sem nenhum provedor configurado não faz nada.
func (h *handlers) processarFilasEmSegundoPlano(ctx context.Context) {
	if len(h.canaisAtendidos()) == 0 {
		return
	}
	t := time.NewTicker(intervaloDaFila)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.processarTodasAsFilas(ctx)
		}
	}
}

// processarTodasAsFilas faz uma volta pelos clientes ativos.
func (h *handlers) processarTodasAsFilas(ctx context.Context) {
	canais := h.canaisAtendidos()
	clientes, err := h.pub.ListClientes(ctx)
	if err != nil {
		h.log.Error("fila de comunicações: listar clientes", "erro", err)
		return
	}
	for _, c := range clientes {
		if c.Status == "cancelado" || c.Status == "provisionando" {
			continue
		}
		pool, err := h.pools.Get(ctx, c.SchemaNome)
		if err != nil {
			h.log.Error("fila de comunicações: banco do cliente", "cliente", c.Slug, "erro", err)
			continue
		}
		q := sqlccliente.New(pool)
		ids, err := q.ListComunicacoesVencidas(ctx, sqlccliente.ListComunicacoesVencidasParams{Canais: canais, Limite: 50})
		if err != nil {
			h.log.Error("fila de comunicações: listar vencidas", "cliente", c.Slug, "erro", err)
			continue
		}
		for _, id := range ids {
			if _, err := h.tentarEnvio(ctx, pool, q, id, idOuNulo(0)); err != nil && !errors.Is(err, errForaDaFila) && !errors.Is(err, comunicacao.ErrDesligado) {
				h.log.Error("fila de comunicações: enviar", "cliente", c.Slug, "comunicacao", id, "erro", err)
			}
		}
	}
}

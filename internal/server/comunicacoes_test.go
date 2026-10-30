package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"uysiarbo/internal/comunicacao"
)

// provedorFalso envia sem internet: guarda o que recebeu e pode ser mandado falhar ou desligar.
type provedorFalso struct {
	enviadas []comunicacao.Mensagem
	erro     error
}

func (p *provedorFalso) Nome() string { return "falso" }

func (p *provedorFalso) Enviar(_ context.Context, m comunicacao.Mensagem) (string, error) {
	if p.erro != nil {
		return "", p.erro
	}
	p.enviadas = append(p.enviadas, m)
	return "ext-1", nil
}

func TestComunicacoes(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.get("/comunicacoes"), http.StatusOK)
	quer(t, ana.get("/comunicacoes/nova"), http.StatusOK)

	nova := func(extra map[string]string) url.Values {
		v := url.Values{"canal": {"whatsapp"}, "destinatario": {"(65) 99999-1234"}, "nome": {"Maria"}, "corpo": {"Olá, sua notificação venceu."}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.post("/comunicacoes", nova(map[string]string{"canal": "sms"})), http.StatusBadRequest, "canal")
	quer(t, ana.post("/comunicacoes", nova(map[string]string{"destinatario": "123"})), http.StatusBadRequest, "telefone")
	quer(t, ana.post("/comunicacoes", nova(map[string]string{"corpo": ""})), http.StatusBadRequest, "mensagem")
	quer(t, ana.post("/comunicacoes", nova(map[string]string{"canal": "email", "destinatario": "nao-e-email"})), http.StatusBadRequest, "assunto")
	quer(t, ana.post("/comunicacoes", nova(map[string]string{"canal": "email", "assunto": "Aviso", "destinatario": "nao-e-email"})), http.StatusBadRequest, "e-mail válido")

	rec := ana.post("/comunicacoes", nova(nil))
	quer(t, rec, http.StatusSeeOther)
	c := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.get(c), http.StatusOK, "Na fila", "5565999991234", "Mensagem colocada na fila", "Enviar agora")
	quer(t, ana.get("/comunicacoes?status=fila&canal=whatsapp&busca=maria"), http.StatusOK, "5565999991234")

	// sem provedor: continua na fila e não gasta tentativa
	m.com.erro = comunicacao.ErrDesligado
	quer(t, ana.post(c+"/enviar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(c+"?aviso=sem_provedor"), http.StatusOK, "continua na fila", "Na fila")

	// provedor falhando: tentativas, até esgotar
	m.com.erro = errors.New("timeout no provedor")
	for i := 0; i < 3; i++ {
		quer(t, ana.post(c+"/enviar", url.Values{}), http.StatusSeeOther)
	}
	quer(t, ana.get(c), http.StatusOK, "Falhou", "3 de 3", "timeout no provedor", "Tentativas esgotadas", "Reenviar")
	quer(t, ana.post(c+"/enviar", url.Values{}), http.StatusBadRequest, "na fila")

	// reenvio com provedor bom
	m.com.erro = nil
	quer(t, ana.post(c+"/reenviar", url.Values{}), http.StatusSeeOther)
	if len(m.com.enviadas) != 1 || m.com.enviadas[0].Destinatario != "5565999991234" {
		t.Fatalf("enviadas = %+v", m.com.enviadas)
	}
	quer(t, ana.get(c), http.StatusOK, "Enviada", "ext-1", "Voltou para a fila", "Marcar como entregue")
	quer(t, ana.post(c+"/cancelar", url.Values{}), http.StatusBadRequest, "fila ou que falhou")
	quer(t, ana.post(c+"/entregue", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(c+"/visualizada", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(c), http.StatusOK, "Visualizada")
	quer(t, ana.post(c+"/entregue", url.Values{}), http.StatusBadRequest, "já enviada")

	// processar a fila: manda as vencidas e cancela
	rec = ana.post("/comunicacoes", nova(map[string]string{"canal": "email", "assunto": "Aviso", "destinatario": "Maria@Exemplo.com", "corpo": "Texto"}))
	quer(t, rec, http.StatusSeeOther)
	c2 := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.post("/comunicacoes/processar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(c2), http.StatusOK, "Enviada", "maria@exemplo.com")
	rec = ana.post("/comunicacoes", nova(nil))
	c3 := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.post(c3+"/cancelar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(c3), http.StatusOK, "Cancelada", "Reenviar")
	quer(t, ana.get("/comunicacoes/999999"), http.StatusNotFound)
}

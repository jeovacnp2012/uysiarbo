package server

import (
	"net/http"
	"net/url"
	"testing"
)

func TestAnonimizarTitular(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	q := m.criarQuarteirao(t, b, "1")
	id := umLocal(t, m, ana, b, q, "numero", "10", "proprietario", "Maria Souza", "proprietario_cpf", cpfBom,
		"proprietario_telefone", "(65) 3000-0000", "proprietario_email", "maria@exemplo.com")

	quer(t, ana.get("/lgpd/anonimizar"), http.StatusOK, "Anonimização")
	quer(t, ana.post("/lgpd/anonimizar/buscar", url.Values{"tipo": {"cpf"}, "valor": {"123"}}), http.StatusBadRequest, "11 dígitos")

	rec := ana.post("/lgpd/anonimizar/buscar", url.Values{"tipo": {"cpf"}, "valor": {"529.982.247-25"}})
	quer(t, rec, http.StatusOK, "Locais (proprietário/inquilino)")
	quer(t, ana.post("/lgpd/anonimizar/buscar", url.Values{"tipo": {"telefone"}, "valor": {"3000-0000"}}), http.StatusOK, "Locais (proprietário/inquilino)")
	quer(t, ana.post("/lgpd/anonimizar/buscar", url.Values{"tipo": {"email"}, "valor": {"outra@exemplo.com"}}), http.StatusOK, "Nenhum registro")

	f := url.Values{"tipo": {"cpf"}, "valor": {"52998224725"}, "motivo": {"Pedido 123 do titular"}}
	quer(t, ana.post("/lgpd/anonimizar/confirmar", f), http.StatusBadRequest, "Marque a confirmação")
	f.Set("confirmo", "1")
	quer(t, ana.post("/lgpd/anonimizar/confirmar", f), http.StatusOK, "1 registro(s) anonimizado(s)")

	l, err := m.cliQ.GetLocal(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if l.Proprietario != "" || l.ProprietarioCpf != "" || l.ProprietarioTelefone != "" || l.ProprietarioEmail != "" {
		t.Fatalf("dados pessoais continuam: %+v", l)
	}
	if l.Numero != "10" {
		t.Fatalf("o endereço deveria ficar: %q", l.Numero)
	}

	sem := m.usuarioCom(t, "Sem Lgpd", "semlgpd@nm.gov.br", "visita_visualizar")
	quer(t, sem.get("/lgpd/anonimizar"), http.StatusForbidden)
}

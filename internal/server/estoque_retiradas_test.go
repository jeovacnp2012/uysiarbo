package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestEstoqueRetiradasEBalanco(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	resp := m.idUsuario(t, emailAna)

	rec := ana.post("/estoque/unidade", url.Values{"nome": {"Almoxarifado"}})
	idUn := strings.TrimPrefix(strings.SplitN(local(rec), "?", 2)[0], "/estoque/unidade/")
	mk := func(nome, saida string) (string, string) {
		rec := ana.post("/estoque/produto", url.Values{"unidade": {idUn}, "nome": {nome}, "tipo": {"insumo"}, "medida": {"un"}, "saida": {saida}, "ativo": {"1"}})
		quer(t, rec, http.StatusSeeOther)
		idP := strings.TrimPrefix(strings.SplitN(local(rec), "?", 2)[0], "/estoque/produto/")
		rec = ana.post("/estoque/lote", url.Values{"produto": {idP}, "numero": {"L1"}, "quantidade": {"10"}, "validade": {"2030-01-01"}})
		quer(t, rec, http.StatusSeeOther)
		return idP, strings.SplitN(local(rec), "?", 2)[0]
	}
	_, loteCampo := mk("Larvicida", "campo")
	_, lotePerm := mk("Bomba costal", "permanente")
	idLC := strings.TrimPrefix(loteCampo, "/estoque/lote/")
	idLP := strings.TrimPrefix(lotePerm, "/estoque/lote/")

	quer(t, ana.get("/estoque/retirada"), http.StatusOK)
	quer(t, ana.get("/estoque/retirada/novo"), http.StatusOK)
	quer(t, ana.post("/estoque/retirada", url.Values{"tipo": {"campo"}}), http.StatusBadRequest, "responsável")
	rec = ana.post("/estoque/retirada", url.Values{"tipo": {"campo"}, "responsavel": {resp}, "previsto": {"2030-01-02"}})
	quer(t, rec, http.StatusSeeOther)
	rt := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.get(rt), http.StatusOK, "Adicionar item", "Larvicida")

	// produto só permanente não sai em retirada de campo
	quer(t, ana.post(rt+"/itens", url.Values{"lote": {idLP}, "quantidade": {"1"}}), http.StatusBadRequest)
	quer(t, ana.post(rt+"/itens", url.Values{"lote": {idLC}, "quantidade": {"11"}}), http.StatusBadRequest, "não cobre")
	quer(t, ana.post(rt+"/itens", url.Values{"lote": {idLC}, "quantidade": {"6"}}), http.StatusSeeOther)
	quer(t, ana.get(loteCampo), http.StatusOK, "Retirada 1")
	quer(t, ana.get("/estoque/produto"), http.StatusOK, "Larvicida")
	quer(t, ana.post(rt+"/itens", url.Values{"lote": {idLC}, "quantidade": {"1"}}), http.StatusBadRequest, "já está")

	// balanço: 4 em estoque + 6 em retirada
	quer(t, ana.get("/estoque/balanco"), http.StatusOK, "Larvicida", "4 un", "6 un", "10 un")

	// excluir lote com retirada aberta é recusado
	quer(t, ana.post(loteCampo+"/excluir", url.Values{}), http.StatusConflict)

	// finalizar: soma não fecha
	var iid string
	pag := ana.get(rt).Body.String()
	if i := strings.Index(pag, `name="usado_`); i >= 0 {
		iid = strings.SplitN(pag[i+len(`name="usado_`):], `"`, 2)[0]
	} else {
		t.Fatal("campos de finalização não encontrados")
	}
	fin := func(u, d, p string) url.Values {
		return url.Values{"usado_" + iid: {u}, "devolvido_" + iid: {d}, "perda_" + iid: {p}}
	}
	quer(t, ana.post(rt+"/finalizar", fin("2", "2", "1")), http.StatusBadRequest, "deve ser 6")
	quer(t, ana.post(rt+"/finalizar", fin("3", "2,5", "0,5")), http.StatusSeeOther)
	quer(t, ana.get(rt), http.StatusOK, "Finalizada")
	quer(t, ana.get(loteCampo), http.StatusOK, "Devolução da retirada 1", "Uso em campo", "6,5")
	quer(t, ana.post(rt+"/itens", url.Values{"lote": {idLC}, "quantidade": {"1"}}), http.StatusBadRequest)
	quer(t, ana.post(rt+"/cancelar", url.Values{}), http.StatusBadRequest)

	// cancelar devolve tudo; remover item devolve a quantidade
	rec = ana.post("/estoque/retirada", url.Values{"tipo": {"permanente"}, "responsavel": {resp}})
	quer(t, rec, http.StatusSeeOther)
	r2 := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.post(r2+"/itens", url.Values{"lote": {idLP}, "quantidade": {"3"}}), http.StatusSeeOther)
	quer(t, ana.get(lotePerm), http.StatusOK, "Saldo", "7")
	quer(t, ana.post(r2+"/cancelar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(r2), http.StatusOK, "Cancelada")
	quer(t, ana.get(lotePerm), http.StatusOK, "Cancelamento da retirada 2")
	quer(t, ana.get("/estoque/retirada?status=cancelada"), http.StatusOK, "Permanente")

	// retirada encerrada pode ir à lixeira
	quer(t, ana.post(r2+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(r2), http.StatusNotFound)
}

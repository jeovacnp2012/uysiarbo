package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestEstoqueUnidadesProdutosELotes(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	// unidade
	quer(t, ana.get("/estoque/unidade"), http.StatusOK)
	quer(t, ana.get("/estoque/unidade/novo"), http.StatusOK)
	quer(t, ana.post("/estoque/unidade", url.Values{"nome": {""}}), http.StatusBadRequest)
	rec := ana.post("/estoque/unidade", url.Values{"nome": {"Vigilância Ambiental"}, "ativo": {"1"}})
	quer(t, rec, http.StatusSeeOther)
	un := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.post("/estoque/unidade", url.Values{"nome": {"vigilância ambiental"}}), http.StatusBadRequest, "Já existe")
	quer(t, ana.get(un), http.StatusOK, "Vigilância Ambiental")
	idUn := strings.TrimPrefix(un, "/estoque/unidade/")

	// produto fracionável
	quer(t, ana.get("/estoque/produto"), http.StatusOK)
	quer(t, ana.get("/estoque/produto/novo"), http.StatusOK, "Fracionável")
	prod := func(extra map[string]string) url.Values {
		v := url.Values{"unidade": {idUn}, "nome": {"Bioraiva frasco"}, "tipo": {"vacina"}, "medida": {"frasco"}, "saida": {"campo_permanente"},
			"fracionavel": {"1"}, "doses": {"25"}, "fracao": {"dose"}, "selecionavel": {"1"}, "categoria": {"vacina"}, "ativo": {"1"}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.post("/estoque/produto", prod(map[string]string{"doses": ""})), http.StatusBadRequest, "Frações por unidade")
	quer(t, ana.post("/estoque/produto", prod(map[string]string{"categoria": ""})), http.StatusBadRequest, "categoria")
	quer(t, ana.post("/estoque/produto", prod(map[string]string{"unidade": "999999"})), http.StatusBadRequest)
	rec = ana.post("/estoque/produto", prod(nil))
	quer(t, rec, http.StatusSeeOther)
	pr := strings.SplitN(local(rec), "?", 2)[0]
	idPr := strings.TrimPrefix(pr, "/estoque/produto/")
	quer(t, ana.post("/estoque/produto", prod(nil)), http.StatusBadRequest, "Já existe")
	quer(t, ana.get(pr), http.StatusOK, "Bioraiva frasco")
	quer(t, ana.get("/estoque/produto?busca=bioraiva&tipo=vacina"), http.StatusOK, "Bioraiva frasco")
	quer(t, ana.get("/estoque/produto?busca=nada"), http.StatusOK, "Nenhum produto")

	// unidade com produto não vai à lixeira
	quer(t, ana.post(un+"/excluir", url.Values{}), http.StatusConflict, "produto")
	quer(t, ana.get(un), http.StatusOK)

	// lote: entrada gera movimentação
	quer(t, ana.get("/estoque/lote"), http.StatusOK)
	quer(t, ana.get("/estoque/lote/novo?produto="+idPr), http.StatusOK)
	lote := func(extra map[string]string) url.Values {
		v := url.Values{"produto": {idPr}, "numero": {"L-001"}, "quantidade": {"10"}, "validade": {"2030-01-31"}, "fornecedor": {"Fornecedor X"}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.post("/estoque/lote", lote(map[string]string{"quantidade": "0"})), http.StatusBadRequest, "maior que zero")
	quer(t, ana.post("/estoque/lote", lote(map[string]string{"validade": "2020-13-01"})), http.StatusBadRequest, "Validade")
	rec = ana.post("/estoque/lote", lote(nil))
	quer(t, rec, http.StatusSeeOther)
	lt := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.post("/estoque/lote", lote(nil)), http.StatusBadRequest, "Já existe")
	quer(t, ana.get(lt), http.StatusOK, "Histórico do lote", "Entrada do lote", "L-001")
	quer(t, ana.get("/estoque/lote?busca=l-001"), http.StatusOK, "Fornecedor X")
	quer(t, ana.get("/estoque/lote?situacao=com_saldo"), http.StatusOK, "L-001")
	quer(t, ana.get("/estoque/lote?situacao=vencido"), http.StatusOK, "Nenhum lote")
	quer(t, ana.get(pr), http.StatusOK, "saldo 10")

	// movimentações
	mov := func(tipo, qtd, desc string) *httptest.ResponseRecorder {
		return ana.post(lt+"/movimentar", url.Values{"tipo": {tipo}, "quantidade": {qtd}, "descricao": {desc}})
	}
	quer(t, mov("perda", "3,5", "Frasco quebrado"), http.StatusSeeOther)
	quer(t, ana.get(lt), http.StatusOK, "Frasco quebrado", "6,5")
	quer(t, mov("baixa", "7", "Vencimento"), http.StatusBadRequest, "não cobre")
	quer(t, mov("baixa", "6,5", ""), http.StatusBadRequest, "motivo")
	quer(t, mov("entrada", "1,5", "Doação"), http.StatusSeeOther)
	quer(t, mov("ajuste", "4", "Contagem física"), http.StatusSeeOther)
	quer(t, ana.get(lt), http.StatusOK, "Contagem física")
	quer(t, ana.get(pr), http.StatusOK, "saldo 4")
	quer(t, mov("troca", "1", "x"), http.StatusBadRequest)

	// edição do lote
	quer(t, ana.post(lt, url.Values{"fornecedor": {"Fornecedor Y"}, "validade": {"2031-02-01"}}), http.StatusSeeOther)
	quer(t, ana.get(lt), http.StatusOK, "Fornecedor Y")

	// produto com lote não vai à lixeira; lote sim, depois o produto, depois a unidade
	quer(t, ana.post(pr+"/excluir", url.Values{}), http.StatusConflict, "lote")
	quer(t, ana.get(pr), http.StatusOK)
	quer(t, ana.post(lt+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(lt), http.StatusNotFound)
	quer(t, ana.post(pr+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(pr), http.StatusNotFound)
	quer(t, ana.get("/lixeira"), http.StatusOK)
}

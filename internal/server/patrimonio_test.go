package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPatrimonio(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	resp := m.idUsuario(t, emailAna)

	quer(t, ana.get("/estoque/patrimonio"), http.StatusOK)
	quer(t, ana.get("/estoque/patrimonio/novo"), http.StatusOK)
	novo := func(extra map[string]string) url.Values {
		v := url.Values{"numero": {"PAT-001"}, "nome": {"Bomba costal"}, "serie": {"S123"}, "entrada": {"2026-01-10"}, "responsavel": {resp}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.post("/estoque/patrimonio", novo(map[string]string{"numero": ""})), http.StatusBadRequest, "número de patrimônio")
	quer(t, ana.post("/estoque/patrimonio", novo(map[string]string{"entrada": "31/02/2026"})), http.StatusBadRequest, "Data de entrada")
	rec := ana.post("/estoque/patrimonio", novo(nil))
	quer(t, rec, http.StatusSeeOther)
	p := strings.SplitN(local(rec), "?", 2)[0]
	quer(t, ana.post("/estoque/patrimonio", novo(map[string]string{"numero": "pat-001"})), http.StatusBadRequest, "Já existe")
	quer(t, ana.get(p), http.StatusOK, "Bomba costal", "Bem cadastrado", "Atribuir")
	quer(t, ana.get("/estoque/patrimonio?busca=s123&status=ativo"), http.StatusOK, "PAT-001")

	acao := func(a string, v url.Values) *httptest.ResponseRecorder { return ana.post(p+"/"+a, v) }
	quer(t, acao("atribuir", url.Values{}), http.StatusBadRequest, "novo responsável")
	quer(t, acao("atribuir", url.Values{"responsavel": {resp}, "descricao": {"Setor B"}}), http.StatusSeeOther)
	quer(t, acao("retorno", url.Values{}), http.StatusBadRequest, "Essa ação não vale")
	quer(t, acao("manutencao", url.Values{}), http.StatusBadRequest, "descrição")
	quer(t, acao("manutencao", url.Values{"descricao": {"Vazamento"}}), http.StatusSeeOther)
	quer(t, ana.get(p), http.StatusOK, "Em manutenção", "Vazamento", "Retorno da manutenção")
	quer(t, acao("atribuir", url.Values{"responsavel": {resp}}), http.StatusBadRequest, "Essa ação não vale")
	quer(t, acao("retorno", url.Values{"descricao": {"Consertada"}}), http.StatusSeeOther)
	quer(t, acao("anotacao", url.Values{"descricao": {"Revisada"}}), http.StatusSeeOther)
	quer(t, acao("baixa", url.Values{"descricao": {"Sucata"}}), http.StatusSeeOther)
	quer(t, ana.get(p), http.StatusOK, "Baixado", "Sucata", "Reativar o bem")
	quer(t, ana.post(p, novo(map[string]string{"nome": "Outro"})), http.StatusBadRequest, "reative")
	quer(t, acao("reativar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(p, novo(map[string]string{"nome": "Bomba costal 20L"})), http.StatusSeeOther)
	quer(t, ana.get(p), http.StatusOK, "Bomba costal 20L", "Reativação")
	quer(t, ana.post(p+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(p), http.StatusNotFound)
}

package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func formCiclo(nome, ini, fim string, extra url.Values) url.Values {
	v := url.Values{"nome": {nome}, "data_inicio": {ini}, "data_final": {fim}}
	for k, vs := range extra {
		v[k] = vs
	}
	return v
}

func TestCiclos(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	quer(t, ana.get("/ciclos"), http.StatusOK, "Nenhum ciclo cadastrado", "Novo ciclo")
	// O catálogo de atividades aparece no formulário.
	quer(t, ana.get("/ciclos/novo"), http.StatusOK, "Nome do ciclo", "LIT", "LIRAa", "BRI", "Atividades mostradas no painel público")

	quer(t, ana.post("/ciclos", formCiclo("X", "2026-01-01", "2026-02-28", nil)), http.StatusBadRequest, "nome do ciclo")
	quer(t, ana.post("/ciclos", formCiclo("01 Ciclo - 2026", "", "2026-02-28", nil)), http.StatusBadRequest, "data de início")
	quer(t, ana.post("/ciclos", formCiclo("01 Ciclo - 2026", "2026-01-01", "", nil)), http.StatusBadRequest, "data final")
	quer(t, ana.post("/ciclos", formCiclo("01 Ciclo - 2026", "2026-03-01", "2026-02-28", nil)), http.StatusBadRequest, "não pode ser anterior")
	quer(t, ana.post("/ciclos", formCiclo("01 Ciclo - 2026", "2026-01-01", "2026-02-28", url.Values{"painel_publico": {"1"}})), http.StatusBadRequest, "ao menos uma atividade do painel")

	cat, _ := m.cliQ.ListAtividades(t.Context())
	if len(cat) != 9 || cat[0].Sigla != "LIT" || cat[8].Sigla != "BRI" {
		t.Fatalf("catálogo de atividades: %+v", cat)
	}
	idAtiv := func(sigla string) string {
		for _, a := range cat {
			if a.Sigla == sigla {
				return s64(int64(a.ID))
			}
		}
		t.Fatalf("sem atividade %s", sigla)
		return ""
	}
	lit, li, pe := idAtiv("LIT"), idAtiv("LI"), idAtiv("PE")

	rec := ana.post("/ciclos", formCiclo("01 Ciclo - 2026", "2026-01-01", "2026-02-28", url.Values{
		"atividade": {lit, li, "9999", "x"}, "painel_publico": {"1"}, "painel_atividade": {lit, pe}, "painel_texto": {"Ciclo 1"},
	}))
	quer(t, rec, http.StatusSeeOther)
	if rec.Header().Get("Location") != "/ciclos" {
		t.Errorf("sem cruzamento, volta à lista: %q", rec.Header().Get("Location"))
	}
	quer(t, ana.post("/ciclos", formCiclo("01 ciclo - 2026", "2026-03-01", "2026-04-30", nil)), http.StatusBadRequest, "Já existe um ciclo")

	cs, _ := m.cliQ.ListCiclos(t.Context())
	if len(cs) != 1 || cs[0].Atividades != "LIT, LI" {
		t.Fatalf("ciclos = %+v", cs)
	}
	id := s64(cs[0].ID)
	// O painel só guarda atividades que o ciclo tem (PE foi descartada); atividades inválidas são ignoradas.
	if p, _ := m.cliQ.ListAtividadesDoPainel(t.Context(), cs[0].ID); len(p) != 1 {
		t.Errorf("painel = %v", p)
	}
	quer(t, ana.get("/ciclos"), http.StatusOK, "01 Ciclo - 2026", "01/01/2026 a 28/02/2026", "Operando", "LIT, LI")
	quer(t, ana.get("/ciclos/"+id), http.StatusOK, "01 Ciclo - 2026", `value="2026-01-01"`, `value="2026-02-28"`, "Ciclo 1")

	// Segundo ciclo cruzando o primeiro: salva, mas leva à ficha com o aviso; e sincroniza com o primeiro.
	rec = ana.post("/ciclos", formCiclo("02 Ciclo - 2026", "2026-02-15", "2026-04-30", url.Values{"sincronizar": {id, "9999"}}))
	quer(t, rec, http.StatusSeeOther)
	if !strings.HasPrefix(rec.Header().Get("Location"), "/ciclos/") {
		t.Fatalf("cruzamento deveria levar à ficha: %q", rec.Header().Get("Location"))
	}
	quer(t, ana.get(rec.Header().Get("Location")), http.StatusOK, "cruza com o de 01 Ciclo - 2026")
	cs, _ = m.cliQ.ListCiclos(t.Context())
	id2 := s64(cs[0].ID) // o mais recente vem primeiro
	if s, _ := m.cliQ.ListCiclosSincronizados(t.Context(), cs[0].ID); len(s) != 1 {
		t.Errorf("sincronizados = %v", s)
	}

	// Editar: fechar o ciclo; sem mudança nada é registrado.
	quer(t, ana.post("/ciclos/"+id, formCiclo("01 Ciclo - 2026", "2026-01-01", "2026-02-28", url.Values{"atividade": {lit, li}, "painel_publico": {"1"}, "painel_atividade": {lit}, "painel_texto": {"Ciclo 1"}})), http.StatusSeeOther)
	quer(t, ana.post("/ciclos/"+id, formCiclo("01 Ciclo - 2026", "2026-01-01", "2026-02-28", url.Values{"atividade": {lit, li}, "painel_publico": {"1"}, "painel_atividade": {lit}, "painel_texto": {"Ciclo 1"}, "fechado": {"1"}})), http.StatusSeeOther)
	quer(t, ana.get("/ciclos"), http.StatusOK, "Fechado")
	quer(t, ana.get("/ciclos/999999"), http.StatusNotFound)

	ev := m.eventos(t)
	if achar(ev, "ciclo.criar") == nil {
		t.Error("faltou ciclo.criar na auditoria")
	}
	edit := achar(ev, "ciclo.editar")
	if edit == nil || mapa(edit.Antes)["situacao"] != "operando" || mapa(edit.Depois)["situacao"] != "fechado" {
		t.Errorf("ciclo.editar: %+v", edit)
	}

	// Exclusão lógica, Lixeira e vínculo: o 1º é destino de sincronização do 2º, então não sai de vez.
	quer(t, ana.post("/ciclos/"+id+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/ciclos/"+id), http.StatusNotFound)
	quer(t, ana.get("/lixeira?tipo=ciclo"), http.StatusOK, "01 Ciclo - 2026", "Ciclos")
	if _, err := m.cliQ.GetCicloQualquer(t.Context(), cs[1].ID); err != nil {
		t.Fatal("o ciclo excluído deveria continuar no banco")
	}
	// O nome volta a poder ser usado.
	quer(t, ana.post("/ciclos", formCiclo("01 Ciclo - 2026", "2026-05-01", "2026-06-30", nil)), http.StatusSeeOther)
	_ = id2
}

func TestCiclosExigemPermissao(t *testing.T) {
	m := novoMundo(t)
	carla := m.comCarla(t)
	for _, p := range []string{"/ciclos", "/ciclos/novo", "/ciclos/1"} {
		quer(t, carla.get(p), http.StatusForbidden)
	}
	quer(t, carla.post("/ciclos", formCiclo("01 Ciclo", "2026-01-01", "2026-02-28", nil)), http.StatusForbidden)
}

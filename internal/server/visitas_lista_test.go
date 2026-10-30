package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestVisitasFiltrosDesativarELote(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)

	quer(t, ana.post("/visitas", c.form(agente, "achou", "1", "a1", "1", "total_eliminado", "1", "tratamento", "1")), http.StatusSeeOther)
	quer(t, ana.post("/visitas", c.form(agente, "pendencia", "fechado")), http.StatusSeeOther)
	vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	if len(vs) != 2 {
		t.Fatalf("visitas = %d", len(vs))
	}
	idComDeposito, idFechada := s64(vs[1].ID), s64(vs[0].ID)

	tem := func(consulta string, quer1, naoQuer string) {
		t.Helper()
		corpo := ana.get("/visitas?" + consulta).Body.String()
		if !strings.Contains(corpo, "href=\"/visitas/"+quer1+"\"") {
			t.Errorf("%s: deveria listar a visita %s", consulta, quer1)
		}
		if naoQuer != "" && strings.Contains(corpo, "href=\"/visitas/"+naoQuer+"\"") {
			t.Errorf("%s: não deveria listar a visita %s", consulta, naoQuer)
		}
	}
	tem("deposito=sim", idComDeposito, idFechada)
	tem("pendencia=fechado", idFechada, idComDeposito)
	tem("tratamento=sim", idComDeposito, idFechada)
	tem("foto=nao", idComDeposito, "")
	tem("ciclo="+s64(c.ciclo)+"&atividade=1&endereco=rua", idFechada, "")
	tem("id="+idFechada, idFechada, idComDeposito)
	tem("de="+c.ontem+"&ate="+c.ontem, idFechada, "")
	quer(t, ana.get("/visitas?foto=sim"), http.StatusOK, "Nenhuma visita encontrada")
	quer(t, ana.get("/visitas?de="+c.hoje), http.StatusOK, "Nenhuma visita encontrada")

	// Desativar pela ficha esconde da lista de quem não vê desativadas e reativar volta.
	quer(t, ana.post("/visitas/"+idFechada+"/ativo", url.Values{"ativo": {"0"}}), http.StatusSeeOther)
	quer(t, ana.get("/visitas/"+idFechada), http.StatusOK, "está desativada", "Reativar visita")
	quer(t, ana.get("/visitas?situacao=desativada"), http.StatusOK, "Desativada")
	quer(t, ana.post("/visitas/"+idFechada+"/ativo", url.Values{"ativo": {"1"}}), http.StatusSeeOther)

	// Lote: desativa as duas, depois reativa.
	rec := ana.post("/visitas/lote", url.Values{"acao": {"desativar"}, "ids": {idComDeposito, idFechada, "999999"}, "voltar": {"/visitas?deposito=sim"}})
	quer(t, rec, http.StatusSeeOther)
	if l := local(rec); !strings.Contains(l, "aviso=lote-2-1") || !strings.HasPrefix(l, "/visitas?deposito=sim") {
		t.Errorf("destino do lote: %q", l)
	}
	quer(t, ana.get("/visitas?aviso=lote-2-1"), http.StatusOK, "2 visita(s) atualizada(s)")
	quer(t, ana.post("/visitas/lote", url.Values{"acao": {"reativar"}, "ids": {idComDeposito, idFechada}}), http.StatusSeeOther)
	quer(t, ana.post("/visitas/lote", url.Values{"acao": {"apagar"}, "ids": {idFechada}}), http.StatusBadRequest)
	quer(t, ana.post("/visitas/lote", url.Values{"acao": {"desativar"}, "voltar": {"//evil.com"}}), http.StatusSeeOther)

	// Quem só consulta não desativa nem mexe em lote.
	carla := m.comCarla(t)
	quer(t, carla.post("/visitas/"+idFechada+"/ativo", url.Values{"ativo": {"0"}}), http.StatusForbidden)
	quer(t, carla.post("/visitas/lote", url.Values{"acao": {"desativar"}, "ids": {idFechada}}), http.StatusForbidden)

	ev := m.eventos(t)
	for _, a := range []string{"visita.desativar", "visita.reativar"} {
		if achar(ev, a) == nil {
			t.Errorf("faltou %s na auditoria", a)
		}
	}
}

package server

import (
	"time"

	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"uysiarbo/internal/db/sqlccliente"
)

func TestLaboratorioAmostrasEResultados(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)

	// Visita sem amostra: não aceita tubito.
	quer(t, ana.post("/visitas", c.form(agente)), http.StatusSeeOther)
	vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	semAmostra := s64(vs[0].ID)
	quer(t, ana.post("/visitas/"+semAmostra+"/amostras", url.Values{"larvas": {"3"}}), http.StatusBadRequest, "coletou amostra")

	// Visita com amostra: numeração automática (padrão).
	quer(t, ana.post("/visitas", c.form(agente, "amostra", "1")), http.StatusSeeOther)
	vs, _ = m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	var id string
	for _, v := range vs {
		if v.ColetouAmostra {
			id = s64(v.ID)
		}
	}
	quer(t, ana.get("/visitas/"+id), http.StatusOK, "Amostras (tubitos)", "gerado automaticamente")
	quer(t, ana.post("/visitas/"+id+"/amostras", url.Values{"deposito": {"X9"}}), http.StatusBadRequest, "depósito válido")
	quer(t, ana.post("/visitas/"+id+"/amostras", url.Values{"larvas": {"abc"}}), http.StatusBadRequest, "Larvas")
	quer(t, ana.post("/visitas/"+id+"/amostras", url.Values{"deposito": {"A2"}, "larvas": {"4"}, "pupas": {"1"}}), http.StatusSeeOther)
	quer(t, ana.post("/visitas/"+id+"/amostras", url.Values{"deposito": {"D1"}, "larvas": {"8"}}), http.StatusSeeOther)
	corpo := ana.get("/visitas/" + id).Body.String()
	for _, quer1 := range []string{"Tubito 1", "Tubito 2", "Em análise"} {
		if !strings.Contains(corpo, quer1) {
			t.Fatalf("a ficha deveria mostrar %q", quer1)
		}
	}

	// Resultado: valida e grava.
	lista, _ := m.cliQ.ListAmostras(t.Context(), listarAmostras())
	if len(lista) != 2 {
		t.Fatalf("amostras = %d", len(lista))
	}
	aid := s64(lista[1].ID)
	quer(t, ana.post("/visitas/"+id+"/amostras/"+aid+"/resultado", url.Values{"status": {"feito"}}), http.StatusBadRequest, "status")
	quer(t, ana.post("/visitas/"+id+"/amostras/"+aid+"/resultado", url.Values{"status": {"concluido"}, "aegypti": {"-1"}}), http.StatusBadRequest, "Aedes aegypti")
	quer(t, ana.post("/visitas/"+id+"/amostras/"+aid+"/resultado", url.Values{"status": {"concluido"}, "aegypti": {"5"}, "albopictus": {"2"}, "negativo": {"0"}}), http.StatusSeeOther)
	a, _ := m.cliQ.GetAmostra(t.Context(), lista[1].ID)
	if a.Status != "concluido" || a.TotalAegypti != 5 || a.TotalAlbopictus != 2 || !a.ResultadoEm.Valid || !a.ResultadoPor.Valid {
		t.Fatalf("resultado gravado: %+v", a)
	}
	quer(t, ana.post("/visitas/"+id+"/amostras/"+aid, url.Values{"deposito": {"B"}, "larvas": {"9"}, "pupas": {"2"}, "observacao": {"corrigido"}}), http.StatusSeeOther)
	// A amostra de outra visita não abre por esta URL.
	quer(t, ana.post("/visitas/"+semAmostra+"/amostras/"+aid+"/resultado", url.Values{"status": {"concluido"}}), http.StatusNotFound)

	// Listagem e filtros.
	quer(t, ana.get("/laboratorio"), http.StatusOK, "Concluído")
	quer(t, ana.get("/laboratorio?status=concluido"), http.StatusOK, "Tubito")
	quer(t, ana.get("/laboratorio?status=em_analise&tubito=999"), http.StatusOK, "Nenhuma amostra encontrada")

	// Excluir: a visita não vai para a lixeira com tubito ativo; o tubito vai.
	quer(t, ana.post("/visitas/"+id+"/excluir", url.Values{}), http.StatusConflict, "Não foi possível excluir")
	quer(t, ana.post("/visitas/"+id+"/amostras/"+aid+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/lixeira?tipo=amostra"), http.StatusOK, "Tubito")

	// Quem só consulta lê o laboratório, mas não altera resultado nem cria tubito.
	carla := m.comCarla(t)
	quer(t, carla.post("/visitas/"+id+"/amostras", url.Values{"larvas": {"1"}}), http.StatusForbidden)
	quer(t, carla.post("/visitas/"+id+"/amostras/"+aid+"/resultado", url.Values{"status": {"concluido"}}), http.StatusForbidden)

	ev := m.eventos(t)
	for _, ac := range []string{"amostra.criar", "amostra.editar", "amostra.resultado", "amostra.excluir"} {
		if achar(ev, ac) == nil {
			t.Errorf("faltou %s na auditoria", ac)
		}
	}
}

func TestLaboratorioTubitoDigitadoECicloFechado(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)
	if err := m.cliQ.SetClienteInfo(t.Context(), sqlccliente.SetClienteInfoParams{Chave: "visita_tubito_automatico", Valor: "0"}); err != nil {
		t.Fatal(err)
	}
	quer(t, ana.post("/visitas", c.form(agente, "amostra", "1")), http.StatusSeeOther)
	vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	id := s64(vs[0].ID)
	quer(t, ana.post("/visitas/"+id+"/amostras", url.Values{"larvas": {"1"}}), http.StatusBadRequest, "número do tubito")
	quer(t, ana.post("/visitas/"+id+"/amostras", url.Values{"tubito": {"T-77"}}), http.StatusSeeOther)
	quer(t, ana.post("/visitas/"+id+"/amostras", url.Values{"tubito": {"T-77"}}), http.StatusBadRequest, "Já existe uma amostra")

	// Ciclo fechado: o resultado exige a permissão própria do laboratório.
	lab := m.usuarioCom(t, "Lia Lab", "lia@nm.gov.br", "visita_visualizar", "visita_laboratorio_visualizar", "visita_laboratorio_adicionar")
	lista, _ := m.cliQ.ListAmostras(t.Context(), listarAmostras())
	aid := s64(lista[0].ID)
	quer(t, lab.post("/visitas/"+id+"/amostras/"+aid+"/resultado", url.Values{"status": {"concluido"}}), http.StatusSeeOther)
	quer(t, ana.post("/ciclos/"+s64(c.ciclo), formCiclo("01 Ciclo", time.Now().UTC().AddDate(0, 0, -60).Format("2006-01-02"),
		time.Now().UTC().AddDate(0, 0, 60).Format("2006-01-02"), url.Values{"atividade": {"1", "3"}, "fechado": {"1"}})), http.StatusSeeOther)
	quer(t, lab.post("/visitas/"+id+"/amostras/"+aid+"/resultado", url.Values{"status": {"em_analise"}}), http.StatusForbidden, "está fechado")
	quer(t, ana.post("/visitas/"+id+"/amostras/"+aid+"/resultado", url.Values{"status": {"em_analise"}}), http.StatusSeeOther)
}

func listarAmostras() sqlccliente.ListAmostrasParams {
	return sqlccliente.ListAmostrasParams{Limite: 100}
}

func TestLaboratorioFiltrosComoNoGeoControle(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)

	quer(t, ana.post("/visitas", c.form(agente, "amostra", "1")), http.StatusSeeOther)
	vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	vid := s64(vs[0].ID)
	quer(t, ana.post("/visitas/"+vid+"/amostras", url.Values{"deposito": {"A1"}, "larvas": {"2"}}), http.StatusSeeOther)
	loc, _ := m.cliQ.GetLocalQualquer(t.Context(), c.local)
	visita, _ := m.cliQ.GetVisita(t.Context(), vs[0].ID)

	acha := "/visitas/" + vid + "#amostras"
	am, err := m.cliQ.ListAmostras(t.Context(), sqlccliente.ListAmostrasParams{Limite: 10})
	if err != nil || len(am) == 0 {
		t.Fatalf("amostras = %d, %v", len(am), err)
	}
	dia := visita.Data.Time.Format("2006-01-02")
	dia2 := visita.Data.Time.AddDate(0, 0, 1).Format("2006-01-02")
	casos := []struct {
		filtro string
		acha   bool
	}{
		{"", true},
		{"numero=" + loc.Numero, true}, {"numero=99999", false},
		{"bairro=" + s64(loc.BairroID), true}, {"bairro=" + s64(loc.BairroID+1000), false},
		{"quarteirao=" + s64(loc.QuarteiraoID), true}, {"quarteirao=" + s64(loc.QuarteiraoID+1000), false},
		{"lado=" + loc.Lado, true}, {"lado=ZZ", false},
		{"visita=" + vid, true}, {"visita=987654", false},
		{"atividade=1", true}, {"atividade=3", false},
		{"semana=" + strconv.Itoa(int(visita.Semana.Int16)), true}, {"semana=" + strconv.Itoa(int(visita.Semana.Int16)%52+1), false},
		{"bairro=" + s64(loc.BairroID) + "&numero=" + loc.Numero + "&atividade=1&visita=" + vid, true},
		{"endereco=" + url.QueryEscape(loc.Endereco[:3]), true}, {"endereco=zzzzzz", false},
		{"tubito=" + url.QueryEscape(am[0].NumeroTubito), true}, {"tubito=zzzz", false},
		{"status=" + am[0].Status, true}, {"status=concluido", false},
		{"ciclo=" + s64(visita.CicloID), true}, {"ciclo=" + s64(visita.CicloID+1000), false},
		{"agente=" + s64(visita.AgenteID), true}, {"agente=" + s64(visita.AgenteID+1000), false},
		{"de=" + dia + "&ate=" + dia, true}, {"de=" + dia2, false}, {"ate=2000-01-01", false},
	}
	for _, k := range casos {
		corpo := ana.get("/laboratorio?" + k.filtro).Body.String()
		if strings.Contains(corpo, acha) != k.acha {
			t.Errorf("filtro %q: deveria achar = %v", k.filtro, k.acha)
		}
	}
	quer(t, ana.get("/laboratorio"), http.StatusOK, "Número", "Bairro", "Quarteirão", "Lado", "ID da visita", "Atividade", "Semana")
}

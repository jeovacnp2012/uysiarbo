package server

import (
	"net/http"
	"net/url"
	"testing"
	"uysiarbo/internal/db/sqlccliente"
)

func TestReconhecimentoGeografico(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	loc, err := m.cliQ.GetLocal(t.Context(), c.local)
	if err != nil {
		t.Fatal(err)
	}
	bairro := s64(loc.BairroID)

	quer(t, ana.get("/reconhecimentos"), http.StatusOK, "Nenhum reconhecimento")
	quer(t, ana.get("/reconhecimentos/novo"), http.StatusOK, "Infraestrutura", "Ocorrência ou risco")

	// Validações: nada é gravado.
	quer(t, ana.post("/reconhecimentos", url.Values{"bairro_id": {bairro}}), http.StatusBadRequest, "Escolha o bairro e o ciclo")
	quer(t, ana.post("/reconhecimentos", url.Values{"bairro_id": {bairro}, "ciclo_id": {s64(c.ciclo)}, "habitantes": {"abc"}}), http.StatusBadRequest, "Habitantes")
	quer(t, ana.post("/reconhecimentos", url.Values{"bairro_id": {bairro}, "ciclo_id": {s64(c.ciclo)}, "latitude": {"-13.8"}}), http.StatusBadRequest, "latitude e longitude")

	form := url.Values{"bairro_id": {bairro}, "ciclo_id": {s64(c.ciclo)}, "data_inicio": {"2026-09-01"}, "data_fim": {"2026-09-10"},
		"imoveis_residenciais": {"120"}, "habitantes": {"400"}, "infra_energia": {"sim"}, "risco_dengue": {"sim"}, "latitude": {"-13,83"}, "longitude": {"-56.08"}}
	quer(t, ana.post("/reconhecimentos", form), http.StatusSeeOther)
	quer(t, ana.post("/reconhecimentos", form), http.StatusBadRequest, "Já existe um reconhecimento")

	lista, err := m.cliQ.ListReconhecimentos(t.Context(), listarReconhecimentos())
	if err != nil || len(lista) != 1 {
		t.Fatalf("lista = %d, %v", len(lista), err)
	}
	id := s64(lista[0].ID)
	quer(t, ana.get("/reconhecimentos"), http.StatusOK, "400")
	quer(t, ana.get("/reconhecimentos/"+id), http.StatusOK, "120", "Reconhecimento geográfico")

	form.Set("habitantes", "450")
	quer(t, ana.post("/reconhecimentos/"+id, form), http.StatusSeeOther)
	quer(t, ana.get("/reconhecimentos/"+id), http.StatusOK, "450")

	// Excluir vai para a lixeira e dá para criar outro do mesmo bairro e ciclo.
	quer(t, ana.post("/reconhecimentos/"+id+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/reconhecimentos/"+id), http.StatusNotFound)
	quer(t, ana.post("/reconhecimentos", form), http.StatusSeeOther)

	sem := m.usuarioCom(t, "Sem Rec", "semrec@nm.gov.br", "visita_visualizar")
	quer(t, sem.get("/reconhecimentos"), http.StatusForbidden)
}

func TestResumoSemanal(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)

	quer(t, ana.get("/resumosemanal"), http.StatusOK, "Nenhum resumo gerado")
	quer(t, ana.post("/resumosemanal", url.Values{"ciclo_id": {s64(c.ciclo)}, "semana": {"99"}}), http.StatusBadRequest, "informe a semana")

	quer(t, ana.post("/visitas", c.form(agente, "achou", "1", "a1", "3", "total_eliminado", "3")), http.StatusSeeOther)
	quer(t, ana.post("/visitas", c.form(agente, "pendencia", "fechado")), http.StatusSeeOther)
	vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas())
	semana := ""
	for _, v := range vs {
		if v.Semana.Valid {
			semana = s64(int64(v.Semana.Int16))
		}
	}
	if semana == "" {
		t.Fatal("visita sem semana")
	}
	quer(t, ana.post("/resumosemanal", url.Values{"ciclo_id": {s64(c.ciclo)}, "semana": {"53"}}), http.StatusBadRequest, "Não há visitas")

	rec := ana.post("/resumosemanal", url.Values{"ciclo_id": {s64(c.ciclo)}, "semana": {semana}})
	quer(t, rec, http.StatusSeeOther)
	destino := rec.Header().Get("Location")
	quer(t, ana.get(destino), http.StatusOK, "Resumo semanal", "1 fechadas", "Depósitos encontrados", "Imprimir")
	quer(t, ana.get("/resumosemanal"), http.StatusOK, "Todos os bairros")

	quer(t, ana.post(destino+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(destino), http.StatusNotFound)

	sem := m.usuarioCom(t, "Sem Res", "semres@nm.gov.br", "visita_visualizar")
	quer(t, sem.get("/resumosemanal"), http.StatusForbidden)
}

func listarReconhecimentos() sqlccliente.ListReconhecimentosParams {
	return sqlccliente.ListReconhecimentosParams{}
}

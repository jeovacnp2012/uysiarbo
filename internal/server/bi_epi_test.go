package server

import (
	"net/http"
	"testing"
)

func TestBiEpidemiologico(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	ctx := t.Context()

	var doenca int64
	if err := m.pool.QueryRow(ctx, `INSERT INTO c_novamutum.doencas (nome) VALUES ('Dengue') RETURNING id`).Scan(&doenca); err != nil {
		t.Fatal(err)
	}
	insere := func(resultado, inicio string) {
		if _, err := m.pool.Exec(ctx, `INSERT INTO c_novamutum.investigacoes (doenca_id, paciente, data_inicio, situacao, resultado)
			VALUES ($1, 'Paciente', $2::date, 'encerrada', $3)`, doenca, inicio, resultado); err != nil {
			t.Fatal(err)
		}
	}
	insere("positivo", "2026-03-10")
	insere("positivo", "2026-03-11")
	insere("negativo", "2026-03-12")
	insere("aguardando", "2026-03-13")
	insere("positivo", "2020-01-05") // fora do período filtrado

	quer(t, ana.get("/bi-epidemiologico?ano=2026"), http.StatusOK,
		"B.I. Epidemiológico", "4 investigações em 2026, 2 confirmadas", "Taxa de confirmação de 66.7%", "Dengue", "SE 10/2026", "Sem bairro",
		"Quatro anos de investigações", "Produção de campo por ciclo", "Fontes e limites", `id="bie-dados"`)
	// outro ano e filtro por doença
	quer(t, ana.get("/bi-epidemiologico?ano=2020"), http.StatusOK, "1 investigações em 2020, 1 confirmadas", "SE 2/2020")
	quer(t, ana.get("/bi-epidemiologico?ano=2026&doenca="+s64(doenca)), http.StatusOK, "Dengue: 4 investigações em 2026")

	sem := m.usuarioCom(t, "Sem Bi", "sembi@nm.gov.br", "visita_visualizar")
	quer(t, sem.get("/bi-epidemiologico"), http.StatusForbidden)
}

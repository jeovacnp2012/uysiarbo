package server

import (
	"net/http"
	"net/url"
	"testing"
)

func TestRetencaoPorPrazo(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	ctx := t.Context()

	insere := func(titulo, situacao, quando string) int64 {
		var id int64
		err := m.pool.QueryRow(ctx, `INSERT INTO c_novamutum.denuncias (cidadao_nome, cidadao_telefone, tipo, titulo, situacao, criado_em, encerrada_em)
			VALUES ('Maria', '65988887777', 'Foco', $1, $2, $3::timestamptz, $3::timestamptz) RETURNING id`, titulo, situacao, quando).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	velha := insere("Antiga", "encerrada", "2020-01-10")
	recente := insere("Recente", "encerrada", "2099-01-10")
	aberta := insere("Aberta", "aguardando", "2020-01-10")

	quer(t, ana.get("/lgpd/retencao"), http.StatusOK, "Retenção de dados")
	quer(t, ana.post("/lgpd/retencao", url.Values{"lgpd_ret_denuncias": {"abc"}}), http.StatusBadRequest, "de 0 a")

	// prazo 0: nada é apagado
	quer(t, ana.post("/lgpd/retencao/executar", url.Values{}), http.StatusOK, "0 registro(s)")

	quer(t, ana.post("/lgpd/retencao", url.Values{"lgpd_ret_denuncias": {"12"}}), http.StatusOK, "1 registro(s) já passaram do prazo")
	quer(t, ana.post("/lgpd/retencao/executar", url.Values{}), http.StatusOK, "1 registro(s) anonimizado(s)")

	nome := func(id int64) string {
		var s string
		if err := m.pool.QueryRow(ctx, `SELECT cidadao_nome FROM c_novamutum.denuncias WHERE id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if nome(velha) != "" {
		t.Fatal("a denúncia vencida deveria ser anonimizada")
	}
	if nome(recente) != "Maria" || nome(aberta) != "Maria" {
		t.Fatal("denúncia recente ou aberta não pode ser mexida")
	}
	// rodar de novo não acha mais nada
	quer(t, ana.post("/lgpd/retencao/executar", url.Values{}), http.StatusOK, "0 registro(s)")

	sem := m.usuarioCom(t, "Sem Ret", "semret@nm.gov.br", "visita_visualizar")
	quer(t, sem.get("/lgpd/retencao"), http.StatusForbidden)
}

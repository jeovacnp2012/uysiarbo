package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"uysiarbo/internal/db/sqlccliente"
)

func TestEntrevistaCondicionalValidaSoOQueAparece(t *testing.T) {
	campos, msg := lerCamposDaConfiguracao(`[
	 {"label":"Viajou?","tipo":"select","opcoes":"Sim, Não","obrigatorio":true},
	 {"label":"Para onde?","tipo":"texto","obrigatorio":true,"cond_campo":"viajou","cond_valor":"Sim"},
	 {"label":"Só febre amarela","tipo":"texto","obrigatorio":true,"exibir_se_agravo":"Febre Amarela"}]`)
	if msg != "" {
		t.Fatal(msg)
	}
	if campos[0].Nome != "viajou" {
		t.Fatalf("identificador = %q", campos[0].Nome)
	}
	post := func(v url.Values) (respostas, string) {
		r, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_ = r.ParseForm()
		return lerRespostas(r, campos, "Dengue")
	}
	if _, m := post(url.Values{"c_viajou": {"Não"}}); m != "" {
		t.Fatalf("não deveria exigir 'Para onde' nem a de febre amarela: %s", m)
	}
	if _, m := post(url.Values{"c_viajou": {"Sim"}}); m == "" {
		t.Fatal("viajou = Sim deveria exigir 'Para onde'")
	}
	resp, m := post(url.Values{"c_viajou": {"Sim"}, "c_para_onde": {"Cuiabá"}})
	if m != "" || resp.texto("para_onde") != "Cuiabá" {
		t.Fatalf("resp = %v, msg = %q", resp, m)
	}
	if _, m := post(url.Values{"c_viajou": {"Talvez"}}); m == "" {
		t.Fatal("opção fora da lista deveria ser recusada")
	}
	if _, msg := lerCamposDaConfiguracao(`[{"label":"A","tipo":"select"}]`); msg == "" {
		t.Fatal("select sem opções deveria ser recusado")
	}
}

func TestInvestigacaoFluxoCompleto(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	campos := `[{"label":"Viajou?","tipo":"select","opcoes":"Sim, Não","obrigatorio":true}]`
	rec := ana.post("/entrevistas", url.Values{"codigo": {"teste_dengue"}, "titulo": {"Dengue"}, "campos": {campos}})
	quer(t, rec, http.StatusSeeOther)
	g, err := m.cliQ.ListEntrevistaGrupos(t.Context())
	if err != nil || len(g) != 1 {
		t.Fatalf("entrevistas = %d, %v", len(g), err)
	}
	quer(t, ana.post("/doencas", url.Values{"nome": {"Dengue"}, "entrevista": {s64(g[0].ID)}}), http.StatusSeeOther)
	ds, _ := m.cliQ.ListDoencas(t.Context())
	if len(ds) != 1 {
		t.Fatalf("doenças = %d", len(ds))
	}
	quer(t, ana.post("/doencas", url.Values{"nome": {"dengue"}}), http.StatusBadRequest) // nome repetido

	quer(t, ana.post("/investigacoes", url.Values{"doenca": {s64(ds[0].ID)}, "paciente": {""}}), http.StatusBadRequest)
	rec = ana.post("/investigacoes", url.Values{"doenca": {s64(ds[0].ID)}, "paciente": {"Maria Souza"}, "data_inicio": {"2026-10-20"}, "endereco": {"Rua A, 10"}})
	quer(t, rec, http.StatusSeeOther)
	caminho := local(rec)
	if !strings.HasPrefix(caminho, "/investigacoes/") {
		t.Fatalf("redirecionou para %q", caminho)
	}
	quer(t, ana.get("/investigacoes"), http.StatusOK)
	corpo := ana.get(caminho)
	quer(t, corpo, http.StatusOK)
	if !strings.Contains(corpo.Body.String(), "Maria Souza") || !strings.Contains(corpo.Body.String(), "Viajou?") {
		t.Fatal("a ficha deveria mostrar o paciente e a pergunta da entrevista")
	}

	// entrevista: obrigatória
	quer(t, ana.post(caminho+"/entrevista", url.Values{}), http.StatusBadRequest)
	quer(t, ana.post(caminho+"/entrevista", url.Values{"c_viajou": {"Sim"}}), http.StatusSeeOther)

	// não encerra sem resultado
	rec = ana.post(caminho+"/encerrar", url.Values{})
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("encerrar sem resultado deveria voltar com erro: %s", local(rec))
	}
	quer(t, ana.post(caminho+"/resultado?valor=positivo", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho+"/encerrar", url.Values{}), http.StatusSeeOther)
	var id int64
	for _, r := range m.investigacoes(t) {
		id = r.ID
		i, _ := m.cliQ.GetInvestigacaoQualquer(t.Context(), id)
		if i.Situacao != "encerrada" || i.Resultado != "positivo" || !i.ResponsavelID.Valid {
			t.Fatalf("estado = %s/%s responsável=%v", i.Situacao, i.Resultado, i.ResponsavelID.Valid)
		}
	}
	// encerrada: dados não mudam; reabrir permite
	quer(t, ana.post(caminho, url.Values{"doenca": {s64(ds[0].ID)}, "paciente": {"Outra"}}), http.StatusConflict)
	quer(t, ana.post(caminho+"/reabrir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(caminho, url.Values{"doenca": {s64(ds[0].ID)}, "paciente": {"Maria S. Souza"}}), http.StatusSeeOther)

	// vínculo de local inexistente
	quer(t, ana.post(caminho+"/local", url.Values{"local": {"999999"}}), http.StatusNotFound)

	// exclusão lógica
	quer(t, ana.post(caminho+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(caminho), http.StatusNotFound)
	if _, err := m.cliQ.GetInvestigacaoQualquer(t.Context(), id); err != nil {
		t.Fatalf("a investigação excluída deveria continuar no banco: %v", err)
	}
}

func (m *mundo) investigacoes(t *testing.T) []sqlccliente.ListInvestigacoesRow {
	t.Helper()
	rows, err := m.cliQ.ListInvestigacoes(t.Context(), sqlccliente.ListInvestigacoesParams{Limite: 100})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]sqlccliente.ListInvestigacoesRow, len(rows))
	copy(out, rows)
	return out
}

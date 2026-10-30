package server

import (
	"net/http"
	"net/url"
	"testing"
)

func TestImportacaoDeInvestigacoes(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.get("/importacoes/nova"), http.StatusOK, "Investigações")
	quer(t, ana.get("/importacoes/modelo/investigacoes.csv"), http.StatusOK, "paciente")

	quer(t, ana.post("/doencas", url.Values{"nome": {"Dengue"}}), http.StatusSeeOther)

	csv := "ID de origem;Doença / agravo;Paciente;Telefone;Data de início;Endereço;Situação;Resultado\n" +
		"900;Dengue;Maria Souza;(65) 99999-0000;28/10/2026;Rua A, 1;encerrada;negativo\n" + // criar
		"901;dengue;João Lima;;2026-10-20;;em atendimento;\n" + // criar (doença sem diferença de caixa)
		"902;Zika;Ana;;;;;\n" + // erro: doença não cadastrada
		"903;Dengue;;;;;;\n" + // erro: sem paciente
		"904;Dengue;Pedro;;32/13/2026;;;\n" + // erro: data
		"905;Dengue;Paula;;;;talvez;\n" + // erro: situação
		"900;Dengue;Repetida;;;;;\n" // erro: repetida na planilha

	id := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "investigacoes"}, "inv.csv", []byte(csv)))
	imp, _ := m.cliQ.GetImportacao(t.Context(), mustID(id))
	if imp.Status != "analisada" || imp.TotalLinhas != 7 || imp.PrevCriar != 2 || imp.PrevErro != 5 {
		t.Fatalf("análise: %+v", imp)
	}
	quer(t, ana.post("/importacoes/"+id+"/executar", url.Values{}), http.StatusSeeOther)
	imp = m.esperarImportacao(t, id)
	if imp.Status != "concluida" || imp.Criados != 2 || imp.Erros != 5 {
		t.Fatalf("resultado: %+v", imp)
	}
	i, err := m.cliQ.GetInvestigacaoPorIDExterno(t.Context(), textoOpcional("900"))
	if err != nil || i.Situacao != "encerrada" || i.Resultado != "negativo" || !i.EncerradaEm.Valid || i.Paciente != "Maria Souza" {
		t.Fatalf("investigação 900: %+v, %v", i, err)
	}

	// repetir não duplica
	id2 := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "investigacoes"}, "inv.csv", []byte(csv)))
	imp2, _ := m.cliQ.GetImportacao(t.Context(), mustID(id2))
	if imp2.PrevCriar != 0 || imp2.PrevIgnorar != 2 {
		t.Fatalf("segunda análise: %+v", imp2)
	}

	// desfazer manda para a Lixeira
	quer(t, ana.post("/importacoes/"+id+"/desfazer", url.Values{}), http.StatusSeeOther)
	if _, err := m.cliQ.GetInvestigacao(t.Context(), i.ID); err == nil {
		t.Fatal("depois de desfazer a investigação deveria estar na lixeira")
	}

	// relatório de totais
	quer(t, ana.get("/relatorios/investigacao-total?gerar=1"), http.StatusOK)
}

package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"uysiarbo/internal/importacao"
)

func TestDataDaImportacao(t *testing.T) {
	for entrada, esperada := range map[string]string{
		"04/01/2026": "2026-01-04", "2026-01-04": "2026-01-04", "04-01-2026": "2026-01-04", "04/01/26": "2026-01-04",
		"46026": "2026-01-04", // número de série do Excel
	} {
		d, ok := importacao.Data(entrada)
		if !ok || d.Format("2006-01-02") != esperada {
			t.Errorf("Data(%q) = %v %v, queria %s", entrada, d, ok, esperada)
		}
	}
	for _, ruim := range []string{"", "31/02/2026", "ontem", "2026-13-01", "12"} {
		if _, ok := importacao.Data(ruim); ok {
			t.Errorf("Data(%q) não deveria valer", ruim)
		}
	}
	_ = time.Second
}

func TestImportacaoDeCiclosDaPreviaAoDesfazer(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.get("/importacoes/nova"), http.StatusOK, "Ciclos")
	quer(t, ana.get("/importacoes/modelo/ciclos.csv"), http.StatusOK, "data_inicio")

	// Um ciclo que já existe, para testar ignorar e atualizar.
	quer(t, ana.post("/ciclos", formCiclo("01 Ciclo - 2026", "2026-01-04", "2026-02-28", url.Values{"atividade": {"1"}})), http.StatusSeeOther)

	csv := "Nome;Data início;Data final;Situação;Atividades;Painel público;Atividades do painel\n" +
		"01 Ciclo - 2026;04/01/2026;28/02/2026;operando;LIT;;\n" + // já existe: ignorar
		"02 Ciclo - 2026;01/03/2026;30/04/2026;operando;LIT, LI, PE;sim;LIT\n" + // criar, painel só com LIT
		"03 Ciclo - 2026;2026-05-01;2026-06-30;fechado;lit;sim;\n" + // criar; painel herda as atividades
		"04 Ciclo - 2026;31/02/2026;30/04/2026;;;;\n" + // erro: data
		"05 Ciclo - 2026;01/07/2026;30/06/2026;;;;\n" + // erro: final antes do início
		"06 Ciclo - 2026;01/07/2026;31/08/2026;talvez;;;\n" + // erro: situação
		"07 Ciclo - 2026;01/09/2026;31/10/2026;;XYZ;;\n" + // erro: atividade desconhecida
		"03 ciclo - 2026;01/11/2026;31/12/2026;;;;\n" // erro: repetida na planilha

	id := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "ciclos"}, "ciclos.csv", []byte(csv)))
	imp, _ := m.cliQ.GetImportacao(t.Context(), mustID(id))
	if imp.Status != "analisada" || imp.TotalLinhas != 8 || imp.PrevCriar != 2 || imp.PrevIgnorar != 1 || imp.PrevErro != 5 {
		t.Fatalf("análise: %+v", imp)
	}
	if cs, _ := m.cliQ.ListCiclos(t.Context()); len(cs) != 1 {
		t.Fatalf("a análise não grava nada: %d ciclos", len(cs))
	}
	corpo := ana.get("/importacoes/" + id + "?acao=erro").Body.String()
	for _, esperado := range []string{"data inválida", "anterior à data de início", "operando ou fechado", "atividade desconhecida", "repetida na planilha"} {
		if !strings.Contains(corpo, esperado) {
			t.Errorf("a prévia deveria explicar o erro %q", esperado)
		}
	}

	quer(t, ana.post("/importacoes/"+id+"/executar", url.Values{}), http.StatusSeeOther)
	imp = m.esperarImportacao(t, id)
	if imp.Status != "concluida" || imp.Criados != 2 || imp.Ignorados != 1 || imp.Erros != 5 {
		t.Fatalf("resultado: %+v", imp)
	}
	cs, _ := m.cliQ.ListCiclos(t.Context())
	if len(cs) != 3 {
		t.Fatalf("ciclos = %d", len(cs))
	}
	porNome := map[string]int64{}
	for _, c := range cs {
		porNome[c.Nome] = c.ID
	}
	c2, _ := m.cliQ.GetCiclo(t.Context(), porNome["02 Ciclo - 2026"])
	if dataISO(c2.DataInicio) != "2026-03-01" || !c2.PainelPublico || c2.Situacao != "operando" {
		t.Errorf("02: %+v", c2)
	}
	if a, _ := m.cliQ.ListAtividadesDoCiclo(t.Context(), c2.ID); len(a) != 3 {
		t.Errorf("atividades do 02 = %v", a)
	}
	if p, _ := m.cliQ.ListAtividadesDoPainel(t.Context(), c2.ID); len(p) != 1 {
		t.Errorf("painel do 02 = %v", p)
	}
	c3, _ := m.cliQ.GetCiclo(t.Context(), porNome["03 Ciclo - 2026"])
	if c3.Situacao != "fechado" || dataISO(c3.DataFinal) != "2026-06-30" {
		t.Errorf("03: %+v", c3)
	}
	if p, _ := m.cliQ.ListAtividadesDoPainel(t.Context(), c3.ID); len(p) != 1 {
		t.Errorf("o painel do 03 deveria herdar as atividades do ciclo: %v", p)
	}

	// Atualizar existente (só com o que vier preenchido) e depois desfazer.
	csv2 := "Nome;Data final;Situação\n01 Ciclo - 2026;15/03/2026;fechado\n"
	// Sem a coluna de início, a planilha precisa do mapeamento: segue para "Ligar as colunas".
	rec := ana.enviarArquivo("/importacoes", map[string]string{"tipo": "ciclos", "atualizar": "1"}, "u.csv", []byte(csv2))
	if rec.Code != http.StatusSeeOther || !strings.HasSuffix(local(rec), "/mapear") {
		t.Fatalf("faltando coluna obrigatória deveria levar ao mapeamento: %d %q", rec.Code, local(rec))
	}
	csv3 := "Nome;Data início;Data final;Situação\n01 Ciclo - 2026;04/01/2026;15/03/2026;fechado\n"
	id3 := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "ciclos", "atualizar": "1"}, "u.csv", []byte(csv3)))
	imp3, _ := m.cliQ.GetImportacao(t.Context(), mustID(id3))
	if imp3.PrevAtualizar != 1 {
		t.Fatalf("atualização: %+v", imp3)
	}
	quer(t, ana.post("/importacoes/"+id3+"/executar", url.Values{}), http.StatusSeeOther)
	m.esperarImportacao(t, id3)
	c1, _ := m.cliQ.GetCiclo(t.Context(), porNome["01 Ciclo - 2026"])
	if c1.Situacao != "fechado" || dataISO(c1.DataFinal) != "2026-03-15" {
		t.Fatalf("01 depois de atualizar: %+v", c1)
	}
	if a, _ := m.cliQ.ListAtividadesDoCiclo(t.Context(), c1.ID); len(a) != 1 {
		t.Errorf("as atividades não vieram na planilha e devem ser mantidas: %v", a)
	}
	quer(t, ana.post("/importacoes/"+id3+"/desfazer", url.Values{}), http.StatusSeeOther)
	c1, _ = m.cliQ.GetCiclo(t.Context(), porNome["01 Ciclo - 2026"])
	if c1.Situacao != "operando" || dataISO(c1.DataFinal) != "2026-02-28" {
		t.Errorf("desfazer deveria restaurar o 01: %+v", c1)
	}
	// Desfazer a primeira: os dois ciclos criados vão para a Lixeira.
	quer(t, ana.post("/importacoes/"+id+"/desfazer", url.Values{}), http.StatusSeeOther)
	if cs, _ := m.cliQ.ListCiclos(t.Context()); len(cs) != 1 {
		t.Errorf("depois de desfazer sobra só o ciclo antigo: %d", len(cs))
	}
	quer(t, ana.get("/lixeira?tipo=ciclo"), http.StatusOK, "02 Ciclo - 2026", "03 Ciclo - 2026")
}

func TestImportacaoDeCiclosExigePermissao(t *testing.T) {
	m := novoMundo(t)
	// Quem importa bairros, mas não gerencia ciclos, não vê nem envia o tipo ciclos.
	soBairros := m.usuarioCom(t, "Edu Lima", "edu@nm.gov.br", "conf_importacao", "bairro_visualizar", "bairro_adicionar")
	if strings.Contains(soBairros.get("/importacoes/nova").Body.String(), "Ciclos de trabalho") {
		t.Error("sem param_ciclo, a opção Ciclos não deveria aparecer")
	}
	rec := soBairros.enviarArquivo("/importacoes", map[string]string{"tipo": "ciclos"}, "c.csv", []byte("nome;data_inicio;data_final\nX;01/01/2026;02/01/2026"))
	quer(t, rec, http.StatusBadRequest, "Escolha o que importar")
}

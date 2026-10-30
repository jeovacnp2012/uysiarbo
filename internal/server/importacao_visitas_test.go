package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
)

func TestImportacaoDeVisitasEDeTubitos(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	if _, err := m.cliQ.SetLocalIdExterno(t.Context(), setLocalExterno(c.local, "L1")); err != nil {
		t.Fatal(err)
	}
	quer(t, ana.get("/importacoes/nova"), http.StatusOK, "Visitas", "Laboratório (tubitos)")
	quer(t, ana.get("/importacoes/modelo/visitas.csv"), http.StatusOK, "id_externo")
	quer(t, ana.get("/importacoes/modelo/amostras.csv"), http.StatusOK, "tubito")

	quer(t, ana.get("/importacoes/nova"), http.StatusOK)
	d := c.ontem
	csv := "id_externo;local;ciclo;atividade;agente;data;a1;coletou_amostra\n" +
		"v1;L1;01 Ciclo;LIT;" + emailAna + ";" + d + ";2;sim\n" + // criar, com depósito e amostra
		"v2;L1;01 Ciclo;LIT;" + emailAna + ";" + d + ";;nao\n" + // criar
		"v3;L9;01 Ciclo;LIT;" + emailAna + ";" + d + ";;\n" + // local desconhecido
		"v4;L1;99 Ciclo;LIT;" + emailAna + ";" + d + ";;\n" + // ciclo desconhecido
		"v5;L1;01 Ciclo;ZZZ;" + emailAna + ";" + d + ";;\n" + // atividade desconhecida
		"v6;L1;01 Ciclo;LIT;Fulano Inexistente;" + d + ";;\n" + // agente desconhecido
		"v1;L1;01 Ciclo;LIT;" + emailAna + ";" + d + ";;\n" // repetida na planilha

	id := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "visitas"}, "v.csv", []byte(csv)))
	imp, _ := m.cliQ.GetImportacao(t.Context(), mustID(id))
	if imp.Status != "analisada" || imp.PrevCriar != 2 || imp.PrevErro != 5 {
		t.Fatalf("análise: %+v", imp)
	}
	if vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas()); len(vs) != 0 {
		t.Fatalf("a análise não grava nada: %d", len(vs))
	}
	corpo := ana.get("/importacoes/" + id + "?acao=erro").Body.String()
	for _, esperado := range []string{"não encontrado", "atividade", "agente", "repetida na planilha"} {
		if !strings.Contains(corpo, esperado) {
			t.Errorf("a prévia deveria explicar o erro %q", esperado)
		}
	}
	quer(t, ana.post("/importacoes/"+id+"/executar", url.Values{}), http.StatusSeeOther)
	imp = m.esperarImportacao(t, id)
	if imp.Status != "concluida" || imp.Criados != 2 || imp.Erros != 5 {
		t.Fatalf("resultado: %+v", imp)
	}
	v1, err := m.cliQ.GetVisitaPorIDExterno(t.Context(), pgtype.Text{String: "v1", Valid: true})
	if err != nil || !v1.AchouDeposito || !v1.ColetouAmostra || !v1.Semana.Valid || v1.LocalID != c.local {
		t.Fatalf("v1: %+v (%v)", v1, err)
	}

	// Repetir a importação não duplica: tudo é ignorado.
	id2 := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "visitas"}, "v.csv", []byte(csv)))
	imp2, _ := m.cliQ.GetImportacao(t.Context(), mustID(id2))
	if imp2.PrevCriar != 0 || imp2.PrevIgnorar < 2 {
		t.Fatalf("reimportação: %+v", imp2)
	}

	// Tubitos.
	csvA := "visita;tubito;deposito;larvas;pupas;aegypti;status\n" +
		"v1;500;A1;4;1;3;concluido\n" + // criar com resultado
		"v1;501;B;2;0;;\n" + // criar, em análise
		"v2;502;A1;1;0;;\n" + // visita não coletou amostra
		"v9;503;A1;1;0;;\n" + // visita desconhecida
		"v1;500;A1;1;0;;\n" // tubito repetido na planilha
	ida := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "amostras"}, "a.csv", []byte(csvA)))
	impA, _ := m.cliQ.GetImportacao(t.Context(), mustID(ida))
	if impA.PrevCriar != 2 || impA.PrevErro != 3 {
		t.Fatalf("análise dos tubitos: %+v", impA)
	}
	quer(t, ana.post("/importacoes/"+ida+"/executar", url.Values{}), http.StatusSeeOther)
	m.esperarImportacao(t, ida)
	ams, _ := m.cliQ.ListAmostrasDaVisita(t.Context(), v1.ID)
	if len(ams) != 2 {
		t.Fatalf("tubitos da v1 = %d", len(ams))
	}
	for _, a := range ams {
		switch a.NumeroTubito {
		case "500":
			if a.Status != "concluido" || a.TotalAegypti != 3 || a.Larvas != 4 {
				t.Errorf("tubito 500: %+v", a)
			}
		case "501":
			if a.Status != "em_analise" {
				t.Errorf("tubito 501: %+v", a)
			}
		default:
			t.Errorf("tubito inesperado %s", a.NumeroTubito)
		}
	}

	// O selo do menu Laboratório conta os tubitos em análise (só o 501).
	quer(t, ana.get("/"), http.StatusOK, `class="selo-contagem" title="Em análise">1<`)

	// Desfazer os tubitos e depois as visitas: tudo vai para a Lixeira.
	quer(t, ana.post("/importacoes/"+ida+"/desfazer", url.Values{}), http.StatusSeeOther)
	if ams, _ := m.cliQ.ListAmostrasDaVisita(t.Context(), v1.ID); len(ams) != 0 {
		t.Errorf("tubitos depois de desfazer = %d", len(ams))
	}
	quer(t, ana.post("/importacoes/"+id+"/desfazer", url.Values{}), http.StatusSeeOther)
	if vs, _ := m.cliQ.ListVisitasRecentes(t.Context(), listarVisitas()); len(vs) != 0 {
		t.Errorf("visitas depois de desfazer = %d", len(vs))
	}
}

func setLocalExterno(id int64, ext string) sqlccliente.SetLocalIdExternoParams {
	return sqlccliente.SetLocalIdExternoParams{ID: id, IDExterno: pgtype.Text{String: ext, Valid: true}}
}

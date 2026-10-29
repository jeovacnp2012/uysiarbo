package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestImportacaoPelaAPIDaPreviaAoResultado(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	m.criarBairro(t, cid, "Centro")
	api := novaAPI(t, m, "importacoes:read", "importacoes:write", "bairros:write")
	corpo := `{"tipo":"bairros","linhas":[
		{"cidade":"Nova Mutum","nome":"Jardim A","zona":"Norte","id_externo":"sis-1","mutirao":true},
		{"cidade":"Nova Mutum","nome":"Centro"},
		{"cidade":"Outra Cidade","nome":"Vila X"},
		{"cidade":"Nova Mutum","nome":"Jardim B","latitude":-13.5}
	]}`

	rec := api.req(http.MethodPost, "/api/v1/importacoes", corpo)
	if rec.Code != http.StatusCreated || !strings.HasPrefix(rec.Header().Get("Location"), "/api/v1/importacoes/") {
		t.Fatalf("criar: %d %s", rec.Code, rec.Body.String())
	}
	var imp apiImportacao
	api.json(rec, &imp)
	if imp.Status != "analisada" || imp.TotalLinhas != 4 || imp.Previa.Criar != 1 || imp.Previa.Ignorar != 1 || imp.Previa.Erro != 2 || imp.Origem != "api_sistema" {
		t.Fatalf("prévia: %+v", imp)
	}
	if n, _ := m.cliQ.ListBairrosApi(t.Context(), listaTodosBairros()); len(n) != 1 {
		t.Fatalf("a prévia não grava nada: %d bairros", len(n))
	}

	// Conferir a prévia.
	url1 := "/api/v1/importacoes/" + s64(imp.ID)
	var obtida apiImportacao
	api.json(api.req(http.MethodGet, url1, ""), &obtida)
	if obtida.ID != imp.ID || obtida.Previa.Erro != 2 {
		t.Errorf("obter: %+v", obtida)
	}
	rec = api.req(http.MethodGet, url1+"/linhas?acao=erro", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "não está cadastrada") || !strings.Contains(rec.Body.String(), "juntas") {
		t.Errorf("linhas com erro: %d %s", rec.Code, rec.Body.String())
	}
	if ids, prox := api.lista(url1 + "/linhas?limit=3"); len(ids) != 3 || prox == "" {
		t.Errorf("paginação das linhas: %v %q", ids, prox)
	}
	quemAPI(t, api.req(http.MethodGet, url1+"/linhas?acao=talvez", ""), http.StatusBadRequest, "invalid_parameter")

	// Executar: 202, acompanhar, resultado.
	rec = api.req(http.MethodPost, url1+"/executar", "", "Idempotency-Key", "exec-1")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("executar: %d %s", rec.Code, rec.Body.String())
	}
	rep := api.req(http.MethodPost, url1+"/executar", "", "Idempotency-Key", "exec-1")
	if rep.Code != http.StatusAccepted || rep.Header().Get("Idempotent-Replayed") != "true" {
		t.Errorf("repetição idempotente: %d %q", rep.Code, rep.Header().Get("Idempotent-Replayed"))
	}
	final := m.esperarImportacao(t, s64(imp.ID))
	if final.Status != "concluida" || final.Criados != 1 || final.Ignorados != 1 || final.Erros != 2 {
		t.Fatalf("resultado: %+v", final)
	}
	api.json(api.req(http.MethodGet, url1, ""), &obtida)
	if obtida.Status != "concluida" || obtida.Andamento.Criados != 1 || obtida.Andamento.Percentual != 100 || obtida.ConcluidoEm == nil {
		t.Errorf("andamento final: %+v", obtida)
	}
	novo, err := m.cliQ.GetBairroPorIdExterno(t.Context(), textoNulo("sis-1"))
	if err != nil || novo.Nome != "Jardim A" || !novo.Mutirao || novo.Zona != "Norte" {
		t.Errorf("bairro criado: %+v %v", novo, err)
	}
	// Executar de novo não vale; descartar depois de concluída também não.
	quemAPI(t, api.req(http.MethodPost, url1+"/executar", ""), http.StatusConflict, "conflict")
	quemAPI(t, api.req(http.MethodPost, url1+"/descartar", ""), http.StatusConflict, "conflict")
	// A auditoria traz a credencial como autora, e o resumo da importação.
	var analisou, concluiu bool
	for _, e := range m.eventos(t) {
		if e.Acao == "importacao.analisar" && e.AtorTipo == "api" {
			analisou = true
		}
		if e.Acao == "importacao.concluir" && e.AtorTipo == "api" && mapa(e.Detalhes)["criados"] == float64(1) {
			concluiu = true
		}
	}
	if !analisou || !concluiu {
		t.Errorf("auditoria: analisar=%v concluir=%v", analisou, concluiu)
	}
	// A tela de importações enxerga o que veio pela API.
	quer(t, m.comAna(t).get("/importacoes/"+s64(imp.ID)), http.StatusOK, "Importação de Bairros", "API do sistema")
}

func TestImportacaoPelaAPIRecusasEEscopos(t *testing.T) {
	m := novoMundo(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	_ = cid
	completa := novaAPI(t, m, "importacoes:read", "importacoes:write", "bairros:write", "locais:write")
	soLeitura := novaAPI(t, m, "importacoes:read")
	semTipo := novaAPI(t, m, "importacoes:write", "importacoes:read")
	linha := `{"cidade":"Nova Mutum","nome":"X Bairro"}`

	// Escopos: precisa de importacoes:write e do write do tipo.
	quemAPI(t, soLeitura.req(http.MethodPost, "/api/v1/importacoes", `{"tipo":"bairros","linhas":[`+linha+`]}`), http.StatusForbidden, "insufficient_scope")
	rec := semTipo.req(http.MethodPost, "/api/v1/importacoes", `{"tipo":"bairros","linhas":[`+linha+`]}`)
	quemAPI(t, rec, http.StatusForbidden, "insufficient_scope")
	if !strings.Contains(rec.Body.String(), "bairros:write") {
		t.Error("a mensagem deveria dizer qual escopo falta")
	}
	quemAPI(t, completa.req(http.MethodGet, "/api/v1/importacoes/1", ""), http.StatusNotFound, "not_found")

	// Pedidos inválidos.
	for corpo, esperado := range map[string]string{
		`{"tipo":"usuarios","linhas":[` + linha + `]}`:                             "validation_error",
		`{"tipo":"bairros","linhas":[]}`:                                           "validation_error",
		`{"tipo":"bairros"}`:                                                       "validation_error",
		`{"tipo":"bairros","linhas":[{"nome":"Sem cidade"}]}`:                      "validation_error",
		`{"tipo":"bairros","linhas":[1,2]}`:                                        "validation_error",
		`{"tipo":"bairros","linhas":"texto"}`:                                      "validation_error",
		`{"tipo":"bairros","linhas":[` + linha + `],"extra":1}`:                    "invalid_json",
		`{"tipo":"bairros","atualizar_existentes":"sim","linhas":[` + linha + `]}`: "validation_error",
		`lixo`: "invalid_json",
	} {
		rec := completa.req(http.MethodPost, "/api/v1/importacoes", corpo)
		if codigoDeErro(t, rec) != esperado {
			t.Errorf("%.60s: code=%q status=%d, queria %s", corpo, codigoDeErro(t, rec), rec.Code, esperado)
		}
	}
	if cs, _ := m.cliQ.ListImportacoes(t.Context()); len(cs) != 0 {
		t.Errorf("nenhuma importação deveria ter sido criada: %d", len(cs))
	}

	// Descartar uma prévia; não dá para executar uma sem nada a gravar; só uma execução por vez.
	var imp apiImportacao
	completa.json(completa.req(http.MethodPost, "/api/v1/importacoes", `{"tipo":"bairros","linhas":[{"cidade":"Cidade Que Não Existe","nome":"Y"}]}`), &imp)
	quemAPI(t, completa.req(http.MethodPost, "/api/v1/importacoes/"+s64(imp.ID)+"/executar", ""), http.StatusConflict, "conflict")
	rec = completa.req(http.MethodPost, "/api/v1/importacoes/"+s64(imp.ID)+"/descartar", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"cancelada"`) {
		t.Errorf("descartar: %d %s", rec.Code, rec.Body.String())
	}
	var ok1 apiImportacao
	completa.json(completa.req(http.MethodPost, "/api/v1/importacoes", `{"tipo":"bairros","linhas":[`+linha+`]}`), &ok1)
	if _, err := m.pool.Exec(t.Context(), `UPDATE c_novamutum.importacoes SET status = 'executando' WHERE id = $1`, ok1.ID); err != nil {
		t.Fatal(err)
	}
	// "executando" sem motor (reinício): a API mostra interrompida, o que libera a fila.
	var vista apiImportacao
	completa.json(completa.req(http.MethodGet, "/api/v1/importacoes/"+s64(ok1.ID), ""), &vista)
	if vista.Status != "interrompida" {
		t.Errorf("status: %q", vista.Status)
	}
	quemAPI(t, completa.req(http.MethodPost, "/api/v1/importacoes/"+s64(ok1.ID)+"/executar", ""), http.StatusAccepted, "")
	final := m.esperarImportacao(t, s64(ok1.ID))
	if final.Status != "concluida" || final.Criados != 1 {
		t.Errorf("retomada pela API: %+v", final)
	}
	// Importar locais por API precisa do locais:write e do bairro/quarteirão já cadastrados.
	b := m.criarBairro(t, cid, "Centro")
	m.criarQuarteirao(t, b, "1")
	locais := `{"tipo":"locais","linhas":[{"bairro":"Centro","quarteirao":"1","tipo":"residencial","endereco":"Rua A","numero":"1","lado":"Par"},{"bairro":"Centro","quarteirao":"2","tipo":"residencial","endereco":"Rua B","numero":"1","lado":"Par"}]}`
	var il apiImportacao
	completa.json(completa.req(http.MethodPost, "/api/v1/importacoes", locais), &il)
	if il.Previa.Criar != 1 || il.Previa.Erro != 1 {
		t.Errorf("locais: %+v", il)
	}
	_ = url.Values{}
}

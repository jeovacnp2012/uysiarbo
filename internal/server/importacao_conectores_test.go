package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"uysiarbo/internal/db/sqlccliente"
)

func TestMapeamentoDeColunasSalvoEReaproveitado(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")
	m.criarQuarteirao(t, b, "1")

	// Colunas com nomes que o sistema não conhece: cai na tela de mapeamento, sem analisar nada ainda.
	csv := "Via;Nr;Quadra;Zona urbana;Categoria;Face\nRua A;10;1;Centro;residencial;Par\nRua B;12;1;Centro;comércio;Par\nRua C;14;9;Centro;residencial;Par\n"
	rec := ana.enviarArquivo("/importacoes", map[string]string{"tipo": "locais", "conector": "planilha"}, "estranha.csv", []byte(csv))
	id := idDaImportacao(t, rec)
	if !strings.HasSuffix(id, "/mapear") {
		t.Fatalf("deveria ir para o mapeamento: %q", local(rec))
	}
	id = strings.TrimSuffix(id, "/mapear")
	imp, _ := m.cliQ.GetImportacao(t.Context(), mustID(id))
	if imp.Status != "mapeando" || len(imp.Arquivo) == 0 || imp.TotalLinhas != 3 {
		t.Fatalf("estado: %+v", imp)
	}
	quer(t, ana.get("/importacoes/"+id), http.StatusOK, "Falta ligar as colunas", "Ligar as colunas")
	tela := ana.get("/importacoes/" + id + "/mapear")
	quer(t, tela, http.StatusOK, "Qual coluna da origem vai para cada campo", "Rua A", "Face", "Quadra", "obrigatório", "Analisar com este mapeamento")

	// Faltando campo obrigatório: recusa e mantém o que a pessoa escolheu.
	form := url.Values{"col_bairro": {"Zona urbana"}, "col_quarteirao": {"Quadra"}, "col_tipo": {"Categoria"}, "col_endereco": {"Via"}, "col_numero": {"Nr"}}
	quer(t, ana.post("/importacoes/"+id+"/mapear", form), http.StatusBadRequest, "Ligue os campos obrigatórios", "lado")
	// Nome de mapeamento inválido.
	form.Set("col_lado", "Face")
	form.Set("salvar_nome", "X")
	quer(t, ana.post("/importacoes/"+id+"/mapear", form), http.StatusBadRequest, "de 2 a 100 caracteres")
	// Escolhendo uma coluna que não existe na planilha: é ignorada (e aí falta o campo).
	bad := url.Values{"col_bairro": {"Zona urbana"}, "col_quarteirao": {"Quadra"}, "col_tipo": {"Categoria"}, "col_endereco": {"Inventada"}, "col_numero": {"Nr"}, "col_lado": {"Face"}}
	quer(t, ana.post("/importacoes/"+id+"/mapear", bad), http.StatusBadRequest, "endereco")

	// Certo, salvando o mapeamento.
	form.Set("salvar_nome", "Planilha da prefeitura")
	rec = ana.post("/importacoes/"+id+"/mapear", form)
	if rec.Code != http.StatusSeeOther || local(rec) != "/importacoes/"+id {
		t.Fatalf("mapear: %d %q %.200s", rec.Code, local(rec), rec.Body.String())
	}
	imp, _ = m.cliQ.GetImportacao(t.Context(), mustID(id))
	if imp.Status != "analisada" || len(imp.Arquivo) != 0 || imp.PrevCriar != 2 || imp.PrevErro != 1 {
		t.Fatalf("depois de mapear: status=%s arquivo=%d criar=%d erro=%d", imp.Status, len(imp.Arquivo), imp.PrevCriar, imp.PrevErro)
	}
	quer(t, ana.get("/importacoes/"+id), http.StatusOK, "Prévia: nada foi gravado ainda", "o quarteirão")
	// O mesmo nome não pode ser salvo de novo para a mesma origem e tipo.
	id2 := strings.TrimSuffix(idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "locais", "conector": "planilha"}, "estranha.csv", []byte(csv))), "/mapear")
	quer(t, ana.post("/importacoes/"+id2+"/mapear", form), http.StatusBadRequest, "Já existe um mapeamento com esse nome")
	ms, _ := m.cliQ.ListMapeamentosDeImportacao(t.Context(), sqlccliente.ListMapeamentosDeImportacaoParams{Conector: "planilha", Tipo: "locais"})
	if len(ms) != 1 || ms[0].Nome != "Planilha da prefeitura" {
		t.Fatalf("mapeamentos salvos: %+v", ms)
	}

	// Reaproveitar: com o mapeamento salvo, vai direto para a prévia.
	rec = ana.enviarArquivo("/importacoes", map[string]string{"tipo": "locais", "conector": "planilha", "mapeamento_id": s64(ms[0].ID)}, "estranha.csv", []byte(csv))
	novo := idDaImportacao(t, rec)
	if strings.Contains(novo, "/mapear") {
		t.Fatalf("com o mapeamento salvo não precisa mapear: %q", local(rec))
	}
	if i, _ := m.cliQ.GetImportacao(t.Context(), mustID(novo)); i.Status != "analisada" || i.PrevCriar != 2 {
		t.Errorf("reaproveitado: %+v", i)
	}
	// A tela de mapeamento oferece o salvo (e o aplica quando escolhido).
	quer(t, ana.get("/importacoes/"+id2+"/mapear"), http.StatusOK, "Planilha da prefeitura")
	// Mapeamento de outro tipo não vale.
	quer(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros", "conector": "planilha", "mapeamento_id": s64(ms[0].ID)}, "b.csv", []byte("cidade;nome\nNova Mutum;X")), http.StatusBadRequest, "não serve para esta origem")
	// "Revisar antes" leva ao mapeamento mesmo com as colunas certas.
	rev := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros", "conector": "planilha", "revisar": "1"}, "b.csv", []byte("cidade;nome\nNova Mutum;Bairro Novo")))
	if !strings.HasSuffix(rev, "/mapear") {
		t.Errorf("revisar: %q", rev)
	}
	// Descartar a que está em "mapeando" apaga o arquivo guardado.
	revID := strings.TrimSuffix(rev, "/mapear")
	quer(t, ana.post("/importacoes/"+revID+"/cancelar", url.Values{}), http.StatusSeeOther)
	if i, _ := m.cliQ.GetImportacao(t.Context(), mustID(revID)); i.Status != "cancelada" || len(i.Arquivo) != 0 {
		t.Errorf("descartada: %+v", i)
	}
	quer(t, ana.get("/importacoes/"+revID+"/mapear"), http.StatusSeeOther) // já não está mapeando

	// Gestão dos salvos: lista, excluir (lixeira), restaurar.
	quer(t, ana.get("/importacoes/mapeamentos"), http.StatusOK, "Planilha da prefeitura", "planilha · locais")
	quer(t, ana.post("/importacoes/mapeamentos/"+s64(ms[0].ID)+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/importacoes/mapeamentos"), http.StatusOK, "Nenhum mapeamento salvo")
	quer(t, ana.get("/lixeira?tipo=mapeamento"), http.StatusOK, "Planilha da prefeitura")
	quer(t, ana.post("/lixeira/mapeamento/"+s64(ms[0].ID)+"/restaurar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/importacoes/mapeamentos"), http.StatusOK, "Planilha da prefeitura")
	if achar(m.eventos(t), "importacao.mapeamento_salvar") == nil || achar(m.eventos(t), "mapeamento.excluir") == nil {
		t.Error("faltaram eventos de auditoria do mapeamento")
	}
}

func TestConectorGeoControleReconheceAsColunasDele(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Jardim das Flores")
	m.criarQuarteirao(t, b, "12")

	csv := "ID;Logradouro;Nº;Lado;Tipo;Bairro;Quadra;Cad_Situacao;Humanos;Proprietario_Whatsapp_Aceita\n" +
		"1700000001;Rua das Palmeiras;120;Par;Residencial;Jardim das Flores;12;Ativo;4;sim\n" +
		"1700000002;Rua das Palmeiras;122;Par;Comércio;Jardim das Flores;12;Desativado;;não\n"
	rec := ana.enviarArquivo("/importacoes", map[string]string{"tipo": "locais", "conector": "geocontrole"}, "geocontrole.csv", []byte(csv))
	id := idDaImportacao(t, rec)
	if strings.Contains(id, "/mapear") {
		t.Fatalf("os nomes do Geo Controle deveriam ser reconhecidos sem mapear: %q", local(rec))
	}
	imp, _ := m.cliQ.GetImportacao(t.Context(), mustID(id))
	if imp.Conector != "geocontrole" || imp.PrevCriar != 2 || imp.PrevErro != 0 {
		linhas, _ := m.cliQ.ListImportacaoLinhas(t.Context(), sqlccliente.ListImportacaoLinhasParams{ImportacaoID: imp.ID, Limite: 10})
		for _, l := range linhas {
			t.Logf("linha %d: %s %s", l.Linha, l.Acao, l.Mensagem)
		}
		t.Fatalf("análise: conector=%s criar=%d erro=%d", imp.Conector, imp.PrevCriar, imp.PrevErro)
	}
	quer(t, ana.post("/importacoes/"+id+"/executar", url.Values{}), http.StatusSeeOther)
	if imp = m.esperarImportacao(t, id); imp.Status != "concluida" || imp.Criados != 2 {
		t.Fatalf("resultado: %+v", imp)
	}
	ls, _ := m.cliQ.ListLocaisApi(t.Context(), sqlccliente.ListLocaisApiParams{Limite: 10, IncluirExcluidos: false})
	if len(ls) != 2 || ls[0].IDExterno.String != "1700000001" || !ls[0].Ativo || ls[1].Ativo || ls[1].Tipo != "comercio" || ls[0].Humanos.Int32 != 4 || !ls[0].AceitaWhatsapp {
		t.Fatalf("locais do Geo Controle: %+v", ls)
	}
	// Bairros do Geo Controle também (cidade pelo nome, produção por "Entra_na_producao").
	bairros := "ID;Nome;Cidade;Zona;Entra_na_Producao\n55;Bairro Novo;Nova Mutum;Norte;não\n"
	bid := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros", "conector": "geocontrole"}, "b.csv", []byte(bairros)))
	quer(t, ana.post("/importacoes/"+bid+"/executar", url.Values{}), http.StatusSeeOther)
	if i := m.esperarImportacao(t, bid); i.Criados != 1 {
		t.Fatalf("bairros: %+v", i)
	}
	novo, _ := m.cliQ.GetBairroPorIdExterno(t.Context(), textoNulo("55"))
	if novo.Nome != "Bairro Novo" || novo.Producao || novo.Zona != "Norte" {
		t.Errorf("bairro: %+v", novo)
	}
	// A tela mostra a origem.
	quer(t, ana.get("/importacoes/nova"), http.StatusOK, "Geo Controle (planilha exportada)", "Geo Controle (API)", "Outra API (JSON)", "não as guarda")
}

func TestConectorAPIRESTBuscaOsDadosESeguraAsCredenciais(t *testing.T) {
	var auth string
	origem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/bairros":
			if auth != "Bearer token-secreto" {
				http.Error(w, "negado", 401)
				return
			}
			_, _ = w.Write([]byte(`{"resultado":{"itens":[{"cidade":"Nova Mutum","nome":"Via API 1","zona":"Norte","id":10},{"cidade":"Nova Mutum","nome":"Via API 2","mutirao":true,"id":11}]}}`))
		case "/objeto":
			_, _ = w.Write([]byte(`{"a":1}`))
		}
	}))
	defer origem.Close()

	redePrivadaNosTestes = true
	defer func() { redePrivadaNosTestes = false }()
	m := novoMundo(t)
	ana := m.comAna(t)
	m.criarCidade(t, "Nova Mutum", "MT")
	enviar := func(extra map[string]string) *httptest.ResponseRecorder {
		campos := map[string]string{"tipo": "bairros", "conector": "api_rest", "url": origem.URL + "/bairros", "token": "token-secreto", "caminho": "resultado.itens"}
		for k, v := range extra {
			campos[k] = v
		}
		return ana.enviarArquivo("/importacoes", campos, "", nil)
	}

	// Os campos da API não bateram com as colunas (id, não id_externo): primeiro precisa mapear, mas nome/cidade batem.
	rec := enviar(nil)
	id := idDaImportacao(t, rec)
	if strings.Contains(id, "/mapear") {
		t.Fatalf("cidade e nome batem sozinhos: %q", local(rec))
	}
	if auth != "Bearer token-secreto" {
		t.Errorf("a busca deveria levar o token: %q", auth)
	}
	imp, _ := m.cliQ.GetImportacao(t.Context(), mustID(id))
	if imp.Conector != "api_rest" || imp.PrevCriar != 2 || imp.OrigemInfo == "" || strings.Contains(imp.OrigemInfo, "/bairros") || strings.Contains(imp.ArquivoNome, "/bairros") {
		t.Fatalf("importação: %+v", imp)
	}
	// As credenciais não ficam em lugar nenhum do banco.
	var achou int
	_ = m.pool.QueryRow(t.Context(), `SELECT count(*) FROM c_novamutum.importacoes WHERE row_to_json(importacoes)::text LIKE '%token-secreto%'`).Scan(&achou)
	if achou != 0 {
		t.Error("o token da API não pode ser guardado")
	}
	quer(t, ana.post("/importacoes/"+id+"/executar", url.Values{}), http.StatusSeeOther)
	if i := m.esperarImportacao(t, id); i.Criados != 2 {
		t.Fatalf("resultado: %+v", i)
	}

	// Erros comuns com mensagem clara.
	quer(t, enviar(map[string]string{"token": "errado"}), http.StatusBadRequest, "HTTP 401")
	quer(t, enviar(map[string]string{"caminho": "nao.existe"}), http.StatusBadRequest, "não existe na resposta")
	quer(t, enviar(map[string]string{"url": origem.URL + "/objeto", "caminho": ""}), http.StatusBadRequest, "não é uma lista")
	quer(t, enviar(map[string]string{"url": "https://user:pw@exemplo.com/x"}), http.StatusBadRequest, "usuário e senha")
	quer(t, enviar(map[string]string{"url": ""}), http.StatusBadRequest, "não é válido")
	// Mapeando a API quando faltam colunas obrigatórias.
	semCidade := ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros", "conector": "api_rest", "url": origem.URL + "/bairros", "token": "token-secreto", "caminho": "resultado.itens", "revisar": "1"}, "", nil)
	if !strings.HasSuffix(idDaImportacao(t, semCidade), "/mapear") {
		t.Errorf("revisar leva ao mapeamento: %q", local(semCidade))
	}
}

func TestConectorAPIRecusaRedeInterna(t *testing.T) {
	m := novoMundo(t) // rede privada NÃO liberada (o padrão de produção)
	ana := m.comAna(t)
	for _, alvo := range []string{"https://127.0.0.1:9/x", "https://localhost/x", "https://169.254.169.254/latest/meta-data", "https://10.1.2.3/x"} {
		rec := ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros", "conector": "api_rest", "url": alvo}, "", nil)
		quer(t, rec, http.StatusBadRequest, "rede interna")
	}
	quer(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros", "conector": "api_rest", "url": "http://exemplo.com/x"}, "", nil), http.StatusBadRequest, "HTTPS")
	if cs, _ := m.cliQ.ListImportacoes(t.Context()); len(cs) != 0 {
		t.Errorf("nada deveria ter sido criado: %d", len(cs))
	}
}

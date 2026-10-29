package server

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"uysiarbo/internal/db/sqlccliente"
)

// enviarArquivo faz o POST multipart da tela de importação.
func (n *navegador) enviarArquivo(caminho string, campos map[string]string, nome string, dados []byte) *httptest.ResponseRecorder {
	n.t.Helper()
	if n.cookies[csrfCookie] == "" {
		n.get("/entrar")
	}
	var corpo bytes.Buffer
	mw := multipart.NewWriter(&corpo)
	_ = mw.WriteField("csrf", n.cookies[csrfCookie])
	for k, v := range campos {
		_ = mw.WriteField(k, v)
	}
	if nome != "" {
		fw, _ := mw.CreateFormFile("arquivo", nome)
		_, _ = fw.Write(dados)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, caminho, io.NopCloser(&corpo))
	req.Host = n.host
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for k, v := range n.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	n.h.ServeHTTP(rec, req)
	return rec
}

// idDaImportacao lê o id do redirecionamento /importacoes/{id}.
func idDaImportacao(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(local(rec), "/importacoes/") {
		t.Fatalf("envio: %d %q %.300s", rec.Code, local(rec), rec.Body.String())
	}
	return strings.TrimPrefix(local(rec), "/importacoes/")
}

// esperarImportacao espera a execução em segundo plano terminar.
func (m *mundo) esperarImportacao(t *testing.T, id string) sqlccliente.Importaco {
	t.Helper()
	limite := time.Now().Add(20 * time.Second)
	for {
		imp, err := m.cliQ.GetImportacao(t.Context(), mustID(id))
		if err != nil {
			t.Fatal(err)
		}
		if imp.Status != "executando" {
			return imp
		}
		if time.Now().After(limite) {
			t.Fatalf("a importação não terminou: %+v", imp)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestModelosDeImportacao(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	for _, tipo := range []string{"bairros", "quarteiroes", "locais"} {
		rec := ana.get("/importacoes/modelo/" + tipo + ".csv")
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") || !strings.Contains(rec.Body.String(), ";") {
			t.Errorf("%s csv: %d", tipo, rec.Code)
		}
		rec = ana.get("/importacoes/modelo/" + tipo + ".xlsx")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "spreadsheetml") || !bytes.HasPrefix(rec.Body.Bytes(), []byte("PK")) {
			t.Errorf("%s xlsx: %d", tipo, rec.Code)
		}
	}
	quer(t, ana.get("/importacoes/modelo/usuarios.csv"), http.StatusNotFound)
	quer(t, ana.get("/importacoes/modelo/bairros.pdf"), http.StatusNotFound)
	quer(t, ana.get("/importacoes/nova"), http.StatusOK, "Bairros", "Quarteirões", "Locais", "Analisar", "Colunas de cada tipo")
	quer(t, ana.get("/importacoes"), http.StatusOK, "Nenhuma importação feita")
	quer(t, ana.get("/"), http.StatusOK, `href="/importacoes"`, `href="/lixeira"`)
}

func TestImportacaoExigePermissaoEPapelSemRestricao(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	carla := m.comCarla(t)
	for _, p := range []string{"/importacoes", "/importacoes/nova", "/importacoes/modelo/bairros.csv", "/importacoes/1"} {
		quer(t, carla.get(p), http.StatusForbidden)
	}
	quer(t, carla.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros"}, "a.csv", []byte("x")), http.StatusForbidden)

	// Quem tem a permissão de importar mas só enxerga parte dos dados não importa.
	restrito := m.usuarioCom(t, "Dora Reis", "dora@nm.gov.br", "conf_importacao", "bairro_visualizar", "bairro_adicionar", "bairro_meubairro")
	quer(t, restrito.get("/importacoes/nova"), http.StatusForbidden)
	quer(t, restrito.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros"}, "a.csv", []byte("cidade;nome\nX;Y")), http.StatusForbidden)

	// Importar locais exige poder editar locais.
	soBairros := m.usuarioCom(t, "Edu Lima", "edu@nm.gov.br", "conf_importacao", "bairro_visualizar", "bairro_adicionar")
	quer(t, soBairros.get("/importacoes/nova"), http.StatusOK, "Bairros")
	if strings.Contains(soBairros.get("/importacoes/nova").Body.String(), "Imóveis e pontos") {
		t.Error("sem local_adicionar, a opção Locais não deveria aparecer")
	}
	rec := soBairros.enviarArquivo("/importacoes", map[string]string{"tipo": "locais"}, "a.csv", []byte("bairro;quarteirao\nX;1"))
	quer(t, rec, http.StatusBadRequest, "Escolha o que importar")
	_ = ana
}

func TestEnvioDePlanilhaRecusado(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	m.criarCidade(t, "Nova Mutum", "MT")
	enviar := func(tipo, nome string, dados string) *httptest.ResponseRecorder {
		campos := map[string]string{"tipo": tipo}
		return ana.enviarArquivo("/importacoes", campos, nome, []byte(dados))
	}
	quer(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros"}, "", nil), http.StatusBadRequest, "Escolha o arquivo")
	quer(t, enviar("", "a.csv", "cidade;nome\nA;B"), http.StatusBadRequest, "Escolha o que importar")
	quer(t, enviar("usuarios", "a.csv", "cidade;nome\nA;B"), http.StatusBadRequest, "Escolha o que importar")
	quer(t, enviar("bairros", "a.csv", ""), http.StatusBadRequest, "Planilha recusada", "vazio")
	quer(t, enviar("bairros", "a.csv", "cidade;nome\n"), http.StatusBadRequest, "não tem linhas de dados")
	// Faltando coluna obrigatória, o sistema leva à tela de ligar as colunas (nada é analisado nem gravado).
	for _, c := range []struct{ tipo, dados string }{{"bairros", "nome;zona\nCentro;Norte"}, {"locais", "bairro;quarteirao\nCentro;1"}} {
		rec := enviar(c.tipo, "a.csv", c.dados)
		if rec.Code != http.StatusSeeOther || !strings.HasSuffix(rec.Header().Get("Location"), "/mapear") {
			t.Errorf("%s: %d %q", c.tipo, rec.Code, rec.Header().Get("Location"))
		}
	}
	if cs, _ := m.cliQ.ListImportacoes(t.Context()); len(cs) != 2 {
		t.Errorf("só as duas aguardando mapeamento deveriam existir: %d", len(cs))
	}
}

func TestImportacaoDeBairrosDaPrevisaoAoDesfazer(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	existente := m.criarBairro(t, cid, "Centro")

	csv := "Cidade;UF;Nome;Zona;Mutirão;Latitude;Longitude;Observação\n" +
		"Nova Mutum;MT;Jardim das Flores;Norte;sim;-13.82;-56.08;ok\n" + // criar
		"Nova Mutum;MT;Centro;Sul;;;;x\n" + // já existe: ignorar (sem "atualizar existentes")
		"Cidade Que Não Existe;;Vila Nova;;;;;x\n" + // erro: cidade
		"Nova Mutum;;jardim das flores;;;;;x\n" + // erro: repetida na planilha
		"Nova Mutum;;Bairro Sem Par;;;-13.8;;x\n" + // erro: latitude sem longitude
		"Nova Mutum;;Vila Esperança;;talvez;;;x\n" // erro: sim/não inválido

	rec := ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros"}, "bairros.csv", []byte(csv))
	id := idDaImportacao(t, rec)
	imp, _ := m.cliQ.GetImportacao(t.Context(), mustID(id))
	if imp.Status != "analisada" || imp.TotalLinhas != 6 || imp.PrevCriar != 1 || imp.PrevIgnorar != 1 || imp.PrevAtualizar != 0 || imp.PrevErro != 4 || imp.ColunasIgnoradas != "Observação" {
		t.Fatalf("análise: %+v", imp)
	}
	if n, _ := m.cliQ.ListBairrosApi(t.Context(), listaTodosBairros()); len(n) != 1 {
		t.Fatalf("a análise não grava nada: bairros = %d", len(n))
	}

	// Prévia: números, filtro por ação, mensagens por linha, colunas ignoradas.
	quer(t, ana.get("/importacoes/"+id), http.StatusOK, "Prévia: nada foi gravado ainda", "serão criadas", "Confirmar e importar", "Observação", "Jardim das Flores")
	corpo := ana.get("/importacoes/" + id + "?acao=erro").Body.String()
	for _, quer := range []string{"não está cadastrada", "repetida na planilha (igual à linha 2)", "juntas", "sim ou não"} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("a prévia deveria explicar o erro %q", quer)
		}
	}
	quer(t, ana.get("/importacoes/"+id+"?acao=ignorar"), http.StatusOK, "já cadastrado")
	quer(t, ana.get("/importacoes"), http.StatusOK, "bairros.csv", "Aguardando confirmação")

	// Linhas com erro, para corrigir e reenviar.
	rec = ana.get("/importacoes/" + id + "/erros.csv")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "\xEF\xBB\xBFCidade;UF;Nome;Zona;Mutirão;Latitude;Longitude;Observação;motivo_do_erro") || strings.Count(rec.Body.String(), "\n") != 5 {
		t.Errorf("erros.csv: %d %q", rec.Code, rec.Body.String())
	}

	// Confirmar e acompanhar.
	rec = ana.post("/importacoes/"+id+"/executar", url.Values{})
	if rec.Code != http.StatusSeeOther || local(rec) != "/importacoes/"+id+"?ok=iniciada" {
		t.Fatalf("executar: %d %q", rec.Code, local(rec))
	}
	imp = m.esperarImportacao(t, id)
	if imp.Status != "concluida" || imp.Criados != 1 || imp.Ignorados != 1 || imp.Erros != 4 || imp.Processadas != 6 || imp.Atualizados != 0 {
		t.Fatalf("resultado: %+v", imp)
	}
	bs, _ := m.cliQ.ListBairrosApi(t.Context(), listaTodosBairros())
	if len(bs) != 2 {
		t.Fatalf("bairros = %d", len(bs))
	}
	var novo sqlccliente.Bairro
	for _, b := range bs {
		if b.Nome == "Jardim das Flores" {
			novo = b
		}
	}
	if !novo.Mutirao || novo.Zona != "Norte" || !novo.Latitude.Valid || novo.Status != "ativo" || novo.Categoria != "bairro" {
		t.Errorf("bairro criado: %+v", novo)
	}
	if c, _ := m.cliQ.GetBairro(t.Context(), existente); c.Zona != "" {
		t.Error("o bairro que já existia não pode mudar sem a opção de atualizar")
	}
	quer(t, ana.get("/importacoes/"+id), http.StatusOK, "Resultado", "1 criada(s)", "Desfazer esta importação")
	quer(t, ana.get("/importacoes/"+id+"/progresso"), http.StatusOK)
	if ana.get("/importacoes/"+id+"/progresso").Header().Get("HX-Refresh") != "true" {
		t.Error("terminada a importação, o progresso manda recarregar a página")
	}
	rel := ana.get("/importacoes/" + id + "/relatorio.csv").Body.String()
	for _, quer := range []string{"linha;analise;resultado;mensagem;registro;Cidade", "criado", "ignorado", "erro", "Jardim das Flores"} {
		if !strings.Contains(rel, quer) {
			t.Errorf("o relatório deveria conter %q", quer)
		}
	}
	// A auditoria tem o resumo (não um evento por bairro).
	if e := achar(m.eventos(t), "importacao.concluir"); e == nil || mapa(e.Detalhes)["criados"] != float64(1) || e.AtorNome != "Ana Souza" {
		t.Errorf("importacao.concluir: %+v", e)
	}
	if achar(m.eventos(t), "importacao.analisar") == nil || achar(m.eventos(t), "importacao.iniciar") == nil {
		t.Error("faltaram eventos de análise e início")
	}

	// Não dá para iniciar de novo, nem confirmar uma concluída.
	quer(t, ana.post("/importacoes/"+id+"/executar", url.Values{}), http.StatusConflict, "não está em um estado")

	// Desfazer: o criado vai para a lixeira (nada é apagado de verdade).
	quer(t, ana.post("/importacoes/"+id+"/desfazer", url.Values{}), http.StatusSeeOther)
	imp, _ = m.cliQ.GetImportacao(t.Context(), mustID(id))
	if imp.Status != "desfeita" || imp.Desfeitos != 1 {
		t.Fatalf("desfeita: %+v", imp)
	}
	if n, _ := m.cliQ.ListBairrosApi(t.Context(), listaTodosBairros()); len(n) != 1 {
		t.Errorf("depois de desfazer sobra só o bairro antigo: %d", len(n))
	}
	if b, err := m.cliQ.GetBairroQualquer(t.Context(), novo.ID); err != nil || !b.DeletedAt.Valid {
		t.Errorf("o bairro criado deveria estar na lixeira, não apagado: %+v %v", b, err)
	}
	quer(t, ana.get("/lixeira?tipo=bairro"), http.StatusOK, "Jardim das Flores")
	quer(t, ana.get("/importacoes/"+id), http.StatusOK, "Desfeita")
	if achar(m.eventos(t), "importacao.desfazer") == nil {
		t.Error("faltou importacao.desfazer")
	}
}

func TestImportacaoAtualizandoExistentesEDesfazendo(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")

	csv := "cidade,nome,zona,ibge\nNova Mutum,Centro,Norte,5106240\nNova Mutum,Centro Velho,,\n"
	id := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros", "atualizar": "1"}, "b.csv", []byte(csv)))
	imp, _ := m.cliQ.GetImportacao(t.Context(), mustID(id))
	if imp.PrevAtualizar != 1 || imp.PrevCriar != 1 || !imp.AtualizarExistentes {
		t.Fatalf("análise: %+v", imp)
	}
	quer(t, ana.get("/importacoes/"+id), http.StatusOK, `Modo "atualizar existentes"`)
	quer(t, ana.post("/importacoes/"+id+"/executar", url.Values{}), http.StatusSeeOther)
	imp = m.esperarImportacao(t, id)
	if imp.Status != "concluida" || imp.Atualizados != 1 || imp.Criados != 1 {
		t.Fatalf("resultado: %+v", imp)
	}
	atual, _ := m.cliQ.GetBairro(t.Context(), b)
	if atual.Zona != "Norte" || atual.Ibge != "5106240" || atual.Nome != "Centro" {
		t.Errorf("atualizado: %+v", atual)
	}
	// Mesma planilha de novo: nada a fazer ("sem mudanças") e a ficha avisa.
	id2 := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros", "atualizar": "1"}, "b.csv", []byte(csv)))
	imp2, _ := m.cliQ.GetImportacao(t.Context(), mustID(id2))
	if imp2.PrevCriar != 0 || imp2.PrevAtualizar != 0 || imp2.PrevIgnorar != 2 {
		t.Errorf("repetição: %+v", imp2)
	}
	quer(t, ana.get("/importacoes/"+id2), http.StatusOK, "Nada a criar ou atualizar")
	quer(t, ana.post("/importacoes/"+id2+"/executar", url.Values{}), http.StatusConflict)

	// Desfazer devolve o estado anterior do atualizado e manda o criado para a lixeira.
	quer(t, ana.post("/importacoes/"+id+"/desfazer", url.Values{}), http.StatusSeeOther)
	volta, _ := m.cliQ.GetBairro(t.Context(), b)
	if volta.Zona != "" || volta.Ibge != "" {
		t.Errorf("o bairro atualizado deveria voltar ao que era: %+v", volta)
	}
	if n, _ := m.cliQ.ListBairrosApi(t.Context(), listaTodosBairros()); len(n) != 1 {
		t.Errorf("bairros ativos = %d, queria 1", len(n))
	}
	// Descartar (cancelar) antes de gravar.
	quer(t, ana.post("/importacoes/"+id2+"/cancelar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/importacoes/"+id2), http.StatusOK, "Descartada")
	quer(t, ana.post("/importacoes/"+id2+"/executar", url.Values{}), http.StatusConflict)
}

func TestImportacaoDeQuarteiroesELocaisPorExcel(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	b := m.criarBairro(t, cid, "Centro")

	xlsx := func(linhas [][]string) []byte {
		f := excelize.NewFile()
		defer f.Close()
		for i, l := range linhas {
			for j, c := range l {
				ref, _ := excelize.CoordinatesToCellName(j+1, i+1)
				_ = f.SetCellValue("Sheet1", ref, c)
			}
		}
		buf, _ := f.WriteToBuffer()
		return buf.Bytes()
	}

	// Quarteirões em Excel.
	qid := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "quarteiroes"}, "q.xlsx", xlsx([][]string{
		{"Bairro", "Quarteirão", "Ativo"}, {"Centro", "1", ""}, {"Centro", "2", "não"}, {"Bairro Inexistente", "9", ""},
	})))
	imp, _ := m.cliQ.GetImportacao(t.Context(), mustID(qid))
	if imp.PrevCriar != 2 || imp.PrevErro != 1 {
		t.Fatalf("análise dos quarteirões: %+v", imp)
	}
	quer(t, ana.post("/importacoes/"+qid+"/executar", url.Values{}), http.StatusSeeOther)
	if imp = m.esperarImportacao(t, qid); imp.Status != "concluida" || imp.Criados != 2 {
		t.Fatalf("quarteirões: %+v", imp)
	}
	qs, _ := m.cliQ.ListQuarteiroesDoBairro(t.Context(), b)
	if len(qs) != 2 || qs[1].Ativo {
		t.Fatalf("quarteirões criados: %+v", qs)
	}

	// Locais: precisam do bairro e do quarteirão já cadastrados; aceitam rótulos do tipo ("Comércio").
	lid := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "locais"}, "l.xlsx", xlsx([][]string{
		{"Bairro", "Quarteirão", "Tipo", "Rua", "Número", "Lado", "Latitude", "Longitude", "Pessoas", "id_externo"},
		{"Centro", "1", "Residencial", "Rua das Palmeiras", "10", "Par", "-13.82", "-56.08", "4", "ext-1"},
		{"Centro", "1", "Comércio", "Rua das Palmeiras", "12", "Par", "", "", "", ""},
		{"Centro", "7", "Residencial", "Rua X", "1", "Par", "", "", "", ""},              // quarteirão inexistente
		{"Centro", "1", "Casa de praia", "Rua Y", "1", "Par", "", "", "", ""},            // tipo inválido
		{"Centro", "1", "Residencial", "Rua das Palmeiras", "10", "par", "", "", "", ""}, // repetida na planilha
		{"Centro", "2", "Terreno baldio", "Rua Z", "S/N", "Ímpar", "", "", "", ""},       // quarteirão desativado
	})))
	imp, _ = m.cliQ.GetImportacao(t.Context(), mustID(lid))
	if imp.PrevCriar != 2 || imp.PrevErro != 4 {
		t.Fatalf("análise dos locais: %+v", imp)
	}
	quer(t, ana.post("/importacoes/"+lid+"/executar", url.Values{}), http.StatusSeeOther)
	if imp = m.esperarImportacao(t, lid); imp.Status != "concluida" || imp.Criados != 2 {
		t.Fatalf("locais: %+v", imp)
	}
	ls, _ := m.cliQ.ListLocaisApi(t.Context(), listaTodosLocais())
	if len(ls) != 2 || ls[0].Tipo != "residencial" || ls[1].Tipo != "comercio" || !ls[0].IDExterno.Valid || ls[0].IDExterno.String != "ext-1" || !ls[0].Humanos.Valid || ls[0].Humanos.Int32 != 4 || !ls[0].CriadoPor.Valid {
		t.Fatalf("locais criados: %+v", ls)
	}
	// A planilha de locais de novo, agora sem repetições: o que existe é ignorado.
	again := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "locais"}, "l.xlsx", xlsx([][]string{
		{"Bairro", "Quarteirão", "Tipo", "Rua", "Número", "Lado"}, {"Centro", "1", "Residencial", "Rua das Palmeiras", "10", "Par"},
	})))
	if i, _ := m.cliQ.GetImportacao(t.Context(), mustID(again)); i.PrevIgnorar != 1 || i.PrevCriar != 0 {
		t.Errorf("repetição: %+v", i)
	}
	// Desfazer os locais manda para a lixeira; os quarteirões só podem ser desfeitos depois (têm locais ativos antes).
	quer(t, ana.post("/importacoes/"+qid+"/desfazer", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/importacoes/"+qid), http.StatusOK, "Desfeita em parte", "já estão em uso")
	quer(t, ana.post("/importacoes/"+lid+"/desfazer", url.Values{}), http.StatusSeeOther)
	if n, _ := m.cliQ.CountLocais(t.Context(), sqlccliente.CountLocaisParams{VerDesativados: true}); n != 0 {
		t.Errorf("locais ativos depois de desfazer = %d", n)
	}
	quer(t, ana.get("/lixeira?tipo=local"), http.StatusOK, "Rua das Palmeiras, 10", "Rua das Palmeiras, 12")
	quer(t, ana.post("/importacoes/"+qid+"/desfazer", url.Values{}), http.StatusSeeOther) // agora os quarteirões saem
	quer(t, ana.get("/importacoes/"+qid), http.StatusOK, "Desfeita")
}

func TestImportacaoInterrompidaEExecucaoUnica(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	cid := m.criarCidade(t, "Nova Mutum", "MT")
	_ = cid
	csv := "cidade;nome\nNova Mutum;Bairro 1\nNova Mutum;Bairro 2\n"
	id := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros"}, "b.csv", []byte(csv)))

	// "executando" sem o motor rodando (o servidor reiniciou): a ficha mostra Interrompida e permite retomar.
	if _, err := m.pool.Exec(t.Context(), `UPDATE c_novamutum.importacoes SET status = 'executando' WHERE id = $1`, mustID(id)); err != nil {
		t.Fatal(err)
	}
	quer(t, ana.get("/importacoes/"+id), http.StatusOK, "Interrompida", "Retomar")
	// enquanto outra estiver "executando", não se inicia uma nova
	id2 := idDaImportacao(t, ana.enviarArquivo("/importacoes", map[string]string{"tipo": "bairros"}, "c.csv", []byte("cidade;nome\nNova Mutum;Bairro 3\n")))
	if _, err := m.pool.Exec(t.Context(), `UPDATE c_novamutum.importacoes SET status = 'executando' WHERE id = $1`, mustID(id)); err != nil {
		t.Fatal(err)
	}
	quer(t, ana.post("/importacoes/"+id2+"/executar", url.Values{}), http.StatusConflict, "Já há uma importação em andamento")
	// ... mas a ficha da primeira, sem motor, volta a ser interrompida e libera a fila.
	ana.get("/importacoes/" + id)
	quer(t, ana.post("/importacoes/"+id+"/retomar", url.Values{}), http.StatusSeeOther)
	if imp := m.esperarImportacao(t, id); imp.Status != "concluida" || imp.Criados != 2 {
		t.Fatalf("retomada: %+v", imp)
	}
	quer(t, ana.post("/importacoes/"+id2+"/executar", url.Values{}), http.StatusSeeOther)
	if imp := m.esperarImportacao(t, id2); imp.Status != "concluida" || imp.Criados != 1 {
		t.Fatalf("segunda: %+v", imp)
	}
	// Interromper algo que não está rodando não faz nada (e não registra).
	quer(t, ana.post("/importacoes/"+id+"/interromper", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/importacoes/999"), http.StatusNotFound)
	quer(t, ana.get("/importacoes/abc"), http.StatusNotFound)
	if got := m.contarGenericos(t); got != 0 {
		t.Errorf("registros genéricos = %d", got)
	}
}

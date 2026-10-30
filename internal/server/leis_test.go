package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func formLei(numero, ano, nome string) url.Values {
	return url.Values{"numero": {numero}, "ano": {ano}, "nome": {nome}}
}

func TestLeisMunicipaisETrechos(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	quer(t, ana.get("/leis"), http.StatusOK, "Nenhuma lei cadastrada", "Nova lei")
	quer(t, ana.get("/leis/nova"), http.StatusOK, "Nome / ementa")
	quer(t, ana.post("/leis", formLei("", "2010", "Código Sanitário")), http.StatusBadRequest, "número da lei")
	quer(t, ana.post("/leis", formLei("12", "1800", "Código Sanitário")), http.StatusBadRequest, "ano da lei")
	quer(t, ana.post("/leis", formLei("12", "2010", "C")), http.StatusBadRequest, "nome da lei")
	quer(t, ana.post("/leis", url.Values{"numero": {"12"}, "ano": {"2010"}, "nome": {"Código Sanitário"}, "publicacao": {"31/02/2010"}}), http.StatusBadRequest, "data de publicação")

	rec := ana.post("/leis", url.Values{"numero": {"12"}, "ano": {"2010"}, "nome": {"Código Sanitário"}, "publicacao": {"2010-05-20"}})
	quer(t, rec, http.StatusSeeOther)
	if !strings.HasPrefix(rec.Header().Get("Location"), "/leis/") {
		t.Fatalf("depois de criar, vai à ficha: %q", rec.Header().Get("Location"))
	}
	quer(t, ana.post("/leis", formLei("12", "2010", "Outro nome")), http.StatusBadRequest, "Já existe uma lei")
	quer(t, ana.get("/leis"), http.StatusOK, "12/2010", "Código Sanitário", "20/05/2010", "Ativa")

	ls, _ := m.cliQ.ListLeis(t.Context())
	if len(ls) != 1 {
		t.Fatalf("leis = %+v", ls)
	}
	id := s64(ls[0].ID)
	quer(t, ana.get("/leis/"+id), http.StatusOK, "Código Sanitário", "Nenhum trecho cadastrado", "Novo trecho")
	quer(t, ana.get("/leis/999999"), http.StatusNotFound)

	// Trechos.
	quer(t, ana.post("/leis/"+id+"/trechos", url.Values{"artigo": {"Art. 1"}, "texto": {" "}}), http.StatusBadRequest, "texto do trecho")
	quer(t, ana.post("/leis/"+id+"/trechos", url.Values{"artigo": {strings.Repeat("x", 61)}, "texto": {"a"}}), http.StatusBadRequest, "no máximo 60")
	quer(t, ana.post("/leis/"+id+"/trechos", url.Values{"artigo": {"Art. 12, §2º"}, "texto": {"É proibido manter água parada."}}), http.StatusSeeOther)
	quer(t, ana.get("/leis/"+id), http.StatusOK, "Art. 12, §2º", "É proibido manter água parada.")
	ts, _ := m.cliQ.ListTrechosDaLei(t.Context(), ls[0].ID)
	if len(ts) != 1 {
		t.Fatalf("trechos = %+v", ts)
	}
	tid := s64(ts[0].ID)
	quer(t, ana.post("/leis/"+id+"/trechos/"+tid, url.Values{"artigo": {"Art. 12, §3º"}, "texto": {"É proibido manter água parada."}}), http.StatusSeeOther)
	quer(t, ana.post("/leis/"+id+"/trechos/"+tid, url.Values{"artigo": {"Art. 12, §3º"}, "texto": {"É proibido manter água parada."}}), http.StatusSeeOther) // igual: nada a registrar
	quer(t, ana.post("/leis/"+id+"/trechos/9999", url.Values{"texto": {"x"}}), http.StatusNotFound)
	quer(t, ana.post("/leis/999999/trechos", url.Values{"texto": {"x"}}), http.StatusNotFound)

	// Editar a lei: desativar.
	quer(t, ana.post("/leis/"+id, formLei("12", "2010", "Código Sanitário")), http.StatusSeeOther)
	quer(t, ana.get("/leis"), http.StatusOK, "Inativa")

	// Não dá para excluir a lei com trecho ativo; depois de excluir o trecho, dá.
	quer(t, ana.post("/leis/"+id+"/excluir", url.Values{}), http.StatusConflict, "trecho ativo")
	quer(t, ana.post("/leis/"+id+"/trechos/"+tid+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/leis/"+id), http.StatusOK, "Nenhum trecho cadastrado")
	quer(t, ana.get("/lixeira?tipo=lei_trecho"), http.StatusOK, "Art. 12, §3º", "Trechos de leis")
	quer(t, ana.post("/leis/"+id+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get("/leis/"+id), http.StatusNotFound)
	quer(t, ana.get("/lixeira?tipo=lei"), http.StatusOK, "Lei 12/2010")
	// O número e o ano podem ser reutilizados depois de excluída.
	quer(t, ana.post("/leis", formLei("12", "2010", "Código Sanitário Novo")), http.StatusSeeOther)

	ev := m.eventos(t)
	for _, a := range []string{"lei.criar", "lei.editar", "lei.trecho_criar", "lei.trecho_editar", "lei_trecho.excluir", "lei.excluir"} {
		if achar(ev, a) == nil {
			t.Errorf("faltou %s na auditoria", a)
		}
	}
	n := 0
	for _, e := range ev {
		if e.Acao == "lei.trecho_editar" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("lei.trecho_editar registrado %d vezes, queria 1", n)
	}
}

func TestLeisExigemPermissao(t *testing.T) {
	m := novoMundo(t)
	carla := m.comCarla(t)
	for _, p := range []string{"/leis", "/leis/nova", "/leis/1", "/parametros/geral", "/parametros/mobilidade"} {
		quer(t, carla.get(p), http.StatusForbidden)
	}
	quer(t, carla.post("/leis", formLei("1", "2020", "Lei")), http.StatusForbidden)
	quer(t, carla.post("/parametros/geral", url.Values{}), http.StatusForbidden)
}

func TestParametrosGeralEMobilidade(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	// Padrões.
	quer(t, ana.get("/parametros/geral"), http.StatusOK, "Numerar os tubitos automaticamente", "CEP padrão")
	quer(t, ana.get("/parametros/mobilidade"), http.StatusOK, "precisão do GPS", `value="20"`, `value="5"`, "Limpar dados")

	// Erros: nada é gravado.
	quer(t, ana.post("/parametros/mobilidade", url.Values{"app_aviso_precisao_gps": {"0"}, "app_coleta_gps_segundos": {"5"}, "app_dias_visitas": {"30"}, "app_dias_campanhas": {"30"}, "app_dias_investigacoes": {"30"}, "app_dias_anexos": {"30"}}), http.StatusBadRequest, "informe um número de 1 a 1000")
	quer(t, ana.post("/parametros/geral", url.Values{"cep_padrao": {"123"}}), http.StatusBadRequest, "8 dígitos")

	// Salvar o geral: o checkbox desmarcado vira "0"; o CEP aceita traço.
	quer(t, ana.post("/parametros/geral", url.Values{"campanha_estoque": {"1"}, "cep_padrao": {"78450-000"}}), http.StatusSeeOther)
	quer(t, ana.get("/parametros/geral?ok=1"), http.StatusOK, "Parâmetros salvos", `value="78450000"`)
	if v, _ := m.cliQ.GetClienteInfo(t.Context(), "visita_tubito_automatico"); v.Valor != "0" {
		t.Errorf("tubito automático = %q", v.Valor)
	}

	// Mobilidade com senha: o hash vai para o banco, nunca para a tela nem para a auditoria.
	mob := func(extra url.Values) url.Values {
		v := url.Values{"app_aviso_precisao_gps": {"15"}, "app_coleta_gps_segundos": {"10"}, "app_dias_visitas": {"45"}, "app_dias_campanhas": {"30"}, "app_dias_investigacoes": {"30"}, "app_dias_anexos": {"30"}}
		for k, vs := range extra {
			v[k] = vs
		}
		return v
	}
	quer(t, ana.post("/parametros/mobilidade", mob(url.Values{"app_senha_limpar_dados": {"123"}})), http.StatusBadRequest, "4 a 64 caracteres")
	quer(t, ana.post("/parametros/mobilidade", mob(url.Values{"app_senha_limpar_dados": {"segredo-do-app"}})), http.StatusSeeOther)
	rec := ana.get("/parametros/mobilidade")
	quer(t, rec, http.StatusOK, `value="15"`, `value="45"`, "Há uma senha definida")
	if strings.Contains(rec.Body.String(), "segredo-do-app") {
		t.Error("a senha não pode voltar à tela")
	}
	info, _ := m.cliQ.GetClienteInfo(t.Context(), chaveSenhaApp)
	if info.Valor == "" || info.Valor == "segredo-do-app" || !strings.HasPrefix(info.Valor, "$argon2") {
		t.Errorf("a senha deveria estar guardada como hash: %q", info.Valor)
	}
	// Sem digitar nada, a senha é mantida; marcando "remover", some.
	quer(t, ana.post("/parametros/mobilidade", mob(nil)), http.StatusSeeOther)
	if i2, _ := m.cliQ.GetClienteInfo(t.Context(), chaveSenhaApp); i2.Valor != info.Valor {
		t.Error("sem digitar nada, a senha deve ser mantida")
	}
	quer(t, ana.post("/parametros/mobilidade", mob(url.Values{"app_senha_limpar_dados_remover": {"1"}})), http.StatusSeeOther)
	if i3, _ := m.cliQ.GetClienteInfo(t.Context(), chaveSenhaApp); i3.Valor != "" {
		t.Error("a senha deveria ter sido removida")
	}

	ev := m.eventos(t)
	g, mo := achar(ev, "parametro.geral"), achar(ev, "parametro.mobilidade")
	if g == nil || mapa(g.Depois)["campanha_estoque"] != "1" || mo == nil {
		t.Fatalf("auditoria: geral=%+v mobilidade=%+v", g, mo)
	}
	for _, e := range ev {
		if e.Acao == "parametro.mobilidade" && (strings.Contains(string(e.Antes), "argon2") || strings.Contains(string(e.Depois), "argon2")) {
			t.Error("o hash da senha não pode ir para a auditoria")
		}
	}
}

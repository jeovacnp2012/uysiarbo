package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValorEmCentavosEReais(t *testing.T) {
	casos := map[string]int64{"": 0, "0": 0, "1.234,56": 123456, "1234,5": 123450, "1234.56": 123456, "R$ 10": 1000, "0,05": 5}
	for in, esperado := range casos {
		if got, ok := valorEmCentavos(in); !ok || got != esperado {
			t.Errorf("valorEmCentavos(%q) = %d, %v; esperado %d", in, got, ok, esperado)
		}
	}
	for _, ruim := range []string{"abc", "1,234", "-5", "1,2,3"} {
		if _, ok := valorEmCentavos(ruim); ok {
			t.Errorf("valorEmCentavos(%q) deveria falhar", ruim)
		}
	}
	if got := reais(123456); got != "R$ 1.234,56" {
		t.Errorf("reais = %q", got)
	}
	if got := reais(5); got != "R$ 0,05" {
		t.Errorf("reais = %q", got)
	}
}

func TestNotificacaoEAutoDeInfracao(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	hoje := time.Now().UTC().Format("2006-01-02")
	depois := time.Now().UTC().AddDate(0, 0, 10).Format("2006-01-02")
	antes := time.Now().UTC().AddDate(0, 0, -10).Format("2006-01-02")

	quer(t, ana.get("/notificacoes"), http.StatusOK)
	quer(t, ana.get("/notificacoes/nova"), http.StatusOK)
	quer(t, ana.get("/notificacoes/nova?local="+s64(c.local)), http.StatusOK)
	quer(t, ana.get("/notificacoes/nova?local=999999"), http.StatusNotFound)
	quer(t, ana.get("/infracoes"), http.StatusOK)

	form := func(extra map[string]string) url.Values {
		v := url.Values{"local": {s64(c.local)}, "motivo": {"Água parada no quintal"}, "data": {hoje}, "prazo": {depois}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.post("/notificacoes", form(map[string]string{"motivo": " "})), http.StatusBadRequest)
	quer(t, ana.post("/notificacoes", form(map[string]string{"prazo": antes})), http.StatusBadRequest)
	quer(t, ana.post("/notificacoes", form(map[string]string{"visita": "999999"})), http.StatusBadRequest)
	quer(t, ana.post("/notificacoes", form(map[string]string{"local": "999999"})), http.StatusNotFound)

	rec := ana.post("/notificacoes", form(nil))
	quer(t, rec, http.StatusSeeOther)
	nCam := local(rec)
	if !strings.HasPrefix(nCam, "/notificacoes/") {
		t.Fatalf("redirecionou para %q", nCam)
	}
	corpo := ana.get(nCam)
	quer(t, corpo, http.StatusOK, "NT-"+time.Now().UTC().Format("2006")+"0000001")
	// a segunda notificação recebe o número seguinte
	quer(t, ana.post("/notificacoes", form(nil)), http.StatusSeeOther)

	// editar
	quer(t, ana.post(nCam, form(map[string]string{"motivo": "Pneus acumulando água"})), http.StatusSeeOther)
	quer(t, ana.post(nCam, form(map[string]string{"motivo": ""})), http.StatusBadRequest)

	// auto: valor inválido volta com erro, válido cria
	autoForm := func(extra map[string]string) url.Values {
		v := url.Values{"descricao": {"Não sanou o problema no prazo"}, "valor": {"1.250,00"}, "data": {hoje}, "vencimento": {depois}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	rec = ana.post(nCam+"/auto", autoForm(map[string]string{"valor": "abc"}))
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("valor inválido deveria voltar com erro: %s", local(rec))
	}
	rec = ana.post(nCam+"/auto", autoForm(nil))
	quer(t, rec, http.StatusSeeOther)
	aCam, _, _ := strings.Cut(local(rec), "?")
	if !strings.HasPrefix(aCam, "/infracoes/") {
		t.Fatalf("redirecionou para %q", aCam)
	}
	quer(t, ana.get(aCam), http.StatusOK, "INF-"+time.Now().UTC().Format("2006")+"00001", "R$ 1.250,00")
	quer(t, ana.get("/infracoes"), http.StatusOK, "INF-")
	quer(t, ana.get(nCam), http.StatusOK, "Autuada")
	// segunda tentativa: a notificação já está autuada
	rec = ana.post(nCam+"/auto", autoForm(nil))
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("segundo auto deveria ser recusado: %s", local(rec))
	}
	quer(t, ana.post(nCam+"/resolver", url.Values{}), http.StatusSeeOther)
	if n := m.notificacaoDe(t, nCam); n != "autuada" {
		t.Fatalf("notificação autuada não pode ser resolvida: situação = %s", n)
	}

	// editar o auto, pagar e cancelar
	quer(t, ana.post(aCam, autoForm(map[string]string{"valor": "2.000,00"})), http.StatusSeeOther)
	quer(t, ana.get(aCam), http.StatusOK, "R$ 2.000,00")
	quer(t, ana.post(aCam, autoForm(map[string]string{"vencimento": antes})), http.StatusBadRequest)
	quer(t, ana.post(aCam+"/pagar", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(aCam), http.StatusOK, "Pago")
	quer(t, ana.post(aCam, autoForm(nil)), http.StatusConflict)
	rec = ana.post(aCam+"/cancelar", url.Values{"texto": {" "}})
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("cancelar sem motivo deveria voltar com erro: %s", local(rec))
	}
	quer(t, ana.post(aCam+"/cancelar", url.Values{"texto": {"Lançado por engano"}}), http.StatusSeeOther)
	if n := m.notificacaoDe(t, nCam); n != "aberta" {
		t.Fatalf("cancelar o auto deveria reabrir a notificação: situação = %s", n)
	}

	// com o auto cancelado, um novo auto é possível; resolver depois
	rec = ana.post(nCam+"/auto", autoForm(nil))
	quer(t, rec, http.StatusSeeOther)
	a2, _, _ := strings.Cut(local(rec), "?")
	// notificação com auto vivo não vai para a lixeira
	rec = ana.post(nCam+"/excluir", url.Values{})
	if strings.Contains(local(rec), "aviso=excluido") {
		t.Fatalf("não deveria excluir com auto ativo: %s", local(rec))
	}
	// auto não cancelado não se exclui
	rec = ana.post(a2+"/excluir", url.Values{})
	if !strings.Contains(local(rec), "caso_erro=") {
		t.Fatalf("auto aberto não deveria ser excluído: %s", local(rec))
	}
	quer(t, ana.post(a2+"/cancelar", url.Values{"texto": {"Duplicado"}}), http.StatusSeeOther)
	quer(t, ana.post(a2+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(a2), http.StatusNotFound)
	quer(t, ana.post(aCam+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(nCam+"/resolver", url.Values{"texto": {"Resolvido pelo morador"}}), http.StatusSeeOther)
	if n := m.notificacaoDe(t, nCam); n != "resolvida" {
		t.Fatalf("situação = %s", n)
	}
	quer(t, ana.post(nCam+"/reabrir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post(nCam+"/cancelar", url.Values{}), http.StatusSeeOther) // sem motivo: volta com erro
	quer(t, ana.post(nCam+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(nCam), http.StatusNotFound)
	quer(t, ana.get("/lixeira"), http.StatusOK)
}

func (m *mundo) notificacaoDe(t *testing.T, caminho string) string {
	t.Helper()
	id, err := parseIDDoCaminho(caminho)
	if err != nil {
		t.Fatal(err)
	}
	n, err := m.cliQ.GetNotificacaoQualquer(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return n.Situacao
}

func parseIDDoCaminho(caminho string) (int64, error) {
	return strconv.ParseInt(caminho[strings.LastIndex(caminho, "/")+1:], 10, 64)
}

func TestAbasDeParametros(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.get("/parametros/geral"), http.StatusOK, "Semanas SINAN", "Numeração")
	ano := strconv.Itoa(time.Now().UTC().Year())
	quer(t, ana.get("/parametros/semanas?ano="+ano), http.StatusOK, "Semanas de "+ano)
	quer(t, ana.get("/parametros/numeracao"), http.StatusOK, "{PREFIXO}-{ANO:4}{NUMERO:7}", "{PREFIXO}-{ANO:4}{NUMERO:5}", "Reset anual")
}

func TestSemanasDoAno(t *testing.T) {
	s := semanasDoAno(2026, "sinan")
	if len(s) != 52 || s[0].Inicio.Format("2006-01-02") != "2026-01-04" {
		t.Fatalf("2026 SINAN: %d semanas, início %v", len(s), s[0].Inicio)
	}
	if n := len(semanasDoAno(2025, "sinan")); n != 53 {
		t.Fatalf("2025 SINAN deveria ter 53 semanas, tem %d", n)
	}
}

func TestContaOvos(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.get("/parametros/contaovos"), http.StatusOK, "API Conta Ovos", "Chave de API")
	quer(t, ana.post("/parametros/contaovos", url.Values{"contaovos_ativo": {"1"}, "contaovos_tipo": {"x"}}), http.StatusBadRequest)
	quer(t, ana.post("/parametros/contaovos", url.Values{"contaovos_tipo": {"privada"}, "contaovos_uf": {"mtx"}}), http.StatusBadRequest)
	quer(t, ana.post("/parametros/contaovos", url.Values{"contaovos_ativo": {"1"}, "contaovos_tipo": {"privada"}, "contaovos_uf": {"mt"},
		"contaovos_municipio": {"Nova Mutum"}, "contaovos_chave": {"SEGREDO-123"}}), http.StatusSeeOther)
	rec := ana.get("/parametros/contaovos?ok=1")
	quer(t, rec, http.StatusOK, "Nova Mutum", "MT")
	if strings.Contains(rec.Body.String(), "SEGREDO-123") {
		t.Fatal("a chave não pode voltar para a tela")
	}
}

func TestCadastroDeAnosDeSemanas(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.get("/parametros/semanas"), http.StatusOK, "Calendário de Semanas SINAN", "02/01/2027", "2028")
	quer(t, ana.get("/parametros/semanas/2026"), http.StatusOK, "Editar ano SINAN", "O ano não pode ser alterado")
	quer(t, ana.get("/parametros/semanas/1999"), http.StatusNotFound)
	f := func(ano, pri, ult, sem string) url.Values {
		return url.Values{"ano": {ano}, "primeiro_dia": {pri}, "ultimo_dia": {ult}, "semanas": {sem}}
	}
	quer(t, ana.post("/parametros/semanas", f("2028", "2028-01-02", "2028-12-29", "52")), http.StatusBadRequest, "pedem 364 dias")
	quer(t, ana.post("/parametros/semanas", f("2026", "2026-01-04", "2027-01-02", "52")), http.StatusConflict, "já está cadastrado")
	quer(t, ana.post("/parametros/semanas", f("2028", "2028-01-02", "2029-01-06", "53")), http.StatusSeeOther)
	quer(t, ana.get("/parametros/semanas"), http.StatusOK, "06/01/2029")
	quer(t, ana.post("/parametros/semanas/2028", f("2028", "2028-01-02", "2029-01-06", "52")), http.StatusBadRequest)
	quer(t, ana.post("/parametros/semanas/2028", f("2028", "2028-01-02", "2028-12-30", "52")), http.StatusSeeOther)
	quer(t, ana.post("/parametros/semanas/2028/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.post("/parametros/semanas/2028/excluir", url.Values{}), http.StatusNotFound)
}

func TestAbasDentroDaPaginaEMenuPelaURL(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.get("/leis"), http.StatusOK, `id="aba-area"`, `hx-boost="true"`, `class="menu-subitem" href="/leis" aria-current`)
	quer(t, ana.get("/leis/nova"), http.StatusOK, `id="aba-area"`, `class="menu-subitem" href="/leis" aria-current`)
	quer(t, ana.get("/parametros/mobilidade"), http.StatusOK, `id="aba-area"`, `class="menu-subitem" href="/parametros/mobilidade" aria-current`)
	quer(t, ana.get("/parametros/contaovos"), http.StatusOK, `href="/parametros/contaovos" aria-current`)
	quer(t, ana.get("/locais/novo"), http.StatusOK, `class="menu-item" href="/locais" aria-current`)
	if strings.Contains(ana.get("/bairros").Body.String(), `id="aba-area"`) {
		t.Error("/bairros não tem abas")
	}
}

package server

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"
)

var linkDeSenha = regexp.MustCompile(`/redefinir-senha\?codigo=([A-Za-z0-9_-]{43})`)

func TestRecuperacaoDeSenha(t *testing.T) {
	m := novoMundo(t)
	n := m.em(t, hostNM)
	pedir := func(email string) {
		t.Helper()
		quer(t, n.post("/esqueci-senha", url.Values{"email": {email}, "g-recaptcha-response": {"ok"}}), http.StatusOK, "Se o e-mail estiver cadastrado")
	}

	quer(t, n.get("/esqueci-senha"), http.StatusOK, "Esqueci a senha")
	quer(t, n.get("/entrar"), http.StatusOK, "/esqueci-senha")

	// e-mail desconhecido: mesma resposta, nada enviado
	pedir("ninguem@nm.gov.br")
	if len(m.com.enviadas) != 0 {
		t.Fatalf("enviou %d e-mails para quem não tem conta", len(m.com.enviadas))
	}

	// captcha recusado
	m.captcha.ok = false
	quer(t, n.post("/esqueci-senha", url.Values{"email": {emailAna}}), http.StatusBadRequest, "robô")
	m.captcha.ok = true

	pedir(emailAna)
	if len(m.com.enviadas) != 1 || m.com.enviadas[0].Destinatario != emailAna {
		t.Fatalf("e-mails enviados = %+v", m.com.enviadas)
	}
	achado := linkDeSenha.FindStringSubmatch(m.com.enviadas[0].Corpo)
	if achado == nil {
		t.Fatalf("o e-mail não traz o link: %q", m.com.enviadas[0].Corpo)
	}
	codigo := achado[1]
	caminho := "/redefinir-senha?codigo=" + codigo

	quer(t, n.get(caminho), http.StatusOK, "Salvar senha")
	quer(t, n.get("/redefinir-senha?codigo=curto"), http.StatusOK, "não vale mais")

	form := func(nova, conf string) url.Values {
		return url.Values{"codigo": {codigo}, "nova": {nova}, "confirmar": {conf}}
	}
	quer(t, n.post("/redefinir-senha", form("Nova-Senha-Forte-9", "outra")), http.StatusBadRequest, "confirmação")
	quer(t, n.post("/redefinir-senha", form("curta1", "curta1")), http.StatusBadRequest, "10 caracteres")
	quer(t, n.post("/redefinir-senha", form("Nova-Senha-Forte-9", "Nova-Senha-Forte-9")), http.StatusOK, "Senha alterada")

	// o link vale uma vez só
	quer(t, n.get(caminho), http.StatusOK, "não vale mais")
	quer(t, n.post("/redefinir-senha", form("Outra-Senha-Forte-9", "Outra-Senha-Forte-9")), http.StatusBadRequest, "não vale mais")

	// a senha antiga caiu e a nova entra
	quer(t, m.em(t, hostNM).entrar(emailAna, senhaAna, ""), http.StatusUnauthorized)
	quer(t, m.em(t, hostNM).entrar(emailAna, "Nova-Senha-Forte-9", ""), http.StatusSeeOther)

	// no máximo 3 pedidos por janela: o 3º chega (já houve 1), o 4º não
	pedir(emailAna)
	pedir(emailAna)
	pedir(emailAna)
	if len(m.com.enviadas) != 3 {
		t.Errorf("e-mails enviados = %d, quer 3 (limite por janela)", len(m.com.enviadas))
	}

	// no console não há recuperação por e-mail
	console := m.em(t, hostConsole)
	quer(t, console.post("/esqueci-senha", url.Values{"email": {emailAna}, "g-recaptcha-response": {"ok"}}), http.StatusOK)
	if len(m.com.enviadas) != 3 {
		t.Errorf("o console enviou e-mail de recuperação")
	}
}

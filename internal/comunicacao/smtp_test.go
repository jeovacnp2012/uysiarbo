package comunicacao

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// servidorSMTP é um servidor SMTP mínimo de teste (sem TLS): guarda o que recebeu e pode recusar o destinatário.
type servidorSMTP struct {
	ln       net.Listener
	recusar  bool
	de, para string
	dados    string
	auth     string
}

func novoServidorSMTP(t *testing.T) *servidorSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &servidorSMTP{ln: ln}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.atender(c)
		}
	}()
	return s
}

func (s *servidorSMTP) porta() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *servidorSMTP) atender(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	w := func(l string) { c.Write([]byte(l + "\r\n")) }
	w("220 teste ESMTP")
	for {
		l, err := r.ReadString('\n')
		if err != nil {
			return
		}
		l = strings.TrimRight(l, "\r\n")
		switch {
		case strings.HasPrefix(l, "EHLO"):
			w("250-teste")
			w("250 AUTH PLAIN")
		case strings.HasPrefix(l, "AUTH PLAIN"):
			s.auth = l
			w("235 ok")
		case strings.HasPrefix(l, "MAIL FROM:"):
			s.de = l
			w("250 ok")
		case strings.HasPrefix(l, "RCPT TO:"):
			if s.recusar {
				w("550 sem caixa")
				continue
			}
			s.para = l
			w("250 ok")
		case l == "DATA":
			w("354 vai")
			var b strings.Builder
			for {
				x, err := r.ReadString('\n')
				if err != nil || x == ".\r\n" {
					break
				}
				b.WriteString(x)
			}
			s.dados = b.String()
			w("250 aceita")
		case l == "QUIT":
			w("221 tchau")
			return
		default:
			w("250 ok")
		}
	}
}

func TestSMTPEnvia(t *testing.T) {
	srv := novoServidorSMTP(t)
	p := &SMTP{Host: "127.0.0.1", Porta: srv.porta(), Usuario: "u", Senha: "s", De: "UYSI ARBO <nao-responda@exemplo.com.br>", Seguranca: SemTLS, Limite: 5 * time.Second}
	if !p.Atende(Email) || p.Atende(WhatsApp) {
		t.Fatal("o SMTP deve atender só e-mail")
	}
	id, err := p.Enviar(context.Background(), Mensagem{Canal: Email, Destinatario: "maria@exemplo.com", Nome: "Maria Souza", Assunto: "Notificação vencida", Corpo: "Olá, açúcar.\nSegunda linha."})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, "@exemplo.com.br>") {
		t.Errorf("id = %q", id)
	}
	if !strings.Contains(srv.de, "nao-responda@exemplo.com.br") || !strings.Contains(srv.para, "maria@exemplo.com") || srv.auth == "" {
		t.Errorf("de=%q para=%q auth=%q", srv.de, srv.para, srv.auth)
	}
	for _, quero := range []string{"Subject: =?utf-8?q?Notifica", "To: \"Maria Souza\" <maria@exemplo.com>", "Content-Type: text/plain; charset=utf-8", "quoted-printable", "=C3=A7"} {
		if !strings.Contains(srv.dados, quero) {
			t.Errorf("mensagem sem %q:\n%s", quero, srv.dados)
		}
	}

	srv.recusar = true
	if _, err = p.Enviar(context.Background(), Mensagem{Canal: Email, Destinatario: "x@exemplo.com", Assunto: "A", Corpo: "B"}); err == nil || !strings.Contains(err.Error(), "destinatário recusado") {
		t.Errorf("recusa: %v", err)
	}
}

func TestSMTPRecusaInjecaoEWhatsApp(t *testing.T) {
	p := &SMTP{Host: "127.0.0.1", Porta: 1, De: "a@b.com", Seguranca: SemTLS}
	if _, err := p.Enviar(context.Background(), Mensagem{Canal: Email, Destinatario: "x@y.com", Assunto: "Oi\r\nBcc: v@t.com", Corpo: "c"}); err == nil || !strings.Contains(err.Error(), "quebra de linha") {
		t.Errorf("injeção: %v", err)
	}
	if _, err := p.Enviar(context.Background(), Mensagem{Canal: WhatsApp, Destinatario: "5565999991234", Corpo: "c"}); !errors.Is(err, ErrDesligado) {
		t.Errorf("whatsapp: %v", err)
	}
	// servidor fora do ar vira erro comum (a fila tenta de novo)
	if _, err := p.Enviar(context.Background(), Mensagem{Canal: Email, Destinatario: "x@y.com", Assunto: "a", Corpo: "c"}); err == nil || errors.Is(err, ErrDesligado) {
		t.Errorf("conexão: %v", err)
	}
}

func TestRoteador(t *testing.T) {
	r := Roteador{Email: &SMTP{Host: "h", Porta: 587, De: "a@b.com"}}
	if !r.Atende(Email) || r.Atende(WhatsApp) || r.Nome() != "smtp" {
		t.Errorf("roteador: %v %v %s", r.Atende(Email), r.Atende(WhatsApp), r.Nome())
	}
	if _, err := r.Enviar(context.Background(), Mensagem{Canal: WhatsApp}); !errors.Is(err, ErrDesligado) {
		t.Errorf("whatsapp sem provedor: %v", err)
	}
	_ = strconv.Itoa
}

package comunicacao

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Segurança da conexão SMTP.
const (
	STARTTLS = "starttls" // porta 587: conexão aberta e promovida a TLS (obrigatório)
	TLS      = "tls"      // porta 465: TLS desde o início
	SemTLS   = "none"     // sem criptografia (só para servidor local de teste)
)

// SMTP envia e-mails por um servidor SMTP.
type SMTP struct {
	Host      string
	Porta     int
	Usuario   string
	Senha     string
	De        string // "Nome <endereco@dominio>" ou só o endereço
	Seguranca string // STARTTLS (padrão), TLS ou SemTLS
	Limite    time.Duration
}

func (s *SMTP) Nome() string { return "smtp" }

func (s *SMTP) Atende(canal string) bool { return canal == Email }

func (s *SMTP) limite() time.Duration {
	if s.Limite > 0 {
		return s.Limite
	}
	return 30 * time.Second
}

func semQuebra(texto string) bool { return !strings.ContainsAny(texto, "\r\n") }

// Montar devolve a mensagem RFC 5322 pronta, sem tocar na rede (separado para poder ser testado).
func (s *SMTP) Montar(m Mensagem, agora time.Time) (de, para string, corpo []byte, err error) {
	from, err := mail.ParseAddress(s.De)
	if err != nil {
		return "", "", nil, fmt.Errorf("remetente inválido: %w", err)
	}
	to, err := mail.ParseAddress(m.Destinatario)
	if err != nil || !semQuebra(m.Destinatario) {
		return "", "", nil, errors.New("destinatário inválido")
	}
	if !semQuebra(m.Assunto) || !semQuebra(m.Nome) {
		return "", "", nil, errors.New("assunto ou nome com quebra de linha")
	}
	if m.Nome != "" {
		to.Name = m.Nome
	}
	idb := make([]byte, 12)
	_, _ = rand.Read(idb)
	dominio := "localhost"
	if i := strings.LastIndex(from.Address, "@"); i >= 0 {
		dominio = from.Address[i+1:]
	}
	var b strings.Builder
	cab := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	cab("From", from.String())
	cab("To", to.String())
	cab("Subject", mime.QEncoding.Encode("utf-8", m.Assunto))
	cab("Date", agora.Format(time.RFC1123Z))
	cab("Message-ID", "<"+hex.EncodeToString(idb)+"@"+dominio+">")
	cab("MIME-Version", "1.0")
	cab("Content-Type", "text/plain; charset=utf-8")
	cab("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	w := quotedprintable.NewWriter(&b)
	_, _ = w.Write([]byte(strings.ReplaceAll(strings.ReplaceAll(m.Corpo, "\r\n", "\n"), "\n", "\r\n")))
	_ = w.Close()
	return from.Address, to.Address, []byte(b.String()), nil
}

func (s *SMTP) Enviar(ctx context.Context, m Mensagem) (string, error) {
	if m.Canal != Email {
		return "", ErrDesligado
	}
	de, para, corpo, err := s.Montar(m, time.Now())
	if err != nil {
		return "", err
	}
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Porta))
	d := net.Dialer{Timeout: s.limite()}
	var conn net.Conn
	if s.Seguranca == TLS {
		conn, err = (&tls.Dialer{NetDialer: &d, Config: &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return "", fmt.Errorf("conexão com o servidor de e-mail: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(s.limite()))
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return "", fmt.Errorf("servidor de e-mail: %w", err)
	}
	defer c.Close()
	if s.Seguranca != TLS && s.Seguranca != SemTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return "", errors.New("o servidor de e-mail não oferece STARTTLS")
		}
		if err = c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return "", fmt.Errorf("STARTTLS: %w", err)
		}
	}
	if s.Usuario != "" {
		if ok, _ := c.Extension("AUTH"); !ok {
			return "", errors.New("o servidor de e-mail não oferece autenticação")
		}
		auth := smtp.PlainAuth("", s.Usuario, s.Senha, s.Host)
		if s.Seguranca == SemTLS {
			auth = semTLSAuth{smtp.PlainAuth("", s.Usuario, s.Senha, s.Host)}
		}
		if err = c.Auth(auth); err != nil {
			return "", fmt.Errorf("autenticação no servidor de e-mail: %w", err)
		}
	}
	if err = c.Mail(de); err != nil {
		return "", fmt.Errorf("remetente recusado: %w", err)
	}
	if err = c.Rcpt(para); err != nil {
		return "", fmt.Errorf("destinatário recusado: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return "", err
	}
	if _, err = w.Write(corpo); err == nil {
		err = w.Close()
	}
	if err != nil {
		return "", fmt.Errorf("envio da mensagem: %w", err)
	}
	_ = c.Quit()
	id := ""
	if i := strings.Index(string(corpo), "Message-ID: <"); i >= 0 {
		resto := string(corpo)[i+len("Message-ID: "):]
		id = strings.TrimSpace(resto[:strings.Index(resto, "\r\n")])
	}
	return id, nil
}

// semTLSAuth deixa o PLAIN valer em conexão sem TLS (o net/smtp recusa por padrão); só para teste local.
type semTLSAuth struct{ smtp.Auth }

func (a semTLSAuth) Start(si *smtp.ServerInfo) (string, []byte, error) {
	si2 := *si
	si2.TLS = true
	return a.Auth.Start(&si2)
}

// Roteador manda cada mensagem ao provedor do seu canal; canal sem provedor fica na fila.
type Roteador map[string]Provedor

func (r Roteador) Nome() string {
	nomes := make([]string, 0, len(r))
	for _, p := range r {
		nomes = append(nomes, p.Nome())
	}
	if len(nomes) == 0 {
		return "desligado"
	}
	return strings.Join(nomes, "+")
}

func (r Roteador) Atende(canal string) bool { p, ok := r[canal]; return ok && p.Atende(canal) }

func (r Roteador) Enviar(ctx context.Context, m Mensagem) (string, error) {
	p, ok := r[m.Canal]
	if !ok {
		return "", ErrDesligado
	}
	return p.Enviar(ctx, m)
}

// ProvedorDe devolve o provedor que atende o canal (ou Desligado).
func (r Roteador) ProvedorDe(canal string) Provedor {
	if p, ok := r[canal]; ok {
		return p
	}
	return Desligado{}
}

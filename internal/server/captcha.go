package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Captcha confere a resposta de um desafio "não sou um robô".
type Captcha interface {
	Verificar(ctx context.Context, resposta, ip string) (bool, error)
}

// GoogleCaptcha valida a resposta do reCAPTCHA v2 no servidor do Google com a chave secreta.
type GoogleCaptcha struct {
	Secret string
	URL    string // padrão: endpoint oficial; mude só em testes
	Client *http.Client
}

const googleSiteVerify = "https://www.google.com/recaptcha/api/siteverify"

func (g GoogleCaptcha) Verificar(ctx context.Context, resposta, ip string) (bool, error) {
	if strings.TrimSpace(resposta) == "" {
		return false, nil
	}
	endpoint := g.URL
	if endpoint == "" {
		endpoint = googleSiteVerify
	}
	cli := g.Client
	if cli == nil {
		cli = &http.Client{Timeout: 5 * time.Second}
	}
	form := url.Values{"secret": {g.Secret}, "response": {resposta}}
	if ip != "" {
		form.Set("remoteip", ip)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := cli.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("reCAPTCHA respondeu %d", resp.StatusCode)
	}
	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	return out.Success, nil
}

// SemCaptcha aprova sempre. Só vale fora de produção, quando as chaves não estão configuradas.
type SemCaptcha struct{}

func (SemCaptcha) Verificar(context.Context, string, string) (bool, error) { return true, nil }

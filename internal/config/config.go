// Package config carrega a configuração da aplicação a partir do ambiente e do arquivo .env.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env             string
	Addr            string
	DatabaseURL     string
	BaseDomain      string // cada cliente é "<subdominio>.<BaseDomain>"
	AdminHost       string // endereço do console da plataforma (sem cliente)
	RecaptchaSite   string // chave pública do reCAPTCHA v2 (vai para a página)
	RecaptchaSecret string // chave secreta: só no servidor, nunca na página
	MapTileURL      string // modelo de endereço dos blocos do mapa base, com {z}/{x}/{y}
	MapAttribution  string // crédito do mapa base, mostrado no canto do mapa

	GeocodeProvider string // "gratuito" (ViaCEP + Nominatim) ou "desligado"
	GeocodeContact  string // e-mail ou site de quem opera o sistema (o Nominatim exige identificação)
	ViaCEPURL       string // para trocar o endereço do serviço (vazio = o oficial)
	NominatimURL    string

	SMTPHost     string // servidor de e-mail de saída (vazio = e-mail desligado)
	SMTPPort     int
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string // remetente: "Nome <endereco@dominio>"
	SMTPSecurity string // starttls (padrão), tls ou none

	ImportPermitirRedePrivada bool // libera a importação por API para endereços da rede interna (só testes e desenvolvimento)

	APIJWTSecret    string // chave que assina os tokens da API (vazia = gerada a cada início, só fora de produção)
	ShutdownTimeout time.Duration
}

// Load lê o arquivo .env (se existir) sem sobrescrever variáveis já definidas no ambiente.
func Load(envFile string) (Config, error) {
	if err := loadEnvFile(envFile); err != nil {
		return Config{}, err
	}
	cfg := Config{
		Env:             get("APP_ENV", "local"),
		Addr:            get("APP_ADDR", ":8090"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		BaseDomain:      strings.ToLower(get("APP_BASE_DOMAIN", "local")),
		AdminHost:       strings.ToLower(get("APP_ADMIN_HOST", "uysiarbo.local")),
		RecaptchaSite:   os.Getenv("RECAPTCHA_SITE_KEY"),
		RecaptchaSecret: os.Getenv("RECAPTCHA_SECRET_KEY"),
		MapTileURL:      get("MAP_TILE_URL", "https://tile.openstreetmap.org/{z}/{x}/{y}.png"),
		GeocodeProvider: strings.ToLower(get("GEOCODE_PROVIDER", "gratuito")),
		GeocodeContact:  os.Getenv("GEOCODE_CONTACT"),
		ViaCEPURL:       os.Getenv("GEOCODE_VIACEP_URL"),
		NominatimURL:    os.Getenv("GEOCODE_NOMINATIM_URL"),
		APIJWTSecret:    os.Getenv("API_JWT_SECRET"),
		SMTPHost:        os.Getenv("SMTP_HOST"), SMTPUser: os.Getenv("SMTP_USER"), SMTPPass: os.Getenv("SMTP_PASS"), SMTPFrom: os.Getenv("SMTP_FROM"),
		SMTPSecurity:              strings.ToLower(get("SMTP_SECURITY", "starttls")),
		ImportPermitirRedePrivada: os.Getenv("IMPORT_PERMITIR_REDE_PRIVADA") == "1",
		MapAttribution:            get("MAP_ATTRIBUTION", "© contribuidores do OpenStreetMap"),
		ShutdownTimeout:           10 * time.Second,
	}
	cfg.SMTPPort = 587
	if cfg.SMTPSecurity == "tls" {
		cfg.SMTPPort = 465
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		p, e := strconv.Atoi(v)
		if e != nil || p < 1 || p > 65535 {
			return Config{}, fmt.Errorf("SMTP_PORT inválida: %q", v)
		}
		cfg.SMTPPort = p
	}
	if cfg.SMTPHost != "" && cfg.SMTPFrom == "" {
		return Config{}, fmt.Errorf("com SMTP_HOST, informe também SMTP_FROM")
	}
	if cfg.SMTPSecurity != "starttls" && cfg.SMTPSecurity != "tls" && cfg.SMTPSecurity != "none" {
		return Config{}, fmt.Errorf("SMTP_SECURITY deve ser starttls, tls ou none")
	}
	if cfg.IsProduction() && (cfg.RecaptchaSite == "" || cfg.RecaptchaSecret == "") {
		return Config{}, fmt.Errorf("em produção, RECAPTCHA_SITE_KEY e RECAPTCHA_SECRET_KEY são obrigatórias")
	}
	if cfg.APIJWTSecret != "" && len(cfg.APIJWTSecret) < 32 {
		return Config{}, fmt.Errorf("API_JWT_SECRET precisa ter pelo menos 32 caracteres")
	}
	if cfg.IsProduction() && cfg.APIJWTSecret == "" {
		return Config{}, fmt.Errorf("em produção, API_JWT_SECRET é obrigatória (use 32 ou mais caracteres aleatórios)")
	}
	return cfg, nil
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("abrir %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, set := os.LookupEnv(key); !set {
			os.Setenv(key, value)
		}
	}
	return sc.Err()
}

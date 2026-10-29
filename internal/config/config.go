// Package config carrega a configuração da aplicação a partir do ambiente e do arquivo .env.
package config

import (
	"bufio"
	"fmt"
	"os"
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
		MapAttribution:  get("MAP_ATTRIBUTION", "© contribuidores do OpenStreetMap"),
		ShutdownTimeout: 10 * time.Second,
	}
	if cfg.IsProduction() && (cfg.RecaptchaSite == "" || cfg.RecaptchaSecret == "") {
		return Config{}, fmt.Errorf("em produção, RECAPTCHA_SITE_KEY e RECAPTCHA_SECRET_KEY são obrigatórias")
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

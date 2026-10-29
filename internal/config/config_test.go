package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLeEnvSemSobrescreverAmbiente(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# comentário\nAPP_ADDR=:9999\nAPP_ENV=\"production\"\nDATABASE_URL=postgres://x\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "staging") // já definida: o arquivo não deve sobrescrever
	os.Unsetenv("APP_ADDR")
	os.Unsetenv("DATABASE_URL")
	t.Cleanup(func() { os.Unsetenv("APP_ADDR"); os.Unsetenv("DATABASE_URL") })

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9999" || cfg.Env != "staging" || cfg.DatabaseURL != "postgres://x" {
		t.Errorf("config inesperada: %+v", cfg)
	}
}

func TestLoadSemArquivoUsaPadroes(t *testing.T) {
	os.Unsetenv("APP_ADDR")
	t.Setenv("APP_ENV", "")
	cfg, err := Load(filepath.Join(t.TempDir(), "nao-existe.env"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8090" || cfg.Env != "local" {
		t.Errorf("padrões inesperados: %+v", cfg)
	}
}

func TestLoadDominiosPadraoEPersonalizados(t *testing.T) {
	os.Unsetenv("APP_BASE_DOMAIN")
	os.Unsetenv("APP_ADMIN_HOST")
	cfg, _ := Load(filepath.Join(t.TempDir(), "nao-existe.env"))
	if cfg.BaseDomain != "local" || cfg.AdminHost != "uysiarbo.local" {
		t.Errorf("padrões: %+v", cfg)
	}
	t.Setenv("APP_BASE_DOMAIN", "InovarSistemas.biz")
	t.Setenv("APP_ADMIN_HOST", "uysiarbo.inovarsistemas.biz")
	cfg, _ = Load(filepath.Join(t.TempDir(), "nao-existe.env"))
	if cfg.BaseDomain != "inovarsistemas.biz" || cfg.AdminHost != "uysiarbo.inovarsistemas.biz" {
		t.Errorf("personalizados: %+v", cfg)
	}
}

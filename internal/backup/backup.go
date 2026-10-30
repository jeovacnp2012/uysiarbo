// Package backup faz cópias de segurança do banco com o pg_dump, uma por schema (o catálogo "public" e
// um por cliente), confere a integridade e restaura. Cada rodada vira uma pasta AAAAMMDD-HHMMSS com os
// arquivos .dump e um manifesto com tamanho e SHA-256 de cada um.
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Arquivo é um item do manifesto.
type Arquivo struct {
	Schema  string `json:"schema"`
	Nome    string `json:"nome"`
	Tamanho int64  `json:"tamanho"`
	SHA256  string `json:"sha256"`
}

// Manifesto descreve uma rodada de backup.
type Manifesto struct {
	CriadoEm time.Time `json:"criado_em"`
	Versao   string    `json:"versao"`
	Arquivos []Arquivo `json:"arquivos"`
}

const nomeManifesto = "manifesto.json"

var padraoPasta = regexp.MustCompile(`^\d{8}-\d{6}$`)
var padraoSchema = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// Config reúne o que o pg_dump e o pg_restore precisam.
type Config struct {
	DatabaseURL string // banco a copiar
	Dir         string // pasta onde as rodadas ficam
	PgBin       string // pasta dos binários do PostgreSQL; vazio = procura no PATH
	Versao      string // versão do sistema, só para constar no manifesto
}

func (c Config) binario(nome string) (string, error) {
	if c.PgBin != "" {
		for _, n := range []string{nome, nome + ".exe"} {
			p := filepath.Join(c.PgBin, n)
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		return "", fmt.Errorf("%s não encontrado em %s", nome, c.PgBin)
	}
	p, err := exec.LookPath(nome)
	if err != nil {
		return "", fmt.Errorf("%s não encontrado no PATH (informe a pasta com PG_BIN_DIR)", nome)
	}
	return p, nil
}

// Disponivel diz se os binários necessários existem.
func (c Config) Disponivel() bool {
	_, e1 := c.binario("pg_dump")
	_, e2 := c.binario("pg_restore")
	return e1 == nil && e2 == nil
}

// ambiente converte a URL de conexão em variáveis PG*, para a senha não aparecer na lista de processos.
func ambiente(rawURL string) ([]string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("URL do banco inválida: %w", err)
	}
	env := os.Environ()
	add := func(k, v string) {
		if v != "" {
			env = append(env, k+"="+v)
		}
	}
	add("PGHOST", u.Hostname())
	add("PGPORT", u.Port())
	add("PGDATABASE", strings.TrimPrefix(u.Path, "/"))
	if u.User != nil {
		add("PGUSER", u.User.Username())
		if p, ok := u.User.Password(); ok {
			add("PGPASSWORD", p)
		}
	}
	add("PGSSLMODE", u.Query().Get("sslmode"))
	return env, nil
}

func executar(ctx context.Context, env []string, bin string, args ...string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	if saida, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w: %s", filepath.Base(bin), err, strings.TrimSpace(string(saida)))
	}
	return nil
}

func sha256De(caminho string) (string, int64, error) {
	f, err := os.Open(caminho)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

// Executar copia cada schema para uma pasta nova e devolve o caminho dela. A pasta só ganha o nome
// definitivo quando tudo deu certo, então uma rodada que falhou no meio não vira "último backup".
func (c Config) Executar(ctx context.Context, schemas []string, agora time.Time) (string, error) {
	dump, err := c.binario("pg_dump")
	if err != nil {
		return "", err
	}
	env, err := ambiente(c.DatabaseURL)
	if err != nil {
		return "", err
	}
	if len(schemas) == 0 {
		return "", errors.New("nenhum schema para copiar")
	}
	if err := os.MkdirAll(c.Dir, 0o700); err != nil {
		return "", err
	}
	final := filepath.Join(c.Dir, agora.UTC().Format("20060102-150405"))
	if _, err := os.Stat(final); err == nil {
		return "", fmt.Errorf("já existe uma rodada em %s", final)
	}
	parcial := final + ".parcial"
	_ = os.RemoveAll(parcial)
	if err := os.Mkdir(parcial, 0o700); err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(parcial)
		}
	}()
	man := Manifesto{CriadoEm: agora.UTC(), Versao: c.Versao}
	for _, s := range schemas {
		if !padraoSchema.MatchString(s) {
			return "", fmt.Errorf("nome de schema inválido: %q", s)
		}
		nome := s + ".dump"
		destino := filepath.Join(parcial, nome)
		if err := executar(ctx, env, dump, "--format=custom", "--no-owner", "--schema="+s, "--file="+destino); err != nil {
			return "", fmt.Errorf("copiar %s: %w", s, err)
		}
		soma, tam, err := sha256De(destino)
		if err != nil {
			return "", err
		}
		man.Arquivos = append(man.Arquivos, Arquivo{Schema: s, Nome: nome, Tamanho: tam, SHA256: soma})
	}
	js, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(parcial, nomeManifesto), js, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(parcial, final); err != nil {
		return "", err
	}
	ok = true
	return final, nil
}

// Rodadas lista as pastas de backup completas, da mais nova para a mais antiga.
func Rodadas(dir string) ([]string, error) {
	itens, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, it := range itens {
		if it.IsDir() && padraoPasta.MatchString(it.Name()) {
			out = append(out, it.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out, nil
}

// Rotacionar apaga as rodadas mais antigas, mantendo as "manter" mais novas. Devolve as apagadas.
func Rotacionar(dir string, manter int) ([]string, error) {
	if manter < 1 {
		return nil, errors.New("manter precisa ser pelo menos 1")
	}
	rodadas, err := Rodadas(dir)
	if err != nil || len(rodadas) <= manter {
		return nil, err
	}
	var apagadas []string
	for _, nome := range rodadas[manter:] {
		if err := os.RemoveAll(filepath.Join(dir, nome)); err != nil {
			return apagadas, err
		}
		apagadas = append(apagadas, nome)
	}
	return apagadas, nil
}

// LerManifesto lê o manifesto de uma rodada.
func LerManifesto(pasta string) (Manifesto, error) {
	var m Manifesto
	js, err := os.ReadFile(filepath.Join(pasta, nomeManifesto))
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(js, &m)
}

// Verificar confere tamanho e SHA-256 de cada arquivo da rodada e pede ao pg_restore que leia o índice
// dele (pega arquivo truncado ou corrompido). Não toca em nenhum banco.
func (c Config) Verificar(ctx context.Context, pasta string) error {
	restore, err := c.binario("pg_restore")
	if err != nil {
		return err
	}
	man, err := LerManifesto(pasta)
	if err != nil {
		return fmt.Errorf("manifesto: %w", err)
	}
	if len(man.Arquivos) == 0 {
		return errors.New("manifesto sem arquivos")
	}
	for _, a := range man.Arquivos {
		if filepath.Base(a.Nome) != a.Nome {
			return fmt.Errorf("nome de arquivo suspeito no manifesto: %q", a.Nome)
		}
		caminho := filepath.Join(pasta, a.Nome)
		soma, tam, err := sha256De(caminho)
		if err != nil {
			return fmt.Errorf("%s: %w", a.Nome, err)
		}
		if tam != a.Tamanho || soma != a.SHA256 {
			return fmt.Errorf("%s: o arquivo não confere com o manifesto (alterado ou corrompido)", a.Nome)
		}
		if err := executar(ctx, os.Environ(), restore, "--list", caminho); err != nil {
			return fmt.Errorf("%s: %w", a.Nome, err)
		}
	}
	return nil
}

// Restaurar carrega um arquivo .dump no banco de destino. Com limpar, apaga antes os objetos que já
// existem no destino (use só em banco de teste ou numa recuperação).
func (c Config) Restaurar(ctx context.Context, arquivo, destinoURL string, limpar bool) error {
	restore, err := c.binario("pg_restore")
	if err != nil {
		return err
	}
	env, err := ambiente(destinoURL)
	if err != nil {
		return err
	}
	args := []string{"--no-owner", "--exit-on-error", "--dbname=" + dbDe(destinoURL)}
	if limpar {
		args = append(args, "--clean", "--if-exists")
	}
	args = append(args, arquivo)
	return executar(ctx, env, restore, args...)
}

func dbDe(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Path, "/")
}

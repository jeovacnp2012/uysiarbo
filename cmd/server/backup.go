package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"uysiarbo/internal/backup"
	"uysiarbo/internal/config"
	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
)

const ajudaBackup = `uso:
  uysiarbo backup [-dir backups] [-manter 14] [-pgbin pasta]   copia o catálogo e cada cliente, confere e apaga as rodadas antigas
  uysiarbo backup listar [-dir backups]                        mostra as rodadas guardadas
  uysiarbo backup verificar [-dir backups] [rodada]            confere SHA-256 e leitura de uma rodada (a mais nova se omitida)
  uysiarbo backup restaurar -arquivo f.dump -banco URL [-limpar] [-sobre-o-atual] [-pgbin pasta]
O padrão da pasta é BACKUP_DIR ou ./backups; dos binários do PostgreSQL, PG_BIN_DIR ou o PATH.`

func padraoOuEnv(chave, padrao string) string {
	if v := os.Getenv(chave); v != "" {
		return v
	}
	return padrao
}

func backupCmd(ctx context.Context, args []string) error {
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}
	sub := ""
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		sub, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	dir := fs.String("dir", padraoOuEnv("BACKUP_DIR", "backups"), "pasta dos backups")
	pgbin := fs.String("pgbin", os.Getenv("PG_BIN_DIR"), "pasta dos binários do PostgreSQL")
	manter := fs.Int("manter", 14, "quantas rodadas guardar")
	arquivo := fs.String("arquivo", "", "arquivo .dump a restaurar")
	banco := fs.String("banco", "", "URL do banco de destino da restauração")
	limpar := fs.Bool("limpar", false, "apaga antes os objetos que já existem no destino")
	sobre := fs.Bool("sobre-o-atual", false, "permite restaurar sobre o banco do sistema (DATABASE_URL)")
	if err := fs.Parse(args); err != nil {
		return errors.New(ajudaBackup)
	}
	c := backup.Config{DatabaseURL: cfg.DatabaseURL, Dir: *dir, PgBin: *pgbin, Versao: version}

	switch sub {
	case "":
		return fazerBackup(ctx, c, *manter)
	case "listar":
		return listarBackups(c.Dir)
	case "verificar":
		pasta, err := pastaDaRodada(c.Dir, fs.Arg(0))
		if err != nil {
			return err
		}
		if err := c.Verificar(ctx, pasta); err != nil {
			return err
		}
		fmt.Println("OK: a rodada confere:", pasta)
		return nil
	case "restaurar":
		if *arquivo == "" || *banco == "" {
			return errors.New(ajudaBackup)
		}
		if *banco == cfg.DatabaseURL && !*sobre {
			return errors.New("o destino é o banco do sistema: para restaurar sobre ele de propósito, acrescente -sobre-o-atual (de preferência restaure num banco de teste antes)")
		}
		if err := c.Restaurar(ctx, *arquivo, *banco, *limpar); err != nil {
			return err
		}
		fmt.Println("Restaurado:", *arquivo)
		return nil
	}
	return errors.New(ajudaBackup)
}

func fazerBackup(ctx context.Context, c backup.Config, manter int) error {
	pool, err := db.Connect(ctx, c.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	clientes, err := sqlc.New(pool).ListClientes(ctx)
	if err != nil {
		return err
	}
	schemas := []string{"public"}
	for _, cl := range clientes {
		if cl.SchemaNome != "" && cl.Status != "provisionando" {
			schemas = append(schemas, cl.SchemaNome)
		}
	}
	pasta, err := c.Executar(ctx, schemas, time.Now())
	if err != nil {
		return err
	}
	if err := c.Verificar(ctx, pasta); err != nil {
		return fmt.Errorf("a cópia foi feita, mas não passou na conferência (%s): %w", pasta, err)
	}
	apagadas, err := backup.Rotacionar(c.Dir, manter)
	if err != nil {
		return err
	}
	fmt.Printf("Backup pronto e conferido: %s (%d schemas)\n", pasta, len(schemas))
	if len(apagadas) > 0 {
		fmt.Printf("Rodadas antigas apagadas: %d\n", len(apagadas))
	}
	fmt.Println("Lembrete: guarde também uma cópia da DADOS_CHAVE do .env, fora do servidor; sem ela os CPFs cifrados não abrem.")
	return nil
}

func listarBackups(dir string) error {
	rodadas, err := backup.Rodadas(dir)
	if err != nil {
		return err
	}
	if len(rodadas) == 0 {
		fmt.Println("Nenhuma rodada em", dir)
		return nil
	}
	for _, r := range rodadas {
		man, err := backup.LerManifesto(filepath.Join(dir, r))
		if err != nil {
			fmt.Printf("%s  (sem manifesto: %v)\n", r, err)
			continue
		}
		var total int64
		for _, a := range man.Arquivos {
			total += a.Tamanho
		}
		fmt.Printf("%s  %d schemas  %.1f MB  versão %s\n", r, len(man.Arquivos), float64(total)/1048576, man.Versao)
	}
	return nil
}

func pastaDaRodada(dir, nome string) (string, error) {
	if nome != "" {
		return filepath.Join(dir, filepath.Base(nome)), nil
	}
	rodadas, err := backup.Rodadas(dir)
	if err != nil {
		return "", err
	}
	if len(rodadas) == 0 {
		return "", fmt.Errorf("nenhuma rodada em %s", dir)
	}
	return filepath.Join(dir, rodadas[0]), nil
}

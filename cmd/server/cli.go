package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"uysiarbo/internal/config"
	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/tenant"
)

const ajuda = `uso:
  uysiarbo                          sobe o servidor
  uysiarbo version                  mostra a versão
  uysiarbo migrate [up|down|status|reset]   migrações do catálogo (schema public)
  uysiarbo migrate-all              catálogo + todos os schemas de clientes, para a versão mais nova
  uysiarbo cliente criar -nome "Prefeitura de X" -slug x [-subdominio x] [-dominio d] [-plano p]
  uysiarbo cliente listar`

// tenantService abre o banco e monta o serviço de clientes. close deve ser chamado ao fim.
func tenantService(ctx context.Context) (svc *tenant.Service, close func(), err error) {
	cfg, err := config.Load(".env")
	if err != nil {
		return nil, nil, err
	}
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	return &tenant.Service{Pool: pool, URL: cfg.DatabaseURL, Log: slog.New(slog.NewTextHandler(os.Stdout, nil))}, pool.Close, nil
}

// migrateAll aplica o catálogo e depois todos os clientes; use após cada atualização do sistema.
func migrateAll(ctx context.Context) error {
	svc, closeFn, err := tenantService(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	if err := db.Migrate(ctx, svc.URL, "up"); err != nil {
		return fmt.Errorf("catálogo: %w", err)
	}
	return svc.MigrateAll(ctx, "up")
}

func clienteCmd(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New(ajuda)
	}
	svc, closeFn, err := tenantService(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	switch args[0] {
	case "criar":
		fs := flag.NewFlagSet("cliente criar", flag.ContinueOnError)
		in := tenant.Input{}
		fs.StringVar(&in.Nome, "nome", "", "nome do cliente (obrigatório)")
		fs.StringVar(&in.Slug, "slug", "", "identificador curto, vira o schema c_<slug> (obrigatório)")
		fs.StringVar(&in.Subdominio, "subdominio", "", "padrão: o slug")
		fs.StringVar(&in.Dominio, "dominio", "", "domínio próprio, opcional")
		fs.StringVar(&in.Plano, "plano", "", "padrão: padrao")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if in.Nome == "" || in.Slug == "" {
			return errors.New("informe -nome e -slug")
		}
		c, err := svc.Provision(ctx, in)
		if err != nil {
			return err
		}
		fmt.Printf("Cliente %q criado: schema %s, subdomínio %s, status %s\n", c.Nome, c.SchemaNome, c.Subdominio, c.Status)
	case "listar":
		clientes, err := sqlc.New(svc.Pool).ListClientes(ctx)
		if err != nil {
			return err
		}
		for _, c := range clientes {
			fmt.Printf("%-12s %-14s %-12s %s\n", c.Slug, c.SchemaNome, c.Status, c.Nome)
		}
		if len(clientes) == 0 {
			fmt.Println("Nenhum cliente cadastrado.")
		}
	default:
		return errors.New(ajuda)
	}
	return nil
}

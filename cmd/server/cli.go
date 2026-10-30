package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"uysiarbo/internal/audit"
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
  uysiarbo cliente criar -nome "Prefeitura de X" -slug x -uf MT [-cidade X] [-fuso America/Cuiaba] [-subdominio x] [-dominio d] [-plano p]
  uysiarbo cliente listar
  uysiarbo cliente sincronizar      reaplica o papel Administrador e as permissões do catálogo em todos os clientes
  uysiarbo master criar -nome "Nome Completo" -email e@mail   cria um master com senha forte gerada (mostrada uma vez)
  uysiarbo master resetar -email e@mail                       gera nova senha forte (mostrada uma vez)
  uysiarbo dados chave              gera uma chave nova para DADOS_CHAVE (cifra de dados pessoais)
  uysiarbo dados cifrar             cifra os dados pessoais antigos que ainda estão em texto puro (precisa de DADOS_CHAVE)
  uysiarbo backup                   copia o catálogo e cada cliente (um .dump por schema), confere e apaga as rodadas antigas
  uysiarbo backup listar|verificar|restaurar   ver "uysiarbo backup -h"
  uysiarbo carga -url URL [-c 20] [-d 30s]   teste de carga simples (vazão, erros e latência)
  uysiarbo master listar`

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
	return &tenant.Service{Pool: pool, URL: cfg.DatabaseURL, Log: slog.New(slog.NewTextHandler(os.Stdout, nil)), Auditoria: audit.Plataforma{Q: sqlc.New(pool)}}, pool.Close, nil
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
		fs.StringVar(&in.Cidade, "cidade", "", "cidade da empresa")
		fs.StringVar(&in.UF, "uf", "", "estado da empresa, como MT (define o fuso horário)")
		fs.StringVar(&in.Fuso, "fuso", "", "fuso horário, como America/Cuiaba (padrão: o do estado)")
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
	case "sincronizar":
		return svc.SincronizarTodos(ctx)
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

// masterCmd administra os usuários master da plataforma. A senha gerada é mostrada UMA vez, aqui.
func masterCmd(ctx context.Context, args []string) error {
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
		fs := flag.NewFlagSet("master criar", flag.ContinueOnError)
		nome := fs.String("nome", "", "nome completo (obrigatório)")
		email := fs.String("email", "", "e-mail de acesso (obrigatório)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		u, senha, err := svc.CriarMaster(ctx, *nome, *email)
		if err != nil {
			return err
		}
		mostrarSenha(u.Nome, u.Email, senha)
	case "resetar":
		fs := flag.NewFlagSet("master resetar", flag.ContinueOnError)
		email := fs.String("email", "", "e-mail do usuário (obrigatório)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		u, senha, err := svc.ResetarSenhaPlataforma(ctx, *email)
		if err != nil {
			return err
		}
		mostrarSenha(u.Nome, u.Email, senha)
	case "listar":
		usuarios, err := sqlc.New(svc.Pool).ListPlataformaUsuarios(ctx)
		if err != nil {
			return err
		}
		for _, u := range usuarios {
			fmt.Printf("%-32s %-8s ativo=%v trocar_senha=%v  %s\n", u.Email, map[bool]string{true: "master", false: "comum"}[u.Master], u.Ativo, u.DeveTrocarSenha, u.Nome)
		}
	default:
		return errors.New(ajuda)
	}
	return nil
}

func mostrarSenha(nome, email, senha string) {
	fmt.Printf("\nUsuário: %s <%s>\nSenha temporária: %s\n\n", nome, email, senha)
	fmt.Println("Anote agora: ela NÃO será mostrada de novo e não fica guardada em lugar nenhum.")
	fmt.Println("No primeiro login, o sistema pedirá que a pessoa escolha a própria senha.")
}

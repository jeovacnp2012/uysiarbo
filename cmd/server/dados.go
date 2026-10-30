package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"uysiarbo/internal/cifra"
	"uysiarbo/internal/config"
	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
)

// colunasCifradas são os dados pessoais guardados cifrados (tabela, coluna).
var colunasCifradas = [][2]string{
	{"locais", "proprietario_cpf"},
	{"denuncias", "cidadao_cpf"},
	{"lvc_testes", "proprietario_cpf"},
}

func dadosCmd(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New(ajuda)
	}
	switch args[0] {
	case "chave":
		k, err := cifra.NovaChave()
		if err != nil {
			return err
		}
		fmt.Println("Coloque no .env (e guarde uma cópia segura FORA do servidor; sem ela os dados cifrados não abrem):")
		fmt.Println("DADOS_CHAVE=" + k)
		return nil
	case "cifrar":
		return cifrarDadosAntigos(ctx)
	}
	return errors.New(ajuda)
}

// cifrarDadosAntigos cifra, em todos os clientes, os valores ainda em texto puro. Pode rodar de novo.
func cifrarDadosAntigos(ctx context.Context) error {
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}
	if err := cifra.Configurar(cfg.DadosChave); err != nil {
		return err
	}
	if !cifra.Ativa() {
		return errors.New("DADOS_CHAVE não está configurada no .env (gere uma com: uysiarbo dados chave)")
	}
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	clientes, err := sqlc.New(pool).ListClientes(ctx)
	if err != nil {
		return err
	}
	for _, c := range clientes {
		if c.Status == "cancelado" {
			continue
		}
		for _, tc := range colunasCifradas {
			tabela := pgx.Identifier{c.SchemaNome, tc[0]}.Sanitize()
			col := pgx.Identifier{tc[1]}.Sanitize()
			rows, err := pool.Query(ctx, "SELECT id, "+col+" FROM "+tabela+" WHERE "+col+" <> '' AND "+col+" NOT LIKE 'v1:%'")
			if err != nil {
				return fmt.Errorf("%s.%s: %w", c.Slug, tc[0], err)
			}
			type par struct {
				id  int64
				val string
			}
			var lote []par
			for rows.Next() {
				var p par
				if err := rows.Scan(&p.id, &p.val); err != nil {
					rows.Close()
					return err
				}
				lote = append(lote, p)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			for _, p := range lote {
				if _, err := pool.Exec(ctx, "UPDATE "+tabela+" SET "+col+" = $1 WHERE id = $2", cifra.Cifrar(p.val), p.id); err != nil {
					return fmt.Errorf("%s.%s: %w", c.Slug, tc[0], err)
				}
			}
			fmt.Printf("%s.%s: %d valores cifrados\n", c.Slug, tc[0], len(lote))
		}
	}
	return nil
}

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"uysiarbo/internal/carga"
)

const ajudaCarga = `uso:
  uysiarbo carga -url http://127.0.0.1:8090/healthz [-c 20] [-d 30s] [-cookie "uysi_sessao=..."] [-host cliente.uysiarbo.local]
Faz GET repetido com -c conexões por -d e mostra vazão, erros e latência (p50/p90/p99).
Teste em ambiente de homologação ou local; não aponte para a produção em horário de uso.`

func cargaCmd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("carga", flag.ContinueOnError)
	url := fs.String("url", "", "URL a testar")
	c := fs.Int("c", 20, "conexões simultâneas")
	d := fs.Duration("d", 30*time.Second, "duração do teste")
	cookie := fs.String("cookie", "", "cabeçalho Cookie (para telas com login)")
	host := fs.String("host", "", "valor do cabeçalho Host")
	if err := fs.Parse(args); err != nil || *url == "" {
		return errors.New(ajudaCarga)
	}
	fmt.Printf("Testando %s com %d conexões por %s...\n", *url, *c, *d)
	r, err := carga.Executar(ctx, carga.Opcoes{URL: *url, Conexoes: *c, Duracao: *d, Cookie: *cookie, Host: *host})
	if err != nil {
		return err
	}
	fmt.Print(r.Resumo())
	return nil
}

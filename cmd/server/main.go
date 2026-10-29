package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"uysiarbo/internal/config"
	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/server"
	"uysiarbo/internal/tenant"
)

// version é preenchida no build (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version":
			fmt.Println("uysiarbo", version)
			return
		case "migrate-all":
			exitOnError(migrateAll(context.Background()))
			return
		case "cliente":
			exitOnError(clienteCmd(context.Background(), os.Args[2:]))
			return
		case "help", "--help", "-h":
			fmt.Println(ajuda)
			return
		case "migrate":
			if err := migrate(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "erro:", err)
				os.Exit(1)
			}
			return
		}
	}
	if err := run(); err != nil {
		slog.Error("falha ao executar", "erro", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}

	var handler slog.Handler = slog.NewTextHandler(os.Stdout, nil)
	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	}
	log := slog.New(handler)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	pools := &tenant.Pools{URL: cfg.DatabaseURL}
	defer pools.Close()

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: server.New(server.Deps{
			Log:      log,
			DB:       pool,
			Catalog:  sqlc.New(pool),
			Resolver: &tenant.Resolver{Pool: pool, BaseDomain: cfg.BaseDomain, AdminHost: cfg.AdminHost},
			Pools:    pools,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("servidor iniciado", "versao", version, "addr", cfg.Addr, "env", cfg.Env)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("encerrando servidor")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}

// migrate aplica as migrações embutidas: "uysiarbo migrate [up|down|status|reset]".
func migrate(args []string) error {
	command := "up"
	if len(args) > 0 {
		command = args[0]
	}
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}
	return db.Migrate(context.Background(), cfg.DatabaseURL, command)
}

func exitOnError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

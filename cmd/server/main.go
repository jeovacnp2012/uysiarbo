package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // fusos horários embutidos (o servidor da VPS pode não ter)

	"uysiarbo/internal/audit"
	"uysiarbo/internal/cifra"
	"uysiarbo/internal/comunicacao"
	"uysiarbo/internal/config"
	"uysiarbo/internal/db"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/geocode"
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
		case "master":
			exitOnError(masterCmd(context.Background(), os.Args[2:]))
			return
		case "dados":
			exitOnError(dadosCmd(context.Background(), os.Args[2:]))
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

	if err := cifra.Configurar(cfg.DadosChave); err != nil {
		return err
	}
	if !cifra.Ativa() {
		log.Warn("DADOS_CHAVE não configurada: CPF e outros dados pessoais ficam em texto puro (só aceitável fora de produção)")
	}

	var captcha server.Captcha = server.SemCaptcha{}
	if cfg.RecaptchaSite != "" && cfg.RecaptchaSecret != "" {
		captcha = server.GoogleCaptcha{Secret: cfg.RecaptchaSecret}
	} else {
		log.Warn("reCAPTCHA não configurado: o login funciona sem o desafio (só aceitável fora de produção)")
	}

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

			Public:                    sqlc.New(pool),
			Captcha:                   captcha,
			RecaptchaSite:             cfg.RecaptchaSite,
			Producao:                  cfg.IsProduction(),
			MapTileURL:                cfg.MapTileURL,
			MapAttribution:            cfg.MapAttribution,
			Geocoder:                  geocodificador(cfg),
			Comunicacao:               provedorDeComunicacao(cfg, log),
			Fundo:                     ctx,
			APIKey:                    chaveDaAPI(cfg, log),
			ImportPermitirRedePrivada: cfg.ImportPermitirRedePrivada,
			Tenants:                   &tenant.Service{Pool: pool, URL: cfg.DatabaseURL, Log: log, Auditoria: audit.Plataforma{Q: sqlc.New(pool)}},
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

// provedorDeComunicacao liga o envio de e-mail quando o SMTP está configurado.
func provedorDeComunicacao(cfg config.Config, log *slog.Logger) comunicacao.Provedor {
	if cfg.SMTPHost == "" {
		log.Warn("SMTP não configurado: os e-mails ficam na fila")
		return comunicacao.Desligado{}
	}
	return comunicacao.Roteador{comunicacao.Email: &comunicacao.SMTP{
		Host: cfg.SMTPHost, Porta: cfg.SMTPPort, Usuario: cfg.SMTPUser, Senha: cfg.SMTPPass, De: cfg.SMTPFrom, Seguranca: cfg.SMTPSecurity,
	}}
}

// geocodificador escolhe a localização automática de endereços pela configuração.
func geocodificador(cfg config.Config) geocode.Geocoder {
	if cfg.GeocodeProvider == "desligado" {
		return geocode.Desligado{}
	}
	return &geocode.Gratuito{ViaCEPURL: cfg.ViaCEPURL, NominatimURL: cfg.NominatimURL, Contato: cfg.GeocodeContact}
}

// chaveDaAPI devolve a chave que assina os tokens da API. Sem API_JWT_SECRET (só permitido fora de
// produção), gera uma ao iniciar: os tokens valem até o servidor reiniciar.
func chaveDaAPI(cfg config.Config, log *slog.Logger) []byte {
	if cfg.APIJWTSecret != "" {
		return []byte(cfg.APIJWTSecret)
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Error("gerar chave da API", "erro", err)
		return nil
	}
	log.Warn("API_JWT_SECRET não definida: chave gerada para esta execução; os tokens da API deixam de valer ao reiniciar")
	return b
}

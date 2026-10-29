// Comando migrate: aplica as migrações do banco. Uso: go run ./cmd/migrate [up|down|status|reset]
package main

import (
	"context"
	"fmt"
	"os"

	"uysiarbo/internal/config"
	"uysiarbo/internal/db"
)

func main() {
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	cfg, err := config.Load(".env")
	if err != nil {
		fail(err)
	}
	if err := db.Migrate(context.Background(), cfg.DatabaseURL, command); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "erro:", err)
	os.Exit(1)
}

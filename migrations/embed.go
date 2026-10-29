// Package migrations embute os .sql do goose no binário: public/ é o catálogo (schema public);
// cliente/ (etapa 1.2) guarda as migrações aplicadas em cada schema de cliente.
package migrations

import "embed"

//go:embed public/*.sql cliente/*.sql
var FS embed.FS

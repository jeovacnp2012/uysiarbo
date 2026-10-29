// Package migrations embute os arquivos .sql do goose no binário.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

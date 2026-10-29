// Package web embute os arquivos estáticos (CSS, JS, imagens) no binário.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var staticFS embed.FS

// Static devolve o conteúdo de web/static, para servir em /static.
func Static() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

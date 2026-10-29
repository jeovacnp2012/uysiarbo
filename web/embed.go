// Package web embute os arquivos estáticos (CSS, JS, imagens) no binário.
package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
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

// Version devolve um identificador curto do conteúdo dos arquivos estáticos. Ele vai na URL
// (?v=...), então o navegador baixa de novo quando algum arquivo muda e guarda em cache enquanto não muda.
func Version() string {
	h := sha256.New()
	_ = fs.WalkDir(staticFS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := staticFS.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write([]byte(path))
		h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:10]
}

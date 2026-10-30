package server

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// enviarBanner faz o POST multipart do editor de banners (campos e, se houver, a imagem de fundo).
func (n *navegador) enviarBanner(caminho string, campos url.Values, fundo []byte) *httptest.ResponseRecorder {
	n.t.Helper()
	if n.cookies[csrfCookie] == "" {
		n.get("/entrar")
	}
	var corpo bytes.Buffer
	mw := multipart.NewWriter(&corpo)
	_ = mw.WriteField("csrf", n.cookies[csrfCookie])
	for k, vs := range campos {
		for _, v := range vs {
			_ = mw.WriteField(k, v)
		}
	}
	if fundo != nil {
		fw, _ := mw.CreateFormFile("fundo", "fundo.png")
		_, _ = fw.Write(fundo)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, caminho, io.NopCloser(&corpo))
	req.Host = n.host
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for k, v := range n.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	n.h.ServeHTTP(rec, req)
	return rec
}

// png1x1 é um PNG válido de 1 pixel.
const png1x1 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func TestBanners(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	quer(t, ana.get("/banners"), http.StatusOK)
	quer(t, ana.get("/banners/novo"), http.StatusOK, "Editor", "Adicionar texto")

	def := `[{"texto":"Dengue: elimine os focos","x":10,"y":20,"fonte":"sans","tamanho":72,"cor":"#ffffff","negrito":true,"sombra":true,"alinhar":"left"},{"texto":"   ","x":1,"y":1,"fonte":"sans","tamanho":20,"cor":"#000000","alinhar":"left"}]`
	novo := func(extra map[string]string) url.Values {
		v := url.Values{"nome": {"Campanha de outubro"}, "largura": {"1080"}, "altura": {"1080"}, "cor_fundo": {"#155a9e"}, "ativo": {"1"}, "definicao": {def}}
		for k, x := range extra {
			v.Set(k, x)
		}
		return v
	}
	quer(t, ana.enviarBanner("/banners", novo(map[string]string{"nome": ""}), nil), http.StatusBadRequest, "nome do banner")
	quer(t, ana.enviarBanner("/banners", novo(map[string]string{"largura": "50"}), nil), http.StatusBadRequest, "entre 100 e 4000")
	quer(t, ana.enviarBanner("/banners", novo(map[string]string{"cor_fundo": "azul"}), nil), http.StatusBadRequest, "cor de fundo")
	quer(t, ana.enviarBanner("/banners", novo(map[string]string{"definicao": "isso não é json"}), nil), http.StatusBadRequest, "inválida")
	quer(t, ana.enviarBanner("/banners", novo(map[string]string{"definicao": `[{"texto":"x","x":150,"y":1,"fonte":"sans","tamanho":20,"cor":"#000000","alinhar":"left"}]`}), nil), http.StatusBadRequest, "fora do banner")
	quer(t, ana.enviarBanner("/banners", novo(map[string]string{"definicao": `[{"texto":"x","x":1,"y":1,"fonte":"comic","tamanho":20,"cor":"#000000","alinhar":"left"}]`}), nil), http.StatusBadRequest, "Fonte inválida")
	quer(t, ana.enviarBanner("/banners", novo(nil), []byte("isso não é imagem")), http.StatusBadRequest, "JPEG, PNG ou WebP")

	png, _ := base64.StdEncoding.DecodeString(png1x1)
	rec := ana.enviarBanner("/banners", novo(nil), png)
	quer(t, rec, http.StatusSeeOther)
	b := strings.SplitN(local(rec), "?", 2)[0]
	// o texto em branco foi descartado e o outro ficou
	quer(t, ana.get(local(rec)), http.StatusOK, "Campanha de outubro", "Banner criado", "Remover a imagem de fundo", "Dengue: elimine os focos")
	quer(t, ana.enviarBanner("/banners", novo(nil), nil), http.StatusBadRequest, "Já existe")

	img := ana.get(b + "/fundo")
	quer(t, img, http.StatusOK)
	if img.Header().Get("Content-Type") != "image/png" || !bytes.Equal(img.Body.Bytes(), png) {
		t.Fatalf("fundo servido errado: %s", img.Header().Get("Content-Type"))
	}
	quer(t, ana.get("/banners?busca=outubro&situacao=ativo"), http.StatusOK, "Campanha de outubro", "Imagem")

	// salvar sem nova imagem mantém a que existe; remover a tira
	quer(t, ana.enviarBanner(b, novo(map[string]string{"nome": "Campanha de novembro"}), nil), http.StatusSeeOther)
	quer(t, ana.get(b+"/fundo"), http.StatusOK)
	quer(t, ana.enviarBanner(b, novo(map[string]string{"remover_fundo": "1", "ativo": "0"}), nil), http.StatusSeeOther)
	quer(t, ana.get(b+"/fundo"), http.StatusNotFound)
	quer(t, ana.get("/banners?situacao=inativo"), http.StatusOK, "Cor lisa")

	quer(t, ana.post(b+"/excluir", url.Values{}), http.StatusSeeOther)
	quer(t, ana.get(b), http.StatusNotFound)
	quer(t, ana.get("/lixeira?tipo=banner"), http.StatusOK, "Campanha de outubro", "Banners")
}

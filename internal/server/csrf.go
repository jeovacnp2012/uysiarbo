package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

const csrfCookie = "uysi_csrf"

type csrfKey struct{}

// csrfToken devolve o token CSRF da requisição, para as telas colocarem nos formulários.
func csrfToken(ctx context.Context) string {
	s, _ := ctx.Value(csrfKey{}).(string)
	return s
}

// ehHTTPS diz se o cliente falou com a gente por HTTPS (direto ou pelo Nginx).
func ehHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// csrf protege contra requisições forjadas por outros sites: cada navegador recebe um token
// aleatório em cookie, e todo POST/PUT/PATCH/DELETE precisa devolvê-lo (no formulário ou no
// cabeçalho X-CSRF-Token) e vir do mesmo site (cabeçalho Origin).
func csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if semCliente(r.URL.Path) || ehAPI(r.URL.Path) { // a API usa token Bearer, sem cookie
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(csrfCookie)
		token := ""
		if err == nil && len(cookie.Value) == 43 {
			token = cookie.Value
		}

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if !origemConfere(r) {
				negarCSRF(w, r)
				return
			}
			enviado := r.Header.Get("X-CSRF-Token")
			if enviado == "" {
				if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
					// envio de arquivo: o token vem num campo do formulário, que só aparece depois de ler o corpo
					// (com limite, já que isso acontece antes do login ser conferido)
					r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						var grande *http.MaxBytesError
						if errors.As(err, &grande) {
							http.Error(w, "Arquivo grande demais.", http.StatusRequestEntityTooLarge)
							return
						}
					}
				} else {
					_ = r.ParseForm()
				}
				enviado = r.PostFormValue("csrf")
			}
			if token == "" || subtle.ConstantTimeCompare([]byte(enviado), []byte(token)) != 1 {
				negarCSRF(w, r)
				return
			}
		}

		if token == "" {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				http.Error(w, "erro interno", http.StatusInternalServerError)
				return
			}
			token = base64.RawURLEncoding.EncodeToString(b)
			http.SetCookie(w, &http.Cookie{
				Name: csrfCookie, Value: token, Path: "/", HttpOnly: true,
				SameSite: http.SameSiteLaxMode, Secure: ehHTTPS(r),
			})
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfKey{}, token)))
	})
}

// origemConfere recusa requisição que declara vir de outro site. Sem Origin (alguns clientes), a
// verificação do token continua valendo.
func origemConfere(r *http.Request) bool {
	origem := r.Header.Get("Origin")
	if origem == "" {
		return true
	}
	u, err := url.Parse(origem)
	return err == nil && u.Host == r.Host
}

func negarCSRF(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Requisição recusada (proteção contra falsificação). Recarregue a página e tente de novo.", http.StatusForbidden)
}

// securityHeaders acrescenta cabeçalhos básicos de segurança a toda resposta.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// Package apiauth reúne o que a API para sistemas externos usa para autenticar: escopos, credenciais
// (client_id e segredo), tokens JWT de vida curta e o limite de chamadas por minuto.
package apiauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------- Escopos

// Recurso é um conjunto de dados que a API expõe; cada um tem os escopos "<nome>:read" e "<nome>:write".
type Recurso struct {
	Nome   string // usado no escopo, ex.: "cidades"
	Rotulo string // mostrado nas telas
}

// Recursos lista o que a API expõe. Cada recurso novo da API entra aqui e ganha os dois escopos.
var Recursos = []Recurso{
	{"cidades", "Cidades"},
	{"bairros", "Bairros"},
	{"quarteiroes", "Quarteirões"},
	{"locais", "Locais"},
	{"importacoes", "Importações de dados"},
}

// Escopo monta o texto do escopo, ex.: Escopo("cidades", true) = "cidades:write".
func Escopo(recurso string, escrita bool) string {
	if escrita {
		return recurso + ":write"
	}
	return recurso + ":read"
}

// EscopoValido diz se o texto é um escopo conhecido.
func EscopoValido(s string) bool {
	nome, modo, ok := strings.Cut(s, ":")
	if !ok || (modo != "read" && modo != "write") {
		return false
	}
	return slices.ContainsFunc(Recursos, func(r Recurso) bool { return r.Nome == nome })
}

// Normalizar tira repetidos e ordena; devolve erro se algum escopo não existir.
func Normalizar(escopos []string) ([]string, error) {
	vistos := map[string]bool{}
	var out []string
	for _, e := range escopos {
		e = strings.TrimSpace(e)
		if e == "" || vistos[e] {
			continue
		}
		if !EscopoValido(e) {
			return nil, fmt.Errorf("escopo desconhecido: %q", e)
		}
		vistos[e] = true
		out = append(out, e)
	}
	slices.Sort(out)
	return out, nil
}

// Permite diz se o conjunto de escopos cobre o pedido. Gravar ("x:write") não dá direito a ler ("x:read").
func Permite(escopos []string, necessario string) bool { return slices.Contains(escopos, necessario) }

// ---------------------------------------------------------------- Credenciais

const (
	prefixoClientID = "uac_"
	prefixoSegredo  = "uas_"
)

func aleatorio(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NovaCredencial gera o par (client_id, segredo) e o hash do segredo para guardar. O segredo só existe
// aqui e na tela que o mostra uma vez.
func NovaCredencial() (clientID, segredo string, hash []byte, err error) {
	id, err := aleatorio(15)
	if err != nil {
		return "", "", nil, err
	}
	seg, hash, err := NovoSegredo()
	if err != nil {
		return "", "", nil, err
	}
	return prefixoClientID + id, seg, hash, nil
}

// NovoSegredo gera só um segredo novo (rotação).
func NovoSegredo() (segredo string, hash []byte, err error) {
	s, err := aleatorio(32)
	if err != nil {
		return "", nil, err
	}
	segredo = prefixoSegredo + s
	return segredo, HashDoSegredo(segredo), nil
}

// HashDoSegredo é o SHA-256 do segredo. Como o segredo tem 256 bits aleatórios, um hash rápido basta
// (diferente de senhas de gente, que são fracas e pedem argon2).
func HashDoSegredo(segredo string) []byte {
	h := sha256.Sum256([]byte(segredo))
	return h[:]
}

// SegredoConfere compara o segredo enviado com o hash guardado, em tempo constante.
func SegredoConfere(enviado string, hash []byte) bool {
	return subtle.ConstantTimeCompare(HashDoSegredo(enviado), hash) == 1
}

// ---------------------------------------------------------------- Token (JWT HS256)

// Claims é o conteúdo do token. Cliente é o identificador (slug) do cliente do sistema: um token de
// um cliente não vale em outro.
type Claims struct {
	Emissor    string   `json:"iss"`
	Credencial int64    `json:"sub_id"`
	Cliente    string   `json:"cid"`
	Versao     int      `json:"ver"`
	Escopos    []string `json:"scp"`
	Emitido    int64    `json:"iat"`
	Expira     int64    `json:"exp"`
	ID         string   `json:"jti"`
}

var (
	ErrTokenInvalido = errors.New("token inválido")
	ErrTokenExpirado = errors.New("token expirado")
)

var cabecalho = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

func assinatura(chave []byte, dados string) string {
	m := hmac.New(sha256.New, chave)
	m.Write([]byte(dados))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// Assinar gera o token. A chave precisa ter pelo menos 32 bytes.
func Assinar(c Claims, chave []byte) (string, error) {
	if len(chave) < 32 {
		return "", errors.New("chave de assinatura curta demais (mínimo 32 bytes)")
	}
	if c.ID == "" {
		id, err := aleatorio(9)
		if err != nil {
			return "", err
		}
		c.ID = id
	}
	corpo, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	dados := cabecalho + "." + base64.RawURLEncoding.EncodeToString(corpo)
	return dados + "." + assinatura(chave, dados), nil
}

// Verificar confere assinatura, formato e validade. Só aceita HS256 (recusa "none" e outros algoritmos).
func Verificar(token string, chave []byte, agora time.Time) (Claims, error) {
	partes := strings.Split(token, ".")
	if len(partes) != 3 || partes[0] != cabecalho || len(chave) < 32 {
		return Claims{}, ErrTokenInvalido
	}
	esperado := assinatura(chave, partes[0]+"."+partes[1])
	if !hmac.Equal([]byte(esperado), []byte(partes[2])) {
		return Claims{}, ErrTokenInvalido
	}
	corpo, err := base64.RawURLEncoding.DecodeString(partes[1])
	if err != nil {
		return Claims{}, ErrTokenInvalido
	}
	var c Claims
	if err := json.Unmarshal(corpo, &c); err != nil || c.Credencial == 0 || c.Cliente == "" {
		return Claims{}, ErrTokenInvalido
	}
	if agora.Unix() >= c.Expira {
		return Claims{}, ErrTokenExpirado
	}
	return c, nil
}

// ---------------------------------------------------------------- Limite de chamadas

// Limitador é um balde de fichas por chave: enche o limite por minuto, aos poucos, e cada chamada gasta uma.
type Limitador struct {
	mu     sync.Mutex
	baldes map[string]*balde
	agora  func() time.Time
}

type balde struct {
	fichas float64
	visto  time.Time
}

// Permitir gasta uma ficha da chave. Devolve se passou, quantas restam (arredondado para baixo) e, se
// recusou, em quanto tempo haverá uma ficha.
func (l *Limitador) Permitir(chave string, porMinuto int) (ok bool, restam int, espera time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.agora == nil {
		l.agora = time.Now
	}
	if l.baldes == nil {
		l.baldes = map[string]*balde{}
	}
	agora := l.agora()
	b := l.baldes[chave]
	cheio := float64(porMinuto)
	if b == nil {
		b = &balde{fichas: cheio, visto: agora}
		l.baldes[chave] = b
	}
	b.fichas = min(cheio, b.fichas+agora.Sub(b.visto).Seconds()*cheio/60)
	b.visto = agora
	if b.fichas < 1 {
		return false, 0, time.Duration((1 - b.fichas) * 60 / cheio * float64(time.Second))
	}
	b.fichas--
	if len(l.baldes) > 10000 { // limpeza simples: esquece os baldes que já encheram de novo
		for k, v := range l.baldes {
			if agora.Sub(v.visto) > 2*time.Minute {
				delete(l.baldes, k)
			}
		}
	}
	return true, int(b.fichas), 0
}

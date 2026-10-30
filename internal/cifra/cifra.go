// Package cifra protege dados pessoais em repouso (LGPD): AES-256-GCM com uma chave da instalação.
//
// O valor guardado no banco vira "v1:" + base64(nonce|texto cifrado). Valor sem esse prefixo é
// tratado como texto puro (dado antigo, ainda não cifrado) e é devolvido como está; sem chave
// configurada (só em desenvolvimento) nada é cifrado.
package cifra

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
)

const prefixo = "v1:"

var (
	mu   sync.RWMutex
	aead cipher.AEAD
)

// Configurar liga a cifra com a chave em base64 (32 bytes). Chave vazia desliga.
func Configurar(chaveB64 string) error {
	mu.Lock()
	defer mu.Unlock()
	if chaveB64 == "" {
		aead = nil
		return nil
	}
	b, err := base64.StdEncoding.DecodeString(chaveB64)
	if err != nil || len(b) != 32 {
		return errors.New("DADOS_CHAVE precisa ser 32 bytes em base64 (gere com: uysiarbo dados chave)")
	}
	blk, err := aes.NewCipher(b)
	if err != nil {
		return err
	}
	aead, err = cipher.NewGCM(blk)
	return err
}

// NovaChave gera uma chave aleatória no formato esperado por Configurar.
func NovaChave() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// Ativa diz se há chave configurada.
func Ativa() bool {
	mu.RLock()
	defer mu.RUnlock()
	return aead != nil
}

// Cifrado diz se o valor já está no formato cifrado.
func Cifrado(s string) bool { return strings.HasPrefix(s, prefixo) }

// Cifrar devolve o valor cifrado. Vazio, já cifrado ou sem chave voltam como entraram.
func Cifrar(s string) string {
	mu.RLock()
	a := aead
	mu.RUnlock()
	if s == "" || a == nil || Cifrado(s) {
		return s
	}
	nonce := make([]byte, a.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(fmt.Sprintf("cifra: sem aleatoriedade: %v", err))
	}
	return prefixo + base64.StdEncoding.EncodeToString(a.Seal(nonce, nonce, []byte(s), nil))
}

// Decifrar devolve o texto puro. Valor sem prefixo volta como está; valor cifrado que não abre
// (chave errada ou dado adulterado) volta vazio, para nunca mostrar lixo como se fosse o dado.
func Decifrar(s string) string {
	if !Cifrado(s) {
		return s
	}
	mu.RLock()
	a := aead
	mu.RUnlock()
	if a == nil {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(s[len(prefixo):])
	if err != nil || len(raw) < a.NonceSize() {
		return ""
	}
	out, err := a.Open(nil, raw[:a.NonceSize()], raw[a.NonceSize():], nil)
	if err != nil {
		return ""
	}
	return string(out)
}

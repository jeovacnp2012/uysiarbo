// Package auth cuida de senhas: hash seguro, verificação e geração de senhas fortes.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parâmetros do argon2id (recomendação OWASP: 19 MiB, 2 passes, 1 thread, com folga).
const (
	memoriaKiB = 64 * 1024
	passes     = 3
	paralelo   = 2
	tamSal     = 16
	tamHash    = 32
)

// Hash devolve a senha em formato PHC ("$argon2id$v=19$m=...,t=...,p=...$sal$hash"). A senha em si
// nunca é guardada.
func Hash(senha string) (string, error) {
	sal := make([]byte, tamSal)
	if _, err := rand.Read(sal); err != nil {
		return "", err
	}
	h := argon2.IDKey([]byte(senha), sal, passes, memoriaKiB, paralelo, tamHash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoriaKiB, passes, paralelo,
		base64.RawStdEncoding.EncodeToString(sal), base64.RawStdEncoding.EncodeToString(h)), nil
}

var ErrHashInvalido = errors.New("hash de senha em formato inválido")

// Confere diz se a senha corresponde ao hash. Um hash malformado é erro, não "senha errada".
func Confere(senha, hash string) (bool, error) {
	partes := strings.Split(hash, "$")
	if len(partes) != 6 || partes[1] != "argon2id" {
		return false, ErrHashInvalido
	}
	var versao int
	if _, err := fmt.Sscanf(partes[2], "v=%d", &versao); err != nil || versao != argon2.Version {
		return false, ErrHashInvalido
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(partes[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, ErrHashInvalido
	}
	sal, err1 := base64.RawStdEncoding.DecodeString(partes[4])
	esperado, err2 := base64.RawStdEncoding.DecodeString(partes[5])
	if err1 != nil || err2 != nil || len(esperado) == 0 {
		return false, ErrHashInvalido
	}
	obtido := argon2.IDKey([]byte(senha), sal, t, m, p, uint32(len(esperado)))
	return subtle.ConstantTimeCompare(obtido, esperado) == 1, nil
}

// Alfabetos sem caracteres que se confundem (0/O, 1/l/I) para a senha poder ser lida e digitada.
const (
	minusculas = "abcdefghijkmnopqrstuvwxyz"
	maiusculas = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	digitos    = "23456789"
	simbolos   = "!@#$%&*?-_"
)

// Gerar cria uma senha forte aleatória, com pelo menos uma minúscula, uma maiúscula, um dígito e um
// símbolo. Usada no reset de senha: quem resetou vê a senha uma única vez.
func Gerar(tamanho int) (string, error) {
	if tamanho < 12 {
		tamanho = 12
	}
	todos := minusculas + maiusculas + digitos + simbolos
	obrigatorios := []string{minusculas, maiusculas, digitos, simbolos}

	out := make([]byte, tamanho)
	for i := range out {
		conj := todos
		if i < len(obrigatorios) {
			conj = obrigatorios[i]
		}
		c, err := sortear(conj)
		if err != nil {
			return "", err
		}
		out[i] = c
	}
	// Embaralha, para os caracteres obrigatórios não ficarem sempre no começo.
	for i := len(out) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		out[i], out[j.Int64()] = out[j.Int64()], out[i]
	}
	return string(out), nil
}

func sortear(conjunto string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(conjunto))))
	if err != nil {
		return 0, err
	}
	return conjunto[n.Int64()], nil
}

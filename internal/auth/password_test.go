package auth

import (
	"strings"
	"testing"
)

func TestHashEConfere(t *testing.T) {
	h, err := Hash("Senha-Correta-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$") || strings.Contains(h, "Senha-Correta-1") {
		t.Fatalf("hash inesperado: %s", h)
	}
	if ok, err := Confere("Senha-Correta-1", h); err != nil || !ok {
		t.Errorf("senha certa: ok=%v err=%v", ok, err)
	}
	if ok, err := Confere("senha-errada", h); err != nil || ok {
		t.Errorf("senha errada: ok=%v err=%v", ok, err)
	}
	h2, _ := Hash("Senha-Correta-1")
	if h == h2 {
		t.Error("o mesmo texto deve gerar hashes diferentes (sal aleatório)")
	}
}

func TestConfereHashMalformado(t *testing.T) {
	for _, h := range []string{"", "texto", "$bcrypt$x$y$z$w", "$argon2id$v=19$m=1,t=1,p=1$@@@$@@@", "$argon2id$v=18$m=1,t=1,p=1$YQ$YQ"} {
		if ok, err := Confere("x", h); ok || err == nil {
			t.Errorf("hash %q deveria ser recusado (ok=%v err=%v)", h, ok, err)
		}
	}
}

func TestGerarSenhaForte(t *testing.T) {
	vistas := map[string]bool{}
	for i := 0; i < 200; i++ {
		s, err := Gerar(16)
		if err != nil {
			t.Fatal(err)
		}
		if len(s) != 16 {
			t.Fatalf("tamanho = %d", len(s))
		}
		if !strings.ContainsAny(s, minusculas) || !strings.ContainsAny(s, maiusculas) ||
			!strings.ContainsAny(s, digitos) || !strings.ContainsAny(s, simbolos) {
			t.Fatalf("faltou algum tipo de caractere em %q", s)
		}
		if strings.ContainsAny(s, "0O1lI") {
			t.Fatalf("caractere ambíguo em %q", s)
		}
		if vistas[s] {
			t.Fatalf("senha repetida: %q", s)
		}
		vistas[s] = true
	}
	if s, _ := Gerar(4); len(s) != 12 {
		t.Errorf("tamanho mínimo deveria ser 12, veio %d", len(s))
	}
}

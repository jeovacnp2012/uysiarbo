package apiauth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

var chave = []byte("0123456789abcdef0123456789abcdef")

func TestEscopos(t *testing.T) {
	for _, bom := range []string{"cidades:read", "locais:write", "quarteiroes:read"} {
		if !EscopoValido(bom) {
			t.Errorf("%q deveria ser válido", bom)
		}
	}
	for _, ruim := range []string{"", "cidades", "cidades:", "cidades:admin", "usuarios:read", ":read", "cidades:read:x"} {
		if EscopoValido(ruim) {
			t.Errorf("%q deveria ser inválido", ruim)
		}
	}
	got, err := Normalizar([]string{"locais:read", " cidades:write ", "locais:read", ""})
	if err != nil || strings.Join(got, ",") != "cidades:write,locais:read" {
		t.Errorf("Normalizar = %v, %v", got, err)
	}
	if _, err := Normalizar([]string{"x:read"}); err == nil {
		t.Error("escopo desconhecido deveria dar erro")
	}
	if Escopo("cidades", true) != "cidades:write" || Escopo("cidades", false) != "cidades:read" {
		t.Error("Escopo")
	}
	// gravar não dá direito de ler
	if Permite([]string{"cidades:write"}, "cidades:read") || !Permite([]string{"cidades:read"}, "cidades:read") {
		t.Error("Permite")
	}
}

func TestCredencial(t *testing.T) {
	id, segredo, hash, err := NovaCredencial()
	if err != nil || !strings.HasPrefix(id, "uac_") || !strings.HasPrefix(segredo, "uas_") || len(segredo) < 40 {
		t.Fatalf("NovaCredencial: %q %q %v", id, segredo, err)
	}
	if !SegredoConfere(segredo, hash) || SegredoConfere(segredo+"x", hash) || SegredoConfere("", hash) {
		t.Error("SegredoConfere")
	}
	id2, segredo2, _, _ := NovaCredencial()
	if id == id2 || segredo == segredo2 {
		t.Error("credenciais deveriam ser únicas")
	}
	novo, hash2, err := NovoSegredo()
	if err != nil || SegredoConfere(segredo, hash2) || !SegredoConfere(novo, hash2) {
		t.Error("NovoSegredo")
	}
}

func TestToken(t *testing.T) {
	agora := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	c := Claims{Emissor: "uysiarbo", Credencial: 7, Cliente: "novamutum", Versao: 2, Escopos: []string{"cidades:read"}, Emitido: agora.Unix(), Expira: agora.Add(15 * time.Minute).Unix()}
	tok, err := Assinar(c, chave)
	if err != nil || strings.Count(tok, ".") != 2 {
		t.Fatalf("Assinar: %q %v", tok, err)
	}
	got, err := Verificar(tok, chave, agora)
	if err != nil || got.Credencial != 7 || got.Cliente != "novamutum" || got.Versao != 2 || got.ID == "" || len(got.Escopos) != 1 {
		t.Fatalf("Verificar: %+v %v", got, err)
	}
	// expirado
	if _, err := Verificar(tok, chave, agora.Add(15*time.Minute)); err != ErrTokenExpirado {
		t.Errorf("expirado: %v", err)
	}
	// chave errada
	if _, err := Verificar(tok, []byte("ffffffffffffffffffffffffffffffff"), agora); err != ErrTokenInvalido {
		t.Errorf("chave errada: %v", err)
	}
	// adulterado: troca o corpo mantendo a assinatura
	partes := strings.Split(tok, ".")
	falso := base64.RawURLEncoding.EncodeToString([]byte(`{"sub_id":1,"cid":"outro","exp":9999999999,"scp":["locais:write"]}`))
	if _, err := Verificar(partes[0]+"."+falso+"."+partes[2], chave, agora); err != ErrTokenInvalido {
		t.Errorf("adulterado: %v", err)
	}
	// "alg":"none" e formatos estranhos
	none := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	for _, ruim := range []string{"", "a.b", "a.b.c.d", none + "." + partes[1] + ".", none + "." + partes[1] + "." + partes[2], "lixo"} {
		if _, err := Verificar(ruim, chave, agora); err == nil {
			t.Errorf("%q deveria ser recusado", ruim)
		}
	}
	// chave curta nunca assina nem confere
	if _, err := Assinar(c, []byte("curta")); err == nil {
		t.Error("chave curta deveria ser recusada")
	}
	if _, err := Verificar(tok, []byte("curta"), agora); err == nil {
		t.Error("chave curta deveria ser recusada na verificação")
	}
	// sem credencial ou sem cliente não vale
	vazio, _ := Assinar(Claims{Expira: agora.Add(time.Hour).Unix()}, chave)
	if _, err := Verificar(vazio, chave, agora); err != ErrTokenInvalido {
		t.Errorf("token vazio: %v", err)
	}
}

func TestLimitador(t *testing.T) {
	agora := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	l := &Limitador{agora: func() time.Time { return agora }}
	for i := 0; i < 5; i++ {
		ok, restam, _ := l.Permitir("a", 5)
		if !ok || restam != 4-i {
			t.Fatalf("chamada %d: ok=%v restam=%d", i+1, ok, restam)
		}
	}
	ok, _, espera := l.Permitir("a", 5)
	if ok || espera <= 0 || espera > 13*time.Second {
		t.Fatalf("a sexta deveria ser recusada: ok=%v espera=%v", ok, espera)
	}
	// outra chave tem o próprio balde
	if ok, _, _ := l.Permitir("b", 5); !ok {
		t.Error("outra credencial não deveria ser afetada")
	}
	// passados 12 s volta uma ficha (5 por minuto)
	agora = agora.Add(12 * time.Second)
	if ok, _, _ := l.Permitir("a", 5); !ok {
		t.Error("deveria ter voltado uma ficha")
	}
	if ok, _, _ := l.Permitir("a", 5); ok {
		t.Error("só voltou uma")
	}
	// depois de muito tempo, enche no máximo o limite
	agora = agora.Add(time.Hour)
	for i := 0; i < 5; i++ {
		if ok, _, _ := l.Permitir("a", 5); !ok {
			t.Fatalf("balde cheio, chamada %d", i+1)
		}
	}
	if ok, _, _ := l.Permitir("a", 5); ok {
		t.Error("o balde não passa do limite")
	}
}

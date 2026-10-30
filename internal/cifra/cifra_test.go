package cifra

import "testing"

func TestCifra(t *testing.T) {
	defer Configurar("")
	if got := Cifrar("12345678909"); got != "12345678909" {
		t.Fatalf("sem chave não cifra: %q", got)
	}
	chave, err := NovaChave()
	if err != nil {
		t.Fatal(err)
	}
	if err := Configurar(chave); err != nil {
		t.Fatal(err)
	}
	c := Cifrar("12345678909")
	if !Cifrado(c) || c == "12345678909" {
		t.Fatalf("não cifrou: %q", c)
	}
	if Cifrar(c) != c {
		t.Error("cifrou duas vezes")
	}
	if Cifrar("12345678909") == c {
		t.Error("o mesmo texto deveria gerar cifras diferentes (nonce)")
	}
	if Decifrar(c) != "12345678909" {
		t.Errorf("decifrou %q", Decifrar(c))
	}
	if Decifrar("12345678909") != "12345678909" {
		t.Error("texto antigo sem prefixo deve voltar como está")
	}
	if Cifrar("") != "" || Decifrar("") != "" {
		t.Error("vazio fica vazio")
	}
	if Decifrar(c[:len(c)-3]+"AAA") != "" {
		t.Error("dado adulterado deve voltar vazio")
	}
	outra, _ := NovaChave()
	Configurar(outra)
	if Decifrar(c) != "" {
		t.Error("chave errada deve voltar vazio")
	}
	if Configurar("curta") == nil {
		t.Error("chave inválida deveria dar erro")
	}
}

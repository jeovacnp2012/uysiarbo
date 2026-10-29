package guia

import (
	"strings"
	"testing"
)

func TestGuiaPadraoCarrega(t *testing.T) {
	g := Padrao
	if len(g.Artigos) < 9 {
		t.Fatalf("artigos = %d, quer pelo menos 9", len(g.Artigos))
	}
	vistos := map[string]bool{}
	for i, a := range g.Artigos {
		if a.Titulo == "" || a.Resumo == "" || a.HTML == "" {
			t.Errorf("artigo %q incompleto", a.Slug)
		}
		if !strings.Contains(a.HTML, "<h1>") {
			t.Errorf("artigo %q sem título no corpo", a.Slug)
		}
		if vistos[a.Slug] {
			t.Errorf("slug repetido %q", a.Slug)
		}
		vistos[a.Slug] = true
		if i > 0 && g.Artigos[i-1].Ordem > a.Ordem {
			t.Errorf("fora de ordem: %q", a.Slug)
		}
	}
	if a, ok := g.Artigo("primeiros-passos"); !ok || a.Permissao != "" {
		t.Errorf("primeiros-passos: %+v %v", a, ok)
	}
	if _, ok := g.Artigo("nao-existe"); ok {
		t.Error("artigo inexistente")
	}
}

func TestEveryPermissaoDoGuiaExisteNoCatalogo(t *testing.T) {
	// Um artigo restrito por uma permissão que não existe nunca apareceria para ninguém (só para o master).
	validas := map[string]bool{"conf_usuario": true, "conf_papel": true, "auditoria_sistema": true, PermissaoMaster: true}
	for _, a := range Padrao.Artigos {
		if a.Permissao != "" && !validas[a.Permissao] {
			t.Errorf("artigo %q usa a permissão %q, que não está na lista conhecida do teste", a.Slug, a.Permissao)
		}
	}
}

func TestPermissaoPorArtigo(t *testing.T) {
	tem := func(codigos ...string) func(string) bool {
		return func(c string) bool {
			for _, x := range codigos {
				if x == c {
					return true
				}
			}
			return false
		}
	}
	slugs := func(as []Artigo) string {
		var s []string
		for _, a := range as {
			s = append(s, a.Slug)
		}
		return strings.Join(s, ",")
	}
	semNada := slugs(Padrao.Visiveis(false, tem()))
	if strings.Contains(semNada, "usuarios") || strings.Contains(semNada, "clientes") || !strings.Contains(semNada, "primeiros-passos") {
		t.Errorf("sem permissões: %s", semNada)
	}
	if v := slugs(Padrao.Visiveis(false, tem("conf_usuario"))); !strings.Contains(v, "usuarios") || strings.Contains(v, "papeis") {
		t.Errorf("com conf_usuario: %s", v)
	}
	if v := slugs(Padrao.Visiveis(false, tem("conf_usuario", "conf_papel", "auditoria_sistema"))); strings.Contains(v, "clientes") {
		t.Errorf("artigos do console são só do master: %s", v)
	}
	todos := slugs(Padrao.Visiveis(true, tem()))
	if len(Padrao.Visiveis(true, tem())) != len(Padrao.Artigos) || !strings.Contains(todos, "clientes") {
		t.Errorf("o master lê tudo: %s", todos)
	}
}

func TestNormalizarIgnoraAcentoECaixa(t *testing.T) {
	if got := Normalizar("Conexão AÇÃO Ênfase"); got != "conexao acao enfase" {
		t.Errorf("Normalizar = %q", got)
	}
}

func TestBuscar(t *testing.T) {
	todos := Padrao.Artigos
	if n := len(Buscar(todos, "")); n != len(todos) {
		t.Errorf("busca vazia devolve tudo: %d de %d", n, len(todos))
	}
	if n := len(Buscar(todos, "   ")); n != len(todos) {
		t.Errorf("busca em branco: %d", n)
	}

	// Sem acento e sem diferenciar caixa.
	r := Buscar(todos, "SENHA temporaria")
	if len(r) == 0 {
		t.Fatal("deveria achar artigos sobre senha temporária")
	}
	if r[0].Trecho == "" {
		t.Error("o resultado deveria trazer um trecho")
	}
	// Título pesa mais: "Senhas" vem antes de quem só cita a palavra.
	if r := Buscar(todos, "senhas"); r[0].Artigo.Slug != "senhas" {
		t.Errorf("o artigo Senhas deveria vir primeiro, veio %q", r[0].Artigo.Slug)
	}
	// Palavra-chave que não está no texto: "redefinir" só existe em palavras.
	if r := Buscar(todos, "redefinir"); len(r) == 0 {
		t.Error("deveria achar pelas palavras-chave")
	}
	// Todas as palavras precisam aparecer.
	if r := Buscar(todos, "senha zzzinexistente"); len(r) != 0 {
		t.Errorf("com uma palavra que não existe, não acha nada: %d", len(r))
	}
	// A busca respeita a lista que recebe (o que a pessoa pode ler).
	sem := Padrao.Visiveis(false, func(string) bool { return false })
	for _, x := range Buscar(sem, "cliente") {
		if x.Artigo.Slug == "clientes" {
			t.Error("a busca não pode devolver o que a pessoa não pode ler")
		}
	}
}

func TestLerArtigoRecusaErros(t *testing.T) {
	casos := map[string]string{
		"sem separador":   "titulo: X\nresumo: Y\n# corpo",
		"sem título":      "resumo: Y\n---\ncorpo",
		"sem resumo":      "titulo: X\n---\ncorpo",
		"chave estranha":  "titulo: X\nresumo: Y\ncor: azul\n---\ncorpo",
		"ordem inválida":  "titulo: X\nresumo: Y\nordem: abc\n---\ncorpo",
		"linha sem chave": "titulo: X\nresumo: Y\nsolto\n---\ncorpo",
	}
	for nome, raw := range casos {
		if _, err := lerArtigo("x", raw); err == nil {
			t.Errorf("%s: deveria dar erro", nome)
		}
	}
	a, err := lerArtigo("ok", "titulo: Olá\nresumo: Resumo\nordem: 3\npermissao: master\n---\n# Título\n\nTexto **forte**.\n")
	if err != nil || a.Ordem != 3 || a.Permissao != "master" || !strings.Contains(a.HTML, "<strong>forte</strong>") {
		t.Errorf("artigo válido: %+v %v", a, err)
	}
}

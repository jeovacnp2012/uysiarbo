// Package guia carrega o Guia do sistema: artigos em Markdown embutidos no binário, com permissão
// por artigo e busca que ignora maiúsculas e acentos.
package guia

import (
	"bytes"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"golang.org/x/text/unicode/norm"
)

//go:embed conteudo/*.md
var conteudoFS embed.FS

// PermissaoMaster, no cabeçalho de um artigo, restringe o artigo ao usuário master.
const PermissaoMaster = "master"

type Artigo struct {
	Slug      string
	Titulo    string
	Resumo    string
	Ordem     int
	Permissao string // código de permissão, "master" ou vazio (qualquer pessoa logada)
	Palavras  string // termos extras para a busca
	HTML      string // corpo já convertido
	texto     string // corpo em texto puro, normalizado, para a busca
	bruto     string // corpo em texto puro original, para o trecho exibido
	titulo    string // título normalizado
	extras    string // resumo + palavras, normalizado
}

// Guia é o conjunto de artigos.
type Guia struct{ Artigos []Artigo }

// Padrao é o guia embutido no binário.
var Padrao = mustCarregar()

func mustCarregar() *Guia {
	g, err := Carregar()
	if err != nil {
		panic("guia inválido: " + err.Error())
	}
	return g
}

// Carregar lê os artigos de conteudo/*.md. Cada arquivo começa com linhas "chave: valor"
// (titulo, resumo, ordem, permissao, palavras), uma linha "---" e o texto em Markdown.
func Carregar() (*Guia, error) {
	arquivos, err := conteudoFS.ReadDir("conteudo")
	if err != nil {
		return nil, err
	}
	g := &Guia{}
	vistos := map[string]bool{}
	for _, f := range arquivos {
		raw, err := conteudoFS.ReadFile("conteudo/" + f.Name())
		if err != nil {
			return nil, err
		}
		a, err := lerArtigo(strings.TrimSuffix(f.Name(), ".md"), string(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name(), err)
		}
		if vistos[a.Slug] {
			return nil, fmt.Errorf("artigo repetido: %s", a.Slug)
		}
		vistos[a.Slug] = true
		g.Artigos = append(g.Artigos, a)
	}
	sort.SliceStable(g.Artigos, func(i, j int) bool {
		if g.Artigos[i].Ordem != g.Artigos[j].Ordem {
			return g.Artigos[i].Ordem < g.Artigos[j].Ordem
		}
		return g.Artigos[i].Titulo < g.Artigos[j].Titulo
	})
	return g, nil
}

func lerArtigo(slug, raw string) (Artigo, error) {
	cab, corpo, ok := strings.Cut(strings.ReplaceAll(raw, "\r\n", "\n"), "\n---\n")
	if !ok {
		return Artigo{}, fmt.Errorf("falta a linha --- depois do cabeçalho")
	}
	a := Artigo{Slug: slug}
	for _, linha := range strings.Split(cab, "\n") {
		linha = strings.TrimSpace(linha)
		if linha == "" {
			continue
		}
		chave, valor, ok := strings.Cut(linha, ":")
		if !ok {
			return Artigo{}, fmt.Errorf("cabeçalho inválido: %q", linha)
		}
		valor = strings.TrimSpace(valor)
		switch strings.TrimSpace(chave) {
		case "titulo":
			a.Titulo = valor
		case "resumo":
			a.Resumo = valor
		case "ordem":
			n, err := strconv.Atoi(valor)
			if err != nil {
				return Artigo{}, fmt.Errorf("ordem inválida: %q", valor)
			}
			a.Ordem = n
		case "permissao":
			a.Permissao = valor
		case "palavras":
			a.Palavras = valor
		default:
			return Artigo{}, fmt.Errorf("chave desconhecida no cabeçalho: %q", chave)
		}
	}
	if a.Titulo == "" || a.Resumo == "" {
		return Artigo{}, fmt.Errorf("titulo e resumo são obrigatórios")
	}
	var html bytes.Buffer
	if err := goldmark.Convert([]byte(corpo), &html); err != nil {
		return Artigo{}, err
	}
	a.HTML = html.String()
	a.bruto = textoPuro(corpo)
	a.texto = Normalizar(a.bruto)
	a.titulo = Normalizar(a.Titulo)
	a.extras = Normalizar(a.Resumo + " " + a.Palavras)
	return a, nil
}

// textoPuro tira a marcação do Markdown que atrapalha a busca e o trecho (títulos, listas, ênfase).
func textoPuro(md string) string {
	var out []string
	for _, l := range strings.Split(md, "\n") {
		l = strings.TrimSpace(l)
		l = strings.TrimLeft(l, "#>-*0123456789. ")
		l = strings.NewReplacer("**", "", "__", "", "`", "").Replace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// Normalizar põe em minúsculas e tira os acentos ("Conexão" vira "conexao").
func Normalizar(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Pode diz se a pessoa pode ler o artigo. pode(codigo) diz se ela tem a permissão; master, se é master.
func (a Artigo) Pode(master bool, pode func(string) bool) bool {
	switch a.Permissao {
	case "":
		return true
	case PermissaoMaster:
		return master
	}
	return master || pode(a.Permissao)
}

// Artigo devolve um artigo pelo endereço.
func (g *Guia) Artigo(slug string) (Artigo, bool) {
	for _, a := range g.Artigos {
		if a.Slug == slug {
			return a, true
		}
	}
	return Artigo{}, false
}

// Visiveis devolve só os artigos que a pessoa pode ler.
func (g *Guia) Visiveis(master bool, pode func(string) bool) []Artigo {
	var out []Artigo
	for _, a := range g.Artigos {
		if a.Pode(master, pode) {
			out = append(out, a)
		}
	}
	return out
}

// Resultado é um artigo encontrado na busca, com um trecho do texto.
type Resultado struct {
	Artigo Artigo
	Trecho string
	Pontos int
}

// Buscar procura nos artigos dados. Todas as palavras da consulta precisam aparecer (no título, no
// resumo, nas palavras-chave ou no texto). Título pesa mais que o resto. Consulta vazia devolve tudo.
func Buscar(artigos []Artigo, consulta string) []Resultado {
	termos := strings.Fields(Normalizar(consulta))
	var out []Resultado
	for _, a := range artigos {
		r := Resultado{Artigo: a}
		if len(termos) == 0 {
			out = append(out, r)
			continue
		}
		ok := true
		for _, t := range termos {
			n := strings.Count(a.titulo, t)*10 + strings.Count(a.extras, t)*4 + strings.Count(a.texto, t)
			if n == 0 {
				ok = false
				break
			}
			r.Pontos += n
		}
		if !ok {
			continue
		}
		r.Trecho = trecho(a, termos[0])
		out = append(out, r)
	}
	if len(termos) > 0 {
		sort.SliceStable(out, func(i, j int) bool { return out[i].Pontos > out[j].Pontos })
	}
	return out
}

// trecho devolve a primeira linha do texto que contém o termo (ou o resumo, se só o título bate).
func trecho(a Artigo, termo string) string {
	linhas := strings.Split(a.bruto, "\n")
	for _, l := range linhas {
		if strings.Contains(Normalizar(l), termo) {
			if r := []rune(l); len(r) > 180 {
				return string(r[:180]) + "…"
			}
			return l
		}
	}
	return a.Resumo
}

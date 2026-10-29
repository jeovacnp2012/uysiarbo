// Package authz define o catálogo de permissões (organizado em blocos e grupos) e decide o que
// uma pessoa pode fazer. O usuário master não está sujeito a nenhuma regra.
package authz

import (
	"bufio"
	"embed"
	"fmt"
	"strings"
)

//go:embed catalogo.txt
var catalogoFS embed.FS

// Permissao é uma ação que um papel pode conceder.
type Permissao struct {
	Codigo     string
	Rotulo     string
	Restritiva bool // limita o escopo dos dados (ex.: "visualizar apenas as minhas")
}

type Grupo struct {
	Nome       string
	Permissoes []Permissao
}

// Bloco é uma área do sistema (Visitas, Locais, Estoque...), com seus grupos de permissões.
type Bloco struct {
	Nome   string
	Grupos []Grupo
}

// Catalogo é o conjunto completo de permissões, na ordem de exibição.
type Catalogo struct {
	Blocos []Bloco
	porCod map[string]Permissao
}

// Padrao é o catálogo embutido no binário (internal/authz/catalogo.txt).
var Padrao = mustLoad()

func mustLoad() *Catalogo {
	raw, err := catalogoFS.ReadFile("catalogo.txt")
	if err != nil {
		panic(err)
	}
	c, err := Parse(string(raw))
	if err != nil {
		panic("catálogo de permissões inválido: " + err.Error())
	}
	return c
}

// Parse lê o formato do catalogo.txt. Linhas em branco e as que começam com ";" são ignoradas.
func Parse(texto string) (*Catalogo, error) {
	c := &Catalogo{porCod: map[string]Permissao{}}
	sc := bufio.NewScanner(strings.NewReader(texto))
	for n := 1; sc.Scan(); n++ {
		linha := strings.TrimSpace(sc.Text())
		switch {
		case linha == "" || strings.HasPrefix(linha, ";"):
		case strings.HasPrefix(linha, "## "):
			b := c.ultimoBloco()
			if b == nil {
				return nil, fmt.Errorf("linha %d: grupo sem bloco", n)
			}
			b.Grupos = append(b.Grupos, Grupo{Nome: strings.TrimPrefix(linha, "## ")})
		case strings.HasPrefix(linha, "# "):
			c.Blocos = append(c.Blocos, Bloco{Nome: strings.TrimPrefix(linha, "# ")})
		default:
			partes := strings.Split(linha, "|")
			if len(partes) != 3 || partes[0] == "" || partes[1] == "" {
				return nil, fmt.Errorf("linha %d: formato inválido %q", n, linha)
			}
			p := Permissao{Codigo: partes[0], Rotulo: partes[1], Restritiva: partes[2] == "R"}
			if _, dup := c.porCod[p.Codigo]; dup {
				return nil, fmt.Errorf("linha %d: permissão repetida %q", n, p.Codigo)
			}
			b := c.ultimoBloco()
			if b == nil {
				return nil, fmt.Errorf("linha %d: permissão sem bloco", n)
			}
			if len(b.Grupos) == 0 {
				b.Grupos = append(b.Grupos, Grupo{Nome: b.Nome})
			}
			g := &b.Grupos[len(b.Grupos)-1]
			g.Permissoes = append(g.Permissoes, p)
			c.porCod[p.Codigo] = p
		}
	}
	return c, sc.Err()
}

func (c *Catalogo) ultimoBloco() *Bloco {
	if len(c.Blocos) == 0 {
		return nil
	}
	return &c.Blocos[len(c.Blocos)-1]
}

// Existe diz se o código pertence ao catálogo.
func (c *Catalogo) Existe(codigo string) bool { _, ok := c.porCod[codigo]; return ok }

// Permissao devolve a permissão pelo código.
func (c *Catalogo) Permissao(codigo string) (Permissao, bool) {
	p, ok := c.porCod[codigo]
	return p, ok
}

// Codigos devolve todos os códigos, na ordem do catálogo. Se incluirRestritivas for falso, omite as
// permissões que apenas limitam o escopo (as que um "Administrador" não deve ter).
func (c *Catalogo) Codigos(incluirRestritivas bool) []string {
	var out []string
	for _, b := range c.Blocos {
		for _, g := range b.Grupos {
			for _, p := range g.Permissoes {
				if incluirRestritivas || !p.Restritiva {
					out = append(out, p.Codigo)
				}
			}
		}
	}
	return out
}

// Invalidos devolve os códigos que não pertencem ao catálogo (para validar o que vem de formulários).
func (c *Catalogo) Invalidos(codigos []string) []string {
	var out []string
	for _, cod := range codigos {
		if !c.Existe(cod) {
			out = append(out, cod)
		}
	}
	return out
}

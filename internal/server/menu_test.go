package server

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

var (
	reGrupoAberto    = regexp.MustCompile(`<details class="menu-grupo" open>\s*<summary[^>]*>.*?<span class="menu-texto">([^<]+)</span>`)
	reGrupoRecolhido = regexp.MustCompile(`<details class="menu-grupo">\s*<summary[^>]*>.*?<span class="menu-texto">([^<]+)</span>`)
)

// gruposDoMenu devolve os nomes dos grupos abertos e dos recolhidos da página.
func gruposDoMenu(corpo string) (abertos, recolhidos []string) {
	// O (?s) deixa o "." atravessar as quebras de linha do ícone SVG.
	for _, m := range regexp.MustCompile(`(?s)`+reGrupoAberto.String()).FindAllStringSubmatch(corpo, -1) {
		abertos = append(abertos, m[1])
	}
	for _, m := range regexp.MustCompile(`(?s)`+reGrupoRecolhido.String()).FindAllStringSubmatch(corpo, -1) {
		recolhidos = append(recolhidos, m[1])
	}
	return abertos, recolhidos
}

func TestMenuPorCategoriaComGruposRecolhidos(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)

	// Na página inicial nenhum grupo está aberto: todos começam recolhidos.
	abertos, recolhidos := gruposDoMenu(ana.get("/").Body.String())
	if len(abertos) != 0 {
		t.Errorf("na página inicial os grupos deveriam estar recolhidos, abertos: %v", abertos)
	}
	if strings.Join(recolhidos, ",") != "Território,Administração" {
		t.Errorf("grupos recolhidos = %v, quer Território e Administração", recolhidos)
	}

	// Numa tela de um grupo, só esse grupo abre.
	abertos, recolhidos = gruposDoMenu(ana.get("/bairros").Body.String())
	if strings.Join(abertos, ",") != "Território" || strings.Join(recolhidos, ",") != "Administração" {
		t.Errorf("em /bairros: abertos=%v recolhidos=%v", abertos, recolhidos)
	}
	abertos, _ = gruposDoMenu(ana.get("/usuarios").Body.String())
	if strings.Join(abertos, ",") != "Administração" {
		t.Errorf("em /usuarios: abertos=%v", abertos)
	}
	// Dentro de um bairro, a categoria dele continua aberta.
	bid := m.criarBairro(t, m.criarCidade(t, "Nova Mutum", "MT"), "Centro")
	abertos, _ = gruposDoMenu(ana.get("/bairros/" + s64(bid)).Body.String())
	if strings.Join(abertos, ",") != "Território" {
		t.Errorf("na ficha do bairro: abertos=%v", abertos)
	}

	// Itens simples (sem grupo) e o item ativo marcado.
	corpo := ana.get("/guia").Body.String()
	for _, quer := range []string{`class="menu-item" href="/"`, `class="menu-item" href="/guia" aria-current`, `class="menu-item" href="/novidades"`, `class="menu-subitem" href="/bairros"`, `class="menu-subitem" href="/auditoria"`} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("o menu deveria conter %q", quer)
		}
	}
	if !strings.Contains(corpo, `class="menu-cliente">Prefeitura de Nova Mutum`) {
		t.Error("o nome do ambiente deveria aparecer no topo do menu")
	}
}

func TestMenuEscondeCategoriasSemPermissao(t *testing.T) {
	m := novoMundo(t)
	carla := m.comCarla(t) // só leitura de visitas: sem Território nem Administração
	abertos, recolhidos := gruposDoMenu(carla.get("/").Body.String())
	if len(abertos)+len(recolhidos) != 0 {
		t.Errorf("a Carla não deveria ter categorias com sub-itens, tem %v %v", abertos, recolhidos)
	}
	corpo := carla.get("/").Body.String()
	for _, quer := range []string{`href="/guia"`, `href="/novidades"`} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("todo mundo vê %s", quer)
		}
	}
}

func TestMenuDoConsoleDoMaster(t *testing.T) {
	m := novoMundo(t)
	console := m.consoleMaster(t)
	rec := console.get("/clientes")
	quer(t, rec, http.StatusOK, "Console da plataforma")
	abertos, recolhidos := gruposDoMenu(rec.Body.String())
	if strings.Join(abertos, ",") != "Plataforma" || strings.Join(recolhidos, ",") != "Administração" {
		t.Errorf("no console, em /clientes: abertos=%v recolhidos=%v", abertos, recolhidos)
	}
	corpo := rec.Body.String()
	for _, quer := range []string{`href="/clientes"`, `href="/plataforma/usuarios"`, `href="/auditoria"`, `href="/componentes"`} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("o menu do console deveria conter %s", quer)
		}
	}
}

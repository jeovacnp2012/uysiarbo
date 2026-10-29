package authz

import (
	"strings"
	"testing"
)

func TestCatalogoPadraoEstaCoerente(t *testing.T) {
	c := Padrao
	if n := len(c.Codigos(true)); n != 171 {
		t.Errorf("permissões = %d, quer 171", n)
	}
	if n := len(c.Blocos); n != 23 {
		t.Errorf("blocos = %d, quer 23", n)
	}
	if n := len(c.Codigos(true)) - len(c.Codigos(false)); n != 11 {
		t.Errorf("restritivas = %d, quer 11", n)
	}
	for _, b := range c.Blocos {
		if len(b.Grupos) == 0 {
			t.Errorf("bloco %q sem grupos", b.Nome)
		}
		for _, g := range b.Grupos {
			if len(g.Permissoes) == 0 {
				t.Errorf("grupo %q/%q vazio", b.Nome, g.Nome)
			}
		}
	}
	p, ok := c.Permissao("visita_visualizar_minha")
	if !ok || !p.Restritiva || p.Rotulo != "Visualizar apenas as minhas" {
		t.Errorf("visita_visualizar_minha = %+v, %v", p, ok)
	}
	if v, _ := c.Permissao("visita_adicionar"); v.Restritiva {
		t.Error("visita_adicionar não é restritiva")
	}
}

func TestParseRecusaErros(t *testing.T) {
	casos := map[string]string{
		"permissão sem bloco": "a_b|Rótulo|",
		"grupo sem bloco":     "## Grupo\n",
		"formato inválido":    "# Bloco\nsó_codigo\n",
		"repetida":            "# B\na|A|\na|B|\n",
		"sem rótulo":          "# B\na||\n",
	}
	for nome, texto := range casos {
		if _, err := Parse(texto); err == nil {
			t.Errorf("%s: deveria dar erro", nome)
		}
	}
	c, err := Parse("; comentário\n\n# B\n## G\na|A|\nb|B|R\n")
	if err != nil || len(c.Codigos(true)) != 2 || len(c.Codigos(false)) != 1 {
		t.Errorf("parse válido falhou: %v", err)
	}
}

func TestInvalidos(t *testing.T) {
	got := Padrao.Invalidos([]string{"visita_adicionar", "nao_existe", "x"})
	if strings.Join(got, ",") != "nao_existe,x" {
		t.Errorf("Invalidos = %v", got)
	}
}

func TestMasterPodeTudoENuncaERestrito(t *testing.T) {
	m := Master()
	for _, cod := range append(Padrao.Codigos(true), "codigo_futuro_que_nao_existe") {
		if !m.Pode(cod) {
			t.Fatalf("master deveria poder %q", cod)
		}
	}
	if m.Restrito("visita_visualizar_minha") {
		t.Error("o master nunca é restringido")
	}
	for _, b := range Padrao.Blocos {
		if !m.PodeBloco(Padrao, b.Nome) {
			t.Errorf("master deveria ver o bloco %q", b.Nome)
		}
	}
}

func TestPrincipalComumSoPodeOQueTem(t *testing.T) {
	p := Com("visita_visualizar", "visita_visualizar_minha")
	if !p.Pode("visita_visualizar") || p.Pode("visita_adicionar") {
		t.Error("Pode errado")
	}
	if !p.Restrito("visita_visualizar_minha") || p.Restrito("local_meubairro") {
		t.Error("Restrito errado")
	}
	if !p.PodeAlguma("x", "visita_visualizar") || p.PodeAlguma("x", "y") {
		t.Error("PodeAlguma errado")
	}
	if !p.PodeBloco(Padrao, "Visitas") || p.PodeBloco(Padrao, "Estoque") {
		t.Error("PodeBloco errado")
	}
	// Só uma permissão restritiva não abre o bloco (ela apenas limita o que já se pode ver).
	if Com("local_meubairro").PodeBloco(Padrao, "Locais") {
		t.Error("permissão restritiva sozinha não deveria liberar o bloco")
	}
	if (Principal{}).Pode("visita_adicionar") {
		t.Error("principal vazio não pode nada")
	}
}

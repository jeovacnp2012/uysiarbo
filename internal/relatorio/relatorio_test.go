package relatorio

import (
	"bytes"
	"strings"
	"testing"
)

func exemplo() Doc {
	return Doc{
		Titulo: "Visitas", Cliente: "Nova Mutum", Gerado: "30/10/2026 08:00",
		Filtros: []Par{{"Ciclo", "05/2026"}},
		Colunas: []Coluna{{Titulo: "Endereço", Peso: 3}, {Titulo: "Total", Peso: 1, Direita: true}},
		Linhas:  [][]string{{"Rua São João, 10", "3"}, {"=SOMA(A1)", "1"}},
		Totais:  []string{"Total", "4"},
	}
}

func TestFormatos(t *testing.T) {
	var csv bytes.Buffer
	if err := exemplo().CSV(&csv); err != nil {
		t.Fatal(err)
	}
	s := csv.String()
	if !strings.HasPrefix(s, "\xEF\xBB\xBF") || !strings.Contains(s, "Endereço;Total") || !strings.Contains(s, "'=SOMA(A1)") {
		t.Fatalf("CSV inesperado: %q", s)
	}
	var x bytes.Buffer
	if err := exemplo().XLSX(&x); err != nil || !bytes.HasPrefix(x.Bytes(), []byte("PK")) {
		t.Fatalf("XLSX inválido: %v", err)
	}
	var p bytes.Buffer
	if err := exemplo().PDF(&p); err != nil || !bytes.HasPrefix(p.Bytes(), []byte("%PDF")) {
		t.Fatalf("PDF inválido: %v", err)
	}
	// Muitas linhas e colunas: quebra de página e paisagem sem falhar.
	d := exemplo()
	d.Colunas = append(d.Colunas, make([]Coluna, 6)...)
	for i := range d.Colunas {
		d.Colunas[i].Peso = 1
	}
	d.Totais = nil
	d.Linhas = nil
	for i := 0; i < 200; i++ {
		l := make([]string, len(d.Colunas))
		l[0] = "texto muito longo para cortar na célula do relatório em PDF número"
		d.Linhas = append(d.Linhas, l)
	}
	p.Reset()
	if err := d.PDF(&p); err != nil {
		t.Fatal(err)
	}
}

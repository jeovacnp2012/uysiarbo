// Package relatorio é o motor de relatórios: um Doc (título, filtros usados, colunas, linhas e totais) que sai
// em tela, CSV, planilha (XLSX) e PDF. Cada relatório do sistema só monta o Doc; os formatos são daqui.
package relatorio

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
)

// Coluna descreve uma coluna do relatório. Peso é a largura relativa no PDF; Direita alinha números.
type Coluna struct {
	Titulo  string
	Peso    float64
	Direita bool
}

// Par é um filtro usado, mostrado no cabeçalho ("Ciclo: 05/2026").
type Par struct{ Nome, Valor string }

// Doc é o conteúdo de um relatório pronto para sair em qualquer formato.
type Doc struct {
	Titulo   string
	Cliente  string
	Gerado   string // "30/10/2026 08:15", já no fuso do cliente
	Filtros  []Par
	Colunas  []Coluna
	Linhas   [][]string
	Totais   []string // opcional: mesma quantidade de colunas
	Cortado  bool     // true quando o relatório passou do limite e foi cortado
	Observar string   // nota exibida no fim (fórmulas, critérios)
}

// MaxLinhas é o teto de linhas de um relatório; acima disso o Doc sai cortado e avisado.
const MaxLinhas = 20000

// TextoSeguro protege planilhas contra injeção de fórmula.
func TextoSeguro(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// CSV escreve o relatório em CSV com BOM e ponto e vírgula (abre direto no Excel brasileiro).
func (d Doc) CSV(w io.Writer) error {
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	cab := make([]string, len(d.Colunas))
	for i, c := range d.Colunas {
		cab[i] = c.Titulo
	}
	if err := cw.Write(cab); err != nil {
		return err
	}
	for _, l := range d.Linhas {
		seg := make([]string, len(l))
		for i, v := range l {
			seg[i] = TextoSeguro(v)
		}
		if err := cw.Write(seg); err != nil {
			return err
		}
	}
	if len(d.Totais) > 0 {
		if err := cw.Write(d.Totais); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// XLSX escreve o relatório como planilha, com o cabeçalho de filtros no alto.
func (d Doc) XLSX(w io.Writer) error {
	f := excelize.NewFile()
	defer f.Close()
	const aba = "Relatório"
	f.SetSheetName("Sheet1", aba)
	negrito, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	linha := 1
	celula := func(col, row int) string { n, _ := excelize.CoordinatesToCellName(col, row); return n }
	_ = f.SetCellValue(aba, celula(1, linha), d.Titulo)
	_ = f.SetCellStyle(aba, celula(1, linha), celula(1, linha), negrito)
	linha++
	for _, p := range d.Filtros {
		_ = f.SetCellValue(aba, celula(1, linha), p.Nome+": "+p.Valor)
		linha++
	}
	linha++
	for i, c := range d.Colunas {
		_ = f.SetCellValue(aba, celula(i+1, linha), c.Titulo)
		_ = f.SetCellStyle(aba, celula(i+1, linha), celula(i+1, linha), negrito)
	}
	linha++
	for _, l := range d.Linhas {
		for i, v := range l {
			_ = f.SetCellValue(aba, celula(i+1, linha), TextoSeguro(v))
		}
		linha++
	}
	if len(d.Totais) > 0 {
		for i, v := range d.Totais {
			_ = f.SetCellValue(aba, celula(i+1, linha), v)
			_ = f.SetCellStyle(aba, celula(i+1, linha), celula(i+1, linha), negrito)
		}
	}
	for i, c := range d.Colunas {
		nome, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(aba, nome, nome, 8+c.Peso*4)
	}
	return f.Write(w)
}

// PDF escreve o relatório em A4 (paisagem quando há muitas colunas), com cabeçalho repetido em cada página.
func (d Doc) PDF(w io.Writer) error {
	orient := "P"
	if len(d.Colunas) > 6 {
		orient = "L"
	}
	pdf := fpdf.New(orient, "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetMargins(10, 12, 10)
	pdf.SetAutoPageBreak(true, 14)
	largura, _ := pdf.GetPageSize()
	util := largura - 20

	soma := 0.0
	for _, c := range d.Colunas {
		p := c.Peso
		if p <= 0 {
			p = 1
		}
		soma += p
	}
	larg := make([]float64, len(d.Colunas))
	for i, c := range d.Colunas {
		p := c.Peso
		if p <= 0 {
			p = 1
		}
		larg[i] = util * p / soma
	}

	cabecalhoTabela := func() {
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetFillColor(21, 90, 158)
		pdf.SetTextColor(255, 255, 255)
		for i, c := range d.Colunas {
			al := "L"
			if c.Direita {
				al = "R"
			}
			pdf.CellFormat(larg[i], 6, tr(c.Titulo), "0", 0, al, true, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetTextColor(0, 0, 0)
	}
	pdf.SetHeaderFunc(func() {
		pdf.SetFont("Helvetica", "B", 12)
		pdf.CellFormat(0, 6, tr(d.Titulo), "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		if d.Cliente != "" {
			pdf.CellFormat(0, 4, tr(d.Cliente), "", 1, "L", false, 0, "")
		}
		pdf.Ln(1)
	})
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(0, 4, tr(fmt.Sprintf("UYSI ARBO · gerado em %s · página %d de {nb}", d.Gerado, pdf.PageNo())), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})
	pdf.AliasNbPages("")
	pdf.AddPage()

	pdf.SetFont("Helvetica", "", 8)
	for _, p := range d.Filtros {
		pdf.CellFormat(0, 4, tr(p.Nome+": "+p.Valor), "", 1, "L", false, 0, "")
	}
	pdf.Ln(2)
	cabecalhoTabela()
	pdf.SetFont("Helvetica", "", 7.5)
	for n, l := range d.Linhas {
		if pdf.GetY() > 190 && orient == "L" || pdf.GetY() > 275 {
			pdf.AddPage()
			cabecalhoTabela()
			pdf.SetFont("Helvetica", "", 7.5)
		}
		if n%2 == 1 {
			pdf.SetFillColor(241, 245, 249)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		for i, v := range l {
			al := "L"
			if d.Colunas[i].Direita {
				al = "R"
			}
			pdf.CellFormat(larg[i], 5, cortar(pdf, tr(v), larg[i]-1), "0", 0, al, true, 0, "")
		}
		pdf.Ln(-1)
	}
	if len(d.Totais) > 0 {
		pdf.SetFont("Helvetica", "B", 7.5)
		pdf.SetFillColor(226, 232, 240)
		for i, v := range d.Totais {
			al := "L"
			if d.Colunas[i].Direita {
				al = "R"
			}
			pdf.CellFormat(larg[i], 5.5, tr(v), "0", 0, al, true, 0, "")
		}
		pdf.Ln(-1)
	}
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "I", 7)
	if d.Cortado {
		pdf.MultiCell(0, 3.5, tr(fmt.Sprintf("Relatório cortado em %d linhas: estreite os filtros para ver o restante.", MaxLinhas)), "", "L", false)
	}
	if d.Observar != "" {
		pdf.MultiCell(0, 3.5, tr(d.Observar), "", "L", false)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// cortar reduz o texto para caber na largura da célula (acrescenta reticências).
func cortar(pdf *fpdf.Fpdf, s string, largura float64) string {
	if pdf.GetStringWidth(s) <= largura {
		return s
	}
	for len(s) > 1 && pdf.GetStringWidth(s+"...") > largura {
		s = s[:len(s)-1]
	}
	return s + "..."
}

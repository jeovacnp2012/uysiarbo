// Package importacao lê planilhas (CSV e XLSX) e descreve as colunas de cada tipo de dado importável.
// Nada aqui grava no banco: a análise, a prévia e a execução ficam no servidor.
package importacao

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const (
	MaxBytes  = 10 << 20 // tamanho máximo do arquivo
	MaxLinhas = 20000    // linhas de dados por importação
)

// Linha é uma linha de dados da planilha, com o número que a pessoa vê no Excel (o cabeçalho é a linha 1).
type Linha struct {
	N       int
	Celulas []string
}

// Planilha é o conteúdo lido: o cabeçalho e as linhas de dados (linhas totalmente em branco são puladas).
type Planilha struct {
	Cabecalho []string
	Linhas    []Linha
}

// Ler lê um arquivo CSV ou XLSX (pelo conteúdo, não só pelo nome).
func Ler(nome string, dados []byte) (Planilha, error) {
	if len(dados) == 0 {
		return Planilha{}, errors.New("o arquivo está vazio")
	}
	if len(dados) > MaxBytes {
		return Planilha{}, fmt.Errorf("o arquivo é grande demais (máximo %d MB)", MaxBytes>>20)
	}
	var (
		p   Planilha
		err error
	)
	if bytes.HasPrefix(dados, []byte("PK")) || strings.HasSuffix(strings.ToLower(nome), ".xlsx") {
		p, err = lerXLSX(dados)
	} else {
		p, err = lerCSV(dados)
	}
	if err != nil {
		return Planilha{}, err
	}
	if len(p.Cabecalho) == 0 {
		return Planilha{}, errors.New("a planilha não tem cabeçalho (a primeira linha deve trazer os nomes das colunas)")
	}
	if len(p.Linhas) == 0 {
		return Planilha{}, errors.New("a planilha não tem linhas de dados")
	}
	if len(p.Linhas) > MaxLinhas {
		return Planilha{}, fmt.Errorf("a planilha tem linhas demais (%d): o máximo é %d por importação, divida em partes", len(p.Linhas), MaxLinhas)
	}
	return p, nil
}

func emBranco(c []string) bool {
	for _, x := range c {
		if strings.TrimSpace(x) != "" {
			return false
		}
	}
	return true
}

func limpar(c []string) []string {
	out := make([]string, len(c))
	for i, x := range c {
		out[i] = strings.TrimSpace(x)
	}
	return out
}

// textoUTF8 converte para UTF-8: se não for UTF-8 válido, trata como Latin-1 (Windows-1252), comum em planilhas antigas.
func textoUTF8(b []byte) string {
	b = bytes.TrimPrefix(b, []byte("\xEF\xBB\xBF"))
	if utf8.Valid(b) {
		return string(b)
	}
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

func detectarSeparador(primeira string) rune {
	melhor, n := ';', strings.Count(primeira, ";")
	for _, s := range []rune{',', '\t'} {
		if c := strings.Count(primeira, string(s)); c > n {
			melhor, n = s, c
		}
	}
	return melhor
}

func lerCSV(dados []byte) (Planilha, error) {
	texto := textoUTF8(dados)
	primeira, _, _ := strings.Cut(texto, "\n")
	r := csv.NewReader(strings.NewReader(texto))
	r.Comma = detectarSeparador(primeira)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var p Planilha
	for {
		reg, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Planilha{}, fmt.Errorf("não consegui ler o CSV: %v", err)
		}
		linha, _ := r.FieldPos(0)
		if p.Cabecalho == nil {
			if emBranco(reg) {
				continue
			}
			p.Cabecalho = limpar(reg)
			continue
		}
		if emBranco(reg) {
			continue
		}
		p.Linhas = append(p.Linhas, Linha{N: linha, Celulas: limpar(reg)})
		if len(p.Linhas) > MaxLinhas {
			break
		}
	}
	return p, nil
}

func lerXLSX(dados []byte) (Planilha, error) {
	f, err := excelize.OpenReader(bytes.NewReader(dados))
	if err != nil {
		return Planilha{}, errors.New("não consegui abrir o arquivo Excel (.xlsx)")
	}
	defer f.Close()
	abas := f.GetSheetList()
	if len(abas) == 0 {
		return Planilha{}, errors.New("o arquivo Excel não tem planilhas")
	}
	rows, err := f.GetRows(abas[0])
	if err != nil {
		return Planilha{}, errors.New("não consegui ler a primeira planilha do arquivo Excel")
	}
	var p Planilha
	for i, reg := range rows {
		if p.Cabecalho == nil {
			if emBranco(reg) {
				continue
			}
			p.Cabecalho = limpar(reg)
			continue
		}
		if emBranco(reg) {
			continue
		}
		p.Linhas = append(p.Linhas, Linha{N: i + 1, Celulas: limpar(reg)})
		if len(p.Linhas) > MaxLinhas {
			break
		}
	}
	return p, nil
}

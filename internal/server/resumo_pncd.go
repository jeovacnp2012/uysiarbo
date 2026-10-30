package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/epi"
)

// pncdDados é o retrato gravado no Resumo Semanal: a ficha do PNCD (campos numerados de 01 a 68),
// com os números do momento em que foi gerada. Os números entre parênteses são os campos da ficha.
type pncdDados struct {
	Versao      int    `json:"versao"`
	Municipio   string `json:"municipio"`     // 02
	IBGE        string `json:"ibge"`          // 03 (código do município, 6 dígitos)
	Bairro      string `json:"bairro"`        // 03
	Zona        string `json:"zona"`          // 04
	Ciclo       string `json:"ciclo"`         // 07
	Inicio      string `json:"inicio"`        // 08
	Fim         string `json:"fim"`           // 09
	Concluido   bool   `json:"concluido"`     // 10
	SemanaEpi   int    `json:"semana_epi"`    // 11
	Atividade   string `json:"atividade"`     // 12
	Sigla       string `json:"sigla"`         // 12
	Quarteiroes int    `json:"quarteiroes"`   // 13
	ImResid     int    `json:"im_resid"`      // 14
	ImCom       int    `json:"im_com"`        // 15
	ImTB        int    `json:"im_tb"`         // 16
	ImPE        int    `json:"im_pe"`         // 17
	ImOutro     int    `json:"im_outro"`      // 18
	ImTotal     int    `json:"im_total"`      // 19
	TratFocal   int    `json:"trat_focal"`    // 20
	TratPeri    int    `json:"trat_peri"`     // 21
	Inspec      int    `json:"inspec"`        // 22
	Amostras    int    `json:"amostras"`      // 23
	Recusada    int    `json:"recusada"`      // 24
	Fechados    int    `json:"fechados"`      // 25
	Recuperados int    `json:"recuperados"`   // 26
	Dep         [7]int `json:"dep"`           // 27 a 33: A1 A2 B C D1 D2 E
	DepTotal    int    `json:"dep_total"`     // 34
	Eliminados  int    `json:"eliminados"`    // 35
	L1Gramas    string `json:"l1_gramas"`     // 37
	L1Dep       int    `json:"l1_dep"`        // 38
	L2Gramas    string `json:"l2_gramas"`     // 40
	L2Dep       int    `json:"l2_dep"`        // 41
	AdulTipo    string `json:"adul_tipo"`     // 42
	AdulCargas  int    `json:"adul_cargas"`   // 43
	Agentes     int    `json:"agentes"`       // 44
	Dias        int    `json:"dias"`          // 45
	LabAeg      [7]int `json:"lab_aeg"`       // 46 a 52: depósitos com Aedes aegypti por tipo
	LabAegTotal int    `json:"lab_aeg_total"` // 53
	LabAlb      [7]int `json:"lab_alb"`
	LabAlbTotal int    `json:"lab_alb_total"`
	// 54 a 59: imóveis com espécies por tipo (residência, comércio, terreno baldio, PE, outros, total)
	ImAeg      [6]int `json:"im_aeg"`
	ImAlb      [6]int `json:"im_alb"`
	ImOutros   [6]int `json:"im_outros"`
	ExempAeg   int    `json:"exemp_aeg"` // 60 a 63 (larvas; pupas, exúvias e adultos não são guardados)
	ExempAlb   int    `json:"exemp_alb"`
	QuartAeg   string `json:"quart_aeg"`   // 64
	QuartAlb   string `json:"quart_alb"`   // 65
	QuartAmbos string `json:"quart_ambos"` // 66
}

const versaoPncd = 1

var tiposDeposito = [7]string{"A1", "A2", "B", "C", "D1", "D2", "E"}

// semanaDoCiclo descreve uma semana epidemiológica que toca o período do ciclo.
type semanaDoCiclo struct {
	Ano, Numero int
	Inicio, Fim time.Time
}

// semanasDoCiclo lista as semanas epidemiológicas do período do ciclo, da primeira à última.
func semanasDoCiclo(c sqlccliente.Ciclo, tipo string) []semanaDoCiclo {
	if !c.DataInicio.Valid || !c.DataFinal.Valid {
		return nil
	}
	var out []semanaDoCiclo
	vistas := map[string]bool{}
	for d := c.DataInicio.Time; !d.After(c.DataFinal.Time); d = d.AddDate(0, 0, 1) {
		s := epi.Calcular(d, tipo)
		k := fmt.Sprintf("%d-%d", s.Ano, s.Numero)
		if vistas[k] {
			continue
		}
		vistas[k] = true
		out = append(out, semanaDoCiclo{Ano: s.Ano, Numero: s.Numero, Inicio: s.Inicio, Fim: s.Fim})
	}
	return out
}

var errSemanaForaDoCiclo = errors.New("a semana não pertence ao ciclo")

var errResumoSemVisitas = errors.New("não há visitas nesta semana, ciclo, bairro e atividade")

// calcularPncd monta a ficha do PNCD de uma semana, bairro e atividade.
func (h *handlers) calcularPncd(ctx context.Context, q *sqlccliente.Queries, r *http.Request, ciclo sqlccliente.Ciclo, semana int, bairro int64, ativ sqlccliente.Atividade) (pncdDados, error) {
	var d pncdDados
	tipoSemana := h.tipoSemana(r, q)
	var sem *semanaDoCiclo
	for _, s := range semanasDoCiclo(ciclo, tipoSemana) {
		if s.Numero == semana {
			s := s
			sem = &s
		}
	}
	if sem == nil {
		return d, errSemanaForaDoCiclo
	}
	loc, err := q.PncdLocalidade(ctx, bairro)
	if err != nil {
		return d, err
	}
	sa := h.escopoVisitas(r)
	smn := pgtype.Int2{Int16: int16(semana), Valid: true}
	n, err := q.PncdResumo(ctx, sqlccliente.PncdResumoParams{CicloID: ciclo.ID, Semana: smn, BairroID: bairro, AtividadeID: ativ.ID, SomenteAgente: sa})
	if err != nil {
		return d, err
	}
	// sem nenhuma atividade de campo (nem pendência), a ficha ficaria toda zerada
	if n.ImTotal == 0 && n.Fechados == 0 && n.Recusadas == 0 && n.Dias == 0 {
		return d, errResumoSemVisitas
	}
	concl, err := q.PncdConcluido(ctx, sqlccliente.PncdConcluidoParams{BairroID: bairro, CicloID: ciclo.ID})
	if err != nil {
		return d, err
	}
	d = pncdDados{
		Versao: versaoPncd, Municipio: loc.Cidade, IBGE: ibge6(loc.Ibge), Bairro: loc.Bairro, Zona: "URBANA",
		Ciclo: ciclo.Nome, Inicio: sem.Inicio.Format(formatoData), Fim: sem.Fim.Format(formatoData), Concluido: concl.Valid && concl.Bool, SemanaEpi: sem.Numero,
		Atividade: ativ.Nome, Sigla: ativ.Sigla, Quarteiroes: int(n.Quarteiroes),
		ImResid: int(n.ImResidencia), ImCom: int(n.ImComercio), ImTB: int(n.ImTb), ImPE: int(n.ImPe), ImOutro: int(n.ImOutro), ImTotal: int(n.ImTotal),
		TratFocal: int(n.TratFocal), TratPeri: int(n.TratPerifocal), Inspec: int(n.ImTotal), Amostras: int(n.Amostras),
		Recusada: int(n.Recusadas), Fechados: int(n.Fechados), Recuperados: int(n.Recuperados),
		Dep:        [7]int{int(n.DepA1), int(n.DepA2), int(n.DepB), int(n.DepC), int(n.DepD1), int(n.DepD2), int(n.DepE)},
		Eliminados: int(n.Eliminados),
		L1Gramas:   gramas(n.L1Gramas), L1Dep: int(n.L1Tratados), L2Gramas: gramas(n.L2Gramas), L2Dep: int(n.L2Tratados),
		AdulTipo: n.AdulticidaTipo, AdulCargas: int(n.AdulticidaCargas), Agentes: int(n.Agentes), Dias: int(n.Dias),
	}
	for _, v := range d.Dep {
		d.DepTotal += v
	}
	if strings.Contains(strings.ToUpper(loc.Bairro), "RURAL") {
		d.Zona = "RURAL"
	}

	pDep := sqlccliente.PncdLabDepositosParams{CicloID: ciclo.ID, Semana: smn, BairroID: bairro, AtividadeID: ativ.ID, SomenteAgente: sa}
	deps, err := q.PncdLabDepositos(ctx, pDep)
	if err != nil {
		return d, err
	}
	for _, x := range deps {
		for i, t := range tiposDeposito {
			if strings.EqualFold(x.Deposito, t) {
				d.LabAeg[i], d.LabAlb[i] = int(x.Aegypti), int(x.Albopictus)
			}
		}
	}
	for i := range tiposDeposito {
		d.LabAegTotal += d.LabAeg[i]
		d.LabAlbTotal += d.LabAlb[i]
	}
	ims, err := q.PncdLabImoveis(ctx, sqlccliente.PncdLabImoveisParams(pDep))
	if err != nil {
		return d, err
	}
	coluna := map[string]int{"residencial": 0, "comercio": 1, "terreno_baldio": 2, "ponto_estrategico": 3, "outro": 4}
	for _, x := range ims {
		c, ok := coluna[x.Tipo]
		if !ok {
			c = 4
		}
		d.ImAeg[c] += int(x.Aegypti)
		d.ImAlb[c] += int(x.Albopictus)
		d.ExempAeg += int(x.LarvasAegypti)
		d.ExempAlb += int(x.LarvasAlbopictus)
	}
	for c := 0; c < 5; c++ {
		d.ImAeg[5] += d.ImAeg[c]
		d.ImAlb[5] += d.ImAlb[c]
	}
	qs, err := q.PncdLabQuarteiroes(ctx, sqlccliente.PncdLabQuarteiroesParams(pDep))
	if err != nil {
		return d, err
	}
	var aeg, alb, ambos []string
	for _, x := range qs {
		switch {
		case x.Aegypti && x.Albopictus:
			ambos = append(ambos, x.Quarteirao)
		case x.Aegypti:
			aeg = append(aeg, x.Quarteirao)
		default:
			alb = append(alb, x.Quarteirao)
		}
	}
	ordenarQuarteiroes(aeg)
	ordenarQuarteiroes(alb)
	ordenarQuarteiroes(ambos)
	d.QuartAeg, d.QuartAlb, d.QuartAmbos = strings.Join(aeg, ", "), strings.Join(alb, ", "), strings.Join(ambos, ", ")
	return d, nil
}

func ordenarQuarteiroes(s []string) {
	sort.SliceStable(s, func(i, j int) bool {
		a, ea := strconv.Atoi(s[i])
		b, eb := strconv.Atoi(s[j])
		if ea == nil && eb == nil {
			return a < b
		}
		return s[i] < s[j]
	})
}

func ibge6(s string) string {
	if len(s) > 6 {
		return s[:6]
	}
	return s
}

func gramas(f float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(f, 'f', 2, 64), "0"), ".")
}

func sn(b bool) string {
	if b {
		return "Sim"
	}
	return "Não"
}

// pncdPDF desenha a ficha "Resumo Semanal do Serviço Antivetorial" (PNCD) em uma página A4.
func pncdPDF(d pncdDados) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetMargins(10, 8, 10)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	const larg = 190.0
	cinza := func() { pdf.SetFillColor(217, 217, 217) }
	branco := func() { pdf.SetFillColor(255, 255, 255) }

	// caixa com texto centralizado
	caixa := func(x, y, w, h float64, texto string, negrito bool, fundo bool, tam float64) {
		if fundo {
			cinza()
		} else {
			branco()
		}
		pdf.SetXY(x, y)
		estilo := ""
		if negrito {
			estilo = "B"
		}
		pdf.SetFont("Helvetica", estilo, tam)
		if strings.Contains(texto, "\n") {
			pdf.Rect(x, y, w, h, "FD")
			linhas := strings.Split(texto, "\n")
			lh := tam * 0.36
			y0 := y + (h-lh*float64(len(linhas)))/2
			for i, l := range linhas {
				pdf.SetXY(x, y0+lh*float64(i))
				pdf.CellFormat(w, lh, tr(l), "0", 0, "C", false, 0, "")
			}
			return
		}
		pdf.CellFormat(w, h, tr(texto), "1", 0, "C", true, 0, "")
	}
	// rótulo com o número do campo à esquerda e o valor embaixo
	campo := func(x, y, w float64, num, rotulo, valor string) {
		caixa(x, y, 6, 4.2, num, false, true, 6)
		caixa(x+6, y, w-6, 4.2, rotulo, false, true, 6.5)
		caixa(x, y+4.2, w, 5, valor, false, false, 7.5)
	}
	faixa := func(y float64, texto string) {
		caixa(10, y, larg, 5.5, texto, true, true, 8.5)
	}

	// cabeçalho
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetXY(10, 10)
	pdf.CellFormat(150, 5, tr("PROGRAMA NACIONAL DE CONTROLE DA DENGUE - PNCD"), "0", 1, "C", false, 0, "")
	pdf.SetX(10)
	pdf.CellFormat(150, 5, tr("RESUMO SEMANAL DO SERVIÇO ANTIVETORIAL"), "0", 0, "C", false, 0, "")
	caixa(165, 9, 6, 4.2, "01", false, true, 6)
	caixa(171, 9, 29, 4.2, "Controle Digitação", false, true, 6.5)
	caixa(165, 13.2, 35, 6, "", false, false, 7)

	y := 24.0
	campo(10, y, 40, "02", "Município", d.Municipio)
	campo(50, y, 100, "03", "Código e nome da localidade", d.IBGE+" / "+d.Bairro)
	campo(150, y, 50, "04", "Zona", d.Zona)
	y += 10.5
	campo(10, y, 30, "05", "Categ. localid.", "BAIRRO")
	campo(40, y, 15, "06", "Tipo", "1")
	campo(55, y, 38, "07", "Ciclo/ano", d.Ciclo)
	campo(93, y, 30, "08", "Data início", d.Inicio)
	campo(123, y, 30, "09", "Data final", d.Fim)
	campo(153, y, 22, "10", "Concluído?", sn(d.Concluido))
	campo(175, y, 25, "11", "Sem. Epidem.", strconv.Itoa(d.SemanaEpi))
	y += 10.5
	campo(10, y, larg, "12", "Atividade", d.Atividade+" ("+d.Sigla+")")

	y += 12
	faixa(y, "RESUMO DO TRABALHO DE CAMPO")
	y += 6
	// 13 e imóveis por tipo
	caixa(10, y, 6, 4.2, "13", false, true, 6)
	caixa(16, y, 20, 4.2, "Total Quart.\nconcl.", false, true, 5.5)
	// bloco "Nº imóveis trabalhados por tipo"
	caixa(37, y, 83, 4.2, "Nº imóveis trabalhados por tipo", false, true, 6.5)
	caixa(121, y, 79, 4.2, "Nº imóveis", false, true, 6.5)
	y1 := y + 4.2
	tipos := []struct {
		n, r string
		v    int
	}{{"14", "Residência", d.ImResid}, {"15", "Comércio", d.ImCom}, {"16", "TB", d.ImTB}, {"17", "PE", d.ImPE}, {"18", "Outro", d.ImOutro}, {"19", "Total", d.ImTotal}}
	x := 37.0
	ws := []float64{16, 15, 11, 11, 13, 17}
	for i, t := range tipos {
		caixa(x, y1, 5, 4.2, t.n, false, true, 5.5)
		caixa(x+5, y1, ws[i]-5, 4.2, t.r, false, true, 5.5)
		caixa(x, y1+4.2, ws[i], 5, strconv.Itoa(t.v), false, false, 7.5)
		x += ws[i]
	}
	imo := []struct {
		n, r string
		v    int
	}{{"20", "Trat Focal", d.TratFocal}, {"21", "Trat perifocal", d.TratPeri}, {"22", "Inspecionados", d.Inspec}}
	x = 121
	wi := []float64{26, 26, 27}
	for i, t := range imo {
		caixa(x, y1, 5, 4.2, t.n, false, true, 5.5)
		caixa(x+5, y1, wi[i]-5, 4.2, t.r, false, true, 5.5)
		caixa(x, y1+4.2, wi[i], 5, strconv.Itoa(t.v), false, false, 7.5)
		x += wi[i]
	}
	caixa(10, y1, 26, 9.2, strconv.Itoa(d.Quarteiroes), false, false, 8)
	y += 14

	// 23 amostras e pendência
	caixa(10, y, 6, 8.4, "23", false, true, 6)
	caixa(16, y, 70, 4.2, "Amostras coletadas", false, true, 6.5)
	caixa(10, y+4.2, 76, 5, strconv.Itoa(d.Amostras), false, false, 7.5)
	caixa(87, y, 113, 4.2, "Pendência", false, true, 6.5)
	pend := []struct {
		n, r string
		v    int
	}{{"24", "Recusada", d.Recusada}, {"25", "Fechados", d.Fechados}, {"26", "Recuperados", d.Recuperados}}
	x = 87
	for _, t := range pend {
		caixa(x, y+4.2, 5, 4.2, t.n, false, true, 5.5)
		caixa(x+5, y+4.2, 32.7, 4.2, t.r, false, true, 6)
		caixa(x, y+8.4, 37.7, 5, strconv.Itoa(t.v), false, false, 7.5)
		x += 37.7
	}
	y += 14.6

	// 27 a 35 depósitos inspecionados
	caixa(10, y, 133, 4.2, "Nº depósitos inspecionados por tipo", false, true, 6.5)
	nums := []string{"27", "28", "29", "30", "31", "32", "33"}
	x = 10
	for i, t := range tiposDeposito {
		caixa(x, y+4.2, 5, 4.2, nums[i], false, true, 5.5)
		caixa(x+5, y+4.2, 13, 4.2, t, false, true, 6)
		caixa(x, y+8.4, 18, 5, strconv.Itoa(d.Dep[i]), false, false, 7.5)
		x += 18
	}
	caixa(x, y+4.2, 5, 4.2, "34", false, true, 5.5)
	caixa(x+5, y+4.2, 12, 4.2, "Total", false, true, 6)
	caixa(x, y+8.4, 17, 5, strconv.Itoa(d.DepTotal), false, false, 7.5)
	caixa(144, y, 6, 4.2, "35", false, true, 6)
	caixa(150, y, 50, 4.2, "Depósito Eliminado", false, true, 6.5)
	caixa(144, y+4.2, 56, 9.2, strconv.Itoa(d.Eliminados), false, false, 8)
	y += 15

	// tratados
	caixa(10, y, 105, 4.2, "Depósitos Tratados", false, true, 6.5)
	caixa(10, y+4.2, 52, 4.2, "Larvicida (1)", false, true, 6)
	caixa(62, y+4.2, 53, 4.2, "Larvicida (2)", false, true, 6)
	caixa(116, y, 84, 4.2, "Adulticida", false, true, 6.5)
	sub := func(x float64, n1, r1, v1 string, n2, r2, v2 string, n3, r3, v3 string) {
		w := 17.0
		for i, t := range [][3]string{{n1, r1, v1}, {n2, r2, v2}, {n3, r3, v3}} {
			xx := x + float64(i)*w
			caixa(xx, y+8.4, 5, 7, t[0], false, true, 5.5)
			caixa(xx+5, y+8.4, w-5, 7, strings.Replace(t[1], " ", "\n", 1), false, true, 5.5)
			caixa(xx, y+15.4, w, 5, t[2], false, false, 7.5)
		}
	}
	sub(10, "36", "Tipo", "L1", "37", "Qtde (Gramas)", d.L1Gramas, "38", "Qt dep.trat.", strconv.Itoa(d.L1Dep))
	sub(62, "39", "Tipo", "L2", "40", "Qtde (gramas)", d.L2Gramas, "41", "Qt dep.trat.", strconv.Itoa(d.L2Dep))
	adu := [][3]string{{"42", "Tipo", d.AdulTipo}, {"43", "Qtde\n(Cargas)", strconv.Itoa(d.AdulCargas)}}
	for i, t := range adu {
		xx := 116 + float64(i)*17
		caixa(xx, y+8.4, 5, 7, t[0], false, true, 5.5)
		caixa(xx+5, y+8.4, 12, 7, t[1], false, true, 5.5)
		caixa(xx, y+15.4, 17, 5, t[2], false, false, 7.5)
	}
	caixa(150, y+4.2, 6, 11.2, "44", false, true, 5.5)
	caixa(156, y+4.2, 20, 11.2, "Total de\nAgentes na\nsemana", false, true, 5.5)
	caixa(150, y+15.4, 26, 5, strconv.Itoa(d.Agentes), false, false, 7.5)
	caixa(177, y+4.2, 6, 11.2, "45", false, true, 5.5)
	caixa(183, y+4.2, 17, 11.2, "Total Dias\nTrabalhados\nna semana", false, true, 5.2)
	caixa(177, y+15.4, 23, 5, strconv.Itoa(d.Dias), false, false, 7.5)
	y += 24

	faixa(y, "RESUMO DO LABORATÓRIO")
	y += 6
	caixa(10, y, larg, 4.2, "Nº depósitos com espécies por tipo", false, true, 6.5)
	y += 4.2
	// cabeçalho 46 a 53
	caixa(10, y, 52, 4.2, "", false, false, 6)
	numsLab := []string{"46", "47", "48", "49", "50", "51", "52"}
	x = 62
	for i, t := range tiposDeposito {
		caixa(x, y, 5, 4.2, numsLab[i], false, true, 5.5)
		caixa(x+5, y, 10, 4.2, t, false, true, 6)
		x += 15
	}
	caixa(x, y, 5, 4.2, "53", false, true, 5.5)
	caixa(x+5, y, 18, 4.2, "Total", false, true, 6)
	y += 4.2
	linhaLab := func(rot string, v [7]int, tot int) {
		caixa(10, y, 52, 4.4, rot, false, false, 7)
		pdf.SetXY(10, y)
		xx := 62.0
		for i := range tiposDeposito {
			caixa(xx, y, 15, 4.4, strconv.Itoa(v[i]), false, false, 7)
			xx += 15
		}
		caixa(xx, y, 23, 4.4, strconv.Itoa(tot), false, false, 7)
		y += 4.4
	}
	linhaLab("Com Aedes aegypti", d.LabAeg, d.LabAegTotal)
	linhaLab("Com Aedes albopictus", d.LabAlb, d.LabAlbTotal)
	y += 2

	// 54 a 63
	caixa(10, y, 112, 4.2, "Nº de imóveis com espécies, por tipo", false, true, 6.5)
	caixa(124, y, 76, 4.2, "Nº de exemplares", false, true, 6.5)
	y += 4.2
	cab := []struct{ n, r string }{{"54", "Resi-\ndência"}, {"55", "Comér-\ncio"}, {"56", "Terreno\nBaldio"}, {"57", "Ponto\nEs-\ntratégico"}, {"58", "Outros"}, {"59", "Total"}}
	caixa(10, y, 36, 9.6, "", false, false, 6)
	x = 46
	for _, c := range cab {
		caixa(x, y, 4.5, 9.6, c.n, false, true, 5)
		caixa(x+4.5, y, 11.5, 9.6, c.r, false, true, 5)
		x += 16
	}
	cabE := []struct{ n, r string }{{"60", "Larvas"}, {"61", "Pupas"}, {"62", "Exúvia\nde pupa"}, {"63", "Adultos"}}
	x = 124
	for _, c := range cabE {
		caixa(x, y, 4.5, 9.6, c.n, false, true, 5)
		caixa(x+4.5, y, 14.5, 9.6, c.r, false, true, 5)
		x += 19
	}
	y += 9.6
	linhaImo := func(rot string, v [6]int, exemp *int) {
		caixa(10, y, 36, 5, rot, false, false, 7)
		xx := 46.0
		for _, n := range v {
			caixa(xx, y, 16, 5, strconv.Itoa(n), false, false, 7)
			xx += 16
		}
		xx = 124
		for i := 0; i < 4; i++ {
			val := "0"
			if i == 0 && exemp != nil {
				val = strconv.Itoa(*exemp)
			}
			caixa(xx, y, 19, 5, val, false, false, 7)
			xx += 19
		}
		y += 5
	}
	linhaImo("Com Aedes aegypti", d.ImAeg, &d.ExempAeg)
	linhaImo("Com Aedes albopictus", d.ImAlb, &d.ExempAlb)
	linhaImo("Outros", d.ImOutros, nil)
	y += 3

	// 64 a 66
	lista := func(x float64, n, rot, txt string) {
		caixa(x, y, 5, 8, n, false, true, 5.5)
		caixa(x+5, y, 53, 8, rot, false, true, 5.5)
		pdf.Rect(x, y+8, 58, 28, "D")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetXY(x+1, y+9)
		pdf.MultiCell(56, 4, tr(txt), "0", "L", false)
	}
	lista(10, "64", "Nº e seq. dos quarteirões com Aedes\naegypti", d.QuartAeg)
	lista(71, "65", "Nº e seq. dos quarteirões com Aedes\nalbopictus", d.QuartAlb)
	lista(132, "66", "Nº e seq. dos quarteirões com Aedes aegypti +\nAedes albopictus", d.QuartAmbos)
	y += 40
	caixa(10, y, 6, 4.4, "67", false, true, 5.5)
	caixa(16, y, 28, 4.4, "Visto do Supervisor", false, true, 6.5)
	caixa(44, y, 88, 4.4, "", false, false, 6)
	caixa(133, y, 6, 4.4, "68", false, true, 5.5)
	caixa(139, y, 28, 4.4, "Data do visto", false, true, 6.5)
	caixa(167, y, 33, 4.4, "", false, false, 6)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

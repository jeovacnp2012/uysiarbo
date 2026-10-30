package views

import "strconv"

// rotuloSelecionado devolve o rótulo da opção marcada (ou o traço, se nenhuma).
func rotuloSelecionado(itens []CidadeOpcao) string {
	for _, o := range itens {
		if o.Selecionada {
			return o.Rotulo
		}
	}
	return "-"
}

func vazioOuTraco(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func vazioOuZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

func pctDe(parte, total int) string {
	if total == 0 {
		return "0%"
	}
	return strconv.Itoa(parte*100/total) + "%"
}

func positivasTexto(n int) string {
	if n == 0 {
		return "Nenhuma positiva"
	}
	return strconv.Itoa(n) + " positiva(s)"
}

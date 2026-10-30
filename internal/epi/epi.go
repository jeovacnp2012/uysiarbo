// Package epi calcula a semana epidemiológica de uma data. O Brasil (SINAN, Ministério da Saúde) conta
// semanas de domingo a sábado, e a semana 1 é a que tem ao menos 4 dias em janeiro; o padrão ISO 8601
// conta de segunda a domingo, e a semana 1 é a que tem a primeira quinta-feira do ano.
package epi

import "time"

// Tipos de semana aceitos.
const (
	SINAN = "sinan"
	ISO   = "iso"
)

// Valido diz se t é um tipo de semana conhecido.
func Valido(t string) bool { return t == SINAN || t == ISO }

// Semana é uma semana epidemiológica: o ano a que pertence, o número (1 a 53) e os dias que a compõem.
type Semana struct {
	Ano    int
	Numero int
	Inicio time.Time
	Fim    time.Time
}

// Calcular devolve a semana epidemiológica da data (só o dia conta) no tipo pedido; tipo desconhecido vale SINAN.
func Calcular(d time.Time, tipo string) Semana {
	dia := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	if tipo == ISO {
		ano, n := dia.ISOWeek()
		ini := dia.AddDate(0, 0, -(int(dia.Weekday())+6)%7) // segunda-feira
		return Semana{Ano: ano, Numero: n, Inicio: ini, Fim: ini.AddDate(0, 0, 6)}
	}
	// SINAN: a semana vai de domingo a sábado e pertence ao ano em que cai a sua quarta-feira
	// (equivale a ter ao menos 4 dos 7 dias nesse ano).
	ini := dia.AddDate(0, 0, -int(dia.Weekday()))
	ano := ini.AddDate(0, 0, 3).Year()
	return Semana{Ano: ano, Numero: numero(ini, ano), Inicio: ini, Fim: ini.AddDate(0, 0, 6)}
}

// numero conta as semanas desde a primeira semana do ano (a que tem a quarta-feira em 1º de janeiro ou depois).
func numero(inicioDaSemana time.Time, ano int) int {
	primeira := primeiraSemanaSINAN(ano)
	return int(inicioDaSemana.Sub(primeira).Hours()/24)/7 + 1
}

// primeiraSemanaSINAN devolve o domingo que abre a semana 1 do ano: a primeira semana (domingo a sábado)
// com ao menos 4 dias em janeiro, isto é, a que contém a primeira quarta-feira do ano.
func primeiraSemanaSINAN(ano int) time.Time {
	j1 := time.Date(ano, time.January, 1, 0, 0, 0, 0, time.UTC)
	// Domingo anterior (ou igual) ao 1º de janeiro; se o 1º cai de quinta a sábado, a semana é do ano anterior.
	dom := j1.AddDate(0, 0, -int(j1.Weekday()))
	if j1.Weekday() >= time.Thursday {
		dom = dom.AddDate(0, 0, 7)
	}
	return dom
}

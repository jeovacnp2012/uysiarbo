package tenant

import (
	"sort"
	"strings"
	"time"
	_ "time/tzdata" // fusos embutidos: o servidor não precisa ter os arquivos do sistema
)

// Fuso é um fuso horário do Brasil (nome IANA) com a descrição mostrada nas telas.
type Fuso struct {
	Nome      string
	Descricao string
}

// FusosBrasil são os fusos usados no Brasil, em ordem de exibição. Dentro de um mesmo estado pode
// haver exceção (o oeste do Amazonas e do Pará, por exemplo); por isso o fuso de um cliente pode ser
// escolhido à mão, e o estado só sugere o padrão.
var FusosBrasil = []Fuso{
	{"America/Sao_Paulo", "Brasília (SP, RJ, MG, ES, GO, DF, PR, SC, RS)"},
	{"America/Bahia", "Bahia"},
	{"America/Fortaleza", "Ceará, Maranhão, Paraíba, Piauí e Rio Grande do Norte"},
	{"America/Recife", "Pernambuco"},
	{"America/Maceio", "Alagoas e Sergipe"},
	{"America/Belem", "Pará (leste) e Amapá"},
	{"America/Santarem", "Pará (oeste)"},
	{"America/Araguaina", "Tocantins"},
	{"America/Cuiaba", "Mato Grosso"},
	{"America/Campo_Grande", "Mato Grosso do Sul"},
	{"America/Manaus", "Amazonas (maior parte), horário de Manaus"},
	{"America/Porto_Velho", "Rondônia"},
	{"America/Boa_Vista", "Roraima"},
	{"America/Rio_Branco", "Acre"},
	{"America/Eirunepe", "Amazonas (extremo oeste)"},
	{"America/Noronha", "Fernando de Noronha"},
}

// FusoPadrao é o fuso de um cliente sem estado informado (horário de Brasília).
const FusoPadrao = "America/Sao_Paulo"

// FusoDoConsole é o fuso do console da plataforma (quem administra fica em Mato Grosso).
const FusoDoConsole = "America/Cuiaba"

var fusoPorUF = map[string]string{
	"AC": "America/Rio_Branco", "AL": "America/Maceio", "AP": "America/Belem", "AM": "America/Manaus",
	"BA": "America/Bahia", "CE": "America/Fortaleza", "DF": "America/Sao_Paulo", "ES": "America/Sao_Paulo",
	"GO": "America/Sao_Paulo", "MA": "America/Fortaleza", "MT": "America/Cuiaba", "MS": "America/Campo_Grande",
	"MG": "America/Sao_Paulo", "PA": "America/Belem", "PB": "America/Fortaleza", "PR": "America/Sao_Paulo",
	"PE": "America/Recife", "PI": "America/Fortaleza", "RJ": "America/Sao_Paulo", "RN": "America/Fortaleza",
	"RS": "America/Sao_Paulo", "RO": "America/Porto_Velho", "RR": "America/Boa_Vista", "SC": "America/Sao_Paulo",
	"SP": "America/Sao_Paulo", "SE": "America/Maceio", "TO": "America/Araguaina",
}

// UFValida diz se a sigla é de um estado brasileiro (ou DF).
func UFValida(uf string) bool { _, ok := fusoPorUF[uf]; return ok }

// UFs devolve as siglas em ordem alfabética.
func UFs() []string {
	out := make([]string, 0, len(fusoPorUF))
	for uf := range fusoPorUF {
		out = append(out, uf)
	}
	sort.Strings(out)
	return out
}

// FusoDaUF devolve o fuso padrão de um estado, ou FusoPadrao se a sigla não for conhecida.
func FusoDaUF(uf string) string {
	if f, ok := fusoPorUF[strings.ToUpper(strings.TrimSpace(uf))]; ok {
		return f
	}
	return FusoPadrao
}

// FusoValido diz se o nome é um dos fusos do Brasil que o sistema aceita e se o sistema o conhece.
func FusoValido(nome string) bool {
	for _, f := range FusosBrasil {
		if f.Nome == nome {
			_, err := time.LoadLocation(nome)
			return err == nil
		}
	}
	return false
}

// Local devolve o fuso pelo nome; se for inválido, devolve o do console (nunca nil).
func Local(nome string) *time.Location {
	if l, err := time.LoadLocation(nome); err == nil {
		return l
	}
	l, _ := time.LoadLocation(FusoDoConsole)
	if l == nil {
		return time.UTC
	}
	return l
}

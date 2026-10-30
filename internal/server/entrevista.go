package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// campoEntrevista é uma pergunta de uma entrevista (o mesmo formato do Geo Controle). Opcoes vem separado por
// vírgula ("Sim, Não"); CondCampo/CondOperador/CondValor escondem a pergunta até que outra tenha certa resposta;
// ExibirSeAgravo limita a pergunta a algumas doenças ("Febre Amarela, Mayaro").
type campoEntrevista struct {
	Nome           string `json:"nome"`
	Label          string `json:"label"`
	Tipo           string `json:"tipo"`
	Opcoes         string `json:"opcoes"`
	Obrigatorio    bool   `json:"obrigatorio"`
	Placeholder    string `json:"placeholder"`
	CondCampo      string `json:"cond_campo"`
	CondOperador   string `json:"cond_operador"`
	CondValor      string `json:"cond_valor"`
	ExibirSeAgravo string `json:"exibir_se_agravo"`
}

var tiposDeCampo = map[string]string{
	"texto": "Texto curto", "textarea": "Texto longo", "numero": "Número", "select": "Escolha uma", "multi_select": "Escolha várias",
}

var reNomeDeCampo = regexp.MustCompile(`^[a-z][a-z0-9_]{0,60}$`)

func (c campoEntrevista) lista() []string {
	var out []string
	for _, o := range strings.Split(c.Opcoes, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

func (c campoEntrevista) exibirPara(doenca string) bool {
	if strings.TrimSpace(c.ExibirSeAgravo) == "" {
		return true
	}
	for _, d := range strings.Split(c.ExibirSeAgravo, ",") {
		if strings.EqualFold(strings.TrimSpace(d), doenca) {
			return true
		}
	}
	return false
}

// respostas guarda texto (uma resposta) ou []string (várias).
type respostas map[string]any

func (r respostas) texto(nome string) string {
	switch v := r[nome].(type) {
	case string:
		return v
	case []any:
		var out []string
		for _, x := range v {
			out = append(out, fmt.Sprint(x))
		}
		return strings.Join(out, ", ")
	case []string:
		return strings.Join(v, ", ")
	}
	return ""
}

func (r respostas) contem(nome, valor string) bool {
	switch v := r[nome].(type) {
	case string:
		return v == valor
	case []string:
		for _, x := range v {
			if x == valor {
				return true
			}
		}
	case []any:
		for _, x := range v {
			if fmt.Sprint(x) == valor {
				return true
			}
		}
	}
	return false
}

// visivel diz se a pergunta aparece para esta doença e estas respostas (a mesma regra roda no navegador).
func (c campoEntrevista) visivel(doenca string, r respostas) bool {
	if !c.exibirPara(doenca) {
		return false
	}
	if c.CondCampo == "" {
		return true
	}
	bate := r.contem(c.CondCampo, c.CondValor)
	if c.CondOperador == "diferente" {
		return !bate
	}
	return bate
}

// lerCamposDaConfiguracao valida a lista de perguntas enviada pelo editor.
func lerCamposDaConfiguracao(texto string) ([]campoEntrevista, string) {
	var campos []campoEntrevista
	if strings.TrimSpace(texto) == "" {
		return nil, ""
	}
	if err := json.Unmarshal([]byte(texto), &campos); err != nil {
		return nil, "A lista de perguntas está inválida."
	}
	if len(campos) > 60 {
		return nil, "Uma entrevista pode ter no máximo 60 perguntas."
	}
	vistos := map[string]bool{}
	for i := range campos {
		c := &campos[i]
		c.Label, c.Nome = strings.TrimSpace(c.Label), strings.TrimSpace(c.Nome)
		if c.Label == "" {
			return nil, "Toda pergunta precisa de um texto (pergunta " + strconv.Itoa(i+1) + ")."
		}
		if c.Nome == "" {
			c.Nome = idDaPergunta(c.Label)
		}
		if !reNomeDeCampo.MatchString(c.Nome) {
			return nil, "O identificador da pergunta " + strconv.Itoa(i+1) + " é inválido (use letras minúsculas, números e _)."
		}
		if vistos[c.Nome] {
			return nil, "Há duas perguntas com o mesmo identificador: " + c.Nome + "."
		}
		vistos[c.Nome] = true
		if _, ok := tiposDeCampo[c.Tipo]; !ok {
			return nil, "Tipo de pergunta inválido na pergunta " + strconv.Itoa(i+1) + "."
		}
		if (c.Tipo == "select" || c.Tipo == "multi_select") && len(c.lista()) == 0 {
			return nil, "Informe as opções da pergunta \"" + c.Label + "\" (separadas por vírgula)."
		}
		if c.CondCampo != "" && c.CondOperador != "diferente" {
			c.CondOperador = "igual"
		}
		if c.CondCampo == c.Nome {
			return nil, "A pergunta \"" + c.Label + "\" não pode depender dela mesma."
		}
	}
	return campos, ""
}

var reNaoNome = regexp.MustCompile(`[^a-z0-9]+`)

// idDaPergunta cria o identificador a partir do texto da pergunta.
func idDaPergunta(label string) string {
	t := strings.ToLower(label)
	t = strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c").Replace(t)
	t = strings.Trim(reNaoNome.ReplaceAllString(t, "_"), "_")
	if t == "" || t[0] < 'a' || t[0] > 'z' {
		t = "p_" + t
	}
	if len(t) > 50 {
		t = strings.TrimRight(t[:50], "_")
	}
	return t
}

// lerRespostas lê o formulário da entrevista e valida só as perguntas visíveis. Devolve as respostas e a mensagem de erro.
func lerRespostas(r *http.Request, campos []campoEntrevista, doenca string) (respostas, string) {
	out := respostas{}
	// primeira passada: valores brutos de todas as perguntas, para avaliar as condições
	for _, c := range campos {
		if c.Tipo == "multi_select" {
			out[c.Nome] = r.PostForm["c_"+c.Nome]
		} else {
			out[c.Nome] = strings.TrimSpace(r.PostFormValue("c_" + c.Nome))
		}
	}
	final := respostas{}
	for _, c := range campos {
		if !c.visivel(doenca, out) {
			continue
		}
		switch c.Tipo {
		case "multi_select":
			var escolhidas []string
			for _, v := range r.PostForm["c_"+c.Nome] {
				if !contemTexto(c.lista(), v) {
					return nil, "Resposta inválida em \"" + c.Label + "\"."
				}
				escolhidas = append(escolhidas, v)
			}
			if c.Obrigatorio && len(escolhidas) == 0 {
				return nil, "Responda: " + c.Label
			}
			if len(escolhidas) > 0 {
				final[c.Nome] = escolhidas
			}
		default:
			v := strings.TrimSpace(r.PostFormValue("c_" + c.Nome))
			if v == "" {
				if c.Obrigatorio {
					return nil, "Responda: " + c.Label
				}
				continue
			}
			if len([]rune(v)) > 2000 {
				return nil, "A resposta de \"" + c.Label + "\" passa de 2000 caracteres."
			}
			switch c.Tipo {
			case "select":
				if !contemTexto(c.lista(), v) {
					return nil, "Resposta inválida em \"" + c.Label + "\"."
				}
			case "numero":
				if _, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err != nil {
					return nil, "\"" + c.Label + "\" precisa ser um número."
				}
			}
			final[c.Nome] = v
		}
	}
	return final, ""
}

func contemTexto(lista []string, v string) bool {
	for _, x := range lista {
		if x == v {
			return true
		}
	}
	return false
}

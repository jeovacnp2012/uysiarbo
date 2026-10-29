package importacao

import "strings"

// Conector descreve de onde vêm os dados de uma importação. Todos acabam entregando uma Planilha (cabeçalho e
// linhas) à mesma análise, prévia e execução; o que muda é como o dado chega e quais nomes de coluna já são conhecidos.
type Conector struct {
	Codigo string
	Rotulo string
	Descr  string
	Fonte  string // "arquivo" (a pessoa envia) ou "api" (o servidor busca num endereço)
	// Candidatos diz, por tipo, que cabeçalhos da origem correspondem a cada coluna do sistema (coluna -> cabeçalhos).
	Candidatos map[string]map[string][]string
}

// Conectores são as origens que o sistema sabe ler. Uma origem nova entra aqui: um registro com seus nomes de coluna
// conhecidos e, se buscar por rede, a Fonte "api" (a busca em si fica em rest.go).
var Conectores = []Conector{
	{
		Codigo: "planilha", Rotulo: "Planilha (Excel ou CSV)", Fonte: "arquivo",
		Descr: "Uma planilha preenchida no modelo do sistema, ou qualquer planilha: na tela seguinte você liga as colunas.",
	},
	{
		Codigo: "geocontrole", Rotulo: "Geo Controle (planilha exportada)", Fonte: "arquivo", Candidatos: candidatosGeoControle,
		Descr: "Planilha exportada do Geo Controle: os nomes de coluna dele já são reconhecidos (locais, bairros e quarteirões).",
	},
	{
		Codigo: "geocontrole_api", Rotulo: "Geo Controle (API)", Fonte: "api", Candidatos: candidatosGeoControle,
		Descr: "Busca os dados no endereço da API do Geo Controle, com os nomes de campo dele já reconhecidos. As credenciais valem só para esta importação e não ficam guardadas.",
	},
	{
		Codigo: "api_rest", Rotulo: "Outra API (JSON)", Fonte: "api",
		Descr: "Busca uma lista em JSON num endereço HTTPS de outro sistema, com token. Você liga os campos às colunas do sistema.",
	},
}

// ConectorPorCodigo acha um conector pelo código.
func ConectorPorCodigo(codigo string) (Conector, bool) {
	for _, c := range Conectores {
		if c.Codigo == codigo {
			return c, true
		}
	}
	return Conector{}, false
}

// candidatosGeoControle são os nomes de coluna do Geo Controle (levantados do portal dele; os que não foram vistos
// de perto estão marcados como prováveis no modelo de dados de docs/). O que não bater, a tela de mapeamento resolve.
var candidatosGeoControle = map[string]map[string][]string{
	"bairros": {
		"cidade": {"cidade", "nome_cidade", "municipio"}, "uf": {"uf", "estado"}, "nome": {"nome", "bairro", "nome_bairro"},
		"categoria": {"categoria"}, "classificacao": {"classificacao"}, "status": {"status", "situacao"}, "ibge": {"ibge", "codigo_ibge"},
		"zona": {"zona"}, "mutirao": {"mutirao"}, "producao": {"entra_na_producao", "producao"}, "latitude": {"latitude", "lat"},
		"longitude": {"longitude", "long", "lng"}, "altitude": {"altitude"}, "distancia_centro": {"distancia_do_centro", "distancia_centro"},
		"id_externo": {"id", "id_bairro"},
	},
	"quarteiroes": {
		"cidade": {"cidade", "nome_cidade", "municipio"}, "uf": {"uf", "estado"}, "bairro": {"bairro", "nome_bairro"},
		"nome": {"nome", "quarteirao", "numero", "numero_quarteirao"}, "ativo": {"ativo", "cad_situacao", "situacao"}, "id_externo": {"id", "id_quarteirao"},
	},
	"locais": {
		"cidade": {"cidade", "nome_cidade", "municipio"}, "uf": {"uf", "estado"}, "bairro": {"bairro", "nome_bairro"},
		"quarteirao": {"quarteirao", "nome_quarteirao", "quadra"}, "tipo": {"tipo", "tipo_do_local"},
		"endereco": {"endereco", "logradouro", "rua"}, "numero": {"numero", "num", "n"}, "lado": {"lado"}, "cep": {"cep"},
		"referencia": {"referencia", "ponto_de_referencia"}, "sequencia": {"sequencia"}, "complemento": {"complemento"},
		"latitude": {"latitude", "lat"}, "longitude": {"longitude", "long", "lng"},
		"proprietario": {"proprietario"}, "proprietario_telefone": {"proprietario_telefone", "telefone"},
		"proprietario_whatsapp": {"proprietario_whatsapp", "whatsapp"}, "proprietario_email": {"proprietario_email", "email"},
		"aceita_whatsapp": {"proprietario_whatsapp_aceita", "aceita_whatsapp"}, "aceita_email": {"proprietario_email_aceita", "aceita_email"},
		"inquilino": {"inquilino"}, "inquilino_telefone": {"inquilino_telefone"},
		"humanos": {"humanos", "pessoas", "habitantes"}, "cachorros": {"cachorros", "caes"}, "gatos": {"gatos"}, "roedores": {"roedores"},
		"ativo": {"cad_situacao", "situacao", "ativo"}, "id_externo": {"id", "id_local"},
	},
}

// MapearComo liga as colunas do tipo às do cabeçalho, nesta ordem de prioridade: o mapa explícito (coluna do sistema
// -> cabeçalho da origem, vazio = não importar), os cabeçalhos candidatos do conector e os nomes e apelidos do próprio tipo.
func (t Tipo) MapearComo(cabecalho []string, explicito map[string]string, candidatos map[string][]string) Mapeamento {
	m := Mapeamento{Posicao: map[string]int{}}
	porNome := map[string]int{}
	for i, h := range cabecalho {
		if strings.TrimSpace(h) == "" {
			continue
		}
		if _, ja := porNome[Normalizar(h)]; !ja {
			porNome[Normalizar(h)] = i
		}
	}
	usadas := map[int]bool{}
	pegar := func(coluna string, nomes ...string) bool {
		for _, n := range nomes {
			if i, ok := porNome[Normalizar(n)]; ok && !usadas[i] {
				m.Posicao[coluna], usadas[i] = i, true
				return true
			}
		}
		return false
	}
	for _, col := range t.Colunas {
		if h, ok := explicito[col.Nome]; ok {
			if h != "" {
				pegar(col.Nome, h)
			}
			continue // escolha explícita (inclusive "não importar") não cai nos palpites
		}
		if pegar(col.Nome, candidatos[col.Nome]...) {
			continue
		}
		pegar(col.Nome, append([]string{col.Nome}, col.Apelidos...)...)
	}
	for i, h := range cabecalho {
		if strings.TrimSpace(h) != "" && !usadas[i] {
			m.Ignoradas = append(m.Ignoradas, h)
		}
	}
	for _, col := range t.Colunas {
		if _, ok := m.Posicao[col.Nome]; col.Obrigatoria && !ok {
			m.Faltando = append(m.Faltando, col.Nome)
		}
	}
	return m
}

// MapaDe devolve, a partir de um Mapeamento, "coluna do sistema -> cabeçalho original" (para salvar ou mostrar).
func (m Mapeamento) MapaDe(cabecalho []string) map[string]string {
	out := map[string]string{}
	for col, i := range m.Posicao {
		if i < len(cabecalho) {
			out[col] = cabecalho[i]
		}
	}
	return out
}

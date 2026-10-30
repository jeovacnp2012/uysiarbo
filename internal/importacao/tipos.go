package importacao

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Coluna descreve uma coluna da planilha de um tipo de dado.
type Coluna struct {
	Nome        string // nome do cabeçalho no modelo, sem acento e em minúsculas
	Rotulo      string // como aparece nas telas
	Obrigatoria bool
	Exemplo     string
	Ajuda       string
	Apelidos    []string // outros nomes aceitos no cabeçalho
}

// Tipo é um tipo de dado importável.
type Tipo struct {
	Codigo  string
	Rotulo  string
	Descr   string
	Colunas []Coluna
}

func c(nome, rotulo string, obrig bool, exemplo, ajuda string, apelidos ...string) Coluna {
	return Coluna{Nome: nome, Rotulo: rotulo, Obrigatoria: obrig, Exemplo: exemplo, Ajuda: ajuda, Apelidos: apelidos}
}

// Tipos são os dados que dá para importar hoje. Os demais entram quando as etapas deles existirem.
var Tipos = []Tipo{
	{
		Codigo: "bairros", Rotulo: "Bairros", Descr: "Bairros e povoados de uma cidade já cadastrada.",
		Colunas: []Coluna{
			c("cidade", "Cidade", true, "Nova Mutum", "Nome da cidade, já cadastrada em Cidades."),
			c("uf", "UF", false, "MT", "Estado; necessário só se houver cidades com o mesmo nome."),
			c("nome", "Nome do bairro", true, "Jardim das Flores", "", "bairro"),
			c("categoria", "Categoria", false, "bairro", "bairro ou povoado (padrão: bairro)."),
			c("classificacao", "Classificação", false, "urbano", "urbano ou rural (padrão: urbano)."),
			c("status", "Situação", false, "ativo", "ativo ou extinto (padrão: ativo).", "situacao"),
			c("ibge", "Código IBGE", false, "", ""),
			c("zona", "Zona", false, "Norte", ""),
			c("mutirao", "Mutirão", false, "nao", "sim ou não (padrão: não)."),
			c("producao", "Entra na produção", false, "sim", "sim ou não (padrão: sim).", "entra_na_producao"),
			c("latitude", "Latitude", false, "-13.8247", "Latitude e longitude juntas."),
			c("longitude", "Longitude", false, "-56.0813", ""),
			c("altitude", "Altitude (m)", false, "", ""),
			c("distancia_centro", "Distância do centro (km)", false, "", ""),
			c("id_externo", "ID de origem", false, "", "Identificador do sistema de origem (opcional, único)."),
		},
	},
	{
		Codigo: "quarteiroes", Rotulo: "Quarteirões", Descr: "Quarteirões de bairros já cadastrados.",
		Colunas: []Coluna{
			c("cidade", "Cidade", false, "Nova Mutum", "Necessária só se houver bairros com o mesmo nome em cidades diferentes."),
			c("uf", "UF", false, "MT", ""),
			c("bairro", "Bairro", true, "Jardim das Flores", "Nome do bairro, já cadastrado."),
			c("nome", "Quarteirão", true, "12", "Número ou nome do quarteirão (até 30 caracteres).", "quarteirao", "quadra"),
			c("ativo", "Ativo", false, "sim", "sim ou não (padrão: sim)."),
			c("id_externo", "ID de origem", false, "", ""),
		},
	},
	{
		Codigo: "locais", Rotulo: "Locais", Descr: "Imóveis e pontos, com o bairro e o quarteirão já cadastrados. O CPF não é importado.",
		Colunas: []Coluna{
			c("cidade", "Cidade", false, "Nova Mutum", "Necessária só se houver bairros com o mesmo nome em cidades diferentes."),
			c("uf", "UF", false, "MT", ""),
			c("bairro", "Bairro", true, "Jardim das Flores", ""),
			c("quarteirao", "Quarteirão", true, "12", "", "quadra"),
			c("tipo", "Tipo", true, "residencial", "residencial, comércio, terreno baldio, ponto estratégico ou outro."),
			c("endereco", "Rua ou avenida", true, "Rua das Palmeiras", "", "rua", "logradouro"),
			c("numero", "Número", true, "120", "Use S/N quando não houver."),
			c("lado", "Lado", true, "Par", "Até 5 caracteres."),
			c("cep", "CEP", false, "78450-000", ""),
			c("referencia", "Referência", false, "", ""),
			c("sequencia", "Sequência", false, "", ""),
			c("complemento", "Complemento", false, "", "Número do complemento."),
			c("latitude", "Latitude", false, "-13.8247", "Latitude e longitude juntas."),
			c("longitude", "Longitude", false, "-56.0813", ""),
			c("proprietario", "Proprietário", false, "", ""),
			c("proprietario_telefone", "Telefone do proprietário", false, "", ""),
			c("proprietario_whatsapp", "WhatsApp do proprietário", false, "", ""),
			c("proprietario_email", "E-mail do proprietário", false, "", ""),
			c("aceita_whatsapp", "Aceita WhatsApp", false, "", "sim ou não."),
			c("aceita_email", "Aceita e-mail", false, "", "sim ou não."),
			c("inquilino", "Inquilino", false, "", ""),
			c("inquilino_telefone", "Telefone do inquilino", false, "", ""),
			c("humanos", "Pessoas", false, "4", "", "pessoas", "habitantes"),
			c("cachorros", "Cachorros", false, "", ""),
			c("gatos", "Gatos", false, "", ""),
			c("roedores", "Roedores", false, "", ""),
			c("ativo", "Ativo", false, "sim", "sim ou não (padrão: sim)."),
			c("id_externo", "ID de origem", false, "", "Se informado, é usado para achar o local depois."),
		},
	},
	{
		Codigo: "ciclos", Rotulo: "Ciclos", Descr: "Ciclos de trabalho, com período, situação e atividades.",
		Colunas: []Coluna{
			c("nome", "Nome do ciclo", true, "01 Ciclo - 2026", "Único; é por ele que um ciclo já existente é reconhecido.", "ciclo"),
			c("data_inicio", "Data de início", true, "04/01/2026", "dd/mm/aaaa ou aaaa-mm-dd.", "inicio", "data_de_inicio"),
			c("data_final", "Data final", true, "28/02/2026", "dd/mm/aaaa ou aaaa-mm-dd.", "final", "fim", "data_fim", "data_de_fim"),
			c("situacao", "Situação", false, "operando", "operando ou fechado (padrão: operando).", "status"),
			c("atividades", "Atividades", false, "LIT, LI, PE", "Siglas separadas por vírgula ou ponto e vírgula: LIT, LI, PE, T, DF, PVE, LIRAa, LIA, BRI."),
			c("permite_fora_periodo", "Permite fora do período", false, "nao", "sim ou não (padrão: não)."),
			c("painel_publico", "Painel público", false, "nao", "sim ou não (padrão: não)."),
			c("painel_atividades", "Atividades do painel", false, "LIT", "Siglas; se vazio e o painel for público, vale o conjunto de atividades do ciclo.", "atividades_do_painel"),
			c("painel_texto", "Nome no painel público", false, "", ""),
		},
	},
	{
		Codigo: "visitas", Rotulo: "Visitas", Descr: "Visitas já feitas, com o local, o ciclo e o agente cadastrados. Só cria: visita que já existe (mesmo ID de origem) é ignorada. Aceita ciclos fechados e agentes desativados.",
		Colunas: []Coluna{
			c("id_externo", "ID de origem", true, "12345", "Identificador da visita no sistema de origem (até 64 caracteres). É o que impede duplicar numa nova importação.", "id", "id_visita", "id_da_visita"),
			c("local", "Local (ID de origem)", true, "678", "O ID de origem do local, já cadastrado (coluna id_externo dos Locais).", "local_id_externo", "id_local", "local_id"),
			c("ciclo", "Ciclo", true, "01 Ciclo - 2026", "Nome do ciclo, já cadastrado."),
			c("atividade", "Atividade", true, "LIT", "Sigla: LIT, LI, PE, T, DF, PVE, LIRAa, LIA ou BRI."),
			c("agente", "Agente", true, "Maria da Silva", "Nome ou e-mail de um usuário já cadastrado.", "usuario"),
			c("data", "Data", true, "28/10/2026", "dd/mm/aaaa ou aaaa-mm-dd."),
			c("hora", "Hora de entrada", false, "08:30", "hh:mm.", "hora_entrada"),
			c("tipo", "Tipo", false, "normal", "normal ou recuperação (padrão: normal).", "tipo_visita"),
			c("pendencia", "Pendência", false, "nenhuma", "nenhuma, fechado, recusado ou tentativa (padrão: nenhuma)."),
			c("achou_deposito", "Achou depósito", false, "sim", "sim ou não (padrão: sim se algum depósito for maior que zero)."),
			c("a1", "Depósitos A1", false, "2", ""),
			c("a2", "Depósitos A2", false, "", ""),
			c("b", "Depósitos B", false, "", ""),
			c("c", "Depósitos C", false, "", ""),
			c("d1", "Depósitos D1", false, "", ""),
			c("d2", "Depósitos D2", false, "", ""),
			c("e", "Depósitos E", false, "", ""),
			c("total_eliminado", "Total eliminado", false, "", ""),
			c("coletou_amostra", "Coletou amostra", false, "nao", "sim ou não."),
			c("fez_tratamento", "Fez tratamento", false, "nao", "sim ou não."),
			c("l1_gramas", "Larvicida L1 (g)", false, "", ""),
			c("l1_tratados", "Larvicida L1 (depósitos)", false, "", ""),
			c("l2_gramas", "Larvicida L2 (g)", false, "", ""),
			c("l2_tratados", "Larvicida L2 (depósitos)", false, "", ""),
			c("adulticida_tipo", "Adulticida (tipo)", false, "", ""),
			c("adulticida_cargas", "Adulticida (cargas)", false, "", ""),
			c("tratamento_eliminados", "Eliminados no tratamento", false, "", ""),
			c("bri_volume", "BRI (volume)", false, "", ""),
			c("latitude", "Latitude", false, "", "Latitude e longitude juntas."),
			c("longitude", "Longitude", false, "", ""),
			c("anotacao", "Anotação", false, "", "Até 4000 caracteres.", "observacao"),
		},
	},
	{
		Codigo: "investigacoes", Rotulo: "Investigações", Descr: "Casos suspeitos e confirmados, com a doença já cadastrada. Só cria: investigação que já existe (mesmo ID de origem) é ignorada.",
		Colunas: []Coluna{
			c("id_externo", "ID de origem", false, "13495", "Identificador no sistema de origem (até 64 caracteres). É o que impede duplicar numa nova importação.", "id", "id_investigacao", "id_de_origem"),
			c("doenca", "Doença / agravo", true, "Dengue", "Nome de uma doença já cadastrada em Doenças e agravos.", "agravo", "doenca_agravo"),
			c("paciente", "Paciente", true, "Maria Souza", "Até 150 caracteres.", "nome", "nome_paciente"),
			c("telefone", "Telefone", false, "(65) 99999-0000", "Até 40 caracteres."),
			c("data_inicio", "Data de início", false, "28/10/2026", "dd/mm/aaaa ou aaaa-mm-dd.", "data", "data_de_inicio"),
			c("endereco", "Endereço", false, "Rua das Acácias, 120", "Texto livre, até 300 caracteres.", "endereco_texto"),
			c("local", "Local (ID de origem)", false, "678", "O ID de origem do local, já cadastrado (opcional).", "local_id_externo", "id_local"),
			c("situacao", "Situação", false, "encerrada", "aguardando, em atendimento ou encerrada (padrão: aguardando)."),
			c("resultado", "Resultado", false, "negativo", "aguardando, positivo, negativo ou inconclusivo (padrão: aguardando)."),
			c("responsavel", "Responsável", false, "Maria da Silva", "Nome ou e-mail de um usuário já cadastrado (opcional)."),
			c("observacao", "Observação", false, "", "Até 2000 caracteres."),
		},
	},
	{
		Codigo: "amostras", Rotulo: "Laboratório (tubitos)", Descr: "Tubitos das visitas já importadas, com o resultado da análise. Só cria: tubito que já existe é ignorado.",
		Colunas: []Coluna{
			c("visita", "Visita (ID de origem)", true, "12345", "O ID de origem da visita, já cadastrada com \"coletou amostra\".", "id_visita", "visita_id_externo", "id_da_visita"),
			c("tubito", "Número do tubito", false, "1045", "Se vazio, usa a numeração automática (só se o município numera automaticamente).", "numero_tubito", "tubete"),
			c("deposito", "Depósito", false, "A1", "A1, A2, B, C, D1, D2 ou E."),
			c("larvas", "Larvas", false, "4", "0 a 9999."),
			c("pupas", "Pupas", false, "1", "0 a 9999."),
			c("observacao", "Observação", false, "", "Até 1000 caracteres."),
			c("status", "Status", false, "concluido", "em análise ou concluído (padrão: concluído se houver algum total, senão em análise)."),
			c("aegypti", "Aedes aegypti", false, "", "Total, 0 a 9999.", "total_aegypti"),
			c("albopictus", "Aedes albopictus", false, "", "Total, 0 a 9999.", "total_albopictus"),
			c("negativo", "Negativo", false, "", "Total, 0 a 9999.", "total_negativo"),
			c("id_externo", "ID de origem do tubito", false, "", "Opcional, único."),
		},
	},
}

// TipoPorCodigo acha um tipo pelo código.
func TipoPorCodigo(codigo string) (Tipo, bool) {
	for _, t := range Tipos {
		if t.Codigo == codigo {
			return t, true
		}
	}
	return Tipo{}, false
}

var semAcento = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e", "í", "i", "ì", "i", "î", "i", "ï", "i",
	"º", "", "ª", "", "°", "", "ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "ù", "u", "û", "u", "ü", "u", "ç", "c", "ñ", "n",
)

// Normalizar põe em minúsculas, tira acentos e junta palavras com "_": "Número do Local" vira "numero_do_local".
func Normalizar(s string) string {
	s = semAcento.Replace(strings.ToLower(strings.TrimSpace(s)))
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '-' || r == '.' || r == '/' }), "_")
}

// Chave compara textos ignorando caixa, acentos e espaços repetidos (para achar bairro, cidade e quarteirão por nome).
func Chave(s string) string {
	return strings.Join(strings.Fields(semAcento.Replace(strings.ToLower(s))), " ")
}

// Mapeamento liga as colunas do tipo às posições da planilha.
type Mapeamento struct {
	Posicao   map[string]int // nome da coluna -> posição na linha
	Ignoradas []string       // cabeçalhos que não conhecemos (não atrapalham, mas avisamos)
	Faltando  []string       // colunas obrigatórias ausentes
}

// Mapear confere o cabeçalho da planilha com as colunas do tipo.
func (t Tipo) Mapear(cabecalho []string) Mapeamento {
	m := Mapeamento{Posicao: map[string]int{}}
	nomes := map[string]string{}
	for _, col := range t.Colunas {
		nomes[col.Nome] = col.Nome
		for _, a := range col.Apelidos {
			nomes[Normalizar(a)] = col.Nome
		}
	}
	for i, h := range cabecalho {
		if strings.TrimSpace(h) == "" {
			continue
		}
		if nome, ok := nomes[Normalizar(h)]; ok {
			if _, dup := m.Posicao[nome]; !dup {
				m.Posicao[nome] = i
				continue
			}
		}
		m.Ignoradas = append(m.Ignoradas, h)
	}
	for _, col := range t.Colunas {
		if _, ok := m.Posicao[col.Nome]; col.Obrigatoria && !ok {
			m.Faltando = append(m.Faltando, col.Nome)
		}
	}
	return m
}

// Valor devolve o texto da coluna na linha ("" se a coluna não existe ou a célula é mais curta).
func (m Mapeamento) Valor(linha []string, nome string) string {
	i, ok := m.Posicao[nome]
	if !ok || i >= len(linha) {
		return ""
	}
	return strings.TrimSpace(linha[i])
}

// Tem diz se a coluna existe na planilha (mesmo com a célula vazia).
func (m Mapeamento) Tem(nome string) bool { _, ok := m.Posicao[nome]; return ok }

// Data lê uma data em dd/mm/aaaa, dd/mm/aa, aaaa-mm-dd, dd-mm-aaaa ou como número de série do Excel.
func Data(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, f := range []string{"02/01/2006", "2006-01-02", "02-01-2006", "02/01/06", "02-01-06"} {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	if n, err := strconv.Atoi(s); err == nil && n > 20000 && n < 80000 { // serial do Excel (dias desde 30/12/1899)
		return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, n), true
	}
	return time.Time{}, false
}

// Booleano lê sim/não, s/n, true/false, 1/0, verdadeiro/falso.
func Booleano(s string) (valor, ok bool) {
	switch Normalizar(s) {
	case "sim", "s", "true", "verdadeiro", "1", "x", "yes", "y", "ativo":
		return true, true
	case "nao", "n", "false", "falso", "0", "no", "inativo", "desativado":
		return false, true
	}
	return false, false
}

// ModeloCSV é o arquivo de exemplo para baixar: cabeçalho e uma linha de exemplo (separador ;, com BOM para o Excel).
func (t Tipo) ModeloCSV() []byte {
	var b bytes.Buffer
	b.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&b)
	w.Comma = ';'
	cab := make([]string, len(t.Colunas))
	ex := make([]string, len(t.Colunas))
	for i, col := range t.Colunas {
		cab[i], ex[i] = col.Nome, col.Exemplo
	}
	_ = w.Write(cab)
	_ = w.Write(ex)
	w.Flush()
	return b.Bytes()
}

// ModeloXLSX é o mesmo modelo em Excel, com uma segunda aba "Ajuda" explicando cada coluna.
func (t Tipo) ModeloXLSX() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	aba := t.Rotulo
	if err := f.SetSheetName("Sheet1", aba); err != nil {
		return nil, err
	}
	for i, col := range t.Colunas {
		ref, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(aba, ref, col.Nome)
		ref2, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue(aba, ref2, col.Exemplo)
	}
	negrito, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	ultima, _ := excelize.CoordinatesToCellName(len(t.Colunas), 1)
	_ = f.SetCellStyle(aba, "A1", ultima, negrito)
	ajuda := "Ajuda"
	if _, err := f.NewSheet(ajuda); err != nil {
		return nil, err
	}
	for i, cab := range []string{"Coluna", "Obrigatória", "Descrição"} {
		ref, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(ajuda, ref, cab)
	}
	_ = f.SetCellStyle(ajuda, "A1", "C1", negrito)
	for i, col := range t.Colunas {
		obrig := "não"
		if col.Obrigatoria {
			obrig = "sim"
		}
		l := i + 2
		ra, _ := excelize.CoordinatesToCellName(1, l)
		rb, _ := excelize.CoordinatesToCellName(2, l)
		rc, _ := excelize.CoordinatesToCellName(3, l)
		_ = f.SetCellValue(ajuda, ra, col.Nome)
		_ = f.SetCellValue(ajuda, rb, obrig)
		_ = f.SetCellValue(ajuda, rc, strings.TrimSpace(col.Rotulo+". "+col.Ajuda))
	}
	_ = f.SetColWidth(ajuda, "A", "A", 24)
	_ = f.SetColWidth(ajuda, "C", "C", 80)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

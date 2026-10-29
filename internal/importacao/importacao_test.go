package importacao

import (
	"strings"
	"testing"
)

func TestLerCSVComSeparadoresEAcentos(t *testing.T) {
	casos := map[string]string{
		"ponto e vírgula": "\xEF\xBB\xBFnome;uf\nCentro;MT\n\nJardim;MT\n",
		"vírgula":         "nome,uf\nCentro,MT\nJardim,MT\n",
		"tabulação":       "nome\tuf\nCentro\tMT\nJardim\tMT\n",
		"aspas":           "nome;uf\n\"Centro\";MT\n\"Jardim\";\"MT\"\n",
		"CRLF":            "nome;uf\r\nCentro;MT\r\nJardim;MT\r\n",
	}
	for nome, texto := range casos {
		p, err := Ler("a.csv", []byte(texto))
		if err != nil || len(p.Linhas) != 2 || p.Cabecalho[0] != "nome" || p.Linhas[0].Celulas[0] != "Centro" || p.Linhas[1].Celulas[1] != "MT" {
			t.Errorf("%s: %+v %v", nome, p, err)
		}
	}
	// A linha em branco não muda a numeração do que vem depois: Jardim está na linha 4 do arquivo.
	p, _ := Ler("a.csv", []byte(casos["ponto e vírgula"]))
	if p.Linhas[0].N != 2 || p.Linhas[1].N != 4 {
		t.Errorf("numeração das linhas: %d e %d", p.Linhas[0].N, p.Linhas[1].N)
	}
	// Latin-1 (planilhas antigas): "São José" em ISO-8859-1.
	latin := append([]byte("nome;uf\nS"), 0xE3, 'o', ' ', 'J', 'o', 's', 0xE9, ';', 'M', 'T', '\n')
	p, err := Ler("a.csv", latin)
	if err != nil || p.Linhas[0].Celulas[0] != "São José" {
		t.Errorf("latin-1: %+v %v", p, err)
	}
	// Espaços nas pontas saem.
	p, _ = Ler("a.csv", []byte("nome;uf\n  Centro  ; MT \n"))
	if p.Linhas[0].Celulas[0] != "Centro" || p.Linhas[0].Celulas[1] != "MT" {
		t.Errorf("trim: %+v", p.Linhas[0])
	}
}

func TestLerRecusaArquivosRuins(t *testing.T) {
	for nome, dados := range map[string]string{
		"vazio":            "",
		"só cabeçalho":     "nome;uf\n",
		"só linhas vazias": "nome;uf\n;\n;\n",
		"lixo xlsx":        "PKnão é um zip de verdade",
	} {
		if _, err := Ler("a.csv", []byte(dados)); err == nil {
			t.Errorf("%s: deveria dar erro", nome)
		}
	}
	grande := strings.Repeat("x", MaxBytes+1)
	if _, err := Ler("a.csv", []byte(grande)); err == nil {
		t.Error("arquivo grande demais deveria dar erro")
	}
	var b strings.Builder
	b.WriteString("nome\n")
	for i := 0; i <= MaxLinhas; i++ {
		b.WriteString("x\n")
	}
	if _, err := Ler("a.csv", []byte(b.String())); err == nil || !strings.Contains(err.Error(), "linhas demais") {
		t.Errorf("linhas demais: %v", err)
	}
}

func TestMapearCabecalho(t *testing.T) {
	bairros, _ := TipoPorCodigo("bairros")
	m := bairros.Mapear([]string{"Cidade", "BAIRRO", "Situação", "Observação", "latitude"})
	if m.Posicao["cidade"] != 0 || m.Posicao["nome"] != 1 || m.Posicao["status"] != 2 || m.Posicao["latitude"] != 4 {
		t.Errorf("posições: %v", m.Posicao)
	}
	if len(m.Ignoradas) != 1 || m.Ignoradas[0] != "Observação" || len(m.Faltando) != 0 {
		t.Errorf("ignoradas=%v faltando=%v", m.Ignoradas, m.Faltando)
	}
	linha := []string{"Nova Mutum", "Centro", "ativo", "x", "-13.8"}
	if m.Valor(linha, "nome") != "Centro" || m.Valor(linha, "ibge") != "" || m.Valor([]string{"a"}, "latitude") != "" || !m.Tem("cidade") || m.Tem("ibge") {
		t.Error("Valor/Tem")
	}
	locais, _ := TipoPorCodigo("locais")
	m = locais.Mapear([]string{"Rua", "Número", "Quadra", "Bairro"})
	if m.Posicao["endereco"] != 0 || m.Posicao["numero"] != 1 || m.Posicao["quarteirao"] != 2 {
		t.Errorf("apelidos: %v", m.Posicao)
	}
	if strings.Join(m.Faltando, ",") != "tipo,lado" {
		t.Errorf("faltando = %v", m.Faltando)
	}
	// Coluna repetida: a segunda vira ignorada.
	m = bairros.Mapear([]string{"cidade", "nome", "bairro"})
	if m.Posicao["nome"] != 1 || len(m.Ignoradas) != 1 {
		t.Errorf("repetida: %v %v", m.Posicao, m.Ignoradas)
	}
}

func TestNormalizarEChave(t *testing.T) {
	for entrada, esperado := range map[string]string{"Número do Local": "numero_do_local", " Latitude ": "latitude", "Entra na produção": "entra_na_producao", "id-externo": "id_externo"} {
		if Normalizar(entrada) != esperado {
			t.Errorf("Normalizar(%q) = %q", entrada, Normalizar(entrada))
		}
	}
	if Chave("  Jardim  São JOSÉ ") != "jardim sao jose" {
		t.Errorf("Chave = %q", Chave("  Jardim  São JOSÉ "))
	}
	for s, v := range map[string]bool{"sim": true, "Sim": true, "S": true, "1": true, "true": true, "não": false, "Nao": false, "n": false, "0": false, "falso": false} {
		got, ok := Booleano(s)
		if !ok || got != v {
			t.Errorf("Booleano(%q) = %v, %v", s, got, ok)
		}
	}
	if _, ok := Booleano("talvez"); ok {
		t.Error("talvez não é booleano")
	}
}

func TestModelosSaoLidosDeVolta(t *testing.T) {
	for _, tipo := range Tipos {
		csvBytes := tipo.ModeloCSV()
		p, err := Ler("modelo.csv", csvBytes)
		if err != nil || len(p.Linhas) != 1 {
			t.Fatalf("%s CSV: %v %+v", tipo.Codigo, err, p)
		}
		if m := tipo.Mapear(p.Cabecalho); len(m.Faltando) != 0 || len(m.Ignoradas) != 0 || len(m.Posicao) != len(tipo.Colunas) {
			t.Errorf("%s CSV: mapeamento %+v", tipo.Codigo, m)
		}
		xlsx, err := tipo.ModeloXLSX()
		if err != nil {
			t.Fatalf("%s XLSX: %v", tipo.Codigo, err)
		}
		p2, err := Ler("modelo.xlsx", xlsx)
		if err != nil || len(p2.Linhas) != 1 || p2.Linhas[0].N != 2 {
			t.Fatalf("%s XLSX lido de volta: %v %+v", tipo.Codigo, err, p2)
		}
		if m := tipo.Mapear(p2.Cabecalho); len(m.Faltando) != 0 || len(m.Posicao) != len(tipo.Colunas) {
			t.Errorf("%s XLSX: mapeamento %+v", tipo.Codigo, m)
		}
		// o exemplo do CSV e do XLSX é o mesmo
		for i, col := range tipo.Colunas {
			if p.Linhas[0].Celulas[i] != col.Exemplo && !(col.Exemplo == "" && (i >= len(p.Linhas[0].Celulas) || p.Linhas[0].Celulas[i] == "")) {
				t.Errorf("%s: exemplo da coluna %s = %q", tipo.Codigo, col.Nome, p.Linhas[0].Celulas[i])
			}
		}
	}
	if _, ok := TipoPorCodigo("usuarios"); ok {
		t.Error("usuários ainda não são importáveis")
	}
}

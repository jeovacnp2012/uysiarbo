package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/importacao"
	"uysiarbo/web/views"
)

// Ações possíveis de cada linha na análise.
const (
	acaoCriar     = "criar"
	acaoAtualizar = "atualizar"
	acaoIgnorar   = "ignorar"
	acaoErro      = "erro"
)

// linhaAnalisada é o resultado da análise de uma linha da planilha: o que seria feito e por quê.
type linhaAnalisada struct {
	N        int
	Acao     string
	Msg      string
	AlvoID   pgtype.Int8
	Dados    []byte
	Antes    []byte
	Original []string
}

// Cargas gravadas em importacao_linhas.dados (o que a execução precisa para gravar).
type cargaBairro struct {
	sqlccliente.CreateBairroParams
	IDExterno string `json:"id_externo"`
}

type cargaQuarteirao struct {
	BairroID  int64  `json:"bairro_id"`
	Nome      string `json:"nome"`
	Ativo     bool   `json:"ativo"`
	IDExterno string `json:"id_externo"`
}

type cargaLocal struct {
	sqlccliente.CreateLocalParams
	Ativo     bool   `json:"ativo"`
	IDExterno string `json:"id_externo"`
}

// resolvedor acha cidades, bairros e quarteirões pelo nome (sem caixa e sem acento).
type resolvedor struct {
	cidades       []sqlccliente.ListCidadesParaImportacaoRow
	cidadesAtivas []sqlccliente.Cidade
	bairros       []sqlccliente.ListBairrosParaImportacaoRow
	quarteiroes   []sqlccliente.ListQuarteiroesParaImportacaoRow
}

func novoResolvedor(ctx context.Context, q *sqlccliente.Queries) (*resolvedor, error) {
	var r resolvedor
	var err error
	if r.cidades, err = q.ListCidadesParaImportacao(ctx); err != nil {
		return nil, err
	}
	if r.cidadesAtivas, err = q.ListCidadesAtivas(ctx); err != nil {
		return nil, err
	}
	if r.bairros, err = q.ListBairrosParaImportacao(ctx); err != nil {
		return nil, err
	}
	if r.quarteiroes, err = q.ListQuarteiroesParaImportacao(ctx); err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *resolvedor) cidade(nome, uf string) (int64, string) {
	var achadas []int64
	for _, c := range r.cidades {
		if importacao.Chave(c.Nome) == importacao.Chave(nome) && (uf == "" || strings.EqualFold(c.Uf, uf)) {
			achadas = append(achadas, c.ID)
		}
	}
	switch len(achadas) {
	case 0:
		return 0, fmt.Sprintf("a cidade %q não está cadastrada (cadastre em Cidades antes)", nome)
	case 1:
		return achadas[0], ""
	}
	return 0, fmt.Sprintf("há mais de uma cidade chamada %q: informe a UF", nome)
}

// bairro acha o bairro pelo nome; cidade e UF (opcionais) desfazem a ambiguidade.
func (r *resolvedor) bairro(cidade, uf, nome string) (int64, string) {
	var cidadeID int64
	if cidade != "" {
		id, msg := r.cidade(cidade, uf)
		if msg != "" {
			return 0, msg
		}
		cidadeID = id
	}
	var achados []int64
	for _, b := range r.bairros {
		if importacao.Chave(b.Nome) == importacao.Chave(nome) && (cidadeID == 0 || b.CidadeID == cidadeID) {
			achados = append(achados, b.ID)
		}
	}
	switch len(achados) {
	case 0:
		return 0, fmt.Sprintf("o bairro %q não está cadastrado", nome)
	case 1:
		return achados[0], ""
	}
	return 0, fmt.Sprintf("há mais de um bairro chamado %q: informe a cidade", nome)
}

func (r *resolvedor) quarteirao(bairroID int64, nome string) (sqlccliente.ListQuarteiroesParaImportacaoRow, bool) {
	for _, q := range r.quarteiroes {
		if q.BairroID == bairroID && importacao.Chave(q.Nome) == importacao.Chave(nome) {
			return q, true
		}
	}
	return sqlccliente.ListQuarteiroesParaImportacaoRow{}, false
}

func erroDeLinha(l *linhaAnalisada, msg string) {
	l.Acao, l.Msg = acaoErro, msg
}

func emJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// valorOu devolve o texto da célula se veio preenchido; senão o atual.
func valorOu(m importacao.Mapeamento, linha []string, coluna, atual string) string {
	if v := m.Valor(linha, coluna); v != "" {
		return v
	}
	return atual
}

// logicoOu lê uma célula sim/não; vazia mantém o atual; texto estranho é erro.
func logicoOu(m importacao.Mapeamento, linha []string, coluna string, atual bool) (bool, string) {
	v := m.Valor(linha, coluna)
	if v == "" {
		return atual, ""
	}
	b, ok := importacao.Booleano(v)
	if !ok {
		return atual, fmt.Sprintf("%s: use sim ou não (veio %q)", coluna, v)
	}
	return b, ""
}

// palavra normaliza um valor de lista fechada: "Ponto Estratégico" vira "ponto_estrategico".
func palavra(s string) string { return importacao.Normalizar(s) }

// ------------------------------------------------------------------- Bairros

func (h *handlers) analisarBairros(ctx context.Context, q *sqlccliente.Queries, p importacao.Planilha, m importacao.Mapeamento, atualizar bool) ([]linhaAnalisada, error) {
	res, err := novoResolvedor(ctx, q)
	if err != nil {
		return nil, err
	}
	existentes, err := q.ListBairrosApi(ctx, sqlccliente.ListBairrosApiParams{Limite: 1 << 30})
	if err != nil {
		return nil, err
	}
	porNome := map[string]sqlccliente.Bairro{}
	porExt := map[string]sqlccliente.Bairro{}
	for _, b := range existentes {
		porNome[strconv.FormatInt(b.CidadeID, 10)+"|"+importacao.Chave(b.Nome)] = b
		if b.IDExterno.Valid {
			porExt[b.IDExterno.String] = b
		}
	}
	vistos := map[string]int{}
	var out []linhaAnalisada
	for _, ln := range p.Linhas {
		l := linhaAnalisada{N: ln.N, Original: ln.Celulas, Acao: acaoCriar}
		v := func(col string) string { return m.Valor(ln.Celulas, col) }
		cid, msg := res.cidade(v("cidade"), v("uf"))
		if v("cidade") == "" {
			cid, msg = 0, "informe a cidade"
		}
		if msg != "" {
			erroDeLinha(&l, msg)
			out = append(out, l)
			continue
		}
		nome := v("nome")
		ext := v("id_externo")
		chave := strconv.FormatInt(cid, 10) + "|" + importacao.Chave(nome)
		if outra, ok := vistos[chave]; ok {
			erroDeLinha(&l, fmt.Sprintf("repetida na planilha (igual à linha %d)", outra))
			out = append(out, l)
			continue
		}
		vistos[chave] = ln.N

		atual, existe := porNome[chave]
		if !existe && ext != "" {
			atual, existe = porExt[ext]
		}
		d := dadosBairro{cidadeID: cid, nome: nome, producao: true, status: "ativo", classificacao: "urbano", categoria: "bairro"}
		if existe {
			d = dadosDoBairro(atual, nil)
			d.cidadeID, d.nome = cid, valorOu(m, ln.Celulas, "nome", atual.Nome)
		}
		d.categoria = palavra(valorOu(m, ln.Celulas, "categoria", d.categoria))
		d.classificacao = palavra(valorOu(m, ln.Celulas, "classificacao", d.classificacao))
		d.status = palavra(valorOu(m, ln.Celulas, "status", d.status))
		d.ibge, d.zona = valorOu(m, ln.Celulas, "ibge", d.ibge), valorOu(m, ln.Celulas, "zona", d.zona)
		d.latitude, d.longitude = valorOu(m, ln.Celulas, "latitude", d.latitude), valorOu(m, ln.Celulas, "longitude", d.longitude)
		d.altitude, d.distancia = valorOu(m, ln.Celulas, "altitude", d.altitude), valorOu(m, ln.Celulas, "distancia_centro", d.distancia)
		var m1, m2 string
		d.mutirao, m1 = logicoOu(m, ln.Celulas, "mutirao", d.mutirao)
		d.producao, m2 = logicoOu(m, ln.Celulas, "producao", d.producao)
		if m1 != "" || m2 != "" {
			erroDeLinha(&l, strings.TrimSpace(m1+" "+m2))
			out = append(out, l)
			continue
		}
		cidadeAtual := int64(0)
		if existe {
			cidadeAtual = atual.CidadeID
		}
		params, msg := validarBairro(d, res.cidadesAtivas, cidadeAtual)
		if msg != "" {
			erroDeLinha(&l, msg)
			out = append(out, l)
			continue
		}
		l.Dados = emJSON(cargaBairro{CreateBairroParams: params, IDExterno: ext})
		switch {
		case !existe:
		case !atualizar:
			l.Acao, l.Msg = acaoIgnorar, "já cadastrado (marque \"atualizar existentes\" para alterar)"
		default:
			antes := sqlccliente.CreateBairroParams{
				CidadeID: atual.CidadeID, Nome: atual.Nome, Mutirao: atual.Mutirao, Status: atual.Status, Ibge: atual.Ibge, Classificacao: atual.Classificacao,
				Zona: atual.Zona, Categoria: atual.Categoria, Latitude: atual.Latitude, Longitude: atual.Longitude, Altitude: atual.Altitude,
				DistanciaCentro: atual.DistanciaCentro, Producao: atual.Producao,
			}
			if antes == params && (ext == "" || (atual.IDExterno.Valid && atual.IDExterno.String == ext)) {
				l.Acao, l.Msg = acaoIgnorar, "sem mudanças"
			} else {
				l.Acao, l.AlvoID, l.Antes = acaoAtualizar, pgtype.Int8{Int64: atual.ID, Valid: true}, emJSON(atual)
			}
		}
		if l.Acao != acaoErro && ext != "" {
			if outro, ok := porExt[ext]; ok && (!existe || outro.ID != atual.ID) {
				erroDeLinha(&l, fmt.Sprintf("o id_externo %q já pertence a outro bairro", ext))
			}
		}
		out = append(out, l)
	}
	return out, nil
}

// ------------------------------------------------------------------- Quarteirões

func (h *handlers) analisarQuarteiroes(ctx context.Context, q *sqlccliente.Queries, p importacao.Planilha, m importacao.Mapeamento, atualizar bool) ([]linhaAnalisada, error) {
	res, err := novoResolvedor(ctx, q)
	if err != nil {
		return nil, err
	}
	porExt := map[string]sqlccliente.ListQuarteiroesParaImportacaoRow{}
	for _, x := range res.quarteiroes {
		if x.IDExterno.Valid {
			porExt[x.IDExterno.String] = x
		}
	}
	vistos := map[string]int{}
	var out []linhaAnalisada
	for _, ln := range p.Linhas {
		l := linhaAnalisada{N: ln.N, Original: ln.Celulas, Acao: acaoCriar}
		v := func(col string) string { return m.Valor(ln.Celulas, col) }
		bid, msg := res.bairro(v("cidade"), v("uf"), v("bairro"))
		if v("bairro") == "" {
			bid, msg = 0, "informe o bairro"
		}
		if msg != "" {
			erroDeLinha(&l, msg)
			out = append(out, l)
			continue
		}
		nome, ext := v("nome"), v("id_externo")
		if msg := validarNomeQuarteirao(nome); msg != "" {
			erroDeLinha(&l, msg)
			out = append(out, l)
			continue
		}
		chave := strconv.FormatInt(bid, 10) + "|" + importacao.Chave(nome)
		if outra, ok := vistos[chave]; ok {
			erroDeLinha(&l, fmt.Sprintf("repetida na planilha (igual à linha %d)", outra))
			out = append(out, l)
			continue
		}
		vistos[chave] = ln.N
		atual, existe := res.quarteirao(bid, nome)
		if !existe && ext != "" {
			if o, ok := porExt[ext]; ok {
				atual, existe = o, true
			}
		}
		ativo := true
		if existe {
			ativo = atual.Ativo
		}
		var m1 string
		if ativo, m1 = logicoOu(m, ln.Celulas, "ativo", ativo); m1 != "" {
			erroDeLinha(&l, m1)
			out = append(out, l)
			continue
		}
		l.Dados = emJSON(cargaQuarteirao{BairroID: bid, Nome: nome, Ativo: ativo, IDExterno: ext})
		switch {
		case !existe:
			if o, ok := porExt[ext]; ext != "" && ok && o.ID != 0 {
				erroDeLinha(&l, fmt.Sprintf("o id_externo %q já pertence a outro quarteirão", ext))
			}
		case !atualizar:
			l.Acao, l.Msg = acaoIgnorar, "já cadastrado (marque \"atualizar existentes\" para alterar)"
		case atual.Ativo == ativo && importacao.Chave(atual.Nome) == importacao.Chave(nome):
			l.Acao, l.Msg = acaoIgnorar, "sem mudanças"
		default:
			full, err := q.GetQuarteirao(ctx, atual.ID)
			if err != nil {
				return nil, err
			}
			l.Acao, l.AlvoID, l.Antes = acaoAtualizar, pgtype.Int8{Int64: atual.ID, Valid: true}, emJSON(full)
		}
		out = append(out, l)
	}
	return out, nil
}

// ------------------------------------------------------------------- Locais

var tiposDeLocalPorPalavra = map[string]string{
	"residencial": "residencial", "residencia": "residencial", "casa": "residencial",
	"comercio": "comercio", "comercial": "comercio",
	"terreno_baldio": "terreno_baldio", "terreno": "terreno_baldio", "baldio": "terreno_baldio",
	"ponto_estrategico": "ponto_estrategico", "pe": "ponto_estrategico",
	"outro": "outro", "outros": "outro",
}

func (h *handlers) analisarLocais(ctx context.Context, q *sqlccliente.Queries, p importacao.Planilha, m importacao.Mapeamento, atualizar bool) ([]linhaAnalisada, error) {
	res, err := novoResolvedor(ctx, q)
	if err != nil {
		return nil, err
	}
	existentes, err := q.ListLocaisParaImportacao(ctx)
	if err != nil {
		return nil, err
	}
	chaveDe := func(qid int64, endereco, numero, lado string, seq, comp int) string {
		return fmt.Sprintf("%d|%s|%s|%s|%d|%d", qid, importacao.Chave(endereco), importacao.Chave(numero), importacao.Chave(lado), seq, comp)
	}
	porNatural := map[string]int64{}
	porExt := map[string]int64{}
	for _, e := range existentes {
		porNatural[chaveDe(e.QuarteiraoID, e.Endereco, e.Numero, e.Lado, int(e.Sequencia), int(e.Complemento))] = e.ID
		if e.IDExterno.Valid {
			porExt[e.IDExterno.String] = e.ID
		}
	}
	vistos := map[string]int{}
	var out []linhaAnalisada
	for _, ln := range p.Linhas {
		l := linhaAnalisada{N: ln.N, Original: ln.Celulas, Acao: acaoCriar}
		v := func(col string) string { return m.Valor(ln.Celulas, col) }
		bid, msg := res.bairro(v("cidade"), v("uf"), v("bairro"))
		if v("bairro") == "" {
			bid, msg = 0, "informe o bairro"
		}
		if msg != "" {
			erroDeLinha(&l, msg)
			out = append(out, l)
			continue
		}
		qt, ok := res.quarteirao(bid, v("quarteirao"))
		if !ok {
			erroDeLinha(&l, fmt.Sprintf("o quarteirão %q não está cadastrado nesse bairro (importe os quarteirões antes)", v("quarteirao")))
			out = append(out, l)
			continue
		}
		ext := v("id_externo")
		seq, comp := 0, 0
		if s := v("sequencia"); s != "" {
			seq, _ = strconv.Atoi(s)
		}
		if s := v("complemento"); s != "" {
			comp, _ = strconv.Atoi(s)
		}
		natural := chaveDe(qt.ID, v("endereco"), v("numero"), v("lado"), seq, comp)
		if outra, ok := vistos[natural]; ok {
			erroDeLinha(&l, fmt.Sprintf("repetida na planilha (igual à linha %d)", outra))
			out = append(out, l)
			continue
		}
		vistos[natural] = ln.N

		var atual sqlccliente.Locai
		existe := false
		alvo, achou := porNatural[natural]
		if !achou && ext != "" {
			alvo, achou = porExt[ext]
		}
		if achou {
			if atual, err = q.GetLocal(ctx, alvo); err != nil {
				return nil, err
			}
			existe = true
		}

		var campos views.LocalCampos
		ativo := true
		if existe {
			campos, ativo = camposDoLocal(atual), atual.Ativo
		}
		tipo := palavra(valorOu(m, ln.Celulas, "tipo", campos.Tipo))
		if t, ok := tiposDeLocalPorPalavra[tipo]; ok {
			tipo = t
		}
		campos.Tipo = tipo
		campos.CEP = valorOu(m, ln.Celulas, "cep", campos.CEP)
		campos.Endereco = valorOu(m, ln.Celulas, "endereco", campos.Endereco)
		campos.Numero = valorOu(m, ln.Celulas, "numero", campos.Numero)
		campos.Referencia = valorOu(m, ln.Celulas, "referencia", campos.Referencia)
		campos.BairroID, campos.QuarteiraoID = strconv.FormatInt(bid, 10), strconv.FormatInt(qt.ID, 10)
		campos.Lado = valorOu(m, ln.Celulas, "lado", campos.Lado)
		campos.Sequencia, campos.Complemento = valorOu(m, ln.Celulas, "sequencia", campos.Sequencia), valorOu(m, ln.Celulas, "complemento", campos.Complemento)
		campos.Latitude, campos.Longitude = valorOu(m, ln.Celulas, "latitude", campos.Latitude), valorOu(m, ln.Celulas, "longitude", campos.Longitude)
		campos.Proprietario = valorOu(m, ln.Celulas, "proprietario", campos.Proprietario)
		campos.ProprietarioTelefone = valorOu(m, ln.Celulas, "proprietario_telefone", campos.ProprietarioTelefone)
		campos.Whatsapp = valorOu(m, ln.Celulas, "proprietario_whatsapp", campos.Whatsapp)
		campos.Email = valorOu(m, ln.Celulas, "proprietario_email", campos.Email)
		campos.Inquilino = valorOu(m, ln.Celulas, "inquilino", campos.Inquilino)
		campos.InquilinoTelefone = valorOu(m, ln.Celulas, "inquilino_telefone", campos.InquilinoTelefone)
		campos.Humanos, campos.Cachorros = valorOu(m, ln.Celulas, "humanos", campos.Humanos), valorOu(m, ln.Celulas, "cachorros", campos.Cachorros)
		campos.Gatos, campos.Roedores = valorOu(m, ln.Celulas, "gatos", campos.Gatos), valorOu(m, ln.Celulas, "roedores", campos.Roedores)
		var m1, m2, m3 string
		campos.AceitaWhatsapp, m1 = logicoOu(m, ln.Celulas, "aceita_whatsapp", campos.AceitaWhatsapp)
		campos.AceitaEmail, m2 = logicoOu(m, ln.Celulas, "aceita_email", campos.AceitaEmail)
		ativo, m3 = logicoOu(m, ln.Celulas, "ativo", ativo)
		if m1+m2+m3 != "" {
			erroDeLinha(&l, strings.TrimSpace(m1+" "+m2+" "+m3))
			out = append(out, l)
			continue
		}
		bairroAtual, quartAtual := int64(0), int64(0)
		if existe {
			bairroAtual, quartAtual = atual.BairroID, atual.QuarteiraoID
		}
		params, msg, err := validarLocal(ctx, q, campos, bairroAtual, quartAtual)
		if err != nil {
			return nil, err
		}
		if msg != "" {
			erroDeLinha(&l, msg)
			out = append(out, l)
			continue
		}
		l.Dados = emJSON(cargaLocal{CreateLocalParams: params, Ativo: ativo, IDExterno: ext})
		switch {
		case !existe:
			if _, usado := porExt[ext]; ext != "" && usado {
				erroDeLinha(&l, fmt.Sprintf("o id_externo %q já pertence a outro local", ext))
			}
		case !atualizar:
			l.Acao, l.Msg = acaoIgnorar, "já cadastrado (marque \"atualizar existentes\" para alterar)"
		default:
			antes := paramsDoLocal(atual)
			depois := params
			antes.CriadoPor, depois.CriadoPor = pgtype.Int8{}, pgtype.Int8{}
			if antes == depois && atual.Ativo == ativo && (ext == "" || (atual.IDExterno.Valid && atual.IDExterno.String == ext)) {
				l.Acao, l.Msg = acaoIgnorar, "sem mudanças"
			} else {
				l.Acao, l.AlvoID, l.Antes = acaoAtualizar, pgtype.Int8{Int64: atual.ID, Valid: true}, emJSON(atual)
			}
		}
		out = append(out, l)
	}
	return out, nil
}

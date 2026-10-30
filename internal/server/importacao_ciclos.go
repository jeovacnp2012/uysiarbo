package server

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/importacao"
)

// cargaCiclo é o que a execução precisa para gravar uma linha de ciclo.
type cargaCiclo struct {
	Nome       string  `json:"nome"`
	Inicio     string  `json:"inicio"` // aaaa-mm-dd
	Final      string  `json:"final"`
	Situacao   string  `json:"situacao"`
	Fora       bool    `json:"fora"`
	Publico    bool    `json:"publico"`
	Texto      string  `json:"texto"`
	Atividades []int16 `json:"atividades"`
	Painel     []int16 `json:"painel"`
	// Trocar* dizem se a planilha trouxe a lista (célula vazia mantém a atual numa atualização).
	TrocarAtividades bool `json:"trocar_atividades"`
	TrocarPainel     bool `json:"trocar_painel"`
}

// antesDoCiclo é o estado guardado para desfazer uma atualização.
type antesDoCiclo struct {
	Ciclo      sqlccliente.Ciclo `json:"ciclo"`
	Atividades []int16           `json:"atividades"`
	Painel     []int16           `json:"painel"`
}

// siglasParaIDs converte "LIT, LI; pe" nos ids do catálogo; sigla desconhecida vira mensagem de erro.
func siglasParaIDs(texto string, cat []sqlccliente.Atividade) ([]int16, string) {
	var ids []int16
	for _, s := range strings.FieldsFunc(texto, func(r rune) bool { return r == ',' || r == ';' || r == '/' || r == ' ' }) {
		i := slices.IndexFunc(cat, func(a sqlccliente.Atividade) bool { return strings.EqualFold(a.Sigla, s) })
		if i < 0 {
			return nil, fmt.Sprintf("atividade desconhecida: %q", s)
		}
		if !slices.Contains(ids, cat[i].ID) {
			ids = append(ids, cat[i].ID)
		}
	}
	slices.Sort(ids)
	return ids, ""
}

func (h *handlers) analisarCiclos(ctx context.Context, q *sqlccliente.Queries, p importacao.Planilha, m importacao.Mapeamento, atualizar bool) ([]linhaAnalisada, error) {
	cat, err := q.ListAtividades(ctx)
	if err != nil {
		return nil, err
	}
	existentes, err := q.ListCiclos(ctx)
	if err != nil {
		return nil, err
	}
	porNome := map[string]sqlccliente.ListCiclosRow{}
	for _, c := range existentes {
		porNome[importacao.Chave(c.Nome)] = c
	}
	vistos := map[string]int{}
	var out []linhaAnalisada
	for _, ln := range p.Linhas {
		l := linhaAnalisada{N: ln.N, Original: ln.Celulas, Acao: acaoCriar}
		v := func(col string) string { return m.Valor(ln.Celulas, col) }
		nome := v("nome")
		chave := importacao.Chave(nome)
		if len([]rune(nome)) < 2 || len([]rune(nome)) > 60 {
			erroDeLinha(&l, "informe o nome do ciclo (de 2 a 60 caracteres)")
			out = append(out, l)
			continue
		}
		if outra, ok := vistos[chave]; ok {
			erroDeLinha(&l, fmt.Sprintf("repetida na planilha (igual à linha %d)", outra))
			out = append(out, l)
			continue
		}
		vistos[chave] = ln.N

		atual, existe := porNome[chave]
		c := cargaCiclo{Nome: nome, Situacao: "operando"}
		var antes antesDoCiclo
		if existe {
			antes.Ciclo = sqlccliente.Ciclo{
				ID: atual.ID, Nome: atual.Nome, DataInicio: atual.DataInicio, DataFinal: atual.DataFinal, Situacao: atual.Situacao,
				PermiteForaPeriodo: atual.PermiteForaPeriodo, PainelPublico: atual.PainelPublico, PainelPublicoTexto: atual.PainelPublicoTexto,
			}
			if antes.Atividades, err = q.ListAtividadesDoCiclo(ctx, atual.ID); err != nil {
				return nil, err
			}
			if antes.Painel, err = q.ListAtividadesDoPainel(ctx, atual.ID); err != nil {
				return nil, err
			}
			c = cargaCiclo{
				Nome: nome, Inicio: dataISO(atual.DataInicio), Final: dataISO(atual.DataFinal), Situacao: atual.Situacao, Fora: atual.PermiteForaPeriodo,
				Publico: atual.PainelPublico, Texto: atual.PainelPublicoTexto, Atividades: antes.Atividades, Painel: antes.Painel,
			}
		}

		// Datas.
		var msgs []string
		for _, d := range []struct {
			col string
			dst *string
		}{{"data_inicio", &c.Inicio}, {"data_final", &c.Final}} {
			if txt := v(d.col); txt != "" {
				t, ok := importacao.Data(txt)
				if !ok {
					msgs = append(msgs, fmt.Sprintf("%s: data inválida (%q), use dd/mm/aaaa", d.col, txt))
					continue
				}
				*d.dst = t.Format(formatoDataISO)
			} else if *d.dst == "" {
				msgs = append(msgs, "informe "+d.col)
			}
		}
		if len(msgs) == 0 {
			ini, _ := time.Parse(formatoDataISO, c.Inicio)
			fim, _ := time.Parse(formatoDataISO, c.Final)
			if fim.Before(ini) {
				msgs = append(msgs, "a data final não pode ser anterior à data de início")
			}
		}
		// Situação e listas.
		if s := v("situacao"); s != "" {
			switch palavra(s) {
			case "operando", "aberto", "ativo":
				c.Situacao = "operando"
			case "fechado", "encerrado":
				c.Situacao = "fechado"
			default:
				msgs = append(msgs, fmt.Sprintf("situação: use operando ou fechado (veio %q)", s))
			}
		}
		if txt := v("atividades"); txt != "" {
			ids, msg := siglasParaIDs(txt, cat)
			if msg != "" {
				msgs = append(msgs, msg)
			}
			c.Atividades, c.TrocarAtividades = ids, true
		}
		if txt := v("painel_atividades"); txt != "" {
			ids, msg := siglasParaIDs(txt, cat)
			if msg != "" {
				msgs = append(msgs, "painel: "+msg)
			}
			c.Painel, c.TrocarPainel = ids, true
		}
		var m1, m2 string
		c.Fora, m1 = logicoOu(m, ln.Celulas, "permite_fora_periodo", c.Fora)
		c.Publico, m2 = logicoOu(m, ln.Celulas, "painel_publico", c.Publico)
		msgs = append(msgs, m1, m2)
		c.Texto = valorOu(m, ln.Celulas, "painel_texto", c.Texto)
		if len([]rune(c.Texto)) > 100 {
			msgs = append(msgs, "o nome do painel público tem no máximo 100 caracteres")
		}
		// Painel: só atividades do ciclo; público sem atividades do painel usa as do ciclo.
		if c.Publico {
			if len(c.Painel) == 0 {
				c.Painel, c.TrocarPainel = slices.Clone(c.Atividades), true
			}
			c.Painel = slices.DeleteFunc(slices.Clone(c.Painel), func(a int16) bool { return !slices.Contains(c.Atividades, a) })
			if len(c.Painel) == 0 {
				msgs = append(msgs, "para o painel público, o ciclo precisa de atividades")
			}
		}
		if msg := strings.TrimSpace(strings.Join(slices.DeleteFunc(msgs, func(s string) bool { return s == "" }), "; ")); msg != "" {
			erroDeLinha(&l, msg)
			out = append(out, l)
			continue
		}

		l.Dados = emJSON(c)
		switch {
		case !existe:
		case !atualizar:
			l.Acao, l.Msg = acaoIgnorar, "já cadastrado (marque \"atualizar existentes\" para alterar)"
		default:
			igual := antes.Ciclo.Nome == c.Nome && dataISO(antes.Ciclo.DataInicio) == c.Inicio && dataISO(antes.Ciclo.DataFinal) == c.Final &&
				antes.Ciclo.Situacao == c.Situacao && antes.Ciclo.PermiteForaPeriodo == c.Fora && antes.Ciclo.PainelPublico == c.Publico &&
				antes.Ciclo.PainelPublicoTexto == c.Texto && slices.Equal(antes.Atividades, c.Atividades) && slices.Equal(antes.Painel, c.Painel)
			if igual {
				l.Acao, l.Msg = acaoIgnorar, "sem mudanças"
			} else {
				l.Acao, l.AlvoID, l.Antes = acaoAtualizar, pgtype.Int8{Int64: atual.ID, Valid: true}, emJSON(antes)
			}
		}
		out = append(out, l)
	}
	return out, nil
}

func aplicarCiclo(ctx context.Context, q *sqlccliente.Queries, ln sqlccliente.ImportacaoLinha) resultadoDaLinha {
	erro := func(msg string) resultadoDaLinha { return resultadoDaLinha{Resultado: "erro", Msg: msg} }
	var c cargaCiclo
	if err := json.Unmarshal(ln.Dados, &c); err != nil {
		return erro("dados da linha ilegíveis")
	}
	ini, e1 := time.Parse(formatoDataISO, c.Inicio)
	fim, e2 := time.Parse(formatoDataISO, c.Final)
	if e1 != nil || e2 != nil {
		return erro("datas ilegíveis")
	}
	pIni, pFim := pgtype.Date{Time: ini, Valid: true}, pgtype.Date{Time: fim, Valid: true}
	id := ln.AlvoID.Int64
	resultado := "atualizado"
	if ln.Acao == acaoCriar {
		novo, err := q.CreateCiclo(ctx, sqlccliente.CreateCicloParams{
			Nome: c.Nome, DataInicio: pIni, DataFinal: pFim, Situacao: c.Situacao, PermiteForaPeriodo: c.Fora,
			PainelPublico: c.Publico, PainelPublicoTexto: c.Texto,
		})
		if err != nil {
			return erro(textoDoErro(err, "já existe um ciclo com esse nome"))
		}
		id, resultado = novo.ID, "criado"
	} else if _, err := q.UpdateCiclo(ctx, sqlccliente.UpdateCicloParams{
		ID: id, Nome: c.Nome, DataInicio: pIni, DataFinal: pFim, Situacao: c.Situacao, PermiteForaPeriodo: c.Fora,
		PainelPublico: c.Publico, PainelPublicoTexto: c.Texto,
	}); err != nil {
		return erro(textoDoErro(err, "já existe um ciclo com esse nome"))
	}
	if ln.Acao == acaoCriar || c.TrocarAtividades {
		if err := trocarAtividadesDoCiclo(ctx, q, id, c.Atividades, c.Painel); err != nil {
			return erro(textoDoErro(err, ""))
		}
	} else if c.TrocarPainel {
		if err := q.ClearAtividadesDoPainel(ctx, id); err != nil {
			return erro(textoDoErro(err, ""))
		}
		if err := q.AddAtividadesAoPainel(ctx, sqlccliente.AddAtividadesAoPainelParams{Ciclo: id, Atividades: c.Painel}); err != nil {
			return erro(textoDoErro(err, ""))
		}
	}
	return resultadoDaLinha{Resultado: resultado, RegistroID: id}
}

// trocarAtividadesDoCiclo substitui as atividades do ciclo e as do painel público.
func trocarAtividadesDoCiclo(ctx context.Context, q *sqlccliente.Queries, id int64, atividades, painel []int16) error {
	for _, e := range []error{
		q.ClearAtividadesDoCiclo(ctx, id),
		q.AddAtividadesAoCiclo(ctx, sqlccliente.AddAtividadesAoCicloParams{Ciclo: id, Atividades: atividades}),
		q.ClearAtividadesDoPainel(ctx, id),
		q.AddAtividadesAoPainel(ctx, sqlccliente.AddAtividadesAoPainelParams{Ciclo: id, Atividades: painel}),
	} {
		if e != nil {
			return e
		}
	}
	return nil
}

func desfazerCiclo(ctx context.Context, q *sqlccliente.Queries, ln sqlccliente.ImportacaoLinha, criado bool, id int64, motivo func(error) string) string {
	if criado {
		return desfazerCriacao(ctx, q, "ciclo", id)
	}
	var a antesDoCiclo
	if err := json.Unmarshal(ln.Antes, &a); err != nil {
		return "o estado anterior não foi guardado"
	}
	if _, err := q.UpdateCiclo(ctx, sqlccliente.UpdateCicloParams{
		ID: id, Nome: a.Ciclo.Nome, DataInicio: a.Ciclo.DataInicio, DataFinal: a.Ciclo.DataFinal, Situacao: a.Ciclo.Situacao,
		PermiteForaPeriodo: a.Ciclo.PermiteForaPeriodo, PainelPublico: a.Ciclo.PainelPublico, PainelPublicoTexto: a.Ciclo.PainelPublicoTexto,
	}); err != nil {
		return motivo(err)
	}
	return motivo(trocarAtividadesDoCiclo(ctx, q, id, a.Atividades, a.Painel))
}

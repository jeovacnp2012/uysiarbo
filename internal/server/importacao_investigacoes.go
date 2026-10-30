package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/importacao"
)

// Importação de investigações: só cria. Investigação que já existe (mesmo ID de origem) é ignorada, então dá para
// repetir a importação sem duplicar. A doença precisa estar cadastrada; o local e o responsável são opcionais.

type cargaInvestigacao struct {
	Create sqlccliente.CreateInvestigacaoImportadaParams `json:"create"`
}

var situacoesImportaveis = map[string]string{
	"aguardando": "aguardando", "aguardando atendimento": "aguardando",
	"em atendimento": "em_atendimento", "em_atendimento": "em_atendimento", "finalizado em campo": "em_atendimento",
	"encerrada": "encerrada", "encerrado": "encerrada",
}

func (h *handlers) analisarInvestigacoes(ctx context.Context, r *http.Request, q *sqlccliente.Queries, p importacao.Planilha, m importacao.Mapeamento) ([]linhaAnalisada, error) {
	doencas, err := q.ListDoencas(ctx)
	if err != nil {
		return nil, err
	}
	porDoenca := map[string]sqlccliente.ListDoencasRow{}
	for _, d := range doencas {
		porDoenca[importacao.Chave(d.Nome)] = d
	}
	usuarios, err := q.ListUsuarios(ctx)
	if err != nil {
		return nil, err
	}
	porAgente := map[string]int64{}
	repetidos := map[string]bool{}
	for _, u := range usuarios {
		for _, k := range []string{importacao.Chave(u.Nome), strings.ToLower(strings.TrimSpace(u.Email))} {
			if k == "" {
				continue
			}
			if _, ok := porAgente[k]; ok {
				repetidos[k] = true
			}
			porAgente[k] = u.ID
		}
	}
	locais := map[string]int64{}
	vistos := map[string]int{}
	var out []linhaAnalisada
	for _, ln := range p.Linhas {
		l := linhaAnalisada{N: ln.N, Original: ln.Celulas, Acao: acaoCriar}
		v := func(col string) string { return m.Valor(ln.Celulas, col) }

		ext := v("id_externo")
		if len(ext) > 64 {
			erroDeLinha(&l, "o ID de origem passa de 64 caracteres")
			out = append(out, l)
			continue
		}
		if ext != "" {
			if outra, ok := vistos[ext]; ok {
				erroDeLinha(&l, fmt.Sprintf("repetida na planilha (igual à linha %d)", outra))
				out = append(out, l)
				continue
			}
			vistos[ext] = ln.N
			if _, err := q.GetInvestigacaoPorIDExterno(ctx, pgtype.Text{String: ext, Valid: true}); err == nil {
				l.Acao, l.Msg = acaoIgnorar, "já cadastrada"
				out = append(out, l)
				continue
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
		}

		var msgs []string
		falha := func(s string) { msgs = append(msgs, s) }
		c := sqlccliente.CreateInvestigacaoImportadaParams{
			EnderecoTexto: v("endereco"), Paciente: v("paciente"), Telefone: v("telefone"), Observacao: v("observacao"),
			Situacao: "aguardando", Resultado: "aguardando",
			CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
		}
		if ext != "" {
			c.IDExterno = pgtype.Text{String: ext, Valid: true}
		}
		d, ok := porDoenca[importacao.Chave(v("doenca"))]
		if !ok {
			falha(fmt.Sprintf("doença %q não cadastrada (cadastre em Doenças e agravos)", v("doenca")))
		} else {
			c.DoencaID, c.GrupoID = d.ID, d.GrupoID
		}
		switch {
		case c.Paciente == "" || len([]rune(c.Paciente)) > 150:
			falha("informe o paciente (até 150 caracteres)")
		case len([]rune(c.Telefone)) > 40:
			falha("telefone com mais de 40 caracteres")
		case len([]rune(c.EnderecoTexto)) > 300:
			falha("endereço com mais de 300 caracteres")
		case len([]rune(c.Observacao)) > 2000:
			falha("observação com mais de 2000 caracteres")
		}
		if txt := v("data_inicio"); txt != "" {
			if t, ok := importacao.Data(txt); ok {
				c.DataInicio = pgtype.Date{Time: t, Valid: true}
			} else {
				falha(fmt.Sprintf("data de início inválida (%q), use dd/mm/aaaa", txt))
			}
		}
		if txt := v("situacao"); txt != "" {
			if s, ok := situacoesImportaveis[palavra(txt)]; ok {
				c.Situacao = s
			} else {
				falha(fmt.Sprintf("situação: use aguardando, em atendimento ou encerrada (veio %q)", txt))
			}
		}
		if txt := v("resultado"); txt != "" {
			if pv := palavra(txt); validoEm(rotulosResultadoInvestigacao, pv) {
				c.Resultado = pv
			} else {
				falha(fmt.Sprintf("resultado: use aguardando, positivo, negativo ou inconclusivo (veio %q)", txt))
			}
		}
		if ext := v("local"); ext != "" {
			id, visto := locais[ext]
			if !visto {
				if lc, err := q.GetLocalPorIdExterno(ctx, pgtype.Text{String: ext, Valid: true}); err == nil {
					id = lc.ID
				} else if !errors.Is(err, pgx.ErrNoRows) {
					return nil, err
				}
				locais[ext] = id
			}
			if id == 0 {
				falha(fmt.Sprintf("local %q não encontrado (importe os locais antes)", ext))
			} else {
				c.LocalID = pgtype.Int8{Int64: id, Valid: true}
			}
		}
		if resp := v("responsavel"); resp != "" {
			chave := importacao.Chave(resp)
			if strings.Contains(resp, "@") {
				chave = strings.ToLower(resp)
			}
			switch {
			case repetidos[chave]:
				falha(fmt.Sprintf("responsável %q: há mais de um usuário com esse nome (use o e-mail)", resp))
			case porAgente[chave] == 0:
				falha(fmt.Sprintf("responsável %q não encontrado", resp))
			default:
				c.ResponsavelID = pgtype.Int8{Int64: porAgente[chave], Valid: true}
			}
		}
		if len(msgs) > 0 {
			erroDeLinha(&l, strings.Join(msgs, "; "))
			out = append(out, l)
			continue
		}
		l.Dados = emJSON(cargaInvestigacao{Create: c})
		out = append(out, l)
	}
	return out, nil
}

func aplicarInvestigacao(ctx context.Context, q *sqlccliente.Queries, ln sqlccliente.ImportacaoLinha, usuarioID int64) resultadoDaLinha {
	var c cargaInvestigacao
	if err := json.Unmarshal(ln.Dados, &c); err != nil {
		return resultadoDaLinha{Resultado: "erro", Msg: "dados da linha ilegíveis"}
	}
	c.Create.CriadoPor = pgtype.Int8{Int64: usuarioID, Valid: usuarioID != 0}
	i, err := q.CreateInvestigacaoImportada(ctx, c.Create)
	if err != nil {
		return resultadoDaLinha{Resultado: "erro", Msg: textoDoErro(err, "já existe uma investigação com esse ID de origem")}
	}
	return resultadoDaLinha{Resultado: "criado", RegistroID: i.ID}
}

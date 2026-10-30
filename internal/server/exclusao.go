package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Exclusão lógica (deleted_at), Lixeira e exclusão definitiva.
//
// Regras (pedido do usuário, 2026-10-29):
//   - "Excluir" nunca apaga de verdade: preenche deleted_at e o registro vai para a Lixeira.
//   - Excluir um registro que ainda tem filhos ativos (bairro com locais, por exemplo) é recusado, com aviso.
//   - A exclusão definitiva exige a permissão conf_exclusao_definitiva, só vale para quem já está na Lixeira e
//     é recusada, com aviso, se houver vínculo com QUALQUER outro registro (inclusive os já excluídos logicamente).

type tipoExcluivel struct {
	Chave   string // usada nas rotas e na auditoria, ex.: "bairro"
	Rotulo  string // "Bairro"
	Plural  string // "Bairros"
	Genero  string // "o" ou "a", para as mensagens
	soft    func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error)
	restore func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error)
	force   func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error)
	// rotulo devolve o nome do registro (mesmo excluído) e se ele está excluído.
	rotulo func(ctx context.Context, q *sqlccliente.Queries, id int64) (nome string, excluido bool, err error)
	// filhosAtivos lista o que impede a exclusão lógica; vinculos, o que impede a definitiva.
	filhosAtivos func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error)
	vinculos     func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error)
	// paiExcluido diz se algum "pai" do registro está excluído (impede restaurar antes dele).
	paiExcluido func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error)
	lixeira     func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error)
}

func plural(n int32, singular, pl string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, pl)
}

func juntar(itens []string) []string {
	if len(itens) == 0 {
		return nil
	}
	return []string{strings.Join(itens, " e ")}
}

var tiposExcluiveis = map[string]*tipoExcluivel{
	"cidade": {
		Chave: "cidade", Rotulo: "Cidade", Plural: "Cidades", Genero: "a",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteCidade(ctx, sqlccliente.SoftDeleteCidadeParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreCidade(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteCidade(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			c, err := q.GetCidadeQualquer(ctx, id)
			return c.Nome + "/" + c.Uf, c.DeletedAt.Valid, err
		},
		filhosAtivos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarFilhosDaCidade(ctx, id)
			if err != nil || n.Ativos == 0 {
				return nil, err
			}
			return []string{plural(n.Ativos, "bairro", "bairros")}, nil
		},
		vinculos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarFilhosDaCidade(ctx, id)
			if err != nil || n.Total == 0 {
				return nil, err
			}
			return []string{plural(n.Total, "bairro", "bairros") + " (contando os excluídos)"}, nil
		},
		paiExcluido: func(context.Context, *sqlccliente.Queries, int64) (string, error) { return "", nil },
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListCidadesExcluidas(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"bairro": {
		Chave: "bairro", Rotulo: "Bairro", Plural: "Bairros", Genero: "o",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteBairro(ctx, sqlccliente.SoftDeleteBairroParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreBairro(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteBairro(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			b, err := q.GetBairroQualquer(ctx, id)
			return b.Nome, b.DeletedAt.Valid, err
		},
		filhosAtivos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarFilhosDoBairro(ctx, id)
			if err != nil {
				return nil, err
			}
			var itens []string
			if n.QuarteiroesAtivos > 0 {
				itens = append(itens, plural(n.QuarteiroesAtivos, "quarteirão", "quarteirões"))
			}
			if n.LocaisAtivos > 0 {
				itens = append(itens, plural(n.LocaisAtivos, "local", "locais"))
			}
			return juntar(itens), nil
		},
		vinculos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarFilhosDoBairro(ctx, id)
			if err != nil {
				return nil, err
			}
			var itens []string
			if n.QuarteiroesTotal > 0 {
				itens = append(itens, plural(n.QuarteiroesTotal, "quarteirão", "quarteirões"))
			}
			if n.LocaisTotal > 0 {
				itens = append(itens, plural(n.LocaisTotal, "local", "locais"))
			}
			if len(itens) > 0 {
				itens[len(itens)-1] += " (contando os excluídos)"
			}
			f, err := q.ContarFichasDoBairro(ctx, id)
			if err != nil {
				return nil, err
			}
			if f.Reconhecimentos > 0 {
				itens = append(itens, plural(f.Reconhecimentos, "reconhecimento geográfico", "reconhecimentos geográficos"))
			}
			if f.Resumos > 0 {
				itens = append(itens, plural(f.Resumos, "resumo semanal", "resumos semanais"))
			}
			return juntar(itens), nil
		},
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			b, err := q.GetBairroQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			if c, err := q.GetCidadeQualquer(ctx, b.CidadeID); err == nil && c.DeletedAt.Valid {
				return "a cidade " + c.Nome + "/" + c.Uf, nil
			}
			return "", nil
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListBairrosExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"quarteirao": {
		Chave: "quarteirao", Rotulo: "Quarteirão", Plural: "Quarteirões", Genero: "o",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteQuarteirao(ctx, sqlccliente.SoftDeleteQuarteiraoParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreQuarteirao(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteQuarteirao(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			x, err := q.GetQuarteiraoQualquer(ctx, id)
			return "Quarteirão " + x.Nome, x.DeletedAt.Valid, err
		},
		filhosAtivos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarFilhosDoQuarteirao(ctx, id)
			if err != nil || n.Ativos == 0 {
				return nil, err
			}
			return []string{plural(n.Ativos, "local", "locais")}, nil
		},
		vinculos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarFilhosDoQuarteirao(ctx, id)
			if err != nil || n.Total == 0 {
				return nil, err
			}
			return []string{plural(n.Total, "local", "locais") + " (contando os excluídos)"}, nil
		},
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			x, err := q.GetQuarteiraoQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			if b, err := q.GetBairroQualquer(ctx, x.BairroID); err == nil && b.DeletedAt.Valid {
				return "o bairro " + b.Nome, nil
			}
			return "", nil
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListQuarteiroesExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"local": {
		Chave: "local", Rotulo: "Local", Plural: "Locais", Genero: "o",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteLocal(ctx, sqlccliente.SoftDeleteLocalParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreLocal(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteLocal(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			l, err := q.GetLocalQualquer(ctx, id)
			return enderecoCompleto(l.Endereco, l.Numero), l.DeletedAt.Valid, err
		},
		filhosAtivos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarVisitasDoLocal(ctx, id)
			if err != nil {
				return nil, err
			}
			var itens []string
			if n.Ativas > 0 {
				itens = append(itens, plural(n.Ativas, "visita ativa", "visitas ativas"))
			}
			if c, err := q.CountNotificacoesAtivasDoLocal(ctx, id); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "notificação ativa", "notificações ativas"))
			}
			if c, err := q.CountArmadilhasAtivasDoLocal(ctx, id); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "armadilha ativa", "armadilhas ativas"))
			}
			if c, err := q.CountBaapAtivosDoLocal(ctx, pgtype.Int8{Int64: id, Valid: true}); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "notificação de animal peçonhento ativa", "notificações de animais peçonhentos ativas"))
			}
			if c, err := q.CountCampanhasAtivasDoLocal(ctx, id); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "campanha antirrábica ativa", "campanhas antirrábicas ativas"))
			}
			if c, err := q.CountDronesAtivosDoLocal(ctx, id); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "solicitação de drone ativa", "solicitações de drone ativas"))
			}
			return itens, nil
		},
		vinculos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarVinculosDoLocal(ctx, id)
			if err != nil {
				return nil, err
			}
			var itens []string
			if v, err := q.ContarVisitasDoLocal(ctx, id); err != nil {
				return nil, err
			} else if v.Total > 0 {
				itens = append(itens, plural(v.Total, "visita", "visitas")+" (contando as excluídas)")
			}
			if c, err := q.CountNotificacoesDoLocalQualquer(ctx, id); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "notificação", "notificações")+" (contando as excluídas)")
			}
			if c, err := q.CountArmadilhasDoLocalQualquer(ctx, id); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "armadilha", "armadilhas")+" (contando as excluídas)")
			}
			if c, err := q.CountBaapDoLocalQualquer(ctx, pgtype.Int8{Int64: id, Valid: true}); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "notificação de animal peçonhento", "notificações de animais peçonhentos")+" (contando as excluídas)")
			}
			if c, err := q.CountCampanhasDoLocalQualquer(ctx, id); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "campanha antirrábica", "campanhas antirrábicas")+" (contando as excluídas)")
			}
			if c, err := q.CountDronesDoLocalQualquer(ctx, id); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "solicitação de drone", "solicitações de drone")+" (contando as excluídas)")
			}
			if c, err := q.CountAutosDoLocalQualquer(ctx, id); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "auto de infração", "autos de infração")+" (contando os excluídos)")
			}
			if n.Grupos > 0 {
				itens = append(itens, plural(n.Grupos, "grupo de locais", "grupos de locais"))
			}
			if n.Acessos > 0 {
				itens = append(itens, plural(n.Acessos, "acesso concedido a uma pessoa", "acessos concedidos a pessoas"))
			}
			return juntar(itens), nil
		},
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			l, err := q.GetLocalQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			if x, err := q.GetQuarteiraoQualquer(ctx, l.QuarteiraoID); err == nil && x.DeletedAt.Valid {
				return "o quarteirão " + x.Nome, nil
			}
			if b, err := q.GetBairroQualquer(ctx, l.BairroID); err == nil && b.DeletedAt.Valid {
				return "o bairro " + b.Nome, nil
			}
			return "", nil
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListLocaisExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"mapeamento": {
		Chave: "mapeamento", Rotulo: "Mapeamento de colunas", Plural: "Mapeamentos de colunas", Genero: "o",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteMapeamento(ctx, sqlccliente.SoftDeleteMapeamentoParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreMapeamento(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteMapeamento(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			m, err := q.GetMapeamentoQualquer(ctx, id)
			return m.Nome, m.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido:  func(context.Context, *sqlccliente.Queries, int64) (string, error) { return "", nil },
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListMapeamentosExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"ciclo": {
		Chave: "ciclo", Rotulo: "Ciclo", Plural: "Ciclos", Genero: "o",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteCiclo(ctx, sqlccliente.SoftDeleteCicloParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreCiclo(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteCiclo(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			c, err := q.GetCicloQualquer(ctx, id)
			return c.Nome, c.DeletedAt.Valid, err
		},
		// As demais atividades do ciclo (notificações, ovitrampas...) entram aqui quando as suas etapas existirem.
		filhosAtivos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarVisitasDoCiclo(ctx, id)
			if err != nil || n.Ativas == 0 {
				return nil, err
			}
			return []string{plural(n.Ativas, "visita ativa", "visitas ativas")}, nil
		},
		vinculos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			var itens []string
			if v, err := q.ContarVisitasDoCiclo(ctx, id); err != nil {
				return nil, err
			} else if v.Total > 0 {
				itens = append(itens, plural(v.Total, "visita", "visitas")+" (contando as excluídas)")
			}
			f, err := q.ContarFichasDoCiclo(ctx, id)
			if err != nil {
				return nil, err
			}
			if f.Reconhecimentos > 0 {
				itens = append(itens, plural(f.Reconhecimentos, "reconhecimento geográfico", "reconhecimentos geográficos"))
			}
			if f.Resumos > 0 {
				itens = append(itens, plural(f.Resumos, "resumo semanal", "resumos semanais"))
			}
			n, err := q.ContarVinculosDoCiclo(ctx, id)
			if err != nil {
				return nil, err
			}
			if n > 0 {
				itens = append(itens, plural(n, "ciclo sincronizado com este", "ciclos sincronizados com este"))
			}
			return juntar(itens), nil
		},
		paiExcluido: func(context.Context, *sqlccliente.Queries, int64) (string, error) { return "", nil },
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListCiclosExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"lei": {
		Chave: "lei", Rotulo: "Lei municipal", Plural: "Leis municipais", Genero: "a",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteLei(ctx, sqlccliente.SoftDeleteLeiParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreLei(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteLei(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			l, err := q.GetLeiQualquer(ctx, id)
			return "Lei " + l.Numero + "/" + strconv.Itoa(int(l.Ano)), l.DeletedAt.Valid, err
		},
		filhosAtivos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarTrechosDaLei(ctx, id)
			if err != nil || n.Ativos == 0 {
				return nil, err
			}
			return []string{plural(n.Ativos, "trecho ativo", "trechos ativos")}, nil
		},
		vinculos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarTrechosDaLei(ctx, id)
			if err != nil || n.Total == 0 {
				return nil, err
			}
			return []string{plural(n.Total, "trecho", "trechos") + " (contando os excluídos)"}, nil
		},
		paiExcluido: func(context.Context, *sqlccliente.Queries, int64) (string, error) { return "", nil },
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListLeisExcluidas(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"lei_trecho": {
		Chave: "lei_trecho", Rotulo: "Trecho de lei", Plural: "Trechos de leis", Genero: "o",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteTrecho(ctx, sqlccliente.SoftDeleteTrechoParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreTrecho(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteTrecho(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			t, err := q.GetTrechoQualquer(ctx, id)
			nome := t.Artigo
			if nome == "" {
				nome = "trecho " + strconv.FormatInt(t.ID, 10)
			}
			return nome, t.DeletedAt.Valid, err
		},
		filhosAtivos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			c, err := q.CountNotificacoesAtivasDoTrecho(ctx, pgtype.Int8{Int64: id, Valid: true})
			if err != nil || c == 0 {
				return nil, err
			}
			return []string{plural(c, "notificação ativa", "notificações ativas")}, nil
		},
		vinculos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			var itens []string
			if c, err := q.CountNotificacoesDoTrecho(ctx, pgtype.Int8{Int64: id, Valid: true}); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "notificação", "notificações")+" (contando as excluídas)")
			}
			if c, err := q.CountAutosDoTrecho(ctx, pgtype.Int8{Int64: id, Valid: true}); err != nil {
				return nil, err
			} else if c > 0 {
				itens = append(itens, plural(c, "auto de infração", "autos de infração")+" (contando os excluídos)")
			}
			return itens, nil
		},
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			t, err := q.GetTrechoQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			if l, err := q.GetLeiQualquer(ctx, t.LeiID); err == nil && l.DeletedAt.Valid {
				return "a lei " + l.Numero + "/" + strconv.Itoa(int(l.Ano)), nil
			}
			return "", nil
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListTrechosExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"investigacao": {
		Chave: "investigacao", Rotulo: "Investigação", Plural: "Investigações", Genero: "a",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteInvestigacao(ctx, sqlccliente.SoftDeleteInvestigacaoParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreInvestigacao(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			// o andamento (eventos e anexos) é só da investigação e sai junto
			if err := q.ForceDeleteCasoEventos(ctx, sqlccliente.ForceDeleteCasoEventosParams{Tipo: "investigacao", CasoID: id}); err != nil {
				return 0, err
			}
			if err := q.ForceDeleteCasoAnexos(ctx, sqlccliente.ForceDeleteCasoAnexosParams{Tipo: "investigacao", CasoID: id}); err != nil {
				return 0, err
			}
			return q.ForceDeleteInvestigacao(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			i, err := q.GetInvestigacaoQualquer(ctx, id)
			return "Investigação " + strconv.FormatInt(i.ID, 10) + " — " + i.Paciente, i.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			i, err := q.GetInvestigacaoQualquer(ctx, id)
			if err != nil || !i.LocalID.Valid {
				return "", err
			}
			l, err := q.GetLocalQualquer(ctx, i.LocalID.Int64)
			if err == nil && l.DeletedAt.Valid {
				return "o local " + l.Endereco, nil
			}
			return "", err
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListInvestigacoesExcluidas(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: "Investigação " + strconv.FormatInt(r.ID, 10) + " — " + r.Paciente, Contexto: r.DoencaNome, Quando: r.DeletedAt.Time, Por: r.Por}
			}
			return out, err
		},
	},
	"denuncia": {
		Chave: "denuncia", Rotulo: "Denúncia", Plural: "Denúncias", Genero: "a",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteDenuncia(ctx, sqlccliente.SoftDeleteDenunciaParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreDenuncia(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			// o andamento (eventos e anexos) é só da denúncia e sai junto
			if err := q.ForceDeleteCasoEventos(ctx, sqlccliente.ForceDeleteCasoEventosParams{Tipo: "denuncia", CasoID: id}); err != nil {
				return 0, err
			}
			if err := q.ForceDeleteCasoAnexos(ctx, sqlccliente.ForceDeleteCasoAnexosParams{Tipo: "denuncia", CasoID: id}); err != nil {
				return 0, err
			}
			return q.ForceDeleteDenuncia(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			d, err := q.GetDenunciaQualquer(ctx, id)
			return "Denúncia " + strconv.FormatInt(d.ID, 10) + " — " + d.Titulo, d.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			d, err := q.GetDenunciaQualquer(ctx, id)
			if err != nil || !d.LocalID.Valid {
				return "", err
			}
			l, err := q.GetLocalQualquer(ctx, d.LocalID.Int64)
			if err == nil && l.DeletedAt.Valid {
				return "o local " + l.Endereco, nil
			}
			return "", err
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListDenunciasExcluidas(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: "Denúncia " + strconv.FormatInt(r.ID, 10) + " — " + r.Titulo, Contexto: r.Tipo, Quando: r.DeletedAt.Time, Por: r.Por}
			}
			return out, err
		},
	},
	"ovitrampa": entradaDeArmadilha(tiposDeArmadilha["ovitrampa"], "a"),
	"edl":       entradaDeArmadilha(tiposDeArmadilha["edl"], "a"),
	"baap": {
		Chave: "baap", Rotulo: "Animal peçonhento", Plural: "Animais peçonhentos", Genero: "a",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteBaap(ctx, sqlccliente.SoftDeleteBaapParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreBaap(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			// o andamento (eventos e anexos) é só da notificação e sai junto
			if err := q.ForceDeleteCasoEventos(ctx, sqlccliente.ForceDeleteCasoEventosParams{Tipo: "baap", CasoID: id}); err != nil {
				return 0, err
			}
			if err := q.ForceDeleteCasoAnexos(ctx, sqlccliente.ForceDeleteCasoAnexosParams{Tipo: "baap", CasoID: id}); err != nil {
				return 0, err
			}
			return q.ForceDeleteBaap(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			b, err := q.GetBaapQualquer(ctx, id)
			return "Animal peçonhento " + strconv.FormatInt(b.ID, 10) + " — " + b.Paciente, b.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			b, err := q.GetBaapQualquer(ctx, id)
			if err != nil || !b.LocalID.Valid {
				return "", err
			}
			l, err := q.GetLocalQualquer(ctx, b.LocalID.Int64)
			if err == nil && l.DeletedAt.Valid {
				return "o local " + l.Endereco, nil
			}
			return "", err
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListBaapExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: "Animal peçonhento " + strconv.FormatInt(r.ID, 10) + " — " + r.Paciente, Contexto: r.Animal, Quando: r.DeletedAt.Time, Por: r.Por}
			}
			return out, err
		},
	},
	"campanha": {
		Chave: "campanha", Rotulo: "Campanha antirrábica", Plural: "Campanhas antirrábicas", Genero: "a",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteCampanha(ctx, sqlccliente.SoftDeleteCampanhaParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreCampanha(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteCampanha(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			c, err := q.GetCampanhaQualquer(ctx, id)
			return "Campanha " + strconv.FormatInt(c.ID, 10), c.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			c, err := q.GetCampanhaQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			l, err := q.GetLocalQualquer(ctx, c.LocalID)
			if err == nil && l.DeletedAt.Valid {
				return "o local " + l.Endereco, nil
			}
			return "", err
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListCampanhasExcluidas(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: "Campanha " + strconv.FormatInt(r.ID, 10), Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.Por}
			}
			return out, err
		},
	},
	"drone": {
		Chave: "drone", Rotulo: "Solicitação de drone", Plural: "Solicitações de drone", Genero: "a",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteDrone(ctx, sqlccliente.SoftDeleteDroneParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreDrone(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			// o andamento (eventos e anexos) é só da solicitação e sai junto
			if err := q.ForceDeleteCasoEventos(ctx, sqlccliente.ForceDeleteCasoEventosParams{Tipo: "drone", CasoID: id}); err != nil {
				return 0, err
			}
			if err := q.ForceDeleteCasoAnexos(ctx, sqlccliente.ForceDeleteCasoAnexosParams{Tipo: "drone", CasoID: id}); err != nil {
				return 0, err
			}
			return q.ForceDeleteDrone(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			d, err := q.GetDroneQualquer(ctx, id)
			return "Solicitação de drone " + strconv.FormatInt(d.ID, 10), d.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			d, err := q.GetDroneQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			l, err := q.GetLocalQualquer(ctx, d.LocalID)
			if err == nil && l.DeletedAt.Valid {
				return "o local " + l.Endereco, nil
			}
			return "", err
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListDronesExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: "Solicitação de drone " + strconv.FormatInt(r.ID, 10), Contexto: r.Descricao, Quando: r.DeletedAt.Time, Por: r.Por}
			}
			return out, err
		},
	},
	"notificacao": {
		Chave: "notificacao", Rotulo: "Notificação", Plural: "Notificações", Genero: "a",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteNotificacao(ctx, sqlccliente.SoftDeleteNotificacaoParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreNotificacao(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			if err := q.ForceDeleteCasoEventos(ctx, sqlccliente.ForceDeleteCasoEventosParams{Tipo: "notificacao", CasoID: id}); err != nil {
				return 0, err
			}
			if err := q.ForceDeleteCasoAnexos(ctx, sqlccliente.ForceDeleteCasoAnexosParams{Tipo: "notificacao", CasoID: id}); err != nil {
				return 0, err
			}
			return q.ForceDeleteNotificacao(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			n, err := q.GetNotificacaoQualquer(ctx, id)
			return "Notificação " + n.Numero, n.DeletedAt.Valid, err
		},
		filhosAtivos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			c, err := q.CountAutosVivosDaNotificacao(ctx, id)
			if err != nil || c == 0 {
				return nil, err
			}
			return []string{plural(c, "auto de infração (exclua antes os autos cancelados)", "autos de infração (exclua antes os autos cancelados)")}, nil
		},
		vinculos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			c, err := q.CountAutosDaNotificacaoQualquer(ctx, id)
			if err != nil || c == 0 {
				return nil, err
			}
			return []string{plural(c, "auto de infração", "autos de infração") + " (contando os excluídos)"}, nil
		},
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			n, err := q.GetNotificacaoQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			l, err := q.GetLocalQualquer(ctx, n.LocalID)
			if err == nil && l.DeletedAt.Valid {
				return "o local " + l.Endereco, nil
			}
			return "", err
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListNotificacoesExcluidas(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: "Notificação " + r.Numero, Contexto: r.Motivo, Quando: r.DeletedAt.Time, Por: r.Por}
			}
			return out, err
		},
	},
	"infracao": {
		Chave: "infracao", Rotulo: "Auto de infração", Plural: "Autos de infração", Genero: "o",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteAuto(ctx, sqlccliente.SoftDeleteAutoParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreAuto(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			if err := q.ForceDeleteCasoEventos(ctx, sqlccliente.ForceDeleteCasoEventosParams{Tipo: "infracao", CasoID: id}); err != nil {
				return 0, err
			}
			if err := q.ForceDeleteCasoAnexos(ctx, sqlccliente.ForceDeleteCasoAnexosParams{Tipo: "infracao", CasoID: id}); err != nil {
				return 0, err
			}
			return q.ForceDeleteAuto(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			a, err := q.GetAutoQualquer(ctx, id)
			return "Auto de infração " + a.Numero, a.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			a, err := q.GetAutoQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			n, err := q.GetNotificacaoQualquer(ctx, a.NotificacaoID)
			if err == nil && n.DeletedAt.Valid {
				return "a notificação " + n.Numero, nil
			}
			return "", err
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListAutosExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: "Auto de infração " + r.Numero, Quando: r.DeletedAt.Time, Por: r.Por}
			}
			return out, err
		},
	},
	"visita": {
		Chave: "visita", Rotulo: "Visita", Plural: "Visitas", Genero: "a",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteVisita(ctx, sqlccliente.SoftDeleteVisitaParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreVisita(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteVisita(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			v, err := q.GetVisitaQualquer(ctx, id)
			return "Visita " + strconv.FormatInt(v.ID, 10), v.DeletedAt.Valid, err
		},
		// Notificações e demais registros da visita entram aqui quando as suas etapas existirem.
		filhosAtivos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarAmostrasDaVisita(ctx, id)
			if err != nil || n.Ativas == 0 {
				return nil, err
			}
			return []string{plural(n.Ativas, "amostra (tubito)", "amostras (tubitos)") + " ainda ativa(s)"}, nil
		},
		vinculos: func(ctx context.Context, q *sqlccliente.Queries, id int64) ([]string, error) {
			n, err := q.ContarAmostrasDaVisita(ctx, id)
			if err != nil || n.Total == 0 {
				return nil, err
			}
			return []string{plural(n.Total, "amostra", "amostras") + " (contando as excluídas)"}, nil
		},
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			v, err := q.GetVisitaQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			if l, err := q.GetLocalQualquer(ctx, v.LocalID); err == nil && l.DeletedAt.Valid {
				return "o local " + enderecoCompleto(l.Endereco, l.Numero), nil
			}
			if c, err := q.GetCicloQualquer(ctx, v.CicloID); err == nil && c.DeletedAt.Valid {
				return "o ciclo " + c.Nome, nil
			}
			return "", nil
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListVisitasExcluidas(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"amostra": {
		Chave: "amostra", Rotulo: "Amostra (tubito)", Plural: "Amostras (tubitos)", Genero: "a",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteAmostra(ctx, sqlccliente.SoftDeleteAmostraParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreAmostra(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteAmostra(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			a, err := q.GetAmostraQualquer(ctx, id)
			return "Tubito " + a.NumeroTubito, a.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			a, err := q.GetAmostraQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			if v, err := q.GetVisitaQualquer(ctx, a.VisitaID); err == nil && v.DeletedAt.Valid {
				return "a visita " + strconv.FormatInt(v.ID, 10), nil
			}
			return "", nil
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListAmostrasExcluidas(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"grupo": {
		Chave: "grupo", Rotulo: "Grupo de locais", Plural: "Grupos de locais", Genero: "o",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteGrupo(ctx, sqlccliente.SoftDeleteGrupoParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreGrupo(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteGrupo(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			g, err := q.GetGrupoQualquer(ctx, id)
			return g.Nome, g.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		// os itens do grupo são só a lista de membros do próprio grupo: saem com ele
		vinculos:    func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido: func(context.Context, *sqlccliente.Queries, int64) (string, error) { return "", nil },
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListGruposExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"reconhecimento": {
		Chave: "reconhecimento", Rotulo: "Reconhecimento geográfico", Plural: "Reconhecimentos geográficos", Genero: "o",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteReconhecimento(ctx, sqlccliente.SoftDeleteReconhecimentoParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreReconhecimento(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteReconhecimento(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			x, err := q.GetReconhecimentoQualquer(ctx, id)
			return "Reconhecimento " + x.BairroNome + " — " + x.CicloNome, x.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido:  func(context.Context, *sqlccliente.Queries, int64) (string, error) { return "", nil },
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListReconhecimentosExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
	"resumosemanal": {
		Chave: "resumosemanal", Rotulo: "Resumo semanal", Plural: "Resumos semanais", Genero: "o",
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteResumoSemanal(ctx, sqlccliente.SoftDeleteResumoSemanalParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreResumoSemanal(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.ForceDeleteResumoSemanal(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			x, err := q.GetResumoSemanalQualquer(ctx, id)
			return "Resumo semanal — semana " + strconv.Itoa(int(x.Semana)) + " (" + x.CicloNome + ")", x.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido:  func(context.Context, *sqlccliente.Queries, int64) (string, error) { return "", nil },
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListResumosSemanaisExcluidos(ctx)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: r.Rotulo, Contexto: r.Contexto, Quando: r.DeletedAt.Time, Por: r.ExcluidoPor}
			}
			return out, err
		},
	},
}

// ordemDaLixeira é a ordem das abas.
var ordemDaLixeira = []string{"local", "quarteirao", "bairro", "cidade", "ciclo", "lei", "lei_trecho", "visita", "investigacao", "denuncia", "campanha", "drone", "baap", "ovitrampa", "edl", "notificacao", "infracao", "amostra", "reconhecimento", "resumosemanal", "grupo", "mapeamento"}

// excluirLogicamente aplica a regra completa: recusa se há filhos ativos; senão marca deleted_at.
// Devolve os motivos da recusa (vazio = excluído), o nome do registro e o erro de banco, se houve.
func excluirLogicamente(ctx context.Context, q *sqlccliente.Queries, t *tipoExcluivel, id int64, por pgtype.Int8) (motivos []string, nome string, err error) {
	nome, excluido, err := t.rotulo(ctx, q, id)
	if err != nil {
		return nil, "", err
	}
	if excluido {
		return nil, nome, nil
	}
	filhos, err := t.filhosAtivos(ctx, q, id)
	if err != nil {
		return nil, nome, err
	}
	if len(filhos) > 0 {
		return []string{fmt.Sprintf("%s tem %s. Exclua ou mova esses registros antes.", artigo(t, nome), strings.Join(filhos, ", "))}, nome, nil
	}
	if _, err := t.soft(ctx, q, id, por); err != nil {
		return nil, nome, err
	}
	return nil, nome, nil
}

func artigo(t *tipoExcluivel, nome string) string {
	return strings.ToUpper(t.Genero[:1]) + t.Genero[1:] + " " + strings.ToLower(t.Rotulo) + " " + nome
}

func porDoRequest(r *http.Request) pgtype.Int8 {
	id := meuID(r)
	return pgtype.Int8{Int64: id, Valid: id != 0}
}

// avisoDeExclusao mostra, em uma página própria, por que a ação foi recusada.
func (h *handlers) avisoDeExclusao(w http.ResponseWriter, r *http.Request, titulo string, motivos []string, voltar string) {
	renderStatus(w, r, http.StatusConflict, views.AvisoDeExclusao(h.layout(r, titulo, ""), views.AvisoDeExclusaoData{Titulo: titulo, Motivos: motivos, Voltar: voltar}))
}

// excluirRegistro é o corpo comum das ações "Excluir" (lógica) de cada tela. A tela confere a permissão e o escopo
// antes de chamar; destino é para onde ir depois de excluir, e voltar, de onde a pessoa veio se a ação for recusada.
func (h *handlers) excluirRegistro(w http.ResponseWriter, r *http.Request, chave string, id int64, destino, voltar string) {
	t := tiposExcluiveis[chave]
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	motivos, nome, err := excluirLogicamente(r.Context(), q, t, id, porDoRequest(r))
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "excluir "+chave, err)
		return
	}
	if len(motivos) > 0 {
		naoAuditar(r)
		h.avisoDeExclusao(w, r, "Não foi possível excluir", motivos, voltar)
		return
	}
	h.auditar(r, audit.Evento{
		Acao: chave + ".excluir", Entidade: chave, EntidadeID: strconv.FormatInt(id, 10), Rotulo: nome,
		Detalhes: map[string]any{"para_a_lixeira": true},
	})
	http.Redirect(w, r, destino, http.StatusSeeOther)
}

// ------------------------------------------------------------------- Lixeira

func (h *handlers) lixeiraPagina(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	atual := r.URL.Query().Get("tipo")
	if _, ok := tiposExcluiveis[atual]; !ok {
		atual = ordemDaLixeira[0]
	}
	d := views.LixeiraData{Atual: atual, CSRF: csrfToken(r.Context()), PodeDefinitivo: h.principal(r).Pode("conf_exclusao_definitiva")}
	for _, chave := range ordemDaLixeira {
		t := tiposExcluiveis[chave]
		linhas, err := t.lixeira(r.Context(), q)
		if err != nil {
			h.falhaInterna(w, r, "listar a lixeira", err)
			return
		}
		d.Abas = append(d.Abas, views.LixeiraAba{Chave: chave, Rotulo: t.Plural, Total: len(linhas), Ativa: chave == atual})
		if chave == atual {
			d.Linhas = linhas
			d.Rotulo = t.Rotulo
		}
	}
	loc := h.fuso(r)
	for i := range d.Linhas {
		d.Linhas[i].QuandoTexto = formatarData(loc, d.Linhas[i].Quando)
	}
	d.Aviso = map[string]string{"restaurado": "Registro restaurado.", "definitivo": "Registro excluído definitivamente."}[r.URL.Query().Get("ok")]
	lay := h.layout(r, "Lixeira", "/lixeira")
	d.Linhas = paginar(r, &lay, d.Linhas)
	render(w, r, views.Lixeira(lay, d))
}

func (h *handlers) registroDaLixeira(w http.ResponseWriter, r *http.Request) (*tipoExcluivel, int64, string, bool, *sqlccliente.Queries, bool) {
	t, ok := tiposExcluiveis[chi.URLParam(r, "tipo")]
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if !ok || err != nil {
		http.NotFound(w, r)
		return nil, 0, "", false, nil, false
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return nil, 0, "", false, nil, false
	}
	nome, excluido, err := t.rotulo(r.Context(), q, id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return nil, 0, "", false, nil, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar registro", err)
		return nil, 0, "", false, nil, false
	}
	return t, id, nome, excluido, q, true
}

func (h *handlers) lixeiraRestaurar(w http.ResponseWriter, r *http.Request) {
	t, id, nome, excluido, q, ok := h.registroDaLixeira(w, r)
	if !ok {
		return
	}
	voltar := "/lixeira?tipo=" + t.Chave
	if !excluido {
		naoAuditar(r)
		http.Redirect(w, r, voltar, http.StatusSeeOther)
		return
	}
	if pai, err := t.paiExcluido(r.Context(), q, id); err != nil {
		h.falhaInterna(w, r, "conferir o pai", err)
		return
	} else if pai != "" {
		naoAuditar(r)
		h.avisoDeExclusao(w, r, "Não foi possível restaurar", []string{fmt.Sprintf("%s está na lixeira. Restaure %s primeiro.", strings.ToUpper(pai[:1])+pai[1:], pai)}, voltar)
		return
	}
	if _, err := t.restore(r.Context(), q, id); violacaoUnica(err) {
		naoAuditar(r)
		h.avisoDeExclusao(w, r, "Não foi possível restaurar", []string{"Já existe outro registro ativo com o mesmo nome ou identificação. Renomeie ou exclua o outro antes de restaurar este."}, voltar)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "restaurar", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: t.Chave + ".restaurar", Entidade: t.Chave, EntidadeID: strconv.FormatInt(id, 10), Rotulo: nome})
	http.Redirect(w, r, voltar+"&ok=restaurado", http.StatusSeeOther)
}

func (h *handlers) lixeiraExcluirDefinitivo(w http.ResponseWriter, r *http.Request) {
	t, id, nome, excluido, q, ok := h.registroDaLixeira(w, r)
	if !ok {
		return
	}
	voltar := "/lixeira?tipo=" + t.Chave
	if !excluido {
		naoAuditar(r)
		h.avisoDeExclusao(w, r, "Não foi possível excluir definitivamente", []string{"Só dá para excluir definitivamente o que já está na lixeira."}, voltar)
		return
	}
	vinc, err := t.vinculos(r.Context(), q, id)
	if err != nil {
		h.falhaInterna(w, r, "conferir vínculos", err)
		return
	}
	if len(vinc) > 0 {
		naoAuditar(r)
		h.avisoDeExclusao(w, r, "Não foi possível excluir definitivamente", []string{
			fmt.Sprintf("%s tem vínculo com outros registros: %s.", artigo(t, nome), strings.Join(vinc, ", ")),
			"A exclusão definitiva só é permitida quando nada mais depende do registro.",
		}, voltar)
		return
	}
	if _, err := t.force(r.Context(), q, id); err != nil {
		naoAuditar(r)
		var pe interface{ SQLState() string }
		if errors.As(err, &pe) && pe.SQLState() == "23503" {
			h.avisoDeExclusao(w, r, "Não foi possível excluir definitivamente", []string{"O registro ainda tem vínculo com outros dados do sistema."}, voltar)
			return
		}
		h.falhaInterna(w, r, "excluir definitivamente", err)
		return
	}
	h.auditar(r, audit.Evento{
		Acao: t.Chave + ".excluir_definitivo", Entidade: t.Chave, EntidadeID: strconv.FormatInt(id, 10), Rotulo: nome,
		Detalhes: map[string]any{"definitivo": true},
	})
	http.Redirect(w, r, voltar+"&ok=definitivo", http.StatusSeeOther)
}

// entradaDeArmadilha monta a entrada da Lixeira de um tipo de armadilha (ovitrampa ou EDL).
func entradaDeArmadilha(t armadilhaTipo, genero string) *tipoExcluivel {
	return &tipoExcluivel{
		Chave: t.Tipo, Rotulo: t.Rotulo, Plural: t.Plural, Genero: genero,
		soft: func(ctx context.Context, q *sqlccliente.Queries, id int64, por pgtype.Int8) (int64, error) {
			return q.SoftDeleteArmadilha(ctx, sqlccliente.SoftDeleteArmadilhaParams{ID: id, Por: por})
		},
		restore: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			return q.RestoreArmadilha(ctx, id)
		},
		force: func(ctx context.Context, q *sqlccliente.Queries, id int64) (int64, error) {
			// as contagens/manutenções são só da armadilha e saem junto
			if err := q.ForceDeleteArmadilhaRegistros(ctx, id); err != nil {
				return 0, err
			}
			return q.ForceDeleteArmadilha(ctx, id)
		},
		rotulo: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, bool, error) {
			a, err := q.GetArmadilhaQualquer(ctx, id)
			return t.Rotulo + " " + strconv.FormatInt(a.ID, 10), a.DeletedAt.Valid, err
		},
		filhosAtivos: func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		vinculos:     func(context.Context, *sqlccliente.Queries, int64) ([]string, error) { return nil, nil },
		paiExcluido: func(ctx context.Context, q *sqlccliente.Queries, id int64) (string, error) {
			a, err := q.GetArmadilhaQualquer(ctx, id)
			if err != nil {
				return "", err
			}
			l, err := q.GetLocalQualquer(ctx, a.LocalID)
			if err == nil && l.DeletedAt.Valid {
				return "o local " + l.Endereco, nil
			}
			return "", err
		},
		lixeira: func(ctx context.Context, q *sqlccliente.Queries) ([]views.LixeiraLinha, error) {
			rows, err := q.ListArmadilhasExcluidas(ctx, t.Tipo)
			out := make([]views.LixeiraLinha, len(rows))
			for i, r := range rows {
				out[i] = views.LixeiraLinha{ID: r.ID, Rotulo: t.Rotulo + " " + strconv.FormatInt(r.ID, 10), Contexto: r.Endereco, Quando: r.DeletedAt.Time, Por: r.Por}
			}
			return out, err
		},
	}
}

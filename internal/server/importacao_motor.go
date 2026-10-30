package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
)

const tamanhoDoLote = 100

// motorImp acompanha as importações que estão rodando neste servidor (o estado verdadeiro fica no banco).
type motorImp struct {
	mu      sync.Mutex
	rodando map[string]context.CancelFunc
}

func chaveImp(schema string, id int64) string { return schema + ":" + strconv.FormatInt(id, 10) }

func (m *motorImp) registrar(chave string, cancel context.CancelFunc) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rodando == nil {
		m.rodando = map[string]context.CancelFunc{}
	}
	if _, ja := m.rodando[chave]; ja {
		return false
	}
	m.rodando[chave] = cancel
	return true
}

func (m *motorImp) liberar(chave string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rodando, chave)
}

func (m *motorImp) esta(chave string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.rodando[chave]
	return ok
}

func (m *motorImp) interromper(chave string) bool {
	m.mu.Lock()
	cancel, ok := m.rodando[chave]
	m.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// origemDaImportacao leva para a execução em segundo plano quem iniciou e de onde (para a auditoria).
type origemDaImportacao struct {
	Ator      audit.Ator
	IP        string
	UserAgent string
	RequestID string
	UsuarioID int64
}

// iniciarExecucao dispara a execução em segundo plano. Devolve false se já havia uma rodando.
func (h *handlers) iniciarExecucao(pool *pgxpool.Pool, schema string, id int64, o origemDaImportacao) bool {
	ctx, cancel := context.WithCancel(context.Background())
	chave := chaveImp(schema, id)
	if !h.imp.registrar(chave, cancel) {
		cancel()
		return false
	}
	go func() {
		defer h.imp.liberar(chave)
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				h.log.Error("importação: pânico", "id", id, "erro", r)
				_ = sqlccliente.New(pool).ConcluirImportacao(context.Background(), sqlccliente.ConcluirImportacaoParams{ID: id, Status: "erro", Mensagem: "erro interno durante a importação"})
			}
		}()
		h.executarImportacao(ctx, pool, id, o)
	}()
	return true
}

// resultadoDaLinha é o que aconteceu ao aplicar uma linha.
type resultadoDaLinha struct {
	Resultado  string // criado, atualizado, ignorado, erro
	Msg        string
	RegistroID int64
}

func (h *handlers) executarImportacao(ctx context.Context, pool *pgxpool.Pool, id int64, o origemDaImportacao) {
	q := sqlccliente.New(pool)
	fim := context.Background()
	imp, err := q.GetImportacao(fim, id)
	if err != nil {
		h.log.Error("importação: buscar", "id", id, "erro", err)
		return
	}
	// linhas com erro de análise não são gravadas: ficam registradas como erro no resultado
	if _, err := q.MarcarLinhasComErroDaImportacao(fim, id); err != nil {
		h.log.Error("importação: marcar erros", "id", id, "erro", err)
	}
	for ctx.Err() == nil {
		pend, err := q.ListImportacaoLinhasPendentes(fim, sqlccliente.ListImportacaoLinhasPendentesParams{ImportacaoID: id, Limit: tamanhoDoLote})
		if err != nil {
			h.falharImportacao(q, id, "falha ao ler as linhas: "+err.Error())
			return
		}
		if len(pend) == 0 {
			break
		}
		if err := h.aplicarLote(ctx, pool, imp, pend, o); err != nil {
			if ctx.Err() != nil {
				break
			}
			h.falharImportacao(q, id, "falha ao gravar: "+err.Error())
			return
		}
		h.atualizarContadores(fim, q, id)
	}
	h.atualizarContadores(fim, q, id)
	atual, _ := q.GetImportacao(fim, id)
	if ctx.Err() != nil {
		_ = q.SetImportacaoStatus(fim, sqlccliente.SetImportacaoStatusParams{ID: id, Status: "interrompida", Mensagem: "Interrompida: use Retomar para continuar de onde parou."})
		return
	}
	msg := fmt.Sprintf("%d criado(s), %d atualizado(s), %d ignorado(s), %d com erro.", atual.Criados, atual.Atualizados, atual.Ignorados, atual.Erros)
	_ = q.ConcluirImportacao(fim, sqlccliente.ConcluirImportacaoParams{ID: id, Status: "concluida", Mensagem: msg})
	reg := audit.Cliente{Q: q}
	if err := reg.Registrar(fim, audit.Evento{
		Ator: o.Ator, IP: o.IP, UserAgent: o.UserAgent, RequestID: o.RequestID,
		Acao: "importacao.concluir", Entidade: "importacao", EntidadeID: strconv.FormatInt(id, 10), Rotulo: imp.ArquivoNome,
		Detalhes: map[string]any{"tipo": imp.Tipo, "criados": atual.Criados, "atualizados": atual.Atualizados, "ignorados": atual.Ignorados, "erros": atual.Erros},
	}); err != nil {
		h.log.Error("ERRO AO GRAVAR AUDITORIA", "acao", "importacao.concluir", "erro", err)
	}
}

func (h *handlers) falharImportacao(q *sqlccliente.Queries, id int64, msg string) {
	h.log.Error("importação falhou", "id", id, "motivo", msg)
	_ = q.ConcluirImportacao(context.Background(), sqlccliente.ConcluirImportacaoParams{ID: id, Status: "erro", Mensagem: msg})
}

// atualizarContadores recalcula o andamento a partir das linhas (fonte única da verdade).
func (h *handlers) atualizarContadores(ctx context.Context, q *sqlccliente.Queries, id int64) {
	rows, err := q.ResumoImportacaoLinhas(ctx, id)
	if err != nil {
		return
	}
	var criados, atualizados, ignorados, erros int32
	for _, r := range rows {
		switch r.Resultado {
		case "criado":
			criados = r.Total
		case "atualizado":
			atualizados = r.Total
		case "ignorado":
			ignorados = r.Total
		case "erro":
			erros = r.Total
		}
	}
	_ = q.AtualizarProgressoImportacao(ctx, sqlccliente.AtualizarProgressoImportacaoParams{
		ID: id, Processadas: criados + atualizados + ignorados + erros, Criados: criados, Atualizados: atualizados, Ignorados: ignorados, Erros: erros,
	})
}

// aplicarLote grava um lote de linhas numa transação, cada linha com o seu savepoint: uma linha que falha
// vira "erro" sem derrubar as outras.
func (h *handlers) aplicarLote(ctx context.Context, pool *pgxpool.Pool, imp sqlccliente.Importaco, linhas []sqlccliente.ImportacaoLinha, o origemDaImportacao) error {
	tx, err := pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	qt := sqlccliente.New(tx)
	for _, ln := range linhas {
		if ctx.Err() != nil {
			break // o que já foi aplicado neste lote é confirmado abaixo
		}
		res := resultadoDaLinha{Resultado: "ignorado", Msg: ln.Mensagem}
		if ln.Acao != acaoIgnorar {
			sp, err := tx.Begin(context.Background())
			if err != nil {
				return err
			}
			res = aplicarLinha(context.Background(), sqlccliente.New(sp), imp.Tipo, ln, o.UsuarioID)
			if res.Resultado == "erro" {
				_ = sp.Rollback(context.Background())
			} else if err := sp.Commit(context.Background()); err != nil {
				return err
			}
		}
		if err := qt.SetImportacaoLinhaResultado(context.Background(), sqlccliente.SetImportacaoLinhaResultadoParams{
			ImportacaoID: imp.ID, Linha: ln.Linha, Resultado: pgtype.Text{String: res.Resultado, Valid: true}, ResultadoMsg: res.Msg,
			RegistroID: pgtype.Int8{Int64: res.RegistroID, Valid: res.RegistroID != 0},
		}); err != nil {
			return err
		}
	}
	return tx.Commit(context.Background())
}

func textoNulo(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func textoDoErro(err error, conflito string) string {
	if violacaoUnica(err) {
		return conflito
	}
	return "não foi possível gravar (" + err.Error() + ")"
}

// aplicarLinha grava uma linha analisada.
func aplicarLinha(ctx context.Context, q *sqlccliente.Queries, tipo string, ln sqlccliente.ImportacaoLinha, usuarioID int64) resultadoDaLinha {
	erro := func(msg string) resultadoDaLinha { return resultadoDaLinha{Resultado: "erro", Msg: msg} }
	switch tipo {
	case "ciclos":
		return aplicarCiclo(ctx, q, ln)
	case "visitas":
		return aplicarVisita(ctx, q, ln, usuarioID)
	case "amostras":
		return aplicarAmostra(ctx, q, ln, usuarioID)
	case "bairros":
		var c cargaBairro
		if err := json.Unmarshal(ln.Dados, &c); err != nil {
			return erro("dados da linha ilegíveis")
		}
		if ln.Acao == acaoCriar {
			b, err := q.CreateBairro(ctx, c.CreateBairroParams)
			if err != nil {
				return erro(textoDoErro(err, "já existe um bairro com esse nome nessa cidade"))
			}
			if c.IDExterno != "" {
				if _, err := q.SetBairroIdExterno(ctx, sqlccliente.SetBairroIdExternoParams{ID: b.ID, IDExterno: textoNulo(c.IDExterno)}); err != nil {
					return erro(textoDoErro(err, "o id_externo já pertence a outro bairro"))
				}
			}
			return resultadoDaLinha{Resultado: "criado", RegistroID: b.ID}
		}
		p := c.CreateBairroParams
		if _, err := q.UpdateBairro(ctx, sqlccliente.UpdateBairroParams{
			ID: ln.AlvoID.Int64, CidadeID: p.CidadeID, Nome: p.Nome, Mutirao: p.Mutirao, Status: p.Status, Ibge: p.Ibge, Classificacao: p.Classificacao,
			Zona: p.Zona, Categoria: p.Categoria, Latitude: p.Latitude, Longitude: p.Longitude, Altitude: p.Altitude, DistanciaCentro: p.DistanciaCentro, Producao: p.Producao,
		}); err != nil {
			return erro(textoDoErro(err, "já existe um bairro com esse nome nessa cidade"))
		}
		if c.IDExterno != "" {
			if atual, err := q.GetBairro(ctx, ln.AlvoID.Int64); err == nil && !atual.IDExterno.Valid {
				if _, err := q.SetBairroIdExterno(ctx, sqlccliente.SetBairroIdExternoParams{ID: atual.ID, IDExterno: textoNulo(c.IDExterno)}); err != nil {
					return erro(textoDoErro(err, "o id_externo já pertence a outro bairro"))
				}
			}
		}
		return resultadoDaLinha{Resultado: "atualizado", RegistroID: ln.AlvoID.Int64}

	case "quarteiroes":
		var c cargaQuarteirao
		if err := json.Unmarshal(ln.Dados, &c); err != nil {
			return erro("dados da linha ilegíveis")
		}
		if ln.Acao == acaoCriar {
			x, err := q.CreateQuarteiraoApi(ctx, sqlccliente.CreateQuarteiraoApiParams{BairroID: c.BairroID, Nome: c.Nome, Ativo: c.Ativo, IDExterno: textoNulo(c.IDExterno)})
			if err != nil {
				return erro(textoDoErro(err, "já existe um quarteirão com esse nome (ou esse id_externo) neste bairro"))
			}
			return resultadoDaLinha{Resultado: "criado", RegistroID: x.ID}
		}
		if _, err := q.UpdateQuarteirao(ctx, sqlccliente.UpdateQuarteiraoParams{ID: ln.AlvoID.Int64, Nome: c.Nome, Ativo: c.Ativo}); err != nil {
			return erro(textoDoErro(err, "já existe um quarteirão com esse nome neste bairro"))
		}
		return resultadoDaLinha{Resultado: "atualizado", RegistroID: ln.AlvoID.Int64}

	case "locais":
		var c cargaLocal
		if err := json.Unmarshal(ln.Dados, &c); err != nil {
			return erro("dados da linha ilegíveis")
		}
		p := c.CreateLocalParams
		if ln.Acao == acaoCriar {
			p.CriadoPor = pgtype.Int8{Int64: usuarioID, Valid: usuarioID != 0}
			l, err := q.CreateLocal(ctx, p)
			if err != nil {
				return erro(textoDoErro(err, "já existe um local com esse endereço, número, lado, sequência e complemento nesse quarteirão"))
			}
			if c.IDExterno != "" {
				if _, err := q.SetLocalIdExterno(ctx, sqlccliente.SetLocalIdExternoParams{ID: l.ID, IDExterno: textoNulo(c.IDExterno)}); err != nil {
					return erro(textoDoErro(err, "o id_externo já pertence a outro local"))
				}
			}
			if !c.Ativo {
				if _, err := q.SetLocalAtivo(ctx, sqlccliente.SetLocalAtivoParams{ID: l.ID, Ativo: false}); err != nil {
					return erro(textoDoErro(err, ""))
				}
			}
			return resultadoDaLinha{Resultado: "criado", RegistroID: l.ID}
		}
		id := ln.AlvoID.Int64
		if _, err := q.UpdateLocal(ctx, atualizacaoDeLocal(id, p)); err != nil {
			return erro(textoDoErro(err, "já existe um local com esse endereço, número, lado, sequência e complemento nesse quarteirão"))
		}
		if atual, err := q.GetLocal(ctx, id); err == nil {
			if atual.Ativo != c.Ativo {
				if _, err := q.SetLocalAtivo(ctx, sqlccliente.SetLocalAtivoParams{ID: id, Ativo: c.Ativo}); err != nil {
					return erro(textoDoErro(err, ""))
				}
			}
			if c.IDExterno != "" && !atual.IDExterno.Valid {
				if _, err := q.SetLocalIdExterno(ctx, sqlccliente.SetLocalIdExternoParams{ID: id, IDExterno: textoNulo(c.IDExterno)}); err != nil {
					return erro(textoDoErro(err, "o id_externo já pertence a outro local"))
				}
			}
		}
		return resultadoDaLinha{Resultado: "atualizado", RegistroID: id}
	}
	return erro("tipo de importação desconhecido")
}

func atualizacaoDeLocal(id int64, p sqlccliente.CreateLocalParams) sqlccliente.UpdateLocalParams {
	return sqlccliente.UpdateLocalParams{
		ID: id, Tipo: p.Tipo, Cep: p.Cep, Endereco: p.Endereco, Numero: p.Numero, Referencia: p.Referencia, BairroID: p.BairroID,
		QuarteiraoID: p.QuarteiraoID, Lado: p.Lado, Sequencia: p.Sequencia, Complemento: p.Complemento, Latitude: p.Latitude,
		Longitude: p.Longitude, Proprietario: p.Proprietario, ProprietarioTelefone: p.ProprietarioTelefone, ProprietarioCpf: p.ProprietarioCpf,
		ProprietarioWhatsapp: p.ProprietarioWhatsapp, ProprietarioEmail: p.ProprietarioEmail, AceitaWhatsapp: p.AceitaWhatsapp,
		AceitaEmail: p.AceitaEmail, Inquilino: p.Inquilino, InquilinoTelefone: p.InquilinoTelefone, Humanos: p.Humanos,
		Cachorros: p.Cachorros, Gatos: p.Gatos, Roedores: p.Roedores,
	}
}

// ------------------------------------------------------------------- Desfazer

// desfazerImportacao apaga o que a importação criou e devolve o que ela atualizou ao estado anterior.
// O que não puder ser desfeito (por exemplo, um bairro que já ganhou quarteirões) fica como está e é contado.
func desfazerImportacao(ctx context.Context, pool *pgxpool.Pool, imp sqlccliente.Importaco) (desfeitos, naoDesfeitos int, err error) {
	q := sqlccliente.New(pool)
	linhas, err := q.ListImportacaoLinhasParaDesfazer(ctx, imp.ID)
	if err != nil {
		return 0, 0, err
	}
	for _, ln := range linhas {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return desfeitos, naoDesfeitos, err
		}
		qt := sqlccliente.New(tx)
		msg := desfazerLinha(ctx, qt, imp.Tipo, ln)
		if msg != "" {
			_ = tx.Rollback(ctx)
			naoDesfeitos++
			_ = q.SetImportacaoLinhaResultado(ctx, sqlccliente.SetImportacaoLinhaResultadoParams{
				ImportacaoID: imp.ID, Linha: ln.Linha, Resultado: ln.Resultado, ResultadoMsg: "não desfeito: " + msg, RegistroID: ln.RegistroID,
			})
			continue
		}
		if err := qt.SetImportacaoLinhaResultado(ctx, sqlccliente.SetImportacaoLinhaResultadoParams{
			ImportacaoID: imp.ID, Linha: ln.Linha, Resultado: pgtype.Text{String: "desfeito", Valid: true}, ResultadoMsg: "desfeito", RegistroID: ln.RegistroID,
		}); err != nil {
			_ = tx.Rollback(ctx)
			return desfeitos, naoDesfeitos, err
		}
		if err := tx.Commit(ctx); err != nil {
			return desfeitos, naoDesfeitos, err
		}
		desfeitos++
	}
	return desfeitos, naoDesfeitos, nil
}

var errTipoDesconhecido = errors.New("tipo desconhecido")

// desfazerCriacao manda para a Lixeira o que a importação criou (exclusão lógica: nada some de verdade).
// Se o registro já ganhou filhos ativos (um bairro com locais, por exemplo), ele é mantido e o motivo é devolvido.
func desfazerCriacao(ctx context.Context, q *sqlccliente.Queries, chave string, id int64) string {
	motivos, _, err := excluirLogicamente(ctx, q, tiposExcluiveis[chave], id, pgtype.Int8{})
	if err != nil {
		return err.Error()
	}
	if len(motivos) > 0 {
		return "já está em uso: " + strings.Join(motivos, " ")
	}
	return ""
}

// desfazerLinha devolve "" se desfez, ou o motivo de não ter desfeito.
func desfazerLinha(ctx context.Context, q *sqlccliente.Queries, tipo string, ln sqlccliente.ImportacaoLinha) string {
	id := ln.RegistroID.Int64
	criado := ln.Resultado.String == "criado"
	motivo := func(err error) string {
		if err == nil {
			return ""
		}
		var pe interface{ SQLState() string }
		if errors.As(err, &pe) && pe.SQLState() == "23503" {
			return "já está em uso por outros dados"
		}
		return err.Error()
	}
	switch tipo {
	case "ciclos":
		return desfazerCiclo(ctx, q, ln, criado, id, motivo)
	case "visitas":
		return desfazerCriacao(ctx, q, "visita", id)
	case "amostras":
		return desfazerCriacao(ctx, q, "amostra", id)
	case "bairros":
		if criado {
			return desfazerCriacao(ctx, q, "bairro", id)
		}
		var b sqlccliente.Bairro
		if err := json.Unmarshal(ln.Antes, &b); err != nil {
			return "o estado anterior não foi guardado"
		}
		_, err := q.UpdateBairro(ctx, sqlccliente.UpdateBairroParams{
			ID: id, CidadeID: b.CidadeID, Nome: b.Nome, Mutirao: b.Mutirao, Status: b.Status, Ibge: b.Ibge, Classificacao: b.Classificacao, Zona: b.Zona,
			Categoria: b.Categoria, Latitude: b.Latitude, Longitude: b.Longitude, Altitude: b.Altitude, DistanciaCentro: b.DistanciaCentro, Producao: b.Producao,
		})
		return motivo(err)
	case "quarteiroes":
		if criado {
			return desfazerCriacao(ctx, q, "quarteirao", id)
		}
		var x sqlccliente.Quarteiro
		if err := json.Unmarshal(ln.Antes, &x); err != nil {
			return "o estado anterior não foi guardado"
		}
		_, err := q.UpdateQuarteirao(ctx, sqlccliente.UpdateQuarteiraoParams{ID: id, Nome: x.Nome, Ativo: x.Ativo})
		return motivo(err)
	case "locais":
		if criado {
			return desfazerCriacao(ctx, q, "local", id)
		}
		var l sqlccliente.Locai
		if err := json.Unmarshal(ln.Antes, &l); err != nil {
			return "o estado anterior não foi guardado"
		}
		if _, err := q.UpdateLocal(ctx, atualizacaoDeLocal(id, paramsDoLocal(l))); err != nil {
			return motivo(err)
		}
		_, err := q.SetLocalAtivo(ctx, sqlccliente.SetLocalAtivoParams{ID: id, Ativo: l.Ativo})
		return motivo(err)
	}
	return errTipoDesconhecido.Error()
}

var _ = time.Second
var _ = pgx.ErrNoRows

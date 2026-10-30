package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/authz"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/importacao"
)

// Importação de visitas e de tubitos (laboratório): só cria. O que já existe (pelo ID de origem da visita e pelo
// número do tubito) é ignorado, então dá para repetir a importação sem duplicar. Aceita ciclos fechados e agentes
// desativados, porque o histórico antigo costuma ter os dois.

// cargaVisita é o que a execução precisa para gravar uma visita já validada.
type cargaVisita struct {
	Create sqlccliente.CreateVisitaParams `json:"create"`
}

// cargaAmostra é o que a execução precisa para gravar um tubito (e o resultado, se veio).
type cargaAmostra struct {
	VisitaID     int64  `json:"visita_id"`
	Numero       string `json:"numero"`
	Deposito     string `json:"deposito"`
	Larvas       int32  `json:"larvas"`
	Pupas        int32  `json:"pupas"`
	Observacao   string `json:"observacao"`
	IDExterno    string `json:"id_externo"`
	TemResultado bool   `json:"tem_resultado"`
	Status       string `json:"status"`
	Aegypti      int32  `json:"aegypti"`
	Albopictus   int32  `json:"albopictus"`
	Negativo     int32  `json:"negativo"`
}

// inteiroDaCelula lê um número inteiro opcional ("3", "3,0"); vazio devolve nil.
func inteiroDaCelula(s string) (*int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil || f != float64(int(f)) {
		return nil, fmt.Errorf("%q não é um número inteiro", s)
	}
	n := int(f)
	return &n, nil
}

// decimalDaCelula lê um número decimal opcional (aceita vírgula); vazio devolve nil.
func decimalDaCelula(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil {
		return nil, fmt.Errorf("%q não é um número", s)
	}
	return &f, nil
}

func (h *handlers) analisarVisitas(ctx context.Context, r *http.Request, q *sqlccliente.Queries, p importacao.Planilha, m importacao.Mapeamento) ([]linhaAnalisada, error) {
	atividades, err := q.ListAtividades(ctx)
	if err != nil {
		return nil, err
	}
	porSigla := map[string]int64{}
	for _, a := range atividades {
		porSigla[strings.ToUpper(a.Sigla)] = int64(a.ID)
	}
	ciclos, err := q.ListCiclos(ctx)
	if err != nil {
		return nil, err
	}
	porCiclo := map[string]int64{}
	for _, c := range ciclos {
		porCiclo[importacao.Chave(c.Nome)] = c.ID
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
	locais := map[string]int64{} // cache: ID de origem do local -> id (0 = não existe)
	vistos := map[string]int{}

	var out []linhaAnalisada
	for _, ln := range p.Linhas {
		l := linhaAnalisada{N: ln.N, Original: ln.Celulas, Acao: acaoCriar}
		v := func(col string) string { return m.Valor(ln.Celulas, col) }

		ext := v("id_externo")
		if ext == "" || len(ext) > 64 {
			erroDeLinha(&l, "informe o ID de origem da visita (até 64 caracteres)")
			out = append(out, l)
			continue
		}
		if outra, ok := vistos[ext]; ok {
			erroDeLinha(&l, fmt.Sprintf("repetida na planilha (igual à linha %d)", outra))
			out = append(out, l)
			continue
		}
		vistos[ext] = ln.N
		if _, err := q.GetVisitaPorIDExterno(ctx, pgtype.Text{String: ext, Valid: true}); err == nil {
			l.Acao, l.Msg = acaoIgnorar, "já cadastrada"
			out = append(out, l)
			continue
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}

		var msgs []string
		falha := func(s string) { msgs = append(msgs, s) }

		// Local, ciclo, atividade e agente.
		extLocal := v("local")
		localID, visto := locais[extLocal]
		if !visto && extLocal != "" {
			if lc, err := q.GetLocalPorIdExterno(ctx, pgtype.Text{String: extLocal, Valid: true}); err == nil {
				localID = lc.ID
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
			locais[extLocal] = localID
		}
		if extLocal == "" {
			falha("informe o local (ID de origem)")
		} else if localID == 0 {
			falha(fmt.Sprintf("local %q não encontrado (importe os locais antes)", extLocal))
		}
		cicloID := porCiclo[importacao.Chave(v("ciclo"))]
		if cicloID == 0 {
			falha(fmt.Sprintf("ciclo %q não encontrado", v("ciclo")))
		}
		atvID := porSigla[strings.ToUpper(v("atividade"))]
		if atvID == 0 {
			falha(fmt.Sprintf("atividade %q desconhecida (use LIT, LI, PE, T, DF, PVE, LIRAa, LIA ou BRI)", v("atividade")))
		}
		chaveAgente := importacao.Chave(v("agente"))
		if strings.Contains(v("agente"), "@") {
			chaveAgente = strings.ToLower(v("agente"))
		}
		agenteID := porAgente[chaveAgente]
		switch {
		case chaveAgente == "":
			falha("informe o agente")
		case repetidos[chaveAgente]:
			falha(fmt.Sprintf("agente %q: há mais de um usuário com esse nome (use o e-mail)", v("agente")))
		case agenteID == 0:
			falha(fmt.Sprintf("agente %q não encontrado", v("agente")))
		}

		// Data e demais campos.
		sv := syncVisita{IDExterno: ext, CicloID: cicloID, AgenteID: agenteID, HoraEntrada: v("hora"), Anotacao: v("anotacao"), AdulticidaTipo: v("adulticida_tipo")}
		if t, ok := importacao.Data(v("data")); ok {
			sv.Data = t.Format(formatoDataISO)
		} else {
			falha(fmt.Sprintf("data inválida (%q), use dd/mm/aaaa", v("data")))
		}
		sv.TipoVisita = "normal"
		if t := v("tipo"); t != "" {
			switch palavra(t) {
			case "normal":
			case "recuperacao":
				sv.TipoVisita = "recuperacao"
			default:
				falha(fmt.Sprintf("tipo: use normal ou recuperação (veio %q)", t))
			}
		}
		sv.Pendencia = "nenhuma"
		if t := v("pendencia"); t != "" {
			if pv := palavra(t); validoEm(rotulosPendencia, pv) {
				sv.Pendencia = pv
			} else {
				falha(fmt.Sprintf("pendência: use nenhuma, fechado, recusado ou tentativa (veio %q)", t))
			}
		}
		numero := func(col string, dst **int) {
			n, err := inteiroDaCelula(v(col))
			if err != nil {
				falha(col + ": " + err.Error())
			}
			*dst = n
		}
		decimal := func(col string, dst **float64) {
			n, err := decimalDaCelula(v(col))
			if err != nil {
				falha(col + ": " + err.Error())
			}
			*dst = n
		}
		numero("a1", &sv.Depositos.A1)
		numero("a2", &sv.Depositos.A2)
		numero("b", &sv.Depositos.B)
		numero("c", &sv.Depositos.C)
		numero("d1", &sv.Depositos.D1)
		numero("d2", &sv.Depositos.D2)
		numero("e", &sv.Depositos.E)
		numero("total_eliminado", &sv.TotalEliminado)
		numero("l1_tratados", &sv.L1Tratados)
		numero("l2_tratados", &sv.L2Tratados)
		numero("adulticida_cargas", &sv.AdulticidaCargas)
		numero("tratamento_eliminados", &sv.TratamentoEliminados)
		decimal("l1_gramas", &sv.L1Gramas)
		decimal("l2_gramas", &sv.L2Gramas)
		decimal("bri_volume", &sv.BriVolume)
		decimal("latitude", &sv.Latitude)
		decimal("longitude", &sv.Longitude)
		var m1, m2, m3 string
		temDeposito := false
		for _, d := range []*int{sv.Depositos.A1, sv.Depositos.A2, sv.Depositos.B, sv.Depositos.C, sv.Depositos.D1, sv.Depositos.D2, sv.Depositos.E} {
			if d != nil && *d > 0 {
				temDeposito = true
			}
		}
		sv.AchouDeposito, m1 = logicoOu(m, ln.Celulas, "achou_deposito", temDeposito && sv.Pendencia == "nenhuma")
		sv.ColetouAmostra, m2 = logicoOu(m, ln.Celulas, "coletou_amostra", false)
		sv.FezTratamento, m3 = logicoOu(m, ln.Celulas, "fez_tratamento", false)
		msgs = append(msgs, m1, m2, m3)
		if msg := juntarMensagens(msgs); msg != "" {
			erroDeLinha(&l, msg)
			out = append(out, l)
			continue
		}

		// Mesmas regras da tela de visita; o importador aceita ciclo fechado e agente desativado.
		val, msg := h.validarVisita(ctx, r, q, authz.Com(permissaoCicloFechado, permissaoAgenteInativo), sv.campos(cicloID, atvID), nil)
		if msg != "" {
			erroDeLinha(&l, msg)
			out = append(out, l)
			continue
		}
		val.create.LocalID = localID
		val.create.IDExterno = pgtype.Text{String: ext, Valid: true}
		l.Dados = emJSON(cargaVisita{Create: val.create})
		out = append(out, l)
	}
	return out, nil
}

func juntarMensagens(msgs []string) string {
	var ok []string
	for _, s := range msgs {
		if s != "" {
			ok = append(ok, s)
		}
	}
	return strings.Join(ok, "; ")
}

func aplicarVisita(ctx context.Context, q *sqlccliente.Queries, ln sqlccliente.ImportacaoLinha, usuarioID int64) resultadoDaLinha {
	var c cargaVisita
	if err := json.Unmarshal(ln.Dados, &c); err != nil {
		return resultadoDaLinha{Resultado: "erro", Msg: "dados da linha ilegíveis"}
	}
	c.Create.CriadoPor = pgtype.Int8{Int64: usuarioID, Valid: usuarioID != 0}
	v, err := q.CreateVisita(ctx, c.Create)
	if err != nil {
		return resultadoDaLinha{Resultado: "erro", Msg: textoDoErro(err, "já existe uma visita com esse ID de origem")}
	}
	return resultadoDaLinha{Resultado: "criado", RegistroID: v.ID}
}

// ------------------------------------------------------------------- Tubitos

func (h *handlers) analisarAmostras(ctx context.Context, r *http.Request, q *sqlccliente.Queries, p importacao.Planilha, m importacao.Mapeamento) ([]linhaAnalisada, error) {
	automatico := h.tubitoAutomatico(r, q)
	visitas := map[string]sqlccliente.Visita{}
	naoAchou := map[string]bool{}
	tubitos := map[string]int{}
	externos := map[string]int{}
	var out []linhaAnalisada
	for _, ln := range p.Linhas {
		l := linhaAnalisada{N: ln.N, Original: ln.Celulas, Acao: acaoCriar}
		v := func(col string) string { return m.Valor(ln.Celulas, col) }
		var msgs []string
		falha := func(s string) { msgs = append(msgs, s) }

		extVisita := v("visita")
		var visita sqlccliente.Visita
		switch {
		case extVisita == "":
			falha("informe a visita (ID de origem)")
		case naoAchou[extVisita]:
			falha(fmt.Sprintf("visita %q não encontrada (importe as visitas antes)", extVisita))
		default:
			var ok bool
			if visita, ok = visitas[extVisita]; !ok {
				got, err := q.GetVisitaPorIDExterno(ctx, pgtype.Text{String: extVisita, Valid: true})
				switch {
				case errors.Is(err, pgx.ErrNoRows):
					naoAchou[extVisita] = true
					falha(fmt.Sprintf("visita %q não encontrada (importe as visitas antes)", extVisita))
				case err != nil:
					return nil, err
				default:
					visita, visitas[extVisita] = got, got
				}
			}
			if visita.ID != 0 {
				if visita.DeletedAt.Valid {
					falha("a visita está na Lixeira")
				} else if !visita.ColetouAmostra {
					falha("a visita não coletou amostra")
				}
			}
		}

		c := cargaAmostra{VisitaID: visita.ID, Numero: v("tubito"), Deposito: strings.ToUpper(v("deposito")), Observacao: v("observacao"), IDExterno: v("id_externo")}
		if c.Numero == "" && !automatico {
			falha("informe o número do tubito (o município usa numeração digitada)")
		}
		if len([]rune(c.Numero)) > 30 || len(c.IDExterno) > 64 || len([]rune(c.Observacao)) > 1000 {
			falha("tubito até 30, ID de origem até 64 e observação até 1000 caracteres")
		}
		if c.Deposito != "" && !validoEm(rotulosDepositoAmostra, c.Deposito) {
			falha(fmt.Sprintf("depósito %q inválido (use A1, A2, B, C, D1, D2 ou E)", v("deposito")))
		}
		total := func(col string, dst *int32, dado *bool) {
			n, err := inteiroDaCelula(v(col))
			switch {
			case err != nil:
				falha(col + ": " + err.Error())
			case n != nil && (*n < 0 || *n > 9999):
				falha(col + ": use de 0 a 9999")
			case n != nil:
				*dst, *dado = int32(*n), true
			}
		}
		var dadoLarvas, dadoPupas, t1, t2, t3 bool
		total("larvas", &c.Larvas, &dadoLarvas)
		total("pupas", &c.Pupas, &dadoPupas)
		total("aegypti", &c.Aegypti, &t1)
		total("albopictus", &c.Albopictus, &t2)
		total("negativo", &c.Negativo, &t3)
		c.Status = "em_analise"
		if t1 || t2 || t3 {
			c.Status = "concluido"
		}
		if s := v("status"); s != "" {
			switch palavra(s) {
			case "em_analise", "analise", "pendente":
				c.Status = "em_analise"
			case "concluido", "finalizado", "concluida":
				c.Status = "concluido"
			default:
				falha(fmt.Sprintf("status: use em análise ou concluído (veio %q)", s))
			}
		}
		c.TemResultado = c.Status == "concluido" || t1 || t2 || t3

		if c.IDExterno != "" {
			if outra, ok := externos[c.IDExterno]; ok {
				falha(fmt.Sprintf("ID de origem repetido na planilha (linha %d)", outra))
			}
			externos[c.IDExterno] = ln.N
		}
		if c.Numero != "" {
			if outra, ok := tubitos[strings.ToLower(c.Numero)]; ok {
				falha(fmt.Sprintf("tubito repetido na planilha (igual à linha %d)", outra))
			}
			tubitos[strings.ToLower(c.Numero)] = ln.N
		}
		if msg := juntarMensagens(msgs); msg != "" {
			erroDeLinha(&l, msg)
			out = append(out, l)
			continue
		}

		// Já existe: ignora (o tubito é reconhecido pelo ID de origem e pelo número).
		existe := false
		if c.IDExterno != "" {
			if _, err := q.GetAmostraPorIdExterno(ctx, pgtype.Text{String: c.IDExterno, Valid: true}); err == nil {
				existe = true
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
		}
		if !existe && c.Numero != "" {
			if _, err := q.GetAmostraPorNumero(ctx, c.Numero); err == nil {
				existe = true
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
		}
		if existe {
			l.Acao, l.Msg = acaoIgnorar, "tubito já cadastrado"
			out = append(out, l)
			continue
		}
		l.Dados = emJSON(c)
		out = append(out, l)
	}
	return out, nil
}

func aplicarAmostra(ctx context.Context, q *sqlccliente.Queries, ln sqlccliente.ImportacaoLinha, usuarioID int64) resultadoDaLinha {
	erro := func(msg string) resultadoDaLinha { return resultadoDaLinha{Resultado: "erro", Msg: msg} }
	var c cargaAmostra
	if err := json.Unmarshal(ln.Dados, &c); err != nil {
		return erro("dados da linha ilegíveis")
	}
	numero := c.Numero
	if numero == "" {
		if err := q.BloquearNumeracaoTubito(ctx); err != nil {
			return erro("não foi possível numerar o tubito")
		}
		n, err := q.ProximoNumeroTubito(ctx)
		if err != nil {
			return erro("não foi possível numerar o tubito")
		}
		numero = n
	}
	a, err := q.CreateAmostraSync(ctx, sqlccliente.CreateAmostraSyncParams{
		VisitaID: c.VisitaID, NumeroTubito: numero, Deposito: c.Deposito, Larvas: c.Larvas, Pupas: c.Pupas, Observacao: c.Observacao,
		CriadoPor: pgtype.Int8{Int64: usuarioID, Valid: usuarioID != 0}, IDExterno: pgtype.Text{String: c.IDExterno, Valid: c.IDExterno != ""},
	})
	if err != nil {
		return erro(textoDoErro(err, "o tubito "+numero+" já existe"))
	}
	if c.TemResultado {
		if _, err := q.UpdateAmostraResultado(ctx, sqlccliente.UpdateAmostraResultadoParams{
			ID: a.ID, Status: c.Status, TotalAegypti: c.Aegypti, TotalAlbopictus: c.Albopictus, TotalNegativo: c.Negativo,
			ResultadoPor: pgtype.Int8{Int64: usuarioID, Valid: usuarioID != 0},
		}); err != nil {
			return erro(textoDoErro(err, ""))
		}
	}
	return resultadoDaLinha{Resultado: "criado", RegistroID: a.ID}
}

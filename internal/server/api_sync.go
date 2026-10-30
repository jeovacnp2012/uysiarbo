package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/authz"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/httpx"
	"uysiarbo/web/views"
)

const (
	maxCorpoSync      = 8 << 20 // as assinaturas em PNG pesam: um lote de 100 visitas cabe folgado
	maxVisitasPorSync = 100
	maxAmostrasVisita = 50
)

type syncAmostra struct {
	IDExterno    string `json:"id_externo"`
	NumeroTubito string `json:"numero_tubito"`
	Deposito     string `json:"deposito"`
	Larvas       int    `json:"larvas"`
	Pupas        int    `json:"pupas"`
	Observacao   string `json:"observacao"`
}

type syncDepositos struct {
	A1 *int `json:"a1"`
	A2 *int `json:"a2"`
	B  *int `json:"b"`
	C  *int `json:"c"`
	D1 *int `json:"d1"`
	D2 *int `json:"d2"`
	E  *int `json:"e"`
}

type syncVisita struct {
	IDExterno            string        `json:"id_externo"`
	LocalID              int64         `json:"local_id"`
	LocalIDExterno       string        `json:"local_id_externo"`
	CicloID              int64         `json:"ciclo_id"`
	Atividade            string        `json:"atividade"` // sigla, ex.: "LIT"
	AgenteID             int64         `json:"agente_id"`
	Data                 string        `json:"data"` // AAAA-MM-DD
	HoraEntrada          string        `json:"hora_entrada"`
	TipoVisita           string        `json:"tipo_visita"`
	Pendencia            string        `json:"pendencia"`
	Anotacao             string        `json:"anotacao"`
	Assinatura           string        `json:"assinatura"`
	AchouDeposito        bool          `json:"achou_deposito"`
	Depositos            syncDepositos `json:"depositos"`
	TotalEliminado       *int          `json:"total_eliminado"`
	ColetouAmostra       bool          `json:"coletou_amostra"`
	FezTratamento        bool          `json:"fez_tratamento"`
	L1Gramas             *float64      `json:"l1_gramas"`
	L1Tratados           *int          `json:"l1_tratados"`
	L2Gramas             *float64      `json:"l2_gramas"`
	L2Tratados           *int          `json:"l2_tratados"`
	AdulticidaTipo       string        `json:"adulticida_tipo"`
	AdulticidaCargas     *int          `json:"adulticida_cargas"`
	TratamentoEliminados *int          `json:"tratamento_eliminados"`
	BriVolume            *float64      `json:"bri_volume"`
	Latitude             *float64      `json:"latitude"`
	Longitude            *float64      `json:"longitude"`
	PrecisaoGps          *float64      `json:"precisao_gps"`
	DataTablet           *time.Time    `json:"data_tablet"`
	Amostras             []syncAmostra `json:"amostras"`
}

type syncPedido struct {
	Dispositivo string       `json:"dispositivo"`
	EnviadoEm   *time.Time   `json:"enviado_em"`
	Visitas     []syncVisita `json:"visitas"`
}

type syncResultado struct {
	IDExterno string   `json:"id_externo"`
	Status    string   `json:"status"` // criada, existente, conflito ou rejeitada
	ID        int64    `json:"id,omitempty"`
	Amostras  int      `json:"amostras_criadas,omitempty"`
	Erros     []string `json:"erros,omitempty"`
	Conflitos []string `json:"campos_divergentes,omitempty"`
}

type syncResposta struct {
	SincronismoID int64           `json:"sincronismo_id"`
	ServidorEm    time.Time       `json:"servidor_em"`
	Criadas       int             `json:"criadas"`
	Existentes    int             `json:"existentes"`
	Conflitos     int             `json:"conflitos"`
	Rejeitadas    int             `json:"rejeitadas"`
	Resultados    []syncResultado `json:"resultados"`
}

func txt(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}

func txtF(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

func (v syncVisita) campos(ciclo, atividade int64) views.VisitaCampos {
	return views.VisitaCampos{
		Ciclo: strconv.FormatInt(ciclo, 10), Atividade: strconv.FormatInt(atividade, 10), Agente: strconv.FormatInt(v.AgenteID, 10),
		Data: v.Data, Hora: v.HoraEntrada, Tipo: v.TipoVisita, Pendencia: v.Pendencia, Achou: v.AchouDeposito,
		A1: txt(v.Depositos.A1), A2: txt(v.Depositos.A2), B: txt(v.Depositos.B), C: txt(v.Depositos.C),
		D1: txt(v.Depositos.D1), D2: txt(v.Depositos.D2), E: txt(v.Depositos.E), TotalEliminado: txt(v.TotalEliminado),
		Amostra: v.ColetouAmostra, Tratamento: v.FezTratamento,
		L1Gramas: txtF(v.L1Gramas), L1Tratados: txt(v.L1Tratados), L2Gramas: txtF(v.L2Gramas), L2Tratados: txt(v.L2Tratados),
		AdulticidaTipo: v.AdulticidaTipo, AdulticidaCargas: txt(v.AdulticidaCargas), TratEliminados: txt(v.TratamentoEliminados),
		Bri: txtF(v.BriVolume), Anotacao: v.Anotacao, Assinatura: v.Assinatura,
		Lat: txtF(v.Latitude), Lon: txtF(v.Longitude), Precisao: txtF(v.PrecisaoGps),
	}
}

// divergencias compara o que o tablet mandou com o que o servidor já tem (o servidor vence).
func divergencias(v syncVisita, atual sqlccliente.Visita, localID int64) []string {
	var out []string
	if localID != 0 && localID != atual.LocalID {
		out = append(out, "local")
	}
	if v.CicloID != 0 && v.CicloID != atual.CicloID {
		out = append(out, "ciclo")
	}
	if v.AgenteID != 0 && v.AgenteID != atual.AgenteID {
		out = append(out, "agente")
	}
	if d := atual.Data.Time.Format("2006-01-02"); v.Data != "" && v.Data != d {
		out = append(out, "data")
	}
	if v.Pendencia != "" && v.Pendencia != atual.Pendencia {
		out = append(out, "pendencia")
	}
	if v.AchouDeposito != atual.AchouDeposito {
		out = append(out, "achou_deposito")
	}
	if v.ColetouAmostra != atual.ColetouAmostra {
		out = append(out, "coletou_amostra")
	}
	if v.FezTratamento != atual.FezTratamento {
		out = append(out, "fez_tratamento")
	}
	return out
}

// apiSync recebe um lote de visitas (com amostras) do tablet. Cada visita é independente: vale a que passar nas
// regras, e a resposta diz o que aconteceu com cada uma. Repetir o envio é seguro (a chave é o id_externo).
func (h *handlers) apiSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var ped syncPedido
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCorpoSync))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ped); err != nil || dec.More() {
		h.apiErro(w, r, http.StatusBadRequest, "invalid_json", "O corpo precisa ser um JSON válido, só com os campos documentados (máximo de 8 MB).")
		return
	}
	if len(ped.Visitas) == 0 || len(ped.Visitas) > maxVisitasPorSync {
		h.apiErroCampo(w, r, "visitas", "envie de 1 a "+strconv.Itoa(maxVisitasPorSync)+" visitas por lote.")
		return
	}
	if len([]rune(ped.Dispositivo)) > 80 {
		h.apiErroCampo(w, r, "dispositivo", "use até 80 caracteres.")
		return
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	atividades, err := q.ListAtividades(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar atividades", err)
		return
	}
	porSigla := map[string]int64{}
	for _, a := range atividades {
		porSigla[strings.ToUpper(a.Sigla)] = int64(a.ID)
	}
	resp := syncResposta{ServidorEm: time.Now().UTC(), Resultados: make([]syncResultado, 0, len(ped.Visitas))}
	totalAmostras := 0
	vistos := map[string]bool{}
	for _, v := range ped.Visitas {
		res := h.sincronizarVisita(ctx, r, q, pool, v, porSigla, vistos)
		totalAmostras += res.Amostras
		switch res.Status {
		case "criada":
			resp.Criadas++
		case "existente":
			resp.Existentes++
		case "conflito":
			resp.Conflitos++
		default:
			resp.Rejeitadas++
		}
		resp.Resultados = append(resp.Resultados, res)
	}
	var credID pgtype.Int8
	if c, ok := credencialDe(ctx); ok {
		credID = pgtype.Int8{Int64: c.ID, Valid: true}
	}
	var enviado pgtype.Timestamptz
	if ped.EnviadoEm != nil {
		enviado = pgtype.Timestamptz{Time: *ped.EnviadoEm, Valid: true}
	}
	s, err := q.CreateSincronismo(ctx, sqlccliente.CreateSincronismoParams{
		CredencialID: credID, Dispositivo: ped.Dispositivo, EnviadoEm: enviado, Visitas: int32(len(ped.Visitas)),
		Criadas: int32(resp.Criadas), Existentes: int32(resp.Existentes), Conflitos: int32(resp.Conflitos),
		Rejeitadas: int32(resp.Rejeitadas), Amostras: int32(totalAmostras),
	})
	if err != nil {
		h.falhaInterna(w, r, "registrar sincronismo", err)
		return
	}
	resp.SincronismoID = s.ID
	httpx.JSON(w, http.StatusOK, resp)
}

func rejeitada(id string, erros ...string) syncResultado {
	return syncResultado{IDExterno: id, Status: "rejeitada", Erros: erros}
}

func (h *handlers) sincronizarVisita(ctx context.Context, r *http.Request, q *sqlccliente.Queries, pool *pgxpool.Pool, v syncVisita, porSigla map[string]int64, vistos map[string]bool) syncResultado {
	ext := strings.TrimSpace(v.IDExterno)
	if ext == "" || len(ext) > 64 {
		return rejeitada(ext, "id_externo: informe de 1 a 64 caracteres (é a chave que torna o reenvio seguro).")
	}
	if vistos[ext] {
		return rejeitada(ext, "id_externo repetido no mesmo lote.")
	}
	vistos[ext] = true
	if len(v.Amostras) > maxAmostrasVisita {
		return rejeitada(ext, "no máximo "+strconv.Itoa(maxAmostrasVisita)+" amostras por visita.")
	}
	localID := v.LocalID
	if localID == 0 && v.LocalIDExterno != "" {
		l, err := q.GetLocalPorIdExterno(ctx, pgtype.Text{String: v.LocalIDExterno, Valid: true})
		if err != nil {
			return rejeitada(ext, "local_id_externo: local não encontrado.")
		}
		localID = l.ID
	}

	// Já recebida antes: o servidor vence; só completa amostras que faltam.
	if atual, err := q.GetVisitaPorIDExterno(ctx, pgtype.Text{String: ext, Valid: true}); err == nil {
		res := syncResultado{IDExterno: ext, Status: "existente", ID: atual.ID}
		if d := divergencias(v, atual, localID); len(d) > 0 {
			res.Status, res.Conflitos = "conflito", d
		}
		if !atual.DeletedAt.Valid && atual.ColetouAmostra {
			n, err := h.sincronizarAmostras(ctx, r, q, atual, v.Amostras)
			if err != nil {
				return rejeitada(ext, err.Error())
			}
			res.Amostras = n
		}
		return res
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return rejeitada(ext, "erro interno ao conferir a visita; tente de novo.")
	}

	if localID == 0 {
		return rejeitada(ext, "informe local_id ou local_id_externo.")
	}
	local, err := q.GetLocal(ctx, localID)
	if err != nil {
		return rejeitada(ext, "o local informado não existe (ou foi excluído).")
	}
	atv := porSigla[strings.ToUpper(strings.TrimSpace(v.Atividade))]
	if atv == 0 {
		return rejeitada(ext, "atividade: use a sigla (LIT, LI, PE, T, DF, PVE, LIRAa, LIA ou BRI).")
	}
	// O tablet não tem as permissões de uma pessoa: ciclo fechado não aceita envio.
	val, msg := h.validarVisita(ctx, r, q, authz.Com(), v.campos(v.CicloID, atv), nil)
	if msg != "" {
		return rejeitada(ext, msg)
	}
	val.create.LocalID = local.ID
	val.create.IDExterno = pgtype.Text{String: ext, Valid: true}
	if v.DataTablet != nil {
		val.create.DataTablet = pgtype.Timestamptz{Time: *v.DataTablet, Valid: true}
	}
	if len(v.Amostras) > 0 && !val.create.ColetouAmostra {
		return rejeitada(ext, "amostras só entram em visita com coletou_amostra verdadeiro e sem pendência.")
	}
	for _, a := range v.Amostras {
		if msg := validarAmostraSync(a); msg != "" {
			return rejeitada(ext, msg)
		}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return rejeitada(ext, "erro interno; tente de novo.")
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	criada, err := qt.CreateVisita(ctx, val.create)
	if violacaoUnica(err) { // outra requisição gravou a mesma visita agora há pouco
		return syncResultado{IDExterno: ext, Status: "existente"}
	} else if err != nil {
		return rejeitada(ext, "erro interno ao gravar a visita; tente de novo.")
	}
	n, err := h.sincronizarAmostras(ctx, r, qt, criada, v.Amostras)
	if err != nil {
		return rejeitada(ext, err.Error())
	}
	h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{
		Acao: "visita.criar", Entidade: "visita", EntidadeID: strconv.FormatInt(criada.ID, 10),
		Rotulo: "Visita em " + enderecoCompleto(local.Endereco, local.Numero),
		Depois: dadosAuditoriaVisita(val.create, val.ciclo.Nome), Detalhes: map[string]any{"id_externo": ext, "origem": "sync", "amostras": n},
	})
	if err := qt.MarcarVisitaSincronizada(ctx, criada.ID); err != nil {
		return rejeitada(ext, "erro interno; tente de novo.")
	}
	if err := tx.Commit(ctx); err != nil {
		return rejeitada(ext, "erro interno ao confirmar; tente de novo.")
	}
	return syncResultado{IDExterno: ext, Status: "criada", ID: criada.ID, Amostras: n}
}

func validarAmostraSync(a syncAmostra) string {
	if a.Deposito != "" && !validoEm(rotulosDepositoAmostra, a.Deposito) {
		return "amostra: depósito inválido."
	}
	if a.Larvas < 0 || a.Larvas > 9999 || a.Pupas < 0 || a.Pupas > 9999 {
		return "amostra: larvas e pupas vão de 0 a 9999."
	}
	if len([]rune(a.Observacao)) > 1000 || len([]rune(a.NumeroTubito)) > 30 || len(a.IDExterno) > 64 {
		return "amostra: observação até 1000, tubito até 30 e id_externo até 64 caracteres."
	}
	return ""
}

// sincronizarAmostras grava as amostras que ainda não existem (pelo id_externo) e devolve quantas criou.
func (h *handlers) sincronizarAmostras(ctx context.Context, r *http.Request, q *sqlccliente.Queries, v sqlccliente.Visita, lista []syncAmostra) (int, error) {
	criadas := 0
	automatico := h.tubitoAutomatico(r, q)
	for _, a := range lista {
		if msg := validarAmostraSync(a); msg != "" {
			return criadas, errors.New(msg)
		}
		ext := strings.TrimSpace(a.IDExterno)
		if ext != "" {
			if _, err := q.GetAmostraPorIdExterno(ctx, pgtype.Text{String: ext, Valid: true}); err == nil {
				continue
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return criadas, errors.New("erro interno ao conferir a amostra; tente de novo.")
			}
		}
		numero := strings.TrimSpace(a.NumeroTubito)
		if numero == "" {
			if !automatico {
				return criadas, errors.New("amostra: informe numero_tubito (o cliente usa numeração digitada).")
			}
			if err := q.BloquearNumeracaoTubito(ctx); err != nil {
				return criadas, errors.New("erro interno na numeração; tente de novo.")
			}
			n, err := q.ProximoNumeroTubito(ctx)
			if err != nil {
				return criadas, errors.New("erro interno na numeração; tente de novo.")
			}
			numero = n
		}
		_, err := q.CreateAmostraSync(ctx, sqlccliente.CreateAmostraSyncParams{
			VisitaID: v.ID, NumeroTubito: numero, Deposito: a.Deposito, Larvas: int32(a.Larvas), Pupas: int32(a.Pupas),
			Observacao: a.Observacao, CriadoPor: pgtype.Int8{}, IDExterno: pgtype.Text{String: ext, Valid: ext != ""},
		})
		if violacaoUnica(err) {
			return criadas, errors.New("amostra: o tubito " + numero + " já existe.")
		} else if err != nil {
			return criadas, errors.New("erro interno ao gravar a amostra; tente de novo.")
		}
		criadas++
	}
	return criadas, nil
}

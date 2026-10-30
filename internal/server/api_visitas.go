package server

import (
	"net/http"
	"time"

	"uysiarbo/internal/db/sqlccliente"
)

type apiAmostra struct {
	ID           int64   `json:"id"`
	IDExterno    *string `json:"id_externo"`
	NumeroTubito string  `json:"numero_tubito"`
	Deposito     string  `json:"deposito"`
	Larvas       int32   `json:"larvas"`
	Pupas        int32   `json:"pupas"`
	Status       string  `json:"status"`
	Aegypti      int32   `json:"total_aegypti"`
	Albopictus   int32   `json:"total_albopictus"`
	Negativo     int32   `json:"total_negativo"`
	Observacao   string  `json:"observacao"`
}

type apiVisita struct {
	ID              int64          `json:"id"`
	IDExterno       *string        `json:"id_externo"`
	LocalID         int64          `json:"local_id"`
	CicloID         int64          `json:"ciclo_id"`
	AtividadeID     int16          `json:"atividade_id"`
	AgenteID        int64          `json:"agente_id"`
	Data            string         `json:"data"`
	TipoVisita      string         `json:"tipo_visita"`
	Pendencia       string         `json:"pendencia"`
	Semana          any            `json:"semana"`
	AchouDeposito   bool           `json:"achou_deposito"`
	Depositos       map[string]any `json:"depositos"`
	TotalEliminado  any            `json:"total_eliminado"`
	ColetouAmostra  bool           `json:"coletou_amostra"`
	FezTratamento   bool           `json:"fez_tratamento"`
	Latitude        any            `json:"latitude"`
	Longitude       any            `json:"longitude"`
	Ativo           bool           `json:"ativo"`
	DataTablet      any            `json:"data_tablet"`
	DataSincronismo any            `json:"data_sincronismo"`
	AtualizadoEm    time.Time      `json:"atualizado_em"`
	ExcluidoEm      any            `json:"excluido_em"`
	Amostras        []apiAmostra   `json:"amostras"`
}

func visitaParaAPI(v sqlccliente.Visita, ams []apiAmostra) apiVisita {
	var ext *string
	if v.IDExterno.Valid {
		ext = &v.IDExterno.String
	}
	var semana any
	if v.Semana.Valid {
		semana = v.Semana.Int16
	}
	if ams == nil {
		ams = []apiAmostra{}
	}
	return apiVisita{
		ID: v.ID, IDExterno: ext, LocalID: v.LocalID, CicloID: v.CicloID, AtividadeID: v.AtividadeID, AgenteID: v.AgenteID,
		Data: v.Data.Time.Format("2006-01-02"), TipoVisita: v.TipoVisita, Pendencia: v.Pendencia, Semana: semana,
		AchouDeposito: v.AchouDeposito,
		Depositos: map[string]any{"a1": optInt(v.DepA1), "a2": optInt(v.DepA2), "b": optInt(v.DepB), "c": optInt(v.DepC),
			"d1": optInt(v.DepD1), "d2": optInt(v.DepD2), "e": optInt(v.DepE)},
		TotalEliminado: optInt(v.TotalEliminado), ColetouAmostra: v.ColetouAmostra, FezTratamento: v.FezTratamento,
		Latitude: optFloat(v.Latitude), Longitude: optFloat(v.Longitude), Ativo: v.Ativo,
		DataTablet: optTempo(v.DataTablet), DataSincronismo: optTempo(v.DataSincronismo), AtualizadoEm: v.AtualizadoEm.Time,
		ExcluidoEm: optTempo(v.DeletedAt), Amostras: ams,
	}
}

// apiVisitasLista devolve as visitas (com as amostras) por cursor, para o tablet baixar o que precisa.
func (h *handlers) apiVisitasLista(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	depois, ok := lerCursor(r)
	if !ok {
		h.apiErro(w, r, http.StatusBadRequest, "invalid_cursor", "cursor inválido: use o next_cursor da resposta anterior.")
		return
	}
	ciclo, ok1 := inteiroDaURL(r, "ciclo_id")
	local, ok2 := inteiroDaURL(r, "local_id")
	agente, ok3 := inteiroDaURL(r, "agente_id")
	desde, ok4 := desdeDaURL(r)
	switch {
	case !ok1:
		h.apiParametroInvalido(w, r, "ciclo_id")
		return
	case !ok2:
		h.apiParametroInvalido(w, r, "local_id")
		return
	case !ok3:
		h.apiParametroInvalido(w, r, "agente_id")
		return
	case !ok4:
		h.apiParametroInvalido(w, r, "updated_since (use RFC 3339, como 2026-10-29T12:00:00Z)")
		return
	}
	limite := limiteDaURL(r)
	rows, err := q.ListVisitasApi(r.Context(), sqlccliente.ListVisitasApiParams{
		Depois: depois, CicloID: ciclo, LocalID: local, AgenteID: agente, IDExterno: r.URL.Query().Get("id_externo"),
		AtualizadoDesde: desde, Limite: limite + 1, IncluirExcluidos: desde.Valid,
	})
	if err != nil {
		h.falhaInterna(w, r, "listar visitas", err)
		return
	}
	mais := int32(len(rows)) > limite
	if mais {
		rows = rows[:limite]
	}
	ids := make([]int64, len(rows))
	for i, v := range rows {
		ids[i] = v.ID
	}
	porVisita := map[int64][]apiAmostra{}
	if len(ids) > 0 {
		ams, err := q.ListAmostrasDasVisitas(r.Context(), ids)
		if err != nil {
			h.falhaInterna(w, r, "listar amostras", err)
			return
		}
		for _, a := range ams {
			var ext *string
			if a.IDExterno.Valid {
				ext = &a.IDExterno.String
			}
			porVisita[a.VisitaID] = append(porVisita[a.VisitaID], apiAmostra{
				ID: a.ID, IDExterno: ext, NumeroTubito: a.NumeroTubito, Deposito: a.Deposito, Larvas: a.Larvas, Pupas: a.Pupas,
				Status: a.Status, Aegypti: a.TotalAegypti, Albopictus: a.TotalAlbopictus, Negativo: a.TotalNegativo, Observacao: a.Observacao,
			})
		}
	}
	dados := make([]apiVisita, len(rows))
	var ultimo int64
	for i, v := range rows {
		dados[i], ultimo = visitaParaAPI(v, porVisita[v.ID]), v.ID
	}
	h.paginaAPI(w, dados, ultimo, mais)
}

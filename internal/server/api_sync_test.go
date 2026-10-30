package server

import (
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"uysiarbo/internal/db/sqlccliente"
)

// lote monta o JSON de um envio com uma visita.
func (c cenarioDeVisita) visitaSync(ext, agente string, extras map[string]any) map[string]any {
	v := map[string]any{
		"id_externo": ext, "local_id": c.local, "ciclo_id": c.ciclo, "atividade": "LIT", "agente_id": agenteNum(agente),
		"data": c.ontem, "hora_entrada": "08:30", "tipo_visita": "normal", "pendencia": "nenhuma",
	}
	for k, x := range extras {
		v[k] = x
	}
	return v
}

func agenteNum(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func corpoSync(visitas ...map[string]any) string {
	b, _ := json.Marshal(map[string]any{"dispositivo": "tablet-01", "visitas": visitas})
	return string(b)
}

func TestAPISyncVisitasEAmostras(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)
	api := novaAPI(t, m, "visitas:read", "visitas:write")
	semEscopo := novaAPI(t, m, "locais:read")

	quer(t, semEscopo.req(http.MethodPost, "/api/v1/sync", corpoSync(c.visitaSync("t1", agente, nil))), http.StatusForbidden)
	quer(t, api.req(http.MethodPost, "/api/v1/sync", `{"visitas":[]}`), http.StatusUnprocessableEntity)
	quer(t, api.req(http.MethodPost, "/api/v1/sync", `{"visitas":[{"id_externo":"x","campo_estranho":1}]}`), http.StatusBadRequest, "invalid_json")

	lote := corpoSync(
		c.visitaSync("t-ok", agente, map[string]any{
			"achou_deposito": true, "depositos": map[string]any{"a1": 2, "b": 1}, "total_eliminado": 2, "coletou_amostra": true,
			"latitude": -13.8, "longitude": -56.08, "data_tablet": "2026-10-29T10:00:00Z",
			"amostras": []map[string]any{{"id_externo": "am-1", "deposito": "A1", "larvas": 4, "pupas": 1}, {"id_externo": "am-2", "deposito": "B", "larvas": 2}},
		}),
		c.visitaSync("t-ruim", agente, map[string]any{"atividade": "ZZZ"}),
		c.visitaSync("t-pend", agente, map[string]any{"pendencia": "fechado", "achou_deposito": true}),
		c.visitaSync("t-semlocal", agente, map[string]any{"local_id": 0}),
		c.visitaSync("t-amostra-sem-flag", agente, map[string]any{"amostras": []map[string]any{{"larvas": 1}}}),
	)
	rec := api.req(http.MethodPost, "/api/v1/sync", lote)
	quer(t, rec, http.StatusOK)
	var resp syncResposta
	api.json(rec, &resp)
	status := map[string]syncResultado{}
	for _, r := range resp.Resultados {
		status[r.IDExterno] = r
	}
	if resp.Criadas != 1 || resp.Rejeitadas != 4 || status["t-ok"].Status != "criada" || status["t-ok"].Amostras != 2 {
		t.Fatalf("resposta: %+v", resp)
	}
	for _, id := range []string{"t-ruim", "t-pend", "t-semlocal", "t-amostra-sem-flag"} {
		if status[id].Status != "rejeitada" || len(status[id].Erros) == 0 {
			t.Errorf("%s deveria ser rejeitada com motivo: %+v", id, status[id])
		}
	}

	// Gravou certo: semana calculada pelo servidor, data do sincronismo, numeração automática.
	v, err := m.cliQ.GetVisitaPorIDExterno(t.Context(), textoPg("t-ok"))
	if err != nil || !v.AchouDeposito || v.DepA1.Int32 != 2 || !v.Semana.Valid || !v.DataSincronismo.Valid || !v.DataTablet.Valid {
		t.Fatalf("visita gravada: %+v (%v)", v, err)
	}
	as, _ := m.cliQ.ListAmostrasDaVisita(t.Context(), v.ID)
	if len(as) != 2 || as[0].NumeroTubito != "1" || as[1].NumeroTubito != "2" {
		t.Fatalf("amostras: %+v", as)
	}

	// Reenvio: idempotente (nada duplica), e divergência vira conflito (o servidor vence).
	rec = api.req(http.MethodPost, "/api/v1/sync", corpoSync(
		c.visitaSync("t-ok", agente, map[string]any{"achou_deposito": true, "coletou_amostra": true, "amostras": []map[string]any{{"id_externo": "am-1", "larvas": 4}, {"id_externo": "am-3", "larvas": 7}}}),
		c.visitaSync("t-ok", agente, nil),
	))
	api.json(rec, &resp)
	if resp.Criadas != 0 || resp.Resultados[0].Status != "existente" || resp.Resultados[0].Amostras != 1 || resp.Resultados[1].Status != "rejeitada" {
		t.Fatalf("reenvio: %+v", resp)
	}
	rec = api.req(http.MethodPost, "/api/v1/sync", corpoSync(c.visitaSync("t-ok", agente, map[string]any{"pendencia": "recusado"})))
	api.json(rec, &resp)
	if resp.Resultados[0].Status != "conflito" || len(resp.Resultados[0].Conflitos) == 0 {
		t.Fatalf("conflito: %+v", resp.Resultados)
	}
	if n, _ := m.cliQ.CountVisitas(t.Context(), listarContagem()); n != 1 {
		t.Fatalf("visitas = %d, esperava 1", n)
	}
	as, _ = m.cliQ.ListAmostrasDaVisita(t.Context(), v.ID)
	if len(as) != 3 {
		t.Fatalf("amostras depois do reenvio = %d", len(as))
	}

	// Download.
	rec = api.req(http.MethodGet, "/api/v1/visitas?id_externo=t-ok", "")
	quer(t, rec, http.StatusOK, `"id_externo":"t-ok"`, `"numero_tubito":"3"`)
	quer(t, api.req(http.MethodGet, "/api/v1/visitas?ciclo_id=abc", ""), http.StatusBadRequest)
	quer(t, semEscopo.req(http.MethodGet, "/api/v1/visitas", ""), http.StatusForbidden)

	// Histórico dos envios e auditoria.
	if n, err := m.cliQ.CountSincronismos(t.Context(), "tablet-01"); err != nil || n != 3 {
		t.Errorf("sincronismos = %d (%v)", n, err)
	}
	if achar(m.eventos(t), "visita.criar") == nil {
		t.Error("faltou visita.criar na auditoria")
	}
	_ = strings.TrimSpace
}

func TestAPISyncCicloFechadoENumeracaoDigitada(t *testing.T) {
	m := novoMundo(t)
	ana := m.comAna(t)
	c := m.cenarioVisita(t, ana)
	agente := m.idUsuario(t, emailAna)
	api := novaAPI(t, m, "visitas:write")
	if err := m.cliQ.SetClienteInfo(t.Context(), setInfo("visita_tubito_automatico", "0")); err != nil {
		t.Fatal(err)
	}
	rec := api.req(http.MethodPost, "/api/v1/sync", corpoSync(
		c.visitaSync("d1", agente, map[string]any{"coletou_amostra": true, "amostras": []map[string]any{{"larvas": 1}}}),
		c.visitaSync("d2", agente, map[string]any{"coletou_amostra": true, "amostras": []map[string]any{{"numero_tubito": "T-9", "larvas": 1}}}),
		c.visitaSync("d3", agente, map[string]any{"coletou_amostra": true, "amostras": []map[string]any{{"numero_tubito": "T-9", "larvas": 1}}}),
	))
	quer(t, rec, http.StatusOK)
	var resp syncResposta
	api.json(rec, &resp)
	if resp.Resultados[0].Status != "rejeitada" || resp.Resultados[1].Status != "criada" || resp.Resultados[2].Status != "rejeitada" {
		t.Fatalf("numeração digitada: %+v", resp.Resultados)
	}
	// Ciclo fechado: o tablet não grava.
	quer(t, ana.post("/ciclos/"+s64(c.ciclo), formCiclo("01 Ciclo", c.ontem[:4]+"-01-01", c.ontem[:4]+"-12-31", url.Values{"atividade": {"1", "3"}, "fechado": {"1"}})), http.StatusSeeOther)
	rec = api.req(http.MethodPost, "/api/v1/sync", corpoSync(c.visitaSync("f1", agente, nil)))
	api.json(rec, &resp)
	if resp.Resultados[0].Status != "rejeitada" || !strings.Contains(strings.Join(resp.Resultados[0].Erros, " "), "fechado") {
		t.Fatalf("ciclo fechado: %+v", resp.Resultados)
	}
}

func textoPg(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

func setInfo(chave, valor string) sqlccliente.SetClienteInfoParams {
	return sqlccliente.SetClienteInfoParams{Chave: chave, Valor: valor}
}

func listarContagem() sqlccliente.CountVisitasParams {
	return sqlccliente.CountVisitasParams{VerDesativadas: true}
}

func urlValores(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Add(kv[i], kv[i+1])
	}
	return v
}

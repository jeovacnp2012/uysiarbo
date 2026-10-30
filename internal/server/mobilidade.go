package server

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// mobilidadeHistorico lista os envios feitos pelos aplicativos (tablets) por POST /api/v1/sync: quando, de que
// aparelho, por qual credencial e o resultado de cada lote.
func (h *handlers) mobilidadeHistorico(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	loc := h.fuso(r)
	qs := r.URL.Query()
	f := views.MobilidadeFiltro{
		Dispositivo: strings.TrimSpace(qs.Get("dispositivo")), Credencial: qs.Get("credencial"),
		De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate")), Problemas: qs.Get("problemas") == "sim",
	}
	// Os limites do dia valem no fuso do cliente; "até" inclui o dia inteiro.
	var de, ate pgtype.Timestamptz
	if t, err := time.ParseInLocation(formatoDataISO, f.De, loc); err == nil {
		de = pgtype.Timestamptz{Time: t, Valid: true}
	} else {
		f.De = ""
	}
	if t, err := time.ParseInLocation(formatoDataISO, f.Ate, loc); err == nil {
		ate = pgtype.Timestamptz{Time: t.AddDate(0, 0, 1), Valid: true}
	} else {
		f.Ate = ""
	}
	cred := idOpcional(f.Credencial)

	pag := paginaDe(r)
	total, err := q.CountSincronismosFiltrados(ctx, sqlccliente.CountSincronismosFiltradosParams{
		Dispositivo: f.Dispositivo, CredencialID: cred, De: de, Ate: ate, Problemas: f.Problemas,
	})
	if err != nil {
		h.falhaInterna(w, r, "contar sincronismos", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListSincronismos(ctx, sqlccliente.ListSincronismosParams{
		Dispositivo: f.Dispositivo, CredencialID: cred, De: de, Ate: ate, Problemas: f.Problemas,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar sincronismos", err)
		return
	}
	d := views.MobilidadeData{Filtro: f, Pag: pag}
	for _, s := range rows {
		l := views.MobilidadeLinha{
			ID: s.ID, Recebido: formatarData(loc, s.RecebidoEm.Time), Dispositivo: s.Dispositivo, Credencial: s.CredencialNome,
			Visitas: int(s.Visitas), Criadas: int(s.Criadas), Existentes: int(s.Existentes), Conflitos: int(s.Conflitos),
			Rejeitadas: int(s.Rejeitadas), Amostras: int(s.Amostras),
		}
		if s.EnviadoEm.Valid {
			l.Enviado = formatarData(loc, s.EnviadoEm.Time)
		}
		d.Linhas = append(d.Linhas, l)
	}
	creds, err := q.ListApiCredenciais(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar credenciais", err)
		return
	}
	for _, c := range creds {
		d.Credenciais = append(d.Credenciais, views.CidadeOpcao{ID: c.ID, Rotulo: c.Nome, Selecionada: c.ID == cred.Int64})
	}
	d.Query = url.Values{}
	for k, v := range map[string]string{"dispositivo": f.Dispositivo, "credencial": f.Credencial, "de": f.De, "ate": f.Ate} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	if f.Problemas {
		d.Query.Set("problemas", "sim")
	}
	render(w, r, views.Mobilidade(h.layout(r, "Histórico de mobilidade", "/mobilidade/historico"), d))
}

package server

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"net/http"
	"strconv"
	"strings"
	"time"
	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"

	"uysiarbo/internal/epi"
	"uysiarbo/web/views"
)

// abaDeParametros diz quais telas (pelo item de menu ativo) mostram a barra de abas de Parâmetros.
var abaDeParametros = map[string]bool{
	"/parametros/geral": true, "/parametros/semanas": true, "/parametros/numeracao": true, "/parametros/mobilidade": true,
	"/leis": true, "/entrevistas": true, "/doencas": true, "/parametros/contaovos": true,
}

// abasParametros monta a barra de abas da área Parâmetros, como no Geo Controle, só com o que a pessoa pode abrir.
func (h *handlers) abasParametros(r *http.Request, ativa string) []views.AbaParam {
	a, _ := authFrom(r.Context())
	if a.sess == nil {
		return nil
	}
	pr := a.sess.Principal
	var abas []views.AbaParam
	add := func(ok bool, rotulo, href string) {
		if ok {
			abas = append(abas, views.AbaParam{Rotulo: rotulo, Href: href, Ativa: href == ativa})
		}
	}
	add(pr.Pode("param_geral"), "Geral", "/parametros/geral")
	add(pr.Pode("param_ciclo"), "Semanas SINAN", "/parametros/semanas")
	add(pr.Pode("param_geral"), "Numeração", "/parametros/numeracao")
	add(pr.Pode("param_mobilidade"), "Mobilidade", "/parametros/mobilidade")
	add(pr.Pode("param_leis"), "Leis municipais", "/leis")
	add(pr.Pode("param_doenca"), "Investigação - Grupos", "/entrevistas")
	add(pr.Pode("param_doenca"), "Investigação - Doenças", "/doencas")
	add(pr.Pode("param_geral"), "API Conta Ovos", "/parametros/contaovos")
	return abas
}

// semanasPagina mostra o cadastro de anos de semanas epidemiológicas (adicionar, editar e excluir), com as semanas de
// cada ano calculadas no padrão do cliente.
func (h *handlers) semanasPagina(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d, err := h.dadosDeSemanas(r, q)
	if err != nil {
		h.falhaInterna(w, r, "listar anos", err)
		return
	}
	// Ano a editar, ou sugestão para o próximo ano a cadastrar (já preenchida com o calendário calculado).
	if txt := chi.URLParam(r, "ano"); txt != "" {
		ano, _ := strconv.Atoi(txt)
		a, err := q.GetSemanaAno(r.Context(), int16(ano))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		d.Editando, d.FAno, d.FPrimeiro, d.FUltimo, d.FSemanas = true, txt, dataISO(a.PrimeiroDia), dataISO(a.UltimoDia), strconv.Itoa(int(a.Semanas))
	} else if len(d.Anos) > 0 {
		prox := d.Anos[0].Ano + 1
		if s := semanasDoAno(prox, epi.SINAN); len(s) > 0 {
			d.FAno, d.FPrimeiro, d.FUltimo, d.FSemanas = strconv.Itoa(prox), s[0].Inicio.Format(formatoDataISO), s[len(s)-1].Fim.Format(formatoDataISO), strconv.Itoa(len(s))
		}
	}
	render(w, r, views.ParametrosSemanas(h.layout(r, "Semanas SINAN", "/parametros/semanas"), d))
}

func (h *handlers) dadosDeSemanas(r *http.Request, q *sqlccliente.Queries) (views.SemanasData, error) {
	tipo := h.tipoSemana(r, q)
	d := views.SemanasData{Tipo: rotuloDoTipoSemana(tipo), CSRF: csrfToken(r.Context())}
	anos, err := q.ListSemanasAnos(r.Context())
	if err != nil {
		return d, err
	}
	for _, a := range anos {
		d.Anos = append(d.Anos, views.SemanaLinha{Ano: int(a.Ano), Semanas: int(a.Semanas), Primeiro: dataBR(a.PrimeiroDia), Ultimo: dataBR(a.UltimoDia)})
	}
	if ano, err := strconv.Atoi(r.URL.Query().Get("ano")); err == nil && ano >= 1990 && ano <= 2100 {
		d.Ano = ano
		for _, s := range semanasDoAno(ano, tipo) {
			d.Detalhe = append(d.Detalhe, views.SemanaDetalhe{Numero: s.Numero, Inicio: s.Inicio.Format(formatoData), Fim: s.Fim.Format(formatoData)})
		}
	}
	return d, nil
}

// semanaAnoSalvar cria (sem {ano} na URL) ou altera (com {ano}) um ano do calendário.
func (h *handlers) semanaAnoSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	editando := chi.URLParam(r, "ano") != ""
	anoTxt := strings.TrimSpace(r.PostFormValue("ano"))
	if editando {
		anoTxt = chi.URLParam(r, "ano")
	}
	priTxt, ultTxt, semTxt := strings.TrimSpace(r.PostFormValue("primeiro_dia")), strings.TrimSpace(r.PostFormValue("ultimo_dia")), strings.TrimSpace(r.PostFormValue("semanas"))
	falha := func(status int, msg string) {
		d, _ := h.dadosDeSemanas(r, q)
		d.Erro, d.Editando, d.FAno, d.FPrimeiro, d.FUltimo, d.FSemanas = msg, editando, anoTxt, priTxt, ultTxt, semTxt
		renderStatus(w, r, status, views.ParametrosSemanas(h.layout(r, "Semanas SINAN", "/parametros/semanas"), d))
	}
	ano, errAno := strconv.Atoi(anoTxt)
	pri, errPri := time.Parse(formatoDataISO, priTxt)
	ult, errUlt := time.Parse(formatoDataISO, ultTxt)
	sem, errSem := strconv.Atoi(semTxt)
	switch {
	case errAno != nil || ano < 1990 || ano > 2100:
		falha(http.StatusBadRequest, "Informe o ano (de 1990 a 2100).")
		return
	case errPri != nil || errUlt != nil:
		falha(http.StatusBadRequest, "Informe o primeiro e o último dia.")
		return
	case errSem != nil || (sem != 52 && sem != 53):
		falha(http.StatusBadRequest, "O total de semanas é 52 ou 53.")
		return
	case int(ult.Sub(pri).Hours()/24)+1 != sem*7:
		falha(http.StatusBadRequest, fmt.Sprintf("Do primeiro ao último dia há %d dias; %d semanas pedem %d dias (de domingo a sábado).", int(ult.Sub(pri).Hours()/24)+1, sem, sem*7))
		return
	}
	pg := func(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }
	ev := audit.Evento{Entidade: "semana_ano", EntidadeID: anoTxt, Rotulo: "Ano " + anoTxt}
	depois := map[string]any{"primeiro_dia": priTxt, "ultimo_dia": ultTxt, "semanas": sem}
	if !editando {
		if _, err := q.CreateSemanaAno(r.Context(), sqlccliente.CreateSemanaAnoParams{Ano: int16(ano), PrimeiroDia: pg(pri), UltimoDia: pg(ult), Semanas: int16(sem)}); violacaoUnica(err) {
			falha(http.StatusConflict, "Esse ano já está cadastrado.")
			return
		} else if err != nil {
			h.falhaInterna(w, r, "criar ano", err)
			return
		}
		ev.Acao, ev.Depois = "semana_ano.criar", depois
		h.auditar(r, ev)
		http.Redirect(w, r, "/parametros/semanas?ok=1", http.StatusSeeOther)
		return
	}
	antes, err := q.GetSemanaAno(r.Context(), int16(ano))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := q.UpdateSemanaAno(r.Context(), sqlccliente.UpdateSemanaAnoParams{Ano: int16(ano), PrimeiroDia: pg(pri), UltimoDia: pg(ult), Semanas: int16(sem)}); err != nil {
		h.falhaInterna(w, r, "salvar ano", err)
		return
	}
	a, dpois := audit.Diferenca(map[string]any{"primeiro_dia": dataISO(antes.PrimeiroDia), "ultimo_dia": dataISO(antes.UltimoDia), "semanas": int(antes.Semanas)}, depois)
	if len(a)+len(dpois) == 0 {
		naoAuditar(r)
	} else {
		ev.Acao, ev.Antes, ev.Depois = "semana_ano.editar", a, dpois
		h.auditar(r, ev)
	}
	http.Redirect(w, r, "/parametros/semanas?ok=1", http.StatusSeeOther)
}

func (h *handlers) semanaAnoExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	ano, err := strconv.Atoi(chi.URLParam(r, "ano"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	n, err := q.DeleteSemanaAno(r.Context(), int16(ano))
	if err != nil {
		h.falhaInterna(w, r, "excluir ano", err)
		return
	}
	if n == 0 {
		http.NotFound(w, r)
		return
	}
	h.auditar(r, audit.Evento{Acao: "semana_ano.excluir", Entidade: "semana_ano", EntidadeID: strconv.Itoa(ano), Rotulo: "Ano " + strconv.Itoa(ano)})
	http.Redirect(w, r, "/parametros/semanas?ok=1", http.StatusSeeOther)
}

// semanasDoAno lista as semanas que pertencem ao ano no tipo pedido, em ordem.
func semanasDoAno(ano int, tipo string) []epi.Semana {
	var out []epi.Semana
	vistas := map[int]bool{}
	for dia := time.Date(ano-1, time.December, 15, 0, 0, 0, 0, time.UTC); dia.Year() <= ano+1 && dia.Before(time.Date(ano+1, time.January, 15, 0, 0, 0, 0, time.UTC)); dia = dia.AddDate(0, 0, 1) {
		s := epi.Calcular(dia, tipo)
		if s.Ano == ano && !vistas[s.Numero] {
			vistas[s.Numero] = true
			out = append(out, s)
		}
	}
	return out
}

// numeracaoPagina mostra o formato e o contador atual da numeração de notificações e autos.
func (h *handlers) numeracaoPagina(w http.ResponseWriter, r *http.Request) {
	_, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	ano := time.Now().In(h.fuso(r)).Year()
	d := views.NumeracaoData{Ano: ano}
	for _, n := range []struct {
		tipo, rotulo, prefixo string
		digitos               int
	}{{"infracao", "Infração", "INF", 5}, {"notificacao", "Notificação", "NT", 7}} {
		var ultimo int
		_ = pool.QueryRow(r.Context(), `SELECT COALESCE((SELECT ultimo FROM numeracoes WHERE tipo = $1 AND ano = $2), 0)`, n.tipo, ano).Scan(&ultimo)
		d.Linhas = append(d.Linhas, views.NumeracaoLinha{
			Tipo: n.rotulo, Prefixo: n.prefixo,
			Formato: fmt.Sprintf("{PREFIXO}-{ANO:4}{NUMERO:%d}", n.digitos), ResetAnual: "Sim",
			Ultimo:  strconv.Itoa(ultimo),
			Proximo: fmt.Sprintf("%s-%d%0*d", n.prefixo, ano, n.digitos, ultimo+1),
		})
	}
	render(w, r, views.ParametrosNumeracao(h.layout(r, "Numeração", "/parametros/numeracao"), d))
}

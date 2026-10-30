package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"uysiarbo/internal/epi"
	"uysiarbo/web/views"
)

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
	add(pr.Pode("param_ciclo"), "Semanas epidemiológicas", "/parametros/semanas")
	add(pr.Pode("param_geral"), "Numeração", "/parametros/numeracao")
	add(pr.Pode("param_mobilidade"), "Mobilidade", "/parametros/mobilidade")
	add(pr.Pode("param_geral"), "API Conta Ovos", "/parametros/contaovos")
	add(pr.Pode("acesso_local"), "Controle de acesso", "/parametros/acesso")
	add(pr.Pode("param_leis"), "Leis municipais", "/leis")
	add(pr.Pode("param_doenca"), "Investigação - Grupos", "/entrevistas")
	add(pr.Pode("param_doenca"), "Investigação - Doenças", "/doencas")
	return abas
}

// semanasPagina mostra o calendário de semanas epidemiológicas no padrão do cliente (calculado, sem cadastro).
func (h *handlers) semanasPagina(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	tipo := h.tipoSemana(r, q)
	hoje := time.Now().In(h.fuso(r))
	d := views.SemanasData{Abas: h.abasParametros(r, "/parametros/semanas"), Tipo: rotuloDoTipoSemana(tipo), Pode: true}
	for ano := hoje.Year() + 1; ano >= hoje.Year()-4; ano-- {
		semanas := semanasDoAno(ano, tipo)
		if len(semanas) == 0 {
			continue
		}
		d.Anos = append(d.Anos, views.SemanaLinha{Ano: ano, Semanas: len(semanas),
			Primeiro: semanas[0].Inicio.Format(formatoData), Ultimo: semanas[len(semanas)-1].Fim.Format(formatoData)})
	}
	if ano, err := strconv.Atoi(r.URL.Query().Get("ano")); err == nil && ano >= 1990 && ano <= 2100 {
		d.Ano = ano
		for _, s := range semanasDoAno(ano, tipo) {
			d.Detalhe = append(d.Detalhe, views.SemanaDetalhe{Numero: s.Numero, Inicio: s.Inicio.Format(formatoData), Fim: s.Fim.Format(formatoData)})
		}
	}
	render(w, r, views.ParametrosSemanas(h.layout(r, "Semanas epidemiológicas", "/parametros/semanas"), d))
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
	d := views.NumeracaoData{Abas: h.abasParametros(r, "/parametros/numeracao"), Ano: ano}
	for _, n := range []struct {
		tipo, rotulo, prefixo string
		digitos               int
	}{{"notificacao", "Notificação", "NT", 7}, {"infracao", "Auto de infração", "INF", 5}} {
		var ultimo int
		_ = pool.QueryRow(r.Context(), `SELECT COALESCE((SELECT ultimo FROM numeracoes WHERE tipo = $1 AND ano = $2), 0)`, n.tipo, ano).Scan(&ultimo)
		d.Linhas = append(d.Linhas, views.NumeracaoLinha{
			Tipo: n.rotulo, Prefixo: n.prefixo,
			Formato: fmt.Sprintf("%s-AAAA%s", n.prefixo, "NNNNNNNNN"[:n.digitos]),
			Ultimo:  strconv.Itoa(ultimo),
			Proximo: fmt.Sprintf("%s-%d%0*d", n.prefixo, ano, n.digitos, ultimo+1),
		})
	}
	render(w, r, views.ParametrosNumeracao(h.layout(r, "Numeração", "/parametros/numeracao"), d))
}

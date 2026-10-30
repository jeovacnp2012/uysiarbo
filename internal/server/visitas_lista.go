package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

var opcoesSimNao = []views.CidadeOpcao{{Valor: "sim", Rotulo: "Sim"}, {Valor: "nao", Rotulo: "Não"}}

func idOpcional(v string) pgtype.Int8 {
	if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
		return pgtype.Int8{Int64: n, Valid: true}
	}
	return pgtype.Int8{}
}

func dataOpcional(v string) pgtype.Date {
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return pgtype.Date{Time: t, Valid: true}
	}
	return pgtype.Date{}
}

// simNaoValido devolve "sim", "nao" ou "" (sem filtro).
func simNaoValido(v string) string {
	if v == "sim" || v == "nao" {
		return v
	}
	return ""
}

// visitasLista mostra a listagem de visitas com os filtros ao vivo (6.2).
func (h *handlers) visitasLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	pr := h.principal(r)
	qs := r.URL.Query()
	txt := func(k string) string { return strings.TrimSpace(qs.Get(k)) }
	f := views.VisitasFiltro{
		ID: txt("id"), Endereco: txt("endereco"), Numero: txt("numero"), Bairro: qs.Get("bairro"), Quarteirao: qs.Get("quarteirao"),
		Lado: txt("lado"), Ciclo: qs.Get("ciclo"), Agente: qs.Get("agente"), Atividade: qs.Get("atividade"), Semana: txt("semana"),
		De: txt("de"), Ate: txt("ate"), Pendencia: qs.Get("pendencia"), Deposito: simNaoValido(qs.Get("deposito")),
		Amostra: simNaoValido(qs.Get("amostra")), Tratamento: simNaoValido(qs.Get("tratamento")), Foto: simNaoValido(qs.Get("foto")),
		Situacao: qs.Get("situacao"),
	}
	if !validoEm(rotulosPendencia, f.Pendencia) {
		f.Pendencia = ""
	}
	if f.Situacao != "ativa" && f.Situacao != "desativada" {
		f.Situacao = ""
	}
	semana := pgtype.Int4{}
	if n, err := strconv.Atoi(f.Semana); err == nil && n >= 1 && n <= 53 {
		semana = pgtype.Int4{Int32: int32(n), Valid: true}
	}
	arg := sqlccliente.ListVisitasParams{
		VerDesativadas: pr.Pode("visita_ver_desativados"), SomenteAgente: h.escopoVisitas(r), IDTexto: f.ID, Endereco: f.Endereco,
		Numero: f.Numero, BairroID: idOpcional(f.Bairro), QuarteiraoID: idOpcional(f.Quarteirao), Lado: f.Lado,
		CicloID: idOpcional(f.Ciclo), AgenteID: idOpcional(f.Agente), AtividadeID: idOpcional(f.Atividade), Semana: semana,
		DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate), Pendencia: f.Pendencia, Deposito: f.Deposito,
		Amostra: f.Amostra, Tratamento: f.Tratamento, Foto: f.Foto, Situacao: f.Situacao,
	}
	d := views.VisitasData{PodeNova: pr.Pode("visita_adicionar"), Filtro: f, PodeLote: pr.Pode("visita_desativar"), Aviso: avisoDoLoteDeVisitas(qs.Get("aviso"))}
	if txt := qs.Get("local"); txt != "" {
		id, err := strconv.ParseInt(txt, 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		l, err := q.GetLocal(ctx, id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if ok, err := h.localVisivel(ctx, r, q, l); err != nil {
			h.falhaInterna(w, r, "checar acesso ao local", err)
			return
		} else if !ok {
			http.NotFound(w, r)
			return
		}
		d.LocalID, d.Local = l.ID, enderecoCompleto(l.Endereco, l.Numero)
		arg.LocalID = pgtype.Int8{Int64: id, Valid: true}
	}

	pag := paginaDe(r)
	total, err := q.CountVisitas(ctx, sqlccliente.CountVisitasParams{
		VerDesativadas: arg.VerDesativadas, LocalID: arg.LocalID, SomenteAgente: arg.SomenteAgente, IDTexto: arg.IDTexto,
		Endereco: arg.Endereco, Numero: arg.Numero, BairroID: arg.BairroID, QuarteiraoID: arg.QuarteiraoID, Lado: arg.Lado,
		CicloID: arg.CicloID, AgenteID: arg.AgenteID, AtividadeID: arg.AtividadeID, Semana: arg.Semana, DataDe: arg.DataDe,
		DataAte: arg.DataAte, Pendencia: arg.Pendencia, Deposito: arg.Deposito, Amostra: arg.Amostra, Tratamento: arg.Tratamento,
		Foto: arg.Foto, Situacao: arg.Situacao,
	})
	if err != nil {
		h.falhaInterna(w, r, "contar visitas", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListVisitas(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar visitas", err)
		return
	}
	for _, v := range rows {
		semana := ""
		if v.Semana.Valid {
			semana = strconv.Itoa(int(v.Semana.Int16))
		}
		d.Linhas = append(d.Linhas, views.VisitaLinha{
			ID: v.ID, Data: dataBR(v.Data), Endereco: enderecoCompleto(v.Endereco, v.Numero), Agente: v.AgenteNome,
			Ciclo: v.CicloNome, Semana: semana, Atividade: v.AtividadeSigla, Tipo: rotuloDe(rotulosTipoVisita, v.TipoVisita),
			Pendencia: rotuloDe(rotulosPendencia, v.Pendencia), Deposito: v.AchouDeposito, Amostra: v.ColetouAmostra,
			Tratou: v.FezTratamento, Ativo: v.Ativo,
		})
	}
	d.Pag = pag

	// Opções dos filtros.
	somente, _ := h.escopoLocais(r)
	ciclos, err := q.ListCiclos(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar ciclos", err)
		return
	}
	for _, c := range ciclos {
		d.Ciclos = append(d.Ciclos, views.CidadeOpcao{ID: c.ID, Rotulo: c.Nome, Selecionada: c.ID == arg.CicloID.Int64})
	}
	atividades, err := q.ListAtividades(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar atividades", err)
		return
	}
	for _, a := range atividades {
		d.Atividades = append(d.Atividades, views.CidadeOpcao{ID: int64(a.ID), Rotulo: a.Sigla + " — " + a.Nome, Selecionada: int64(a.ID) == arg.AtividadeID.Int64})
	}
	usuarios, err := q.ListUsuarios(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar agentes", err)
		return
	}
	for _, u := range usuarios {
		d.Agentes = append(d.Agentes, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome, Selecionada: u.ID == arg.AgenteID.Int64})
	}
	bairros, err := q.ListBairrosAtivosParaLocal(ctx, somente)
	if err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	for _, b := range bairros {
		d.Bairros = append(d.Bairros, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome, Selecionada: b.ID == arg.BairroID.Int64})
	}
	if arg.BairroID.Valid {
		qts, err := q.ListQuarteiroesAtivosDoBairro(ctx, sqlccliente.ListQuarteiroesAtivosDoBairroParams{BairroID: arg.BairroID.Int64, SomenteUsuario: somente})
		if err != nil {
			h.falhaInterna(w, r, "listar quarteirões", err)
			return
		}
		for _, x := range qts {
			d.Quarteiroes = append(d.Quarteiroes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: x.ID == arg.QuarteiraoID.Int64})
		}
	}
	d.Pendencias = marcarOpcao(rotulosPendencia, f.Pendencia)
	d.Deposito, d.Amostra, d.Tratamento, d.Foto = marcarOpcao(opcoesSimNao, f.Deposito), marcarOpcao(opcoesSimNao, f.Amostra),
		marcarOpcao(opcoesSimNao, f.Tratamento), marcarOpcao(opcoesSimNao, f.Foto)

	d.Query = url.Values{}
	for k, v := range map[string]string{
		"id": f.ID, "endereco": f.Endereco, "numero": f.Numero, "bairro": f.Bairro, "quarteirao": f.Quarteirao, "lado": f.Lado,
		"ciclo": f.Ciclo, "agente": f.Agente, "atividade": f.Atividade, "semana": f.Semana, "de": f.De, "ate": f.Ate,
		"pendencia": f.Pendencia, "deposito": f.Deposito, "amostra": f.Amostra, "tratamento": f.Tratamento, "foto": f.Foto,
		"situacao": f.Situacao,
	} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	if d.LocalID != 0 {
		d.Query.Set("local", strconv.FormatInt(d.LocalID, 10))
	}
	render(w, r, views.Visitas(h.layout(r, "Visitas", "/visitas"), d))
}

// mudarAtivoDaVisita desativa ou reativa uma visita (sem apagar) e audita; devolve false se nada mudou.
func (h *handlers) mudarAtivoDaVisita(r *http.Request, q *sqlccliente.Queries, v sqlccliente.Visita, ativo bool) (bool, error) {
	n, err := q.SetVisitaAtiva(r.Context(), sqlccliente.SetVisitaAtivaParams{ID: v.ID, Ativo: ativo})
	if err != nil || n == 0 {
		return false, err
	}
	acao := "visita.desativar"
	if ativo {
		acao = "visita.reativar"
	}
	h.auditar(r, audit.Evento{
		Acao: acao, Entidade: "visita", EntidadeID: strconv.FormatInt(v.ID, 10), Rotulo: "Visita " + strconv.FormatInt(v.ID, 10),
		Antes: map[string]any{"ativo": v.Ativo}, Depois: map[string]any{"ativo": ativo},
	})
	return true, nil
}

// visitaAtivo desativa ou reativa uma visita pela ficha.
func (h *handlers) visitaAtivo(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	v, ok := h.carregarVisita(w, r, q)
	if !ok {
		return
	}
	if ok, msg := h.visitaEditavel(r, q, v); !ok {
		http.Error(w, msg, http.StatusForbidden)
		return
	}
	ativo := r.PostFormValue("ativo") == "1"
	if _, err := h.mudarAtivoDaVisita(r, q, v, ativo); err != nil {
		h.falhaInterna(w, r, "mudar situação da visita", err)
		return
	}
	codigo := map[bool]string{true: "reativada", false: "desativada"}[ativo]
	http.Redirect(w, r, fmt.Sprintf("/visitas/%d?aviso=%s", v.ID, codigo), http.StatusSeeOther)
}

// visitasLote desativa ou reativa várias visitas marcadas na listagem.
// Cada visita passa pelas mesmas regras da ficha (acesso e ciclo fechado); as que não passam são puladas.
func (h *handlers) visitasLote(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	acao := r.PostFormValue("acao")
	if acao != "desativar" && acao != "reativar" {
		http.Error(w, "Ação inválida.", http.StatusBadRequest)
		return
	}
	ids := r.PostForm["ids"]
	if len(ids) == 0 || len(ids) > 500 {
		http.Redirect(w, r, "/visitas?aviso=lote-vazio", http.StatusSeeOther)
		return
	}
	var feitas, puladas int
	for _, txt := range ids {
		id, err := strconv.ParseInt(txt, 10, 64)
		if err != nil {
			puladas++
			continue
		}
		v, err := q.GetVisita(r.Context(), id)
		if err != nil {
			puladas++
			continue
		}
		if ok, err := h.visitaVisivel(r.Context(), r, q, v); err != nil || !ok {
			puladas++
			continue
		}
		if ok, _ := h.visitaEditavel(r, q, v); !ok {
			puladas++
			continue
		}
		mudou, err := h.mudarAtivoDaVisita(r, q, v, acao == "reativar")
		if err != nil {
			h.falhaInterna(w, r, "mudar situação da visita", err)
			return
		}
		if mudou {
			feitas++
		}
	}
	dest := safeVoltar(r.PostFormValue("voltar"))
	sep := "?"
	if strings.Contains(dest, "?") {
		sep = "&"
	}
	http.Redirect(w, r, fmt.Sprintf("%s%saviso=lote-%d-%d", dest, sep, feitas, puladas), http.StatusSeeOther)
}

// safeVoltar aceita só caminhos internos da listagem de visitas.
func safeVoltar(v string) string {
	if strings.HasPrefix(v, "/visitas") && !strings.HasPrefix(v, "//") && !strings.ContainsAny(v, "\r\n") {
		if i := strings.Index(v, "aviso="); i > 0 {
			v = strings.TrimRight(v[:i], "?&")
		}
		return v
	}
	return "/visitas"
}

// avisoDoLoteDeVisitas traduz ?aviso=lote-feitas-puladas numa frase.
func avisoDoLoteDeVisitas(codigo string) string {
	if codigo == "lote-vazio" {
		return "Marque ao menos uma visita."
	}
	var f, p int
	if _, err := fmt.Sscanf(codigo, "lote-%d-%d", &f, &p); err != nil {
		return ""
	}
	msg := fmt.Sprintf("%d visita(s) atualizada(s)", f)
	if p > 0 {
		msg += fmt.Sprintf(" e %d pulada(s) (sem acesso, ciclo fechado ou inexistente)", p)
	}
	return msg + "."
}

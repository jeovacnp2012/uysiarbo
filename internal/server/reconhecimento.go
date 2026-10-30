package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Reconhecimento Geográfico: a ficha de um bairro em um ciclo (imóveis, habitantes, infraestrutura, riscos, animais
// e dados geográficos). Um por bairro e ciclo; "meu bairro" restringe aos bairros em que a pessoa é responsável.

var camposNumericosReconhecimento = []struct {
	Nome, Rotulo string
	Max          int
}{
	{"imoveis_residenciais", "Imóveis residenciais", 999999}, {"imoveis_comerciais", "Imóveis comerciais", 999999},
	{"imoveis_outros", "Outros imóveis", 999999}, {"habitantes", "Habitantes", 9999999}, {"quarteiroes", "Quarteirões", 99999},
	{"pontos_estrategicos", "Pontos estratégicos", 99999}, {"armadilhas", "Armadilhas", 99999}, {"terrenos_baldios", "Terrenos baldios", 99999},
	{"caes", "Cães", 999999}, {"gatos", "Gatos", 999999}, {"roedores", "Roedores", 999999},
}

var infraestruturaReconhecimento = [][2]string{
	{"energia", "Energia elétrica"}, {"agua", "Água tratada"}, {"esgoto", "Esgoto"}, {"lavanderia", "Lavanderia"}, {"privada", "Privada"},
	{"lixo", "Coleta de lixo"}, {"telefonia", "Telefonia"}, {"transporte", "Transporte"}, {"rua", "Rua pavimentada"}, {"escola", "Escola"},
	{"posto_saude", "Posto de saúde"}, {"acesso", "Acesso ao bairro"}, {"pacs", "PACS"},
}

var riscosReconhecimento = [][2]string{
	{"malaria", "Malária"}, {"dengue", "Dengue"}, {"esquistossomose", "Esquistossomose"}, {"leishmaniose", "Leishmaniose"},
	{"febre_maculosa", "Febre maculosa"}, {"peste", "Peste"}, {"chagas", "Doença de Chagas"}, {"febre_amarela", "Febre amarela"},
}

var rotulosSimNaoIgnorado = []views.CidadeOpcao{{Valor: "ignorado", Rotulo: "Ignorado"}, {Valor: "sim", Rotulo: "Sim"}, {Valor: "nao", Rotulo: "Não"}}

func (h *handlers) reconhecimentoEscopo(r *http.Request) pgtype.Int8 {
	if h.principal(r).Restrito("reconhecimento_meubairro") {
		id := meuID(r)
		if id == 0 {
			id = -1
		}
		return pgtype.Int8{Int64: id, Valid: true}
	}
	return pgtype.Int8{}
}

func (h *handlers) reconhecimentosLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	arg := sqlccliente.ListReconhecimentosParams{CicloID: idOpcional(qs.Get("ciclo")), BairroID: idOpcional(qs.Get("bairro")), SomenteUsuario: h.reconhecimentoEscopo(r)}
	rows, err := q.ListReconhecimentos(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar reconhecimentos", err)
		return
	}
	d := views.ReconhecimentosData{PodeAdicionar: h.principal(r).Pode("reconhecimento_adicionar"), Aviso: avisoDeReconhecimento(qs.Get("aviso"))}
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.ReconhecimentoLinha{ID: x.ID, Bairro: x.Bairro, Ciclo: x.Ciclo, Inicio: dataBR(x.DataInicio), Fim: dataBR(x.DataFim),
			Imoveis: int(x.Imoveis), Habitantes: int(x.Habitantes)})
	}
	if d.Ciclos, d.Bairros, err = h.opcoesCicloBairro(ctx, q, r, arg.CicloID.Int64, arg.BairroID.Int64); err != nil {
		h.falhaInterna(w, r, "opções", err)
		return
	}
	lay := h.layout(r, "Reconhecimento geográfico", "/reconhecimentos")
	d.Linhas = paginar(r, &lay, d.Linhas)
	render(w, r, views.Reconhecimentos(lay, d))
}

func avisoDeReconhecimento(codigo string) string {
	switch codigo {
	case "salvo":
		return "Reconhecimento salvo."
	case "excluido":
		return "Reconhecimento enviado para a lixeira."
	}
	return ""
}

// opcoesCicloBairro monta as listas de ciclo e de bairro (só os bairros ao alcance da pessoa).
func (h *handlers) opcoesCicloBairro(ctx context.Context, q *sqlccliente.Queries, r *http.Request, ciclo, bairro int64) ([]views.CidadeOpcao, []views.CidadeOpcao, error) {
	cs, err := q.ListCiclos(ctx)
	if err != nil {
		return nil, nil, err
	}
	somente, _ := h.escopoLocais(r)
	if e := h.reconhecimentoEscopo(r); e.Valid {
		somente = e
	}
	bs, err := q.ListBairrosAtivosParaLocal(ctx, somente)
	if err != nil {
		return nil, nil, err
	}
	var ciclos, bairros []views.CidadeOpcao
	for _, c := range cs {
		ciclos = append(ciclos, views.CidadeOpcao{ID: c.ID, Rotulo: c.Nome, Selecionada: c.ID == ciclo})
	}
	for _, b := range bs {
		bairros = append(bairros, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome, Selecionada: b.ID == bairro})
	}
	return ciclos, bairros, nil
}

func (h *handlers) reconhecimentoNovo(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := views.ReconhecimentoForm{PodeEditar: true, CSRF: csrfToken(r.Context())}
	d.Ciclos, d.Bairros, err = h.opcoesCicloBairro(r.Context(), q, r, 0, 0)
	if err != nil {
		h.falhaInterna(w, r, "opções", err)
		return
	}
	completarReconhecimento(&d, nil, nil, nil)
	render(w, r, views.ReconhecimentoFicha(h.layout(r, "Novo reconhecimento", "/reconhecimentos"), d))
}

func (h *handlers) reconhecimentoFicha(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	x, err := q.GetReconhecimento(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar reconhecimento", err)
		return
	}
	if ok, err := h.bairroDoReconhecimentoVisivel(r.Context(), q, r, x.BairroID); err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	d := views.ReconhecimentoForm{
		ID: x.ID, PodeEditar: h.principal(r).Pode("reconhecimento_adicionar"), CSRF: csrfToken(r.Context()),
		Inicio: dataISO(x.DataInicio), Fim: dataISO(x.DataFim), Observacao: x.Observacao,
		Latitude: floatTexto(x.Latitude), Longitude: floatTexto(x.Longitude), Altitude: floatTexto(x.Altitude), Distancia: floatTexto(x.DistanciaCentro),
		Aviso: avisoDeReconhecimento(r.URL.Query().Get("aviso")),
	}
	d.Ciclos, d.Bairros, err = h.opcoesCicloBairro(r.Context(), q, r, x.CicloID, x.BairroID)
	if err != nil {
		h.falhaInterna(w, r, "opções", err)
		return
	}
	d.Fixo = true
	nums := map[string]pgtype.Int4{
		"imoveis_residenciais": x.ImoveisResidenciais, "imoveis_comerciais": x.ImoveisComerciais, "imoveis_outros": x.ImoveisOutros,
		"habitantes": x.Habitantes, "quarteiroes": x.Quarteiroes, "pontos_estrategicos": x.PontosEstrategicos, "armadilhas": x.Armadilhas,
		"terrenos_baldios": x.TerrenosBaldios, "caes": x.Caes, "gatos": x.Gatos, "roedores": x.Roedores,
	}
	valores := map[string]string{}
	for k, v := range nums {
		if v.Valid {
			valores[k] = strconv.Itoa(int(v.Int32))
		}
	}
	var infra, riscos map[string]string
	_ = json.Unmarshal(x.Infraestrutura, &infra)
	_ = json.Unmarshal(x.Riscos, &riscos)
	completarReconhecimento(&d, valores, infra, riscos)
	render(w, r, views.ReconhecimentoFicha(h.layout(r, "Reconhecimento", "/reconhecimentos"), d))
}

func floatTexto(f pgtype.Float8) string {
	if !f.Valid {
		return ""
	}
	return strconv.FormatFloat(f.Float64, 'f', -1, 64)
}

func completarReconhecimento(d *views.ReconhecimentoForm, numeros, infra, riscos map[string]string) {
	for _, c := range camposNumericosReconhecimento {
		d.Numeros = append(d.Numeros, views.CampoNumero{Nome: c.Nome, Rotulo: c.Rotulo, Valor: numeros[c.Nome]})
	}
	montar := func(itens [][2]string, valores map[string]string) []views.ItemSimNao {
		var out []views.ItemSimNao
		for _, it := range itens {
			v := valores[it[0]]
			if v == "" {
				v = "ignorado"
			}
			opcoes := make([]views.CidadeOpcao, len(rotulosSimNaoIgnorado))
			for i, o := range rotulosSimNaoIgnorado {
				o.Selecionada = o.Valor == v
				opcoes[i] = o
			}
			out = append(out, views.ItemSimNao{Chave: it[0], Rotulo: it[1], Opcoes: opcoes})
		}
		return out
	}
	d.Infraestrutura = montar(infraestruturaReconhecimento, infra)
	d.Riscos = montar(riscosReconhecimento, riscos)
}

func (h *handlers) bairroDoReconhecimentoVisivel(ctx context.Context, q *sqlccliente.Queries, r *http.Request, bairroID int64) (bool, error) {
	e := h.reconhecimentoEscopo(r)
	if !e.Valid {
		return true, nil
	}
	bs, err := q.ListBairrosAtivosParaLocal(ctx, e)
	if err != nil {
		return false, err
	}
	for _, b := range bs {
		if b.ID == bairroID {
			return true, nil
		}
	}
	return false, nil
}

func decimalOpcional(s, nome string, min, max float64) (pgtype.Float8, string) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return pgtype.Float8{}, ""
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < min || f > max {
		return pgtype.Float8{}, fmt.Sprintf("%s: informe um número entre %g e %g.", nome, min, max)
	}
	return pgtype.Float8{Float64: f, Valid: true}, ""
}

func dataOpcionalTexto(s, nome string) (pgtype.Date, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Date{}, ""
	}
	t, err := time.Parse(formatoDataISO, s)
	if err != nil {
		return pgtype.Date{}, nome + ": data inválida."
	}
	return pgtype.Date{Time: t, Valid: true}, ""
}

// reconhecimentoSalvar cria (id vazio) ou atualiza um reconhecimento.
func (h *handlers) reconhecimentoSalvar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	id, _ := idDaURL(r)
	f := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }

	var antes sqlccliente.Reconhecimento
	bairro, ciclo := idOpcional(f("bairro_id")), idOpcional(f("ciclo_id"))
	if id != 0 {
		antes, err = q.GetReconhecimento(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar reconhecimento", err)
			return
		}
		bairro, ciclo = pgtype.Int8{Int64: antes.BairroID, Valid: true}, pgtype.Int8{Int64: antes.CicloID, Valid: true}
	}

	var erros []string
	add := func(m string) {
		if m != "" {
			erros = append(erros, m)
		}
	}
	if !bairro.Valid || !ciclo.Valid {
		add("Escolha o bairro e o ciclo.")
	} else if ok, err := h.bairroDoReconhecimentoVisivel(ctx, q, r, bairro.Int64); err != nil || !ok {
		add("Escolha um bairro da lista.")
	} else if existe, err := q.ExisteReconhecimento(ctx, sqlccliente.ExisteReconhecimentoParams{BairroID: bairro.Int64, CicloID: ciclo.Int64, ID: id}); err == nil && existe {
		add("Já existe um reconhecimento deste bairro neste ciclo.")
	}
	inicio, m := dataOpcionalTexto(f("data_inicio"), "Data de início")
	add(m)
	fim, m := dataOpcionalTexto(f("data_fim"), "Data de fim")
	add(m)
	if inicio.Valid && fim.Valid && fim.Time.Before(inicio.Time) {
		add("A data de fim não pode ser anterior à de início.")
	}
	numeros := map[string]pgtype.Int4{}
	valores := map[string]string{}
	for _, c := range camposNumericosReconhecimento {
		n, m := inteiroOpcional(f(c.Nome), c.Rotulo, c.Max)
		add(m)
		numeros[c.Nome], valores[c.Nome] = n, f(c.Nome)
	}
	lat, m := decimalOpcional(f("latitude"), "Latitude", -90, 90)
	add(m)
	lon, m := decimalOpcional(f("longitude"), "Longitude", -180, 180)
	add(m)
	if lat.Valid != lon.Valid {
		add("Informe latitude e longitude juntas.")
	}
	alt, m := decimalOpcional(f("altitude"), "Altitude", -500, 9000)
	add(m)
	dist, m := decimalOpcional(f("distancia_centro"), "Distância do centro", 0, 5000)
	add(m)
	if len(f("observacao")) > 2000 {
		add("A observação passa de 2000 caracteres.")
	}
	infra, riscos := map[string]string{}, map[string]string{}
	for _, it := range infraestruturaReconhecimento {
		infra[it[0]] = simNaoIgnoradoValido(f("infra_" + it[0]))
	}
	for _, it := range riscosReconhecimento {
		riscos[it[0]] = simNaoIgnoradoValido(f("risco_" + it[0]))
	}

	if len(erros) > 0 {
		d := views.ReconhecimentoForm{ID: id, Fixo: id != 0, PodeEditar: true, CSRF: csrfToken(ctx), Erro: strings.Join(erros, " "),
			Inicio: f("data_inicio"), Fim: f("data_fim"), Latitude: f("latitude"), Longitude: f("longitude"), Altitude: f("altitude"),
			Distancia: f("distancia_centro"), Observacao: f("observacao")}
		d.Ciclos, d.Bairros, _ = h.opcoesCicloBairro(ctx, q, r, ciclo.Int64, bairro.Int64)
		completarReconhecimento(&d, valores, infra, riscos)
		renderStatus(w, r, http.StatusBadRequest, views.ReconhecimentoFicha(h.layout(r, "Reconhecimento", "/reconhecimentos"), d))
		return
	}
	infraJSON, _ := json.Marshal(infra)
	riscosJSON, _ := json.Marshal(riscos)
	rotulo := "Bairro " + strconv.FormatInt(bairro.Int64, 10)
	if x, err := q.GetReconhecimentoQualquer(ctx, id); id != 0 && err == nil {
		rotulo = x.BairroNome + " — " + x.CicloNome
	}
	if id == 0 {
		novo, err := q.CreateReconhecimento(ctx, sqlccliente.CreateReconhecimentoParams{
			BairroID: bairro.Int64, CicloID: ciclo.Int64, DataInicio: inicio, DataFim: fim,
			ImoveisResidenciais: numeros["imoveis_residenciais"], ImoveisComerciais: numeros["imoveis_comerciais"], ImoveisOutros: numeros["imoveis_outros"],
			Habitantes: numeros["habitantes"], Quarteiroes: numeros["quarteiroes"], PontosEstrategicos: numeros["pontos_estrategicos"],
			Armadilhas: numeros["armadilhas"], TerrenosBaldios: numeros["terrenos_baldios"], Caes: numeros["caes"], Gatos: numeros["gatos"], Roedores: numeros["roedores"],
			Latitude: lat, Longitude: lon, Altitude: alt, DistanciaCentro: dist, Infraestrutura: infraJSON, Riscos: riscosJSON,
			Observacao: f("observacao"), CriadoPor: porDoRequest(r),
		})
		if err != nil {
			h.falhaInterna(w, r, "criar reconhecimento", err)
			return
		}
		h.auditar(r, audit.Evento{Acao: "reconhecimento.criar", Entidade: "reconhecimento", EntidadeID: s64txt(novo.ID), Rotulo: rotulo,
			Depois: map[string]any{"bairro": novo.BairroID, "ciclo": novo.CicloID}})
		http.Redirect(w, r, "/reconhecimentos/"+s64txt(novo.ID)+"?aviso=salvo", http.StatusSeeOther)
		return
	}
	novo, err := q.UpdateReconhecimento(ctx, sqlccliente.UpdateReconhecimentoParams{
		ID: id, DataInicio: inicio, DataFim: fim,
		ImoveisResidenciais: numeros["imoveis_residenciais"], ImoveisComerciais: numeros["imoveis_comerciais"], ImoveisOutros: numeros["imoveis_outros"],
		Habitantes: numeros["habitantes"], Quarteiroes: numeros["quarteiroes"], PontosEstrategicos: numeros["pontos_estrategicos"],
		Armadilhas: numeros["armadilhas"], TerrenosBaldios: numeros["terrenos_baldios"], Caes: numeros["caes"], Gatos: numeros["gatos"], Roedores: numeros["roedores"],
		Latitude: lat, Longitude: lon, Altitude: alt, DistanciaCentro: dist, Infraestrutura: infraJSON, Riscos: riscosJSON, Observacao: f("observacao"),
	})
	if err != nil {
		h.falhaInterna(w, r, "atualizar reconhecimento", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "reconhecimento.editar", Entidade: "reconhecimento", EntidadeID: s64txt(novo.ID), Rotulo: rotulo,
		Antes:  map[string]any{"habitantes": antes.Habitantes.Int32, "residenciais": antes.ImoveisResidenciais.Int32},
		Depois: map[string]any{"habitantes": novo.Habitantes.Int32, "residenciais": novo.ImoveisResidenciais.Int32}})
	http.Redirect(w, r, "/reconhecimentos/"+s64txt(novo.ID)+"?aviso=salvo", http.StatusSeeOther)
}

func simNaoIgnoradoValido(v string) string {
	if v == "sim" || v == "nao" {
		return v
	}
	return "ignorado"
}

func (h *handlers) reconhecimentoExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.excluirRegistro(w, r, "reconhecimento", id, "/reconhecimentos?aviso=excluido", "/reconhecimentos/"+s64txt(id))
}

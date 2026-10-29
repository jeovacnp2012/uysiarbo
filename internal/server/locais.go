package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// tiposDeLocal é a lista fechada de tipos (valor gravado, rótulo mostrado).
var tiposDeLocal = [][2]string{
	{"residencial", "Residencial"}, {"comercio", "Comércio"}, {"terreno_baldio", "Terreno baldio"},
	{"ponto_estrategico", "Ponto estratégico"}, {"outro", "Outro"},
}

func rotuloTipoLocal(v string) string {
	for _, t := range tiposDeLocal {
		if t[0] == v {
			return t[1]
		}
	}
	return v
}

// escopoLocais devolve as restrições do papel: "apenas atribuídos" (por bairro) e "apenas ponto estratégico".
func (h *handlers) escopoLocais(r *http.Request) (somente pgtype.Int8, soPE bool) {
	pr := h.principal(r)
	if pr.Restrito("local_meubairro") {
		if id := meuID(r); id != 0 {
			somente = pgtype.Int8{Int64: id, Valid: true}
		} else {
			somente = pgtype.Int8{Int64: -1, Valid: true}
		}
	}
	return somente, pr.Restrito("local_pe")
}

// localVisivel diz se o local está ao alcance de quem pede.
func (h *handlers) localVisivel(ctx context.Context, r *http.Request, q *sqlccliente.Queries, l sqlccliente.Locai) (bool, error) {
	somente, soPE := h.escopoLocais(r)
	if soPE && l.Tipo != "ponto_estrategico" {
		return false, nil
	}
	if somente.Valid {
		resp, err := q.ListResponsaveisDoBairro(ctx, l.BairroID)
		if err != nil {
			return false, err
		}
		return slices.Contains(resp, somente.Int64), nil
	}
	return true, nil
}

// ------------------------------------------------------------------- Lista

func (h *handlers) locaisLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.LocaisFiltro{Busca: strings.TrimSpace(qs.Get("busca")), Bairro: qs.Get("bairro"), Tipo: qs.Get("tipo"), Situacao: qs.Get("situacao")}
	if !slices.ContainsFunc(tiposDeLocal, func(t [2]string) bool { return t[0] == f.Tipo }) {
		f.Tipo = ""
	}
	if f.Situacao != "ativo" && f.Situacao != "inativo" {
		f.Situacao = ""
	}
	bairroID := pgtype.Int8{}
	if n, err := strconv.ParseInt(f.Bairro, 10, 64); err == nil && n > 0 {
		bairroID = pgtype.Int8{Int64: n, Valid: true}
	}
	somente, soPE := h.escopoLocais(r)

	pag := views.Pagination{Page: 1, PerPage: 20}
	if n, err := strconv.Atoi(qs.Get("pagina")); err == nil && n > 1 {
		pag.Page = n
	}
	total, err := q.CountLocais(ctx, sqlccliente.CountLocaisParams{Busca: f.Busca, BairroID: bairroID, Tipo: f.Tipo, Situacao: f.Situacao, SomenteUsuario: somente, SomentePe: soPE})
	if err != nil {
		h.falhaInterna(w, r, "contar locais", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	rows, err := q.ListLocais(ctx, sqlccliente.ListLocaisParams{
		Busca: f.Busca, BairroID: bairroID, Tipo: f.Tipo, Situacao: f.Situacao, SomenteUsuario: somente, SomentePe: soPE,
		Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
	})
	if err != nil {
		h.falhaInterna(w, r, "listar locais", err)
		return
	}
	bairros, err := q.ListBairrosAtivosParaLocal(ctx, somente)
	if err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	opcoes := make([]views.CidadeOpcao, len(bairros))
	for i, b := range bairros {
		opcoes[i] = views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome, Selecionada: b.ID == bairroID.Int64}
	}
	linhas := make([]views.LocalLinha, len(rows))
	for i, l := range rows {
		linhas[i] = views.LocalLinha{
			ID: l.ID, Endereco: enderecoCompleto(l.Endereco, l.Numero), Tipo: rotuloTipoLocal(l.Tipo), Bairro: l.BairroNome,
			Quarteirao: l.QuarteiraoNome, Lado: l.Lado, Sequencia: textoInt(l.Sequencia), Complemento: textoInt(l.Complemento),
			Proprietario: l.Proprietario, Habitantes: textoInt(l.Humanos), Ativo: l.Ativo,
		}
	}
	query := url.Values{}
	for k, v := range map[string]string{"busca": f.Busca, "bairro": f.Bairro, "tipo": f.Tipo, "situacao": f.Situacao} {
		if v != "" {
			query.Set(k, v)
		}
	}
	render(w, r, views.Locais(h.layout(r, "Locais", "/locais"), views.LocaisData{
		Filtro: f, Bairros: opcoes, Tipos: opcoesTipo(f.Tipo), Linhas: linhas, Pag: pag, Query: query,
		PodeEditar: h.principal(r).Pode("local_adicionar"), Restrito: somente.Valid || soPE,
	}))
}

func enderecoCompleto(endereco, numero string) string { return endereco + ", " + numero }

func textoInt(v pgtype.Int4) string {
	if !v.Valid {
		return ""
	}
	return strconv.Itoa(int(v.Int32))
}

func opcoesTipo(atual string) []views.CidadeOpcao {
	out := make([]views.CidadeOpcao, len(tiposDeLocal))
	for i, t := range tiposDeLocal {
		out[i] = views.CidadeOpcao{Rotulo: t[1], Valor: t[0], Selecionada: t[0] == atual}
	}
	return out
}

// ------------------------------------------------------------------- Validação

var (
	reCEP      = regexp.MustCompile(`^[0-9]{5}-?[0-9]{3}$`)
	reSoDigito = regexp.MustCompile(`\D`)
)

// cpfValido confere os dois dígitos verificadores (e recusa sequências repetidas como 111.111.111-11).
func cpfValido(d string) bool {
	if len(d) != 11 || strings.Count(d, string(d[0])) == 11 {
		return false
	}
	dv := func(n int) int {
		soma := 0
		for i := 0; i < n; i++ {
			soma += int(d[i]-'0') * (n + 1 - i)
		}
		r := soma * 10 % 11
		if r == 10 {
			return 0
		}
		return r
	}
	return dv(9) == int(d[9]-'0') && dv(10) == int(d[10]-'0')
}

func formatarCPF(d string) string {
	if len(d) != 11 {
		return d
	}
	return d[:3] + "." + d[3:6] + "." + d[6:9] + "-" + d[9:]
}

// mascararCPF esconde o miolo para a auditoria (dado pessoal não vai inteiro para o histórico).
func mascararCPF(d string) string {
	if len(d) != 11 {
		return ""
	}
	return "***." + d[3:6] + "." + d[6:9] + "-**"
}

func inteiroOpcional(s, nome string, max int) (pgtype.Int4, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Int4{}, ""
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > max {
		return pgtype.Int4{}, fmt.Sprintf("%s: informe um número inteiro de 0 a %d.", nome, max)
	}
	return pgtype.Int4{Int32: int32(n), Valid: true}, ""
}

func lerLocal(r *http.Request) views.LocalCampos {
	v := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	return views.LocalCampos{
		Tipo: v("tipo"), CEP: v("cep"), Endereco: v("endereco"), Numero: v("numero"), Referencia: v("referencia"),
		BairroID: v("bairro_id"), QuarteiraoID: v("quarteirao_id"), Lado: v("lado"), Sequencia: v("sequencia"), Complemento: v("complemento"),
		Latitude: v("latitude"), Longitude: v("longitude"),
		Proprietario: v("proprietario"), ProprietarioTelefone: v("proprietario_telefone"), ProprietarioCPF: v("proprietario_cpf"),
		Whatsapp: v("proprietario_whatsapp"), Email: v("proprietario_email"),
		AceitaWhatsapp: r.PostFormValue("aceita_whatsapp") == "1", AceitaEmail: r.PostFormValue("aceita_email") == "1",
		Inquilino: v("inquilino"), InquilinoTelefone: v("inquilino_telefone"),
		Humanos: v("humanos"), Cachorros: v("cachorros"), Gatos: v("gatos"), Roedores: v("roedores"),
	}
}

func camposDoLocal(l sqlccliente.Locai) views.LocalCampos {
	return views.LocalCampos{
		Tipo: l.Tipo, CEP: l.Cep, Endereco: l.Endereco, Numero: l.Numero, Referencia: l.Referencia,
		BairroID: strconv.FormatInt(l.BairroID, 10), QuarteiraoID: strconv.FormatInt(l.QuarteiraoID, 10), Lado: l.Lado, Sequencia: textoInt(l.Sequencia), Complemento: textoInt(l.Complemento),
		Latitude: textoFloat(l.Latitude), Longitude: textoFloat(l.Longitude),
		Proprietario: l.Proprietario, ProprietarioTelefone: l.ProprietarioTelefone, ProprietarioCPF: formatarCPF(l.ProprietarioCpf),
		Whatsapp: l.ProprietarioWhatsapp, Email: l.ProprietarioEmail, AceitaWhatsapp: l.AceitaWhatsapp, AceitaEmail: l.AceitaEmail,
		Inquilino: l.Inquilino, InquilinoTelefone: l.InquilinoTelefone,
		Humanos: textoInt(l.Humanos), Cachorros: textoInt(l.Cachorros), Gatos: textoInt(l.Gatos), Roedores: textoInt(l.Roedores),
	}
}

// validarLocal confere os campos. bairroAtual/quarteiraoAtual deixam um local antigo continuar num bairro
// extinto ou num quarteirão desativado, desde que a pessoa não mude essa escolha.
func validarLocal(ctx context.Context, q *sqlccliente.Queries, c views.LocalCampos, bairroAtual, quarteiraoAtual int64) (sqlccliente.CreateLocalParams, string, error) {
	var p sqlccliente.CreateLocalParams
	if !slices.ContainsFunc(tiposDeLocal, func(t [2]string) bool { return t[0] == c.Tipo }) {
		return p, "Escolha o tipo do local.", nil
	}
	switch {
	case len([]rune(c.Endereco)) < 2 || len([]rune(c.Endereco)) > 200:
		return p, "Informe o endereço (rua ou avenida), de 2 a 200 caracteres.", nil
	case c.Numero == "" || len([]rune(c.Numero)) > 15:
		return p, "Informe o número (use S/N quando não houver), com até 15 caracteres.", nil
	case c.Lado == "" || len([]rune(c.Lado)) > 5:
		return p, "Informe o lado da quadra (até 5 caracteres, como Par, Ímpar, A).", nil
	case len([]rune(c.Referencia)) > 200:
		return p, "A referência pode ter no máximo 200 caracteres.", nil
	}
	cep := ""
	if c.CEP != "" {
		if !reCEP.MatchString(c.CEP) {
			return p, "CEP inválido: use 8 números, como 78450-000.", nil
		}
		d := reSoDigito.ReplaceAllString(c.CEP, "")
		cep = d[:5] + "-" + d[5:]
	}

	bairroID, _ := strconv.ParseInt(c.BairroID, 10, 64)
	quarteiraoID, _ := strconv.ParseInt(c.QuarteiraoID, 10, 64)
	b, err := q.GetBairro(ctx, bairroID)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, "Escolha o bairro.", nil
	} else if err != nil {
		return p, "", err
	}
	if b.Status != "ativo" && b.ID != bairroAtual {
		return p, "Escolha um bairro ativo.", nil
	}
	qt, err := q.GetQuarteirao(ctx, quarteiraoID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && qt.BairroID != b.ID) {
		return p, "Escolha um quarteirão do bairro selecionado.", nil
	} else if err != nil {
		return p, "", err
	}
	if !qt.Ativo && qt.ID != quarteiraoAtual {
		return p, "Esse quarteirão está desativado. Escolha outro.", nil
	}

	seq, msg := inteiroOpcional(c.Sequencia, "Sequência", 99999)
	if msg != "" {
		return p, msg, nil
	}
	comp, msg := inteiroOpcional(c.Complemento, "Complemento", 99999)
	if msg != "" {
		return p, msg, nil
	}
	lat, msg := numeroOpcional(c.Latitude, "Latitude", -90, 90)
	if msg != "" {
		return p, msg, nil
	}
	lon, msg := numeroOpcional(c.Longitude, "Longitude", -180, 180)
	if msg != "" {
		return p, msg, nil
	}
	if lat.Valid != lon.Valid {
		return p, "Informe latitude e longitude juntas, ou deixe as duas em branco.", nil
	}
	var cont [4]pgtype.Int4
	for i, campo := range []struct{ v, nome string }{{c.Humanos, "Humanos"}, {c.Cachorros, "Cachorros"}, {c.Gatos, "Gatos"}, {c.Roedores, "Roedores"}} {
		if cont[i], msg = inteiroOpcional(campo.v, campo.nome, 1000); msg != "" {
			return p, msg, nil
		}
	}

	cpf := reSoDigito.ReplaceAllString(c.ProprietarioCPF, "")
	if cpf != "" && !cpfValido(cpf) {
		return p, "CPF inválido: confira os números.", nil
	}
	if c.Email != "" {
		if a, err := mail.ParseAddress(c.Email); err != nil || a.Address != c.Email || len(c.Email) > 150 {
			return p, "E-mail do proprietário inválido.", nil
		}
	}
	for _, t := range []struct{ v, nome string }{{c.Proprietario, "O nome do proprietário"}, {c.Inquilino, "O nome do inquilino"}} {
		if len([]rune(t.v)) > 150 {
			return p, t.nome + " pode ter no máximo 150 caracteres.", nil
		}
	}
	for _, t := range []string{c.ProprietarioTelefone, c.Whatsapp, c.InquilinoTelefone} {
		if len([]rune(t)) > 20 {
			return p, "Telefone: no máximo 20 caracteres.", nil
		}
	}

	return sqlccliente.CreateLocalParams{
		Tipo: c.Tipo, Cep: cep, Endereco: c.Endereco, Numero: c.Numero, Referencia: c.Referencia, BairroID: b.ID, QuarteiraoID: qt.ID,
		Lado: c.Lado, Sequencia: seq, Complemento: comp, Latitude: lat, Longitude: lon,
		Proprietario: c.Proprietario, ProprietarioTelefone: c.ProprietarioTelefone, ProprietarioCpf: cpf,
		ProprietarioWhatsapp: c.Whatsapp, ProprietarioEmail: c.Email, AceitaWhatsapp: c.AceitaWhatsapp, AceitaEmail: c.AceitaEmail,
		Inquilino: c.Inquilino, InquilinoTelefone: c.InquilinoTelefone,
		Humanos: cont[0], Cachorros: cont[1], Gatos: cont[2], Roedores: cont[3],
	}, "", nil
}

// dadosAuditoriaLocal descreve o local para o histórico; o CPF entra mascarado.
func dadosAuditoriaLocal(p sqlccliente.CreateLocalParams, bairro, quarteirao string) map[string]any {
	n := func(v pgtype.Int4) any {
		if !v.Valid {
			return nil
		}
		return v.Int32
	}
	f := func(v pgtype.Float8) any {
		if !v.Valid {
			return nil
		}
		return v.Float64
	}
	return map[string]any{
		"tipo": rotuloTipoLocal(p.Tipo), "cep": p.Cep, "endereco": p.Endereco, "numero": p.Numero, "referencia": p.Referencia,
		"bairro": bairro, "quarteirao": quarteirao, "lado": p.Lado, "sequencia": n(p.Sequencia), "complemento": n(p.Complemento),
		"latitude": f(p.Latitude), "longitude": f(p.Longitude), "proprietario": p.Proprietario,
		"proprietario_telefone": p.ProprietarioTelefone, "proprietario_cpf": mascararCPF(p.ProprietarioCpf),
		"proprietario_whatsapp": p.ProprietarioWhatsapp, "proprietario_email": p.ProprietarioEmail,
		"aceita_whatsapp": p.AceitaWhatsapp, "aceita_email": p.AceitaEmail, "inquilino": p.Inquilino, "inquilino_telefone": p.InquilinoTelefone,
		"humanos": n(p.Humanos), "cachorros": n(p.Cachorros), "gatos": n(p.Gatos), "roedores": n(p.Roedores),
	}
}

func paramsDoLocal(l sqlccliente.Locai) sqlccliente.CreateLocalParams {
	return sqlccliente.CreateLocalParams{
		Tipo: l.Tipo, Cep: l.Cep, Endereco: l.Endereco, Numero: l.Numero, Referencia: l.Referencia, BairroID: l.BairroID, QuarteiraoID: l.QuarteiraoID,
		Lado: l.Lado, Sequencia: l.Sequencia, Complemento: l.Complemento, Latitude: l.Latitude, Longitude: l.Longitude,
		Proprietario: l.Proprietario, ProprietarioTelefone: l.ProprietarioTelefone, ProprietarioCpf: l.ProprietarioCpf,
		ProprietarioWhatsapp: l.ProprietarioWhatsapp, ProprietarioEmail: l.ProprietarioEmail, AceitaWhatsapp: l.AceitaWhatsapp, AceitaEmail: l.AceitaEmail,
		Inquilino: l.Inquilino, InquilinoTelefone: l.InquilinoTelefone, Humanos: l.Humanos, Cachorros: l.Cachorros, Gatos: l.Gatos, Roedores: l.Roedores,
	}
}

// ------------------------------------------------------------------- Formulário

func (h *handlers) montarFormLocal(ctx context.Context, r *http.Request, q *sqlccliente.Queries, id int64, c views.LocalCampos, erro string) (views.LocalFormData, error) {
	somente, _ := h.escopoLocais(r)
	bairros, err := q.ListBairrosAtivosParaLocal(ctx, somente)
	if err != nil {
		return views.LocalFormData{}, err
	}
	escolhido, _ := strconv.ParseInt(c.BairroID, 10, 64)
	opcoes := make([]views.CidadeOpcao, 0, len(bairros)+1)
	achou := false
	for _, b := range bairros {
		achou = achou || b.ID == escolhido
		opcoes = append(opcoes, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome + " — " + b.CidadeNome + "/" + b.CidadeUf, Selecionada: b.ID == escolhido})
	}
	if !achou && escolhido != 0 { // bairro extinto onde o local já estava
		if b, err := q.GetBairro(ctx, escolhido); err == nil {
			opcoes = append(opcoes, views.CidadeOpcao{ID: b.ID, Rotulo: b.Nome + " (extinto)", Selecionada: true})
		}
	}
	form := views.LocalFormData{
		ID: id, Campos: c, Bairros: opcoes, Tipos: opcoesTipo(c.Tipo), PodeEditar: h.principal(r).Pode("local_adicionar"),
		Erro: erro, CSRF: csrfToken(ctx),
	}
	if escolhido != 0 {
		qs, err := q.ListQuarteiroesAtivosDoBairro(ctx, escolhido)
		if err != nil {
			return form, err
		}
		atual, _ := strconv.ParseInt(c.QuarteiraoID, 10, 64)
		achou = false
		for _, x := range qs {
			achou = achou || x.ID == atual
			form.Quarteiroes = append(form.Quarteiroes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome, Selecionada: x.ID == atual})
		}
		if !achou && atual != 0 { // quarteirão desativado onde o local já estava
			if x, err := q.GetQuarteirao(ctx, atual); err == nil && x.BairroID == escolhido {
				form.Quarteiroes = append(form.Quarteiroes, views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome + " (desativado)", Selecionada: true})
			}
		}
	}
	return form, nil
}

func (h *handlers) localNovoForm(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	c := views.LocalCampos{Tipo: "residencial"}
	if b := r.URL.Query().Get("bairro"); b != "" {
		c.BairroID = b
	}
	form, err := h.montarFormLocal(r.Context(), r, q, 0, c, "")
	if err != nil {
		h.falhaInterna(w, r, "montar formulário", err)
		return
	}
	render(w, r, views.LocalForm(h.layout(r, "Novo local", "/locais"), form))
}

func (h *handlers) localEditarForm(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	l, err := q.GetLocal(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar local", err)
		return
	}
	if visivel, err := h.localVisivel(ctx, r, q, l); err != nil {
		h.falhaInterna(w, r, "escopo do local", err)
		return
	} else if !visivel {
		http.NotFound(w, r)
		return
	}
	form, err := h.montarFormLocal(ctx, r, q, id, camposDoLocal(l), "")
	if err != nil {
		h.falhaInterna(w, r, "montar formulário", err)
		return
	}
	render(w, r, views.LocalForm(h.layout(r, "Local", "/locais"), form))
}

// localQuarteiroes devolve as opções de quarteirão do bairro escolhido (HTMX, ao trocar o bairro).
func (h *handlers) localQuarteiroes(w http.ResponseWriter, r *http.Request) {
	bid, err := strconv.ParseInt(r.URL.Query().Get("bairro_id"), 10, 64)
	if err != nil || bid <= 0 {
		render(w, r, views.OpcoesDeQuarteirao(nil))
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if visivel, err := h.bairroVisivelParaLocal(r.Context(), r, q, bid); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return
	} else if !visivel {
		http.NotFound(w, r)
		return
	}
	qs, err := q.ListQuarteiroesAtivosDoBairro(r.Context(), bid)
	if err != nil {
		h.falhaInterna(w, r, "listar quarteirões", err)
		return
	}
	ops := make([]views.CidadeOpcao, len(qs))
	for i, x := range qs {
		ops[i] = views.CidadeOpcao{ID: x.ID, Rotulo: x.Nome}
	}
	render(w, r, views.OpcoesDeQuarteirao(ops))
}

// bairroVisivelParaLocal respeita "local: apenas atribuídos" (os bairros em que a pessoa é responsável).
func (h *handlers) bairroVisivelParaLocal(ctx context.Context, r *http.Request, q *sqlccliente.Queries, bairroID int64) (bool, error) {
	somente, _ := h.escopoLocais(r)
	if !somente.Valid {
		return true, nil
	}
	resp, err := q.ListResponsaveisDoBairro(ctx, bairroID)
	if err != nil {
		return false, err
	}
	return slices.Contains(resp, somente.Int64), nil
}

// ------------------------------------------------------------------- Salvar

func (h *handlers) localSalvar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	var id int64
	if chi.URLParam(r, "id") != "" {
		var ok bool
		if id, ok = idDaURL(r); !ok {
			http.NotFound(w, r)
			return
		}
	}
	q, pool, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	var atual sqlccliente.Locai
	if id != 0 {
		if atual, err = q.GetLocal(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "buscar local", err)
			return
		}
		if visivel, err := h.localVisivel(ctx, r, q, atual); err != nil {
			h.falhaInterna(w, r, "escopo do local", err)
			return
		} else if !visivel {
			http.NotFound(w, r)
			return
		}
	}

	c := lerLocal(r)
	falha := func(msg string) {
		form, e := h.montarFormLocal(ctx, r, q, id, c, msg)
		if e != nil {
			h.falhaInterna(w, r, "montar formulário", e)
			return
		}
		renderStatus(w, r, http.StatusBadRequest, views.LocalForm(h.layout(r, "Local", "/locais"), form))
	}
	p, msg, err := validarLocal(ctx, q, c, atual.BairroID, atual.QuarteiraoID)
	if err != nil {
		h.falhaInterna(w, r, "validar local", err)
		return
	}
	if msg != "" {
		falha(msg)
		return
	}
	// "apenas atribuídos": não dá para pôr um local num bairro que a pessoa não enxerga.
	if ok, err := h.bairroVisivelParaLocal(ctx, r, q, p.BairroID); err != nil {
		h.falhaInterna(w, r, "escopo do bairro", err)
		return
	} else if !ok {
		falha("Escolha um bairro em que você é responsável.")
		return
	}
	if _, soPE := h.escopoLocais(r); soPE && p.Tipo != "ponto_estrategico" {
		falha("O seu papel só cadastra pontos estratégicos.")
		return
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		h.falhaInterna(w, r, "iniciar transação", err)
		return
	}
	defer tx.Rollback(ctx)
	qt := sqlccliente.New(tx)
	const duplicado = "Já existe um local com esse endereço, número, lado, sequência e complemento nesse quarteirão."
	novoID := id
	if id == 0 {
		p.CriadoPor = pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}
		l, err := qt.CreateLocal(ctx, p)
		if violacaoUnica(err) {
			falha(duplicado)
			return
		} else if err != nil {
			h.falhaInterna(w, r, "criar local", err)
			return
		}
		novoID = l.ID
	} else if _, err := qt.UpdateLocal(ctx, sqlccliente.UpdateLocalParams{
		ID: id, Tipo: p.Tipo, Cep: p.Cep, Endereco: p.Endereco, Numero: p.Numero, Referencia: p.Referencia, BairroID: p.BairroID,
		QuarteiraoID: p.QuarteiraoID, Lado: p.Lado, Sequencia: p.Sequencia, Complemento: p.Complemento, Latitude: p.Latitude,
		Longitude: p.Longitude, Proprietario: p.Proprietario, ProprietarioTelefone: p.ProprietarioTelefone, ProprietarioCpf: p.ProprietarioCpf,
		ProprietarioWhatsapp: p.ProprietarioWhatsapp, ProprietarioEmail: p.ProprietarioEmail, AceitaWhatsapp: p.AceitaWhatsapp,
		AceitaEmail: p.AceitaEmail, Inquilino: p.Inquilino, InquilinoTelefone: p.InquilinoTelefone, Humanos: p.Humanos,
		Cachorros: p.Cachorros, Gatos: p.Gatos, Roedores: p.Roedores,
	}); violacaoUnica(err) {
		falha(duplicado)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "salvar local", err)
		return
	}

	nomeBairro, nomeQuart := nomesDoLocal(ctx, qt, p.BairroID, p.QuarteiraoID)
	depois := dadosAuditoriaLocal(p, nomeBairro, nomeQuart)
	rotulo := enderecoCompleto(p.Endereco, p.Numero)
	ev := audit.Evento{Entidade: "local", EntidadeID: strconv.FormatInt(novoID, 10), Rotulo: rotulo}
	if id == 0 {
		ev.Acao, ev.Depois = "local.criar", depois
	} else {
		bAntes, qAntes := nomesDoLocal(ctx, qt, atual.BairroID, atual.QuarteiraoID)
		a, dep := audit.Diferenca(dadosAuditoriaLocal(paramsDoLocal(atual), bAntes, qAntes), depois)
		if len(a)+len(dep) == 0 {
			naoAuditar(r)
		} else {
			ev.Acao, ev.Antes, ev.Depois = "local.editar", a, dep
		}
	}
	if ev.Acao != "" {
		h.auditarCom(audit.Cliente{Q: qt}, r, ev)
	}
	if err := tx.Commit(ctx); err != nil {
		h.falhaInterna(w, r, "confirmar local", err)
		return
	}
	http.Redirect(w, r, "/locais", http.StatusSeeOther)
}

func nomesDoLocal(ctx context.Context, q *sqlccliente.Queries, bairroID, quarteiraoID int64) (string, string) {
	var b, qu string
	if x, err := q.GetBairro(ctx, bairroID); err == nil {
		b = x.Nome
	}
	if x, err := q.GetQuarteirao(ctx, quarteiraoID); err == nil {
		qu = x.Nome
	}
	return b, qu
}

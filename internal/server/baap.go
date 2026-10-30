package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// BAAP (9.5): busca ativa de acidentes com animais peçonhentos. Cada registro é a notificação de um acidente; o andamento
// vem da ficha comum de casos (9.1). Dados do paciente (nome, mãe, nascimento, telefone) nunca vão para a auditoria.

var rotulosSituacaoBaap = []views.CidadeOpcao{
	{Valor: "aguardando", Rotulo: "Aguardando"},
	{Valor: "em_atendimento", Rotulo: "Em atendimento"},
	{Valor: "encerrada", Rotulo: "Encerrada"},
}

var animaisBaap = opcoesSimples("Serpente", "Aranha", "Escorpião", "Lagarta", "Abelha", "Lacraia", "Outro", "Ignorado")

var locaisDePicada = opcoesSimples("Cabeça", "Braço", "Ante-Braço", "Mão", "Dedo da Mão", "Tronco", "Coxa", "Perna", "Pé", "Dedo do Pé", "Ignorado")

func opcoesSimples(valores ...string) []views.CidadeOpcao {
	out := make([]views.CidadeOpcao, len(valores))
	for i, v := range valores {
		out[i] = views.CidadeOpcao{Valor: v, Rotulo: v}
	}
	return out
}

func init() {
	registrarTipoDeCaso(tipoDeCaso{
		Slug: "baap", Rotulo: "Animal peçonhento",
		Ver: "zoonose_baap_visualizar", Editar: "zoonose_baap_anotacao", Encaminhar: "zoonose_baap_encaminhar",
		Existe: func(ctx context.Context, q *sqlccliente.Queries, id int64) bool {
			_, err := q.GetBaap(ctx, id)
			return err == nil
		},
		Caminho: func(id int64) string { return "/zoonose/baap/" + strconv.FormatInt(id, 10) },
	})
}

// escopoBaap: quem só vê as próprias notificações (criadas ou atribuídas a si).
func (h *handlers) escopoBaap(r *http.Request) pgtype.Int8 {
	if h.principal(r).Pode("zoonose_baap_visualizar_minha") {
		return pgtype.Int8{Int64: meuID(r), Valid: true}
	}
	return pgtype.Int8{}
}

func (h *handlers) baapVisivel(r *http.Request, b sqlccliente.BaapNotificaco) bool {
	if !h.principal(r).Pode("zoonose_baap_visualizar_minha") {
		return true
	}
	me := meuID(r)
	return b.CriadoPor.Int64 == me || b.ResponsavelID.Int64 == me
}

func ordemDeBaap(ordem, dir string) (string, string) {
	switch ordem {
	case "id", "paciente", "animal", "data", "situacao":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func rotuloBaap(b sqlccliente.BaapNotificaco) string {
	return "Animal peçonhento " + strconv.FormatInt(b.ID, 10)
}

func (h *handlers) baapLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.BaapFiltro{
		Busca: strings.TrimSpace(qs.Get("busca")), Situacao: qs.Get("situacao"), Animal: qs.Get("animal"), Bairro: qs.Get("bairro"),
		De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate")),
	}
	if !validoEm(rotulosSituacaoBaap, f.Situacao) {
		f.Situacao = ""
	}
	if !validoEm(animaisBaap, f.Animal) {
		f.Animal = ""
	}
	ordem, dir := ordemDeBaap(qs.Get("ordem"), qs.Get("dir"))
	minhas := h.escopoBaap(r)
	arg := sqlccliente.ListBaapParams{
		Busca: f.Busca, Situacao: f.Situacao, Animal: f.Animal, BairroID: idOpcional(f.Bairro), DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate),
		SomenteMinhas: minhas.Valid, UsuarioID: minhas.Int64, Ordem: ordem, Dir: dir,
	}
	pag := paginaDe(r)
	total, err := q.CountBaap(ctx, sqlccliente.CountBaapParams{
		Busca: arg.Busca, Situacao: arg.Situacao, Animal: arg.Animal, BairroID: arg.BairroID, DataDe: arg.DataDe, DataAte: arg.DataAte,
		SomenteMinhas: arg.SomenteMinhas, UsuarioID: arg.UsuarioID,
	})
	if err != nil {
		h.falhaInterna(w, r, "contar notificações de animais peçonhentos", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListBaap(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar notificações de animais peçonhentos", err)
		return
	}
	d := views.BaapData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: h.principal(r).Pode("zoonose_baap_adicionar")}
	for _, x := range rows {
		animal := x.Animal
		if x.Animal == "Outro" && x.AnimalOutro != "" {
			animal = x.AnimalOutro
		}
		d.Linhas = append(d.Linhas, views.BaapLinha{
			ID: x.ID, IDTexto: strconv.FormatInt(x.ID, 10), Sinam: x.Sinam, Paciente: x.Paciente, Animal: animal, Data: dataBR(x.DataNotificado),
			Endereco: x.Endereco, Bairro: x.Bairro, Situacao: x.Situacao, SituacaoRotulo: rotuloDe(rotulosSituacaoBaap, x.Situacao), Responsavel: x.Responsavel,
		})
	}
	if d.Bairros, err = h.opcoesDeBairros(r, q, arg.BairroID.Int64); err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	d.Situacoes, d.Animais = marcarOpcao(rotulosSituacaoBaap, f.Situacao), marcarOpcao(animaisBaap, f.Animal)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "situacao": f.Situacao, "animal": f.Animal, "bairro": f.Bairro, "de": f.De, "ate": f.Ate} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Baap(h.layout(r, "Animais peçonhentos", "/zoonose/baap"), d))
}

func (h *handlers) baapNova(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d, err := h.formularioDeBaap(r, q, views.BaapFormData{Data: h.hojeNoFuso(r).Time.Format("2006-01-02"), Animal: "", LocalPicada: "Ignorado"}, 0)
	if err != nil {
		h.falhaInterna(w, r, "montar formulário", err)
		return
	}
	render(w, r, views.BaapNova(h.layout(r, "Nova notificação de animal peçonhento", "/zoonose/baap"), d))
}

func (h *handlers) formularioDeBaap(r *http.Request, q *sqlccliente.Queries, d views.BaapFormData, bairro int64) (views.BaapFormData, error) {
	d.CSRF = csrfToken(r.Context())
	d.Animais = marcarOpcao(animaisBaap, d.Animal)
	d.LocaisPicada = marcarOpcao(locaisDePicada, d.LocalPicada)
	var err error
	d.Bairros, err = h.opcoesDeBairros(r, q, bairro)
	return d, err
}

// lerDadosDoBaap lê e valida o formulário. Devolve a mensagem de erro, se houver.
func (h *handlers) lerDadosDoBaap(r *http.Request, q *sqlccliente.Queries, id int64) (views.BaapFormData, sqlccliente.CreateBaapParams, string) {
	t := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	d := views.BaapFormData{
		ID: id, Sinam: t("sinam"), Data: t("data"), Paciente: t("paciente"), NomeMae: t("nome_mae"), Nascimento: t("nascimento"),
		Telefone: t("telefone"), Telefone2: t("telefone2"), Animal: t("animal"), AnimalOutro: t("animal_outro"), Especie: t("especie"),
		LocalPicada: t("local_picada"), SituacaoPaciente: t("situacao_paciente"), AtuacaoVGA: t("atuacao_vga"), Observacao: t("observacao"),
		Endereco: t("endereco"), Numero: t("numero"), Referencia: t("referencia"),
	}
	bairro := idOpcional(t("bairro"))
	if d.Animal != "Outro" {
		d.AnimalOutro = ""
	}
	p := sqlccliente.CreateBaapParams{
		Sinam: d.Sinam, DataNotificado: dataOpcional(d.Data), Paciente: d.Paciente, NomeMae: d.NomeMae, DataNascimento: dataOpcional(d.Nascimento),
		Telefone: d.Telefone, Telefone2: d.Telefone2, Animal: d.Animal, AnimalOutro: d.AnimalOutro, Especie: d.Especie, LocalPicada: d.LocalPicada,
		SituacaoPaciente: d.SituacaoPaciente, AtuacaoVga: d.AtuacaoVGA, Observacao: d.Observacao, EnderecoTexto: d.Endereco, Numero: d.Numero,
		Referencia: d.Referencia, BairroID: bairro,
	}
	curto := func(s string, n int) bool { return len([]rune(s)) > n }
	msg := ""
	switch {
	case d.Paciente == "" || curto(d.Paciente, 150):
		msg = "Informe o paciente (até 150 caracteres)."
	case !validoEm(animaisBaap, d.Animal):
		msg = "Escolha o animal."
	case !validoEm(locaisDePicada, d.LocalPicada):
		msg = "Escolha o local da picada."
	case !p.DataNotificado.Valid:
		msg = "Informe a data da notificação."
	case d.Nascimento != "" && !p.DataNascimento.Valid:
		msg = "Data de nascimento inválida."
	case d.Animal == "Outro" && d.AnimalOutro == "":
		msg = "Diga qual é o animal."
	case curto(d.NomeMae, 150) || curto(d.Telefone, 40) || curto(d.Telefone2, 40) || curto(d.Sinam, 30) || curto(d.AnimalOutro, 100) || curto(d.Especie, 100):
		msg = "Algum campo passa do tamanho permitido."
	case curto(d.SituacaoPaciente, 2000) || curto(d.AtuacaoVGA, 200) || curto(d.Observacao, 4000):
		msg = "Algum texto passa do tamanho permitido."
	case curto(d.Endereco, 200) || curto(d.Numero, 20) || curto(d.Referencia, 200):
		msg = "Algum campo do endereço passa do tamanho permitido."
	case bairro.Valid && !h.bairroEscolhivel(r, q, bairro.Int64):
		msg = "O bairro escolhido não existe."
	}
	d, _ = h.formularioDeBaap(r, q, d, bairro.Int64)
	return d, p, msg
}

func (h *handlers) baapCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d, p, msg := h.lerDadosDoBaap(r, q, 0)
	if msg != "" {
		d.Erro = msg
		renderStatus(w, r, http.StatusBadRequest, views.BaapNova(h.layout(r, "Nova notificação de animal peçonhento", "/zoonose/baap"), d))
		return
	}
	p.CriadoPor = pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}
	n, err := q.CreateBaap(r.Context(), p)
	if err != nil {
		h.falhaInterna(w, r, "criar notificação de animal peçonhento", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "baap", n.ID, "criacao", "", "", "Aguardando", 0)
	h.auditar(r, audit.Evento{Acao: "baap.criar", Entidade: "baap", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloBaap(n),
		Depois: map[string]any{"animal": n.Animal, "local_picada": n.LocalPicada}})
	http.Redirect(w, r, "/zoonose/baap/"+strconv.FormatInt(n.ID, 10), http.StatusSeeOther)
}

func (h *handlers) carregarBaap(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.BaapNotificaco, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.BaapNotificaco{}, false
	}
	n, err := q.GetBaap(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !h.baapVisivel(r, n)) {
		http.NotFound(w, r)
		return n, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar notificação de animal peçonhento", err)
		return n, false
	}
	return n, true
}

func avisoDoBaap(c string) string {
	switch c {
	case "local":
		return "Local atualizado."
	case "visita":
		return "Visita atualizada."
	case "dados":
		return "Dados salvos."
	case "situacao":
		return "Situação atualizada."
	}
	return ""
}

func (h *handlers) baapFicha(w http.ResponseWriter, r *http.Request) { h.mostrarBaap(w, r, nil) }

func (h *handlers) mostrarBaap(w http.ResponseWriter, r *http.Request, dadosForm *views.BaapFormData) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarBaap(w, r, q)
	if !ok {
		return
	}
	pr := h.principal(r)
	fechada := n.Situacao == "encerrada"
	animal := n.Animal
	if n.Animal == "Outro" && n.AnimalOutro != "" {
		animal = n.AnimalOutro
	}
	if n.Especie != "" {
		animal += " (" + n.Especie + ")"
	}
	d := views.BaapFichaData{
		ID: n.ID, IDTexto: strconv.FormatInt(n.ID, 10), Sinam: n.Sinam, Paciente: n.Paciente, Animal: animal, Especie: n.Especie,
		LocalPicada: n.LocalPicada, Data: dataBR(n.DataNotificado), Telefone: n.Telefone, Endereco: strings.TrimSpace(n.EnderecoTexto + " " + n.Numero),
		VisitaID: n.VisitaID.Int64, Situacao: n.Situacao, SituacaoRotulo: rotuloDe(rotulosSituacaoBaap, n.Situacao), Fechada: fechada,
		PodeEditar: pr.Pode("zoonose_baap_editar") && !fechada, PodeAtender: pr.Pode("zoonose_baap_atender"),
		PodeEncerrar: pr.Pode("zoonose_baap_encerrar"), PodeLocal: pr.Pode("zoonose_baap_local") && !fechada,
		PodeVisita: pr.Pode("zoonose_baap_atender") && !fechada, CSRF: csrfToken(ctx), Aviso: avisoDoBaap(r.URL.Query().Get("aviso")),
	}
	if n.ResponsavelID.Valid {
		if u, err := q.GetUsuarioByID(ctx, n.ResponsavelID.Int64); err == nil {
			d.Responsavel = u.Nome
		}
	}
	if n.BairroID.Valid {
		if b, err := q.GetBairro(ctx, n.BairroID.Int64); err == nil {
			d.Bairro = b.Nome
		}
	}
	if n.LocalID.Valid {
		d.LocalID = n.LocalID.Int64
		if l, err := q.GetLocalQualquer(ctx, n.LocalID.Int64); err == nil {
			d.LocalEndereco = enderecoCompleto(l.Endereco, l.Numero)
		}
	}
	if dadosForm != nil {
		d.Dados = *dadosForm
	} else if d.PodeEditar {
		f := views.BaapFormData{
			ID: n.ID, Sinam: n.Sinam, Data: dataISO(n.DataNotificado), Paciente: n.Paciente, NomeMae: n.NomeMae, Nascimento: dataISO(n.DataNascimento),
			Telefone: n.Telefone, Telefone2: n.Telefone2, Animal: n.Animal, AnimalOutro: n.AnimalOutro, Especie: n.Especie, LocalPicada: n.LocalPicada,
			SituacaoPaciente: n.SituacaoPaciente, AtuacaoVGA: n.AtuacaoVga, Observacao: n.Observacao, Endereco: n.EnderecoTexto, Numero: n.Numero,
			Referencia: n.Referencia,
		}
		if d.Dados, err = h.formularioDeBaap(r, q, f, n.BairroID.Int64); err != nil {
			h.falhaInterna(w, r, "montar formulário", err)
			return
		}
	}
	if d.PodeLocal {
		d.Busca = strings.TrimSpace(r.URL.Query().Get("busca_local"))
		if d.Busca != "" {
			d.Buscou = true
			achados, err := q.BuscarLocaisParaVinculo(ctx, d.Busca)
			if err != nil {
				h.falhaInterna(w, r, "buscar locais", err)
				return
			}
			for _, l := range achados {
				d.Locais = append(d.Locais, views.LocalVinculo{ID: l.ID, Endereco: enderecoCompleto(l.Endereco, l.Numero), Bairro: l.BairroNome})
			}
		}
	}
	if d.Caso, err = h.fichaDoCaso(r, q, tiposDeCaso["baap"], n.ID, h.acoesDoBaap(r, n)); err != nil {
		h.falhaInterna(w, r, "montar andamento", err)
		return
	}
	render(w, r, views.BaapFicha(h.layout(r, "BAAP "+d.IDTexto, "/zoonose/baap"), d))
}

func (h *handlers) acoesDoBaap(r *http.Request, n sqlccliente.BaapNotificaco) []views.CasoAcao {
	pr := h.principal(r)
	base := "/zoonose/baap/" + strconv.FormatInt(n.ID, 10)
	var acoes []views.CasoAcao
	if n.Situacao == "encerrada" && pr.Pode("zoonose_baap_atender") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Reabrir", URL: base + "/reabrir", Icone: "undo", Confirmar: "Reabrir esta notificação?"})
	}
	if pr.Pode("zoonose_baap_encerrar") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Excluir", URL: base + "/excluir", Icone: "trash", Perigo: true, Confirmar: "Enviar esta notificação para a lixeira?"})
	}
	return acoes
}

func (h *handlers) baapSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	antes, ok := h.carregarBaap(w, r, q)
	if !ok {
		return
	}
	if antes.Situacao == "encerrada" {
		http.Error(w, "Notificação encerrada: reabra para editar.", http.StatusConflict)
		return
	}
	d, p, msg := h.lerDadosDoBaap(r, q, antes.ID)
	if msg != "" {
		d.Erro = msg
		w.WriteHeader(http.StatusBadRequest)
		h.mostrarBaap(w, r, &d)
		return
	}
	novo, err := q.UpdateBaapDados(r.Context(), sqlccliente.UpdateBaapDadosParams{
		ID: antes.ID, Sinam: p.Sinam, DataNotificado: p.DataNotificado, Paciente: p.Paciente, NomeMae: p.NomeMae, DataNascimento: p.DataNascimento,
		Telefone: p.Telefone, Telefone2: p.Telefone2, Animal: p.Animal, AnimalOutro: p.AnimalOutro, Especie: p.Especie, LocalPicada: p.LocalPicada,
		SituacaoPaciente: p.SituacaoPaciente, AtuacaoVga: p.AtuacaoVga, Observacao: p.Observacao, EnderecoTexto: p.EnderecoTexto, Numero: p.Numero,
		Referencia: p.Referencia, BairroID: p.BairroID,
	})
	if err != nil {
		h.falhaInterna(w, r, "salvar notificação de animal peçonhento", err)
		return
	}
	ev := audit.Evento{Acao: "baap.editar", Entidade: "baap", EntidadeID: strconv.FormatInt(antes.ID, 10), Rotulo: rotuloBaap(novo)}
	ev.Antes, ev.Depois = audit.Diferenca(
		map[string]any{"animal": antes.Animal, "especie": antes.Especie, "local_picada": antes.LocalPicada, "sinam": antes.Sinam, "endereco": antes.EnderecoTexto},
		map[string]any{"animal": novo.Animal, "especie": novo.Especie, "local_picada": novo.LocalPicada, "sinam": novo.Sinam, "endereco": novo.EnderecoTexto})
	h.auditar(r, ev)
	http.Redirect(w, r, "/zoonose/baap/"+strconv.FormatInt(antes.ID, 10)+"?aviso=dados", http.StatusSeeOther)
}

func (h *handlers) baapLocal(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarBaap(w, r, q)
	if !ok {
		return
	}
	if n.Situacao == "encerrada" {
		http.Error(w, "Notificação encerrada: reabra para editar.", http.StatusConflict)
		return
	}
	alvo := idOpcional(r.PostFormValue("local"))
	texto := "Local desvinculado"
	if alvo.Valid {
		l, ok := h.droneLocal(w, r, q, alvo.Int64)
		if !ok {
			return
		}
		texto = "Local vinculado: " + enderecoCompleto(l.Endereco, l.Numero)
	}
	if _, err := q.SetBaapLocal(r.Context(), sqlccliente.SetBaapLocalParams{ID: n.ID, LocalID: alvo}); err != nil {
		h.falhaInterna(w, r, "vincular local", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "baap", n.ID, "comentario", texto, "", "", 0)
	h.auditar(r, audit.Evento{Acao: "baap.local", Entidade: "baap", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloBaap(n),
		Antes: map[string]any{"local": n.LocalID.Int64}, Depois: map[string]any{"local": alvo.Int64}})
	http.Redirect(w, r, "/zoonose/baap/"+strconv.FormatInt(n.ID, 10)+"?aviso=local#local", http.StatusSeeOther)
}

func (h *handlers) baapVisita(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarBaap(w, r, q)
	if !ok {
		return
	}
	if n.Situacao == "encerrada" {
		http.Error(w, "Notificação encerrada: reabra para editar.", http.StatusConflict)
		return
	}
	alvo := idOpcional(strings.TrimSpace(r.PostFormValue("visita")))
	texto := "Visita desvinculada"
	if alvo.Valid {
		v, err := q.GetVisita(r.Context(), alvo.Int64)
		if err != nil {
			voltarAoCaso(w, r, tiposDeCaso["baap"], n.ID, "Visita não encontrada.")
			return
		}
		if ok, err := h.visitaVisivel(r.Context(), r, q, v); err != nil || !ok {
			http.NotFound(w, r)
			return
		}
		texto = "Visita vinculada: nº " + strconv.FormatInt(v.ID, 10)
	}
	if _, err := q.SetBaapVisita(r.Context(), sqlccliente.SetBaapVisitaParams{ID: n.ID, VisitaID: alvo}); err != nil {
		h.falhaInterna(w, r, "vincular visita", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "baap", n.ID, "comentario", texto, "", "", 0)
	h.auditar(r, audit.Evento{Acao: "baap.visita", Entidade: "baap", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloBaap(n),
		Antes: map[string]any{"visita": n.VisitaID.Int64}, Depois: map[string]any{"visita": alvo.Int64}})
	http.Redirect(w, r, "/zoonose/baap/"+strconv.FormatInt(n.ID, 10)+"?aviso=visita", http.StatusSeeOther)
}

func (h *handlers) mudarSituacaoDoBaap(w http.ResponseWriter, r *http.Request, acao string, regra func(n sqlccliente.BaapNotificaco) (string, pgtype.Int8, string)) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarBaap(w, r, q)
	if !ok {
		return
	}
	sit, resp, erro := regra(n)
	if erro != "" {
		voltarAoCaso(w, r, tiposDeCaso["baap"], n.ID, erro)
		return
	}
	if _, err := q.SetBaapSituacao(r.Context(), sqlccliente.SetBaapSituacaoParams{ID: n.ID, Situacao: sit, ResponsavelID: resp}); err != nil {
		h.falhaInterna(w, r, "mudar situação", err)
		return
	}
	texto := strings.TrimSpace(r.PostFormValue("texto"))
	if len([]rune(texto)) > 2000 {
		texto = string([]rune(texto)[:2000])
	}
	_ = h.casoEvento(r.Context(), q, r, "baap", n.ID, "situacao", texto, rotuloDe(rotulosSituacaoBaap, n.Situacao), rotuloDe(rotulosSituacaoBaap, sit), 0)
	h.auditar(r, audit.Evento{Acao: "baap." + acao, Entidade: "baap", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloBaap(n),
		Antes: map[string]any{"situacao": n.Situacao}, Depois: map[string]any{"situacao": sit}})
	http.Redirect(w, r, "/zoonose/baap/"+strconv.FormatInt(n.ID, 10)+"?aviso=situacao", http.StatusSeeOther)
}

func (h *handlers) baapAtender(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDoBaap(w, r, "atender", func(n sqlccliente.BaapNotificaco) (string, pgtype.Int8, string) {
		if n.Situacao != "aguardando" {
			return "", pgtype.Int8{}, "Esta notificação já está em atendimento ou encerrada."
		}
		return "em_atendimento", pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, ""
	})
}

func (h *handlers) baapEncerrar(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDoBaap(w, r, "encerrar", func(n sqlccliente.BaapNotificaco) (string, pgtype.Int8, string) {
		if n.Situacao != "em_atendimento" {
			return "", pgtype.Int8{}, "Só uma notificação em atendimento pode ser encerrada."
		}
		return "encerrada", n.ResponsavelID, ""
	})
}

func (h *handlers) baapReabrir(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDoBaap(w, r, "reabrir", func(n sqlccliente.BaapNotificaco) (string, pgtype.Int8, string) {
		if n.Situacao != "encerrada" {
			return "", pgtype.Int8{}, "Só uma notificação encerrada pode ser reaberta."
		}
		return "em_atendimento", pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, ""
	})
}

func (h *handlers) baapExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarBaap(w, r, q)
	if !ok {
		return
	}
	h.excluirRegistro(w, r, "baap", n.ID, "/zoonose/baap?aviso=excluido", "/zoonose/baap/"+strconv.FormatInt(n.ID, 10))
}

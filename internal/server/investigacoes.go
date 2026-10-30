package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Investigação (9.2): caso de uma doença/agravo, com paciente, local, entrevista e resultado. O andamento (linha do
// tempo, comentários, encaminhamento e anexos) vem da ficha comum de casos (9.1).

var rotulosSituacaoInvestigacao = []views.CidadeOpcao{
	{Valor: "aguardando", Rotulo: "Aguardando atendimento"},
	{Valor: "em_atendimento", Rotulo: "Em atendimento"},
	{Valor: "encerrada", Rotulo: "Encerrada"},
}

var rotulosResultadoInvestigacao = []views.CidadeOpcao{
	{Valor: "aguardando", Rotulo: "Aguardando"},
	{Valor: "positivo", Rotulo: "Positivo"},
	{Valor: "negativo", Rotulo: "Negativo"},
	{Valor: "inconclusivo", Rotulo: "Inconclusivo"},
}

func init() {
	registrarTipoDeCaso(tipoDeCaso{
		Slug: "investigacao", Rotulo: "Investigação",
		Ver: "investigacao_visualizar", Editar: "investigacao_anotacao", Encaminhar: "investigacao_encaminhar",
		Existe: func(ctx context.Context, q *sqlccliente.Queries, id int64) bool {
			_, err := q.GetInvestigacao(ctx, id)
			return err == nil
		},
		Caminho: func(id int64) string { return "/investigacoes/" + strconv.FormatInt(id, 10) },
	})
}

// escopoInvestigacoes: quem só vê as próprias investigações (criadas ou atribuídas a si).
func (h *handlers) escopoInvestigacoes(r *http.Request) pgtype.Int8 {
	if h.principal(r).Pode("investigacao_visualizar_minha") {
		return pgtype.Int8{Int64: meuID(r), Valid: true}
	}
	return pgtype.Int8{}
}

func (h *handlers) investigacaoVisivel(r *http.Request, i sqlccliente.Investigaco) bool {
	if !h.principal(r).Pode("investigacao_visualizar_minha") {
		return true
	}
	me := meuID(r)
	return i.CriadoPor.Int64 == me || i.ResponsavelID.Int64 == me
}

func ordemDeInvestigacoes(ordem, dir string) (string, string) {
	switch ordem {
	case "id", "paciente", "doenca", "endereco", "situacao", "resultado", "data":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func (h *handlers) opcoesDeDoencas(r *http.Request, q *sqlccliente.Queries, escolhida int64, soAtivas bool) ([]views.CidadeOpcao, error) {
	ds, err := q.ListDoencas(r.Context())
	if err != nil {
		return nil, err
	}
	var out []views.CidadeOpcao
	for _, d := range ds {
		if soAtivas && !d.Ativo && d.ID != escolhida {
			continue
		}
		out = append(out, views.CidadeOpcao{ID: d.ID, Rotulo: d.Nome, Selecionada: d.ID == escolhida})
	}
	return out, nil
}

func (h *handlers) investigacoesLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	pr := h.principal(r)
	qs := r.URL.Query()
	f := views.InvestigacoesFiltro{
		Busca: strings.TrimSpace(qs.Get("busca")), Doenca: qs.Get("doenca"), Situacao: qs.Get("situacao"),
		Resultado: qs.Get("resultado"), De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate")),
	}
	if !validoEm(rotulosSituacaoInvestigacao, f.Situacao) {
		f.Situacao = ""
	}
	if !validoEm(rotulosResultadoInvestigacao, f.Resultado) {
		f.Resultado = ""
	}
	ordem, dir := ordemDeInvestigacoes(qs.Get("ordem"), qs.Get("dir"))
	arg := sqlccliente.ListInvestigacoesParams{
		SomenteUsuario: h.escopoInvestigacoes(r), Busca: f.Busca, DoencaID: idOpcional(f.Doenca), Situacao: f.Situacao,
		Resultado: f.Resultado, DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate), Ordem: ordem, Dir: dir,
	}
	pag := paginaDe(r)
	total, err := q.CountInvestigacoes(ctx, sqlccliente.CountInvestigacoesParams{
		SomenteUsuario: arg.SomenteUsuario, Busca: arg.Busca, DoencaID: arg.DoencaID, Situacao: arg.Situacao,
		Resultado: arg.Resultado, DataDe: arg.DataDe, DataAte: arg.DataAte,
	})
	if err != nil {
		h.falhaInterna(w, r, "contar investigações", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListInvestigacoes(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar investigações", err)
		return
	}
	d := views.InvestigacoesData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: pr.Pode("investigacao_nova")}
	for _, i := range rows {
		d.Linhas = append(d.Linhas, views.InvestigacaoLinha{
			ID: i.ID, IDTexto: strconv.FormatInt(i.ID, 10), Paciente: i.Paciente, Telefone: i.Telefone, Doenca: i.DoencaNome,
			Data: dataBR(i.DataInicio), Endereco: i.Endereco, Situacao: i.Situacao, SituacaoRotulo: rotuloDe(rotulosSituacaoInvestigacao, i.Situacao),
			Resultado: i.Resultado, ResultadoRotulo: rotuloDe(rotulosResultadoInvestigacao, i.Resultado), Responsavel: i.Responsavel, TemLocal: i.TemLocal,
		})
	}
	if d.Doencas, err = h.opcoesDeDoencas(r, q, arg.DoencaID.Int64, false); err != nil {
		h.falhaInterna(w, r, "listar doenças", err)
		return
	}
	d.Situacoes, d.Resultados = marcarOpcao(rotulosSituacaoInvestigacao, f.Situacao), marcarOpcao(rotulosResultadoInvestigacao, f.Resultado)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "doenca": f.Doenca, "situacao": f.Situacao, "resultado": f.Resultado, "de": f.De, "ate": f.Ate} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Investigacoes(h.layout(r, "Investigações", "/investigacoes"), d))
}

func (h *handlers) investigacaoNova(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	ds, err := h.opcoesDeDoencas(r, q, 0, true)
	if err != nil {
		h.falhaInterna(w, r, "listar doenças", err)
		return
	}
	d := views.InvestigacaoFormData{Doencas: ds, DataInicio: time.Now().In(h.fuso(r)).Format("2006-01-02"), CSRF: csrfToken(r.Context())}
	render(w, r, views.InvestigacaoNova(h.layout(r, "Nova investigação", "/investigacoes"), d))
}

// lerDadosDaInvestigacao lê e valida o formulário de dados do caso.
func (h *handlers) lerDadosDaInvestigacao(r *http.Request, q *sqlccliente.Queries, id int64) (views.InvestigacaoFormData, sqlccliente.Doenca, string) {
	t := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	d := views.InvestigacaoFormData{
		ID: id, Paciente: t("paciente"), Telefone: t("telefone"), DataInicio: t("data_inicio"), Endereco: t("endereco"),
		Observacao: t("observacao"), CSRF: csrfToken(r.Context()),
	}
	did := idOpcional(t("doenca"))
	d.Doencas, _ = h.opcoesDeDoencas(r, q, did.Int64, true)
	var doenca sqlccliente.Doenca
	switch {
	case !did.Valid:
		return d, doenca, "Escolha a doença ou agravo."
	case d.Paciente == "" || len([]rune(d.Paciente)) > 150:
		return d, doenca, "Informe o nome do paciente (até 150 caracteres)."
	case len([]rune(d.Telefone)) > 40:
		return d, doenca, "O telefone passa de 40 caracteres."
	case len([]rune(d.Endereco)) > 300:
		return d, doenca, "O endereço passa de 300 caracteres."
	case len([]rune(d.Observacao)) > 2000:
		return d, doenca, "As observações passam de 2000 caracteres."
	case d.DataInicio != "" && !dataOpcional(d.DataInicio).Valid:
		return d, doenca, "Data de início inválida."
	}
	doenca, err := q.GetDoenca(r.Context(), did.Int64)
	if err != nil {
		return d, doenca, "A doença escolhida não existe."
	}
	return d, doenca, ""
}

func (h *handlers) investigacaoCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d, doenca, msg := h.lerDadosDaInvestigacao(r, q, 0)
	if msg != "" {
		d.Erro = msg
		renderStatus(w, r, http.StatusBadRequest, views.InvestigacaoNova(h.layout(r, "Nova investigação", "/investigacoes"), d))
		return
	}
	i, err := q.CreateInvestigacao(r.Context(), sqlccliente.CreateInvestigacaoParams{
		DoencaID: doenca.ID, EnderecoTexto: d.Endereco, Paciente: d.Paciente, Telefone: d.Telefone, DataInicio: dataOpcional(d.DataInicio),
		Observacao: d.Observacao, GrupoID: doenca.GrupoID, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
	})
	if err != nil {
		h.falhaInterna(w, r, "criar investigação", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "investigacao", i.ID, "criacao", "", "", "Aguardando atendimento", 0)
	h.auditar(r, audit.Evento{Acao: "investigacao.criar", Entidade: "investigacao", EntidadeID: strconv.FormatInt(i.ID, 10), Rotulo: rotuloInvestigacao(i),
		Depois: map[string]any{"doenca": doenca.Nome, "paciente": i.Paciente}})
	http.Redirect(w, r, "/investigacoes/"+strconv.FormatInt(i.ID, 10), http.StatusSeeOther)
}

func rotuloInvestigacao(i sqlccliente.Investigaco) string {
	return "Investigação " + strconv.FormatInt(i.ID, 10) + " — " + i.Paciente
}

// carregarInvestigacao lê a investigação da URL e confere o escopo da pessoa.
func (h *handlers) carregarInvestigacao(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.Investigaco, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.Investigaco{}, false
	}
	i, err := q.GetInvestigacao(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !h.investigacaoVisivel(r, i)) {
		http.NotFound(w, r)
		return i, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar investigação", err)
		return i, false
	}
	return i, true
}

func (h *handlers) investigacaoFicha(w http.ResponseWriter, r *http.Request) {
	h.mostrarInvestigacao(w, r, "", nil)
}

// mostrarInvestigacao desenha a ficha; erroDados e dadosForm permitem reexibir o formulário com o erro.
func (h *handlers) mostrarInvestigacao(w http.ResponseWriter, r *http.Request, erroEntrevista string, dadosForm *views.InvestigacaoFormData) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	i, ok := h.carregarInvestigacao(w, r, q)
	if !ok {
		return
	}
	pr := h.principal(r)
	doenca, err := q.GetDoenca(ctx, i.DoencaID)
	if err != nil {
		h.falhaInterna(w, r, "buscar doença", err)
		return
	}
	loc := h.fuso(r)
	d := views.InvestigacaoFichaData{
		ID: i.ID, IDTexto: strconv.FormatInt(i.ID, 10), Doenca: doenca.Nome, Paciente: i.Paciente, Telefone: i.Telefone,
		Data: dataBR(i.DataInicio), Endereco: i.EnderecoTexto, Situacao: i.Situacao, SituacaoRotulo: rotuloDe(rotulosSituacaoInvestigacao, i.Situacao),
		Resultado: i.Resultado, ResultadoRotulo: rotuloDe(rotulosResultadoInvestigacao, i.Resultado), Observacao: i.Observacao,
		Encerrada: i.Situacao == "encerrada", CSRF: csrfToken(ctx), ErroEntrevista: erroEntrevista,
		PodeEditar:     pr.Pode("investigacao_editar") && i.Situacao != "encerrada",
		PodeLocal:      pr.Pode("investigacao_local") && i.Situacao != "encerrada",
		PodeEntrevista: pr.Pode("investigacao_entrevista") && i.Situacao != "encerrada",
		Aviso:          avisoDaInvestigacao(r.URL.Query().Get("aviso")),
	}
	_ = loc
	if i.ResponsavelID.Valid {
		if u, err := q.GetUsuarioByID(ctx, i.ResponsavelID.Int64); err == nil {
			d.Responsavel = u.Nome
		}
	}
	if i.LocalID.Valid {
		d.LocalID = i.LocalID.Int64
		if l, err := q.GetLocal(ctx, i.LocalID.Int64); err == nil {
			d.LocalEndereco = enderecoCompleto(l.Endereco, l.Numero)
		}
	}
	if dadosForm != nil {
		d.Dados = *dadosForm
	} else if d.PodeEditar {
		ds, err := h.opcoesDeDoencas(r, q, i.DoencaID, true)
		if err != nil {
			h.falhaInterna(w, r, "listar doenças", err)
			return
		}
		d.Dados = views.InvestigacaoFormData{
			ID: i.ID, Doencas: ds, Paciente: i.Paciente, Telefone: i.Telefone, DataInicio: dataISO(i.DataInicio), Endereco: i.EnderecoTexto,
			Observacao: i.Observacao, CSRF: d.CSRF,
		}
	}
	// busca de local para vincular
	if d.PodeLocal {
		d.Busca = strings.TrimSpace(r.URL.Query().Get("busca_local"))
		if d.Busca != "" {
			d.Buscou = true
			somente, _ := h.escopoLocais(r)
			_ = somente
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
	// entrevista
	if doenca.GrupoID.Valid {
		g, err := q.GetEntrevistaGrupo(ctx, doenca.GrupoID.Int64)
		if err == nil {
			d.EntrevistaTitulo = g.Titulo
			d.Entrevista = camposParaATela(g.Campos, i.Entrevista, doenca.Nome, r)
		}
	}
	acoes := h.acoesDaInvestigacao(r, i)
	if d.Caso, err = h.fichaDoCaso(r, q, tiposDeCaso["investigacao"], i.ID, acoes); err != nil {
		h.falhaInterna(w, r, "montar andamento da investigação", err)
		return
	}
	render(w, r, views.InvestigacaoFicha(h.layout(r, "Investigação "+d.IDTexto, "/investigacoes"), d))
}

func avisoDaInvestigacao(c string) string {
	switch c {
	case "local":
		return "Local atualizado."
	case "entrevista":
		return "Entrevista salva."
	case "dados":
		return "Dados salvos."
	case "situacao":
		return "Situação atualizada."
	}
	return ""
}

// camposParaATela prepara as perguntas da entrevista, com as respostas já dadas e a visibilidade inicial.
func camposParaATela(camposJSON, respostasJSON []byte, doenca string, r *http.Request) []views.EntrevistaCampo {
	var campos []campoEntrevista
	_ = json.Unmarshal(camposJSON, &campos)
	resp := respostas{}
	_ = json.Unmarshal(respostasJSON, &resp)
	// em reenvio com erro, as respostas vêm do formulário
	if r.Method == http.MethodPost {
		resp = respostas{}
		for _, c := range campos {
			if c.Tipo == "multi_select" {
				resp[c.Nome] = r.PostForm["c_"+c.Nome]
			} else {
				resp[c.Nome] = strings.TrimSpace(r.PostFormValue("c_" + c.Nome))
			}
		}
	}
	var out []views.EntrevistaCampo
	for _, c := range campos {
		if !c.exibirPara(doenca) {
			continue
		}
		ec := views.EntrevistaCampo{
			Nome: c.Nome, Label: c.Label, Tipo: c.Tipo, Placeholder: c.Placeholder, Obrigatorio: c.Obrigatorio, Opcoes: c.lista(),
			CondCampo: c.CondCampo, CondOp: c.CondOperador, CondValor: c.CondValor, Oculto: !c.visivel(doenca, resp),
		}
		if c.Tipo == "multi_select" {
			ec.Valores = strings.Split(resp.texto(c.Nome), ", ")
		} else {
			ec.Valor = resp.texto(c.Nome)
		}
		out = append(out, ec)
	}
	return out
}

func (h *handlers) acoesDaInvestigacao(r *http.Request, i sqlccliente.Investigaco) []views.CasoAcao {
	pr := h.principal(r)
	base := "/investigacoes/" + strconv.FormatInt(i.ID, 10)
	var acoes []views.CasoAcao
	if i.Situacao != "encerrada" {
		if pr.Pode("investigacao_atender") && i.Situacao == "aguardando" {
			acoes = append(acoes, views.CasoAcao{Rotulo: "Assumir atendimento", URL: base + "/atender", Icone: "check"})
		}
		if pr.Pode("investigacao_resultado") {
			for _, v := range []string{"positivo", "negativo", "inconclusivo"} {
				if v != i.Resultado {
					acoes = append(acoes, views.CasoAcao{Rotulo: "Resultado " + v, URL: base + "/resultado?valor=" + v, Icone: "flask"})
				}
			}
		}
		if pr.Pode("investigacao_encerrar") {
			acoes = append(acoes, views.CasoAcao{Rotulo: "Encerrar", URL: base + "/encerrar", Icone: "lock", Confirmar: "Encerrar esta investigação?"})
		}
	} else if pr.Pode("investigacao_reabrir") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Reabrir", URL: base + "/reabrir", Icone: "undo", Confirmar: "Reabrir esta investigação?"})
	}
	if pr.Pode("investigacao_desativar") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Excluir", URL: base + "/excluir", Icone: "trash", Perigo: true, Confirmar: "Enviar esta investigação para a lixeira?"})
	}
	return acoes
}

func (h *handlers) investigacaoSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	antes, ok := h.carregarInvestigacao(w, r, q)
	if !ok {
		return
	}
	if antes.Situacao == "encerrada" {
		http.Error(w, "Investigação encerrada: reabra para editar.", http.StatusConflict)
		return
	}
	d, doenca, msg := h.lerDadosDaInvestigacao(r, q, antes.ID)
	if msg != "" {
		d.Erro = msg
		w.WriteHeader(http.StatusBadRequest)
		h.mostrarInvestigacao(w, r, "", &d)
		return
	}
	grupo := antes.GrupoID
	if doenca.ID != antes.DoencaID {
		grupo = doenca.GrupoID
	}
	novo, err := q.UpdateInvestigacaoDados(r.Context(), sqlccliente.UpdateInvestigacaoDadosParams{
		ID: antes.ID, DoencaID: doenca.ID, EnderecoTexto: d.Endereco, Paciente: d.Paciente, Telefone: d.Telefone, Observacao: d.Observacao,
		DataInicio: dataOpcional(d.DataInicio), GrupoID: grupo,
	})
	if err != nil {
		h.falhaInterna(w, r, "salvar investigação", err)
		return
	}
	ev := audit.Evento{Acao: "investigacao.editar", Entidade: "investigacao", EntidadeID: strconv.FormatInt(antes.ID, 10), Rotulo: rotuloInvestigacao(novo)}
	ev.Antes, ev.Depois = audit.Diferenca(
		map[string]any{"doenca": antes.DoencaID, "paciente": antes.Paciente, "telefone": antes.Telefone, "data": dataISO(antes.DataInicio), "endereco": antes.EnderecoTexto, "observacao": antes.Observacao},
		map[string]any{"doenca": novo.DoencaID, "paciente": novo.Paciente, "telefone": novo.Telefone, "data": dataISO(novo.DataInicio), "endereco": novo.EnderecoTexto, "observacao": novo.Observacao})
	h.auditar(r, ev)
	http.Redirect(w, r, "/investigacoes/"+strconv.FormatInt(antes.ID, 10)+"?aviso=dados", http.StatusSeeOther)
}

func (h *handlers) investigacaoLocal(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	i, ok := h.carregarInvestigacao(w, r, q)
	if !ok {
		return
	}
	if i.Situacao == "encerrada" {
		http.Error(w, "Investigação encerrada: reabra para editar.", http.StatusConflict)
		return
	}
	alvo := idOpcional(r.PostFormValue("local"))
	rotulo := ""
	if alvo.Valid {
		l, err := q.GetLocal(r.Context(), alvo.Int64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if ok, err := h.localVisivel(r.Context(), r, q, l); err != nil {
			h.falhaInterna(w, r, "checar acesso ao local", err)
			return
		} else if !ok {
			http.NotFound(w, r)
			return
		}
		rotulo = enderecoCompleto(l.Endereco, l.Numero)
	}
	if _, err := q.SetInvestigacaoLocal(r.Context(), sqlccliente.SetInvestigacaoLocalParams{ID: i.ID, LocalID: alvo}); err != nil {
		h.falhaInterna(w, r, "vincular local", err)
		return
	}
	texto := "Local desvinculado"
	if alvo.Valid {
		texto = "Local vinculado: " + rotulo
	}
	_ = h.casoEvento(r.Context(), q, r, "investigacao", i.ID, "comentario", texto, "", "", 0)
	h.auditar(r, audit.Evento{Acao: "investigacao.local", Entidade: "investigacao", EntidadeID: strconv.FormatInt(i.ID, 10), Rotulo: rotuloInvestigacao(i),
		Antes: map[string]any{"local": i.LocalID.Int64}, Depois: map[string]any{"local": alvo.Int64}})
	http.Redirect(w, r, "/investigacoes/"+strconv.FormatInt(i.ID, 10)+"?aviso=local#local", http.StatusSeeOther)
}

func (h *handlers) investigacaoEntrevista(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	i, ok := h.carregarInvestigacao(w, r, q)
	if !ok {
		return
	}
	if i.Situacao == "encerrada" {
		http.Error(w, "Investigação encerrada: reabra para editar.", http.StatusConflict)
		return
	}
	doenca, err := q.GetDoenca(r.Context(), i.DoencaID)
	if err != nil || !doenca.GrupoID.Valid {
		http.Error(w, "Esta doença não tem entrevista.", http.StatusConflict)
		return
	}
	g, err := q.GetEntrevistaGrupo(r.Context(), doenca.GrupoID.Int64)
	if err != nil {
		h.falhaInterna(w, r, "buscar entrevista", err)
		return
	}
	var campos []campoEntrevista
	_ = json.Unmarshal(g.Campos, &campos)
	resp, msg := lerRespostas(r, campos, doenca.Nome)
	if msg != "" {
		w.WriteHeader(http.StatusBadRequest)
		h.mostrarInvestigacao(w, r, msg, nil)
		return
	}
	dados, _ := json.Marshal(resp)
	if _, err := q.SetInvestigacaoEntrevista(r.Context(), sqlccliente.SetInvestigacaoEntrevistaParams{ID: i.ID, Entrevista: dados, GrupoID: doenca.GrupoID}); err != nil {
		h.falhaInterna(w, r, "salvar entrevista", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "investigacao", i.ID, "comentario", "Entrevista registrada ("+g.Titulo+")", "", "", 0)
	h.auditar(r, audit.Evento{Acao: "investigacao.entrevista", Entidade: "investigacao", EntidadeID: strconv.FormatInt(i.ID, 10), Rotulo: rotuloInvestigacao(i),
		Depois: map[string]any{"entrevista": g.Titulo, "respostas": len(resp)}})
	http.Redirect(w, r, "/investigacoes/"+strconv.FormatInt(i.ID, 10)+"?aviso=entrevista#entrevista", http.StatusSeeOther)
}

// mudarSituacao é o caminho comum das ações de andamento (atender, resultado, encerrar, reabrir).
func (h *handlers) mudarSituacaoDaInvestigacao(w http.ResponseWriter, r *http.Request, acao string, regra func(i sqlccliente.Investigaco) (situacao, resultado string, responsavel pgtype.Int8, erro string)) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	i, ok := h.carregarInvestigacao(w, r, q)
	if !ok {
		return
	}
	sit, res, resp, erro := regra(i)
	if erro != "" {
		voltarAoCaso(w, r, tiposDeCaso["investigacao"], i.ID, erro)
		return
	}
	if _, err := q.SetInvestigacaoSituacao(r.Context(), sqlccliente.SetInvestigacaoSituacaoParams{ID: i.ID, Situacao: sit, Resultado: res, ResponsavelID: resp}); err != nil {
		h.falhaInterna(w, r, "mudar situação da investigação", err)
		return
	}
	de := rotuloDe(rotulosSituacaoInvestigacao, i.Situacao) + " / " + rotuloDe(rotulosResultadoInvestigacao, i.Resultado)
	para := rotuloDe(rotulosSituacaoInvestigacao, sit) + " / " + rotuloDe(rotulosResultadoInvestigacao, res)
	_ = h.casoEvento(r.Context(), q, r, "investigacao", i.ID, "situacao", "", de, para, 0)
	h.auditar(r, audit.Evento{Acao: "investigacao." + acao, Entidade: "investigacao", EntidadeID: strconv.FormatInt(i.ID, 10), Rotulo: rotuloInvestigacao(i),
		Antes: map[string]any{"situacao": i.Situacao, "resultado": i.Resultado}, Depois: map[string]any{"situacao": sit, "resultado": res}})
	http.Redirect(w, r, "/investigacoes/"+strconv.FormatInt(i.ID, 10)+"?aviso=situacao", http.StatusSeeOther)
}

func (h *handlers) investigacaoAtender(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDaInvestigacao(w, r, "atender", func(i sqlccliente.Investigaco) (string, string, pgtype.Int8, string) {
		if i.Situacao != "aguardando" {
			return "", "", pgtype.Int8{}, "Esta investigação já está em atendimento ou encerrada."
		}
		return "em_atendimento", i.Resultado, pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, ""
	})
}

func (h *handlers) investigacaoResultado(w http.ResponseWriter, r *http.Request) {
	valor := r.URL.Query().Get("valor")
	h.mudarSituacaoDaInvestigacao(w, r, "resultado", func(i sqlccliente.Investigaco) (string, string, pgtype.Int8, string) {
		if valor != "positivo" && valor != "negativo" && valor != "inconclusivo" {
			return "", "", pgtype.Int8{}, "Resultado inválido."
		}
		if i.Situacao == "encerrada" {
			return "", "", pgtype.Int8{}, "Investigação encerrada: reabra para mudar o resultado."
		}
		sit, resp := i.Situacao, i.ResponsavelID
		if sit == "aguardando" {
			sit, resp = "em_atendimento", pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}
		}
		return sit, valor, resp, ""
	})
}

func (h *handlers) investigacaoEncerrar(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDaInvestigacao(w, r, "encerrar", func(i sqlccliente.Investigaco) (string, string, pgtype.Int8, string) {
		if i.Situacao == "encerrada" {
			return "", "", pgtype.Int8{}, "Esta investigação já está encerrada."
		}
		if i.Resultado == "aguardando" {
			return "", "", pgtype.Int8{}, "Defina o resultado (positivo, negativo ou inconclusivo) antes de encerrar."
		}
		return "encerrada", i.Resultado, i.ResponsavelID, ""
	})
}

func (h *handlers) investigacaoReabrir(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDaInvestigacao(w, r, "reabrir", func(i sqlccliente.Investigaco) (string, string, pgtype.Int8, string) {
		if i.Situacao != "encerrada" {
			return "", "", pgtype.Int8{}, "Só uma investigação encerrada pode ser reaberta."
		}
		return "em_atendimento", i.Resultado, i.ResponsavelID, ""
	})
}

func (h *handlers) investigacaoExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	i, ok := h.carregarInvestigacao(w, r, q)
	if !ok {
		return
	}
	h.excluirRegistro(w, r, "investigacao", i.ID, "/investigacoes?aviso=excluido", "/investigacoes/"+strconv.FormatInt(i.ID, 10))
}

var _ = chi.URLParam

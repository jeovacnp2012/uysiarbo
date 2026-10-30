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

// Solicitação de drone (9.5): pedido de sobrevoo de um local (casa fechada, calha, laje, terreno de difícil acesso).
// O andamento (linha do tempo, anotações, encaminhamento e anexos) vem da ficha comum de casos (9.1).

var rotulosSituacaoDrone = []views.CidadeOpcao{
	{Valor: "aguardando", Rotulo: "Aguardando"},
	{Valor: "em_atendimento", Rotulo: "Em atendimento"},
	{Valor: "finalizado", Rotulo: "Atendimento finalizado"},
	{Valor: "encerrada", Rotulo: "Encerrada"},
}

func init() {
	registrarTipoDeCaso(tipoDeCaso{
		Slug: "drone", Rotulo: "Solicitação de drone",
		Ver: "dronesolicitacao_visualizar", Editar: "dronesolicitacao_anotacao", Encaminhar: "dronesolicitacao_encaminhar",
		Existe: func(ctx context.Context, q *sqlccliente.Queries, id int64) bool {
			_, err := q.GetDrone(ctx, id)
			return err == nil
		},
		Caminho: func(id int64) string { return "/drones/" + strconv.FormatInt(id, 10) },
	})
}

func ordemDeDrones(ordem, dir string) (string, string) {
	switch ordem {
	case "id", "data", "endereco", "situacao":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func rotuloDrone(d sqlccliente.DroneSolicitaco) string {
	return "Solicitação de drone " + strconv.FormatInt(d.ID, 10)
}

func (h *handlers) dronesLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.DronesFiltro{
		Busca: strings.TrimSpace(qs.Get("busca")), Situacao: qs.Get("situacao"), Bairro: qs.Get("bairro"),
		De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate")),
	}
	if !validoEm(rotulosSituacaoDrone, f.Situacao) {
		f.Situacao = ""
	}
	ordem, dir := ordemDeDrones(qs.Get("ordem"), qs.Get("dir"))
	somente, _ := h.escopoLocais(r)
	arg := sqlccliente.ListDronesParams{
		Busca: f.Busca, Situacao: f.Situacao, BairroID: idOpcional(f.Bairro), DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate),
		SomenteUsuario: somente.Valid, UsuarioID: somente.Int64, Ordem: ordem, Dir: dir,
	}
	pag := paginaDe(r)
	total, err := q.CountDrones(ctx, sqlccliente.CountDronesParams{
		Busca: arg.Busca, Situacao: arg.Situacao, BairroID: arg.BairroID, DataDe: arg.DataDe, DataAte: arg.DataAte,
		SomenteUsuario: arg.SomenteUsuario, UsuarioID: arg.UsuarioID,
	})
	if err != nil {
		h.falhaInterna(w, r, "contar solicitações de drone", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListDrones(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar solicitações de drone", err)
		return
	}
	d := views.DronesData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: h.principal(r).Pode("dronesolicitacao_novo")}
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.DroneLinha{
			ID: x.ID, IDTexto: strconv.FormatInt(x.ID, 10), Descricao: x.Descricao, Data: dataBR(x.DataAbertura), Endereco: x.Endereco,
			Bairro: x.Bairro, AbertoPor: x.AbertoPor, Situacao: x.Situacao, SituacaoRotulo: rotuloDe(rotulosSituacaoDrone, x.Situacao),
			Responsavel: x.Responsavel,
		})
	}
	if d.Bairros, err = h.opcoesDeBairros(r, q, arg.BairroID.Int64); err != nil {
		h.falhaInterna(w, r, "listar bairros", err)
		return
	}
	d.Situacoes = marcarOpcao(rotulosSituacaoDrone, f.Situacao)
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"busca": f.Busca, "situacao": f.Situacao, "bairro": f.Bairro, "de": f.De, "ate": f.Ate} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Drones(h.layout(r, "Solicitações de drone", "/drones"), d))
}

func (h *handlers) droneLocal(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, id int64) (sqlccliente.Locai, bool) {
	l, err := q.GetLocal(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return l, false
	}
	if ok, err := h.localVisivel(r.Context(), r, q, l); err != nil {
		h.falhaInterna(w, r, "checar acesso ao local", err)
		return l, false
	} else if !ok {
		http.NotFound(w, r)
		return l, false
	}
	return l, true
}

func (h *handlers) droneNova(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	if id := idOpcional(r.URL.Query().Get("local")); id.Valid {
		l, ok := h.droneLocal(w, r, q, id.Int64)
		if !ok {
			return
		}
		render(w, r, views.DroneNova(h.layout(r, "Nova solicitação de drone", "/drones"), views.DroneFormData{
			LocalID: l.ID, LocalEndereco: enderecoCompleto(l.Endereco, l.Numero), Data: h.hojeNoFuso(r).Time.Format("2006-01-02"), CSRF: csrfToken(ctx),
		}))
		return
	}
	e := views.DroneEscolherLocal{Busca: strings.TrimSpace(r.URL.Query().Get("busca_local"))}
	if e.Busca != "" {
		e.Buscou = true
		achados, err := q.BuscarLocaisParaVinculo(ctx, e.Busca)
		if err != nil {
			h.falhaInterna(w, r, "buscar locais", err)
			return
		}
		for _, l := range achados {
			e.Locais = append(e.Locais, views.LocalVinculo{ID: l.ID, Endereco: enderecoCompleto(l.Endereco, l.Numero), Bairro: l.BairroNome})
		}
	}
	render(w, r, views.DroneEscolher(h.layout(r, "Nova solicitação de drone", "/drones"), e))
}

func (h *handlers) droneCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	localID, _ := strconv.ParseInt(r.PostFormValue("local"), 10, 64)
	l, ok := h.droneLocal(w, r, q, localID)
	if !ok {
		return
	}
	d := views.DroneFormData{
		LocalID: l.ID, LocalEndereco: enderecoCompleto(l.Endereco, l.Numero), CSRF: csrfToken(r.Context()),
		Descricao: strings.TrimSpace(r.PostFormValue("descricao")), Data: strings.TrimSpace(r.PostFormValue("data")),
	}
	data := dataOpcional(d.Data)
	switch {
	case d.Descricao == "":
		d.Erro = "Descreva por que o sobrevoo é necessário."
	case len([]rune(d.Descricao)) > 2000:
		d.Erro = "A descrição passa de 2000 caracteres."
	case !data.Valid:
		d.Erro = "Informe a data da abertura."
	}
	if d.Erro != "" {
		renderStatus(w, r, http.StatusBadRequest, views.DroneNova(h.layout(r, "Nova solicitação de drone", "/drones"), d))
		return
	}
	n, err := q.CreateDrone(r.Context(), sqlccliente.CreateDroneParams{
		LocalID: l.ID, Descricao: d.Descricao, DataAbertura: data, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
	})
	if err != nil {
		h.falhaInterna(w, r, "criar solicitação de drone", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "drone", n.ID, "criacao", "", "", "Aguardando", 0)
	h.auditar(r, audit.Evento{Acao: "drone.criar", Entidade: "drone", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDrone(n),
		Depois: map[string]any{"local": n.LocalID, "descricao": n.Descricao}})
	http.Redirect(w, r, "/drones/"+strconv.FormatInt(n.ID, 10), http.StatusSeeOther)
}

func (h *handlers) carregarDrone(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.DroneSolicitaco, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.DroneSolicitaco{}, false
	}
	n, err := q.GetDrone(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return n, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar solicitação de drone", err)
		return n, false
	}
	// quem só vê os próprios bairros só enxerga solicitações de locais ao seu alcance (local na lixeira: continua acessível)
	if l, err := q.GetLocal(r.Context(), n.LocalID); err == nil {
		if ok, err := h.localVisivel(r.Context(), r, q, l); err != nil {
			h.falhaInterna(w, r, "checar acesso ao local", err)
			return n, false
		} else if !ok {
			http.NotFound(w, r)
			return n, false
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		h.falhaInterna(w, r, "buscar local", err)
		return n, false
	}
	return n, true
}

func avisoDoDrone(c string) string {
	switch c {
	case "visita":
		return "Visita atualizada."
	case "situacao":
		return "Situação atualizada."
	}
	return ""
}

func (h *handlers) droneFicha(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDrone(w, r, q)
	if !ok {
		return
	}
	pr := h.principal(r)
	fechada := n.Situacao == "encerrada"
	d := views.DroneFichaData{
		ID: n.ID, IDTexto: strconv.FormatInt(n.ID, 10), Descricao: n.Descricao, Data: dataBR(n.DataAbertura), LocalID: n.LocalID,
		VisitaID: n.VisitaID.Int64, Situacao: n.Situacao, SituacaoRotulo: rotuloDe(rotulosSituacaoDrone, n.Situacao), Fechada: fechada,
		PodeVisita: pr.Pode("dronesolicitacao_atender") && !fechada, PodeAtender: pr.Pode("dronesolicitacao_atender"),
		PodeEncerrar: pr.Pode("dronesolicitacao_encerrar"), CSRF: csrfToken(ctx), Aviso: avisoDoDrone(r.URL.Query().Get("aviso")),
	}
	if n.CriadoPor.Valid {
		if u, err := q.GetUsuarioByID(ctx, n.CriadoPor.Int64); err == nil {
			d.AbertoPor = u.Nome
		}
	}
	if n.ResponsavelID.Valid {
		if u, err := q.GetUsuarioByID(ctx, n.ResponsavelID.Int64); err == nil {
			d.Responsavel = u.Nome
		}
	}
	if l, err := q.GetLocalQualquer(ctx, n.LocalID); err == nil {
		d.LocalEndereco = enderecoCompleto(l.Endereco, l.Numero)
		if b, err := q.GetBairro(ctx, l.BairroID); err == nil {
			d.Bairro = b.Nome
		}
	}
	if d.Caso, err = h.fichaDoCaso(r, q, tiposDeCaso["drone"], n.ID, h.acoesDoDrone(r, n)); err != nil {
		h.falhaInterna(w, r, "montar andamento da solicitação de drone", err)
		return
	}
	render(w, r, views.DroneFicha(h.layout(r, "Solicitação de drone "+d.IDTexto, "/drones"), d))
}

func (h *handlers) acoesDoDrone(r *http.Request, n sqlccliente.DroneSolicitaco) []views.CasoAcao {
	pr := h.principal(r)
	base := "/drones/" + strconv.FormatInt(n.ID, 10)
	var acoes []views.CasoAcao
	if n.Situacao == "encerrada" && pr.Pode("dronesolicitacao_atender") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Reabrir", URL: base + "/reabrir", Icone: "undo", Confirmar: "Reabrir esta solicitação?"})
	}
	if pr.Pode("dronesolicitacao_encerrar") {
		acoes = append(acoes, views.CasoAcao{Rotulo: "Excluir", URL: base + "/excluir", Icone: "trash", Perigo: true, Confirmar: "Enviar esta solicitação para a lixeira?"})
	}
	return acoes
}

// droneVisita liga (ou desliga) a visita feita por causa da solicitação.
func (h *handlers) droneVisita(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDrone(w, r, q)
	if !ok {
		return
	}
	if n.Situacao == "encerrada" {
		http.Error(w, "Solicitação encerrada: reabra para editar.", http.StatusConflict)
		return
	}
	alvo := idOpcional(strings.TrimSpace(r.PostFormValue("visita")))
	texto := "Visita desvinculada"
	if alvo.Valid {
		v, err := q.GetVisita(r.Context(), alvo.Int64)
		if err != nil {
			voltarAoCaso(w, r, tiposDeCaso["drone"], n.ID, "Visita não encontrada.")
			return
		}
		if ok, err := h.visitaVisivel(r.Context(), r, q, v); err != nil || !ok {
			http.NotFound(w, r)
			return
		}
		texto = "Visita vinculada: nº " + strconv.FormatInt(v.ID, 10)
	}
	if _, err := q.SetDroneVisita(r.Context(), sqlccliente.SetDroneVisitaParams{ID: n.ID, VisitaID: alvo}); err != nil {
		h.falhaInterna(w, r, "vincular visita", err)
		return
	}
	_ = h.casoEvento(r.Context(), q, r, "drone", n.ID, "comentario", texto, "", "", 0)
	h.auditar(r, audit.Evento{Acao: "drone.visita", Entidade: "drone", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDrone(n),
		Antes: map[string]any{"visita": n.VisitaID.Int64}, Depois: map[string]any{"visita": alvo.Int64}})
	http.Redirect(w, r, "/drones/"+strconv.FormatInt(n.ID, 10)+"?aviso=visita", http.StatusSeeOther)
}

// mudarSituacaoDoDrone é o caminho comum das ações de andamento; regra devolve a situação nova, o responsável e o erro.
func (h *handlers) mudarSituacaoDoDrone(w http.ResponseWriter, r *http.Request, acao string, regra func(n sqlccliente.DroneSolicitaco) (string, pgtype.Int8, string)) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDrone(w, r, q)
	if !ok {
		return
	}
	sit, resp, erro := regra(n)
	if erro != "" {
		voltarAoCaso(w, r, tiposDeCaso["drone"], n.ID, erro)
		return
	}
	if _, err := q.SetDroneSituacao(r.Context(), sqlccliente.SetDroneSituacaoParams{ID: n.ID, Situacao: sit, ResponsavelID: resp}); err != nil {
		h.falhaInterna(w, r, "mudar situação da solicitação de drone", err)
		return
	}
	texto := strings.TrimSpace(r.PostFormValue("texto"))
	if len([]rune(texto)) > 2000 {
		texto = string([]rune(texto)[:2000])
	}
	_ = h.casoEvento(r.Context(), q, r, "drone", n.ID, "situacao", texto, rotuloDe(rotulosSituacaoDrone, n.Situacao), rotuloDe(rotulosSituacaoDrone, sit), 0)
	h.auditar(r, audit.Evento{Acao: "drone." + acao, Entidade: "drone", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDrone(n),
		Antes: map[string]any{"situacao": n.Situacao}, Depois: map[string]any{"situacao": sit}})
	http.Redirect(w, r, "/drones/"+strconv.FormatInt(n.ID, 10)+"?aviso=situacao", http.StatusSeeOther)
}

func (h *handlers) droneAtender(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDoDrone(w, r, "atender", func(n sqlccliente.DroneSolicitaco) (string, pgtype.Int8, string) {
		if n.Situacao != "aguardando" {
			return "", pgtype.Int8{}, "Esta solicitação já está em atendimento ou fechada."
		}
		return "em_atendimento", pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, ""
	})
}

func (h *handlers) droneFinalizar(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDoDrone(w, r, "finalizar", func(n sqlccliente.DroneSolicitaco) (string, pgtype.Int8, string) {
		if n.Situacao != "em_atendimento" {
			return "", pgtype.Int8{}, "Só uma solicitação em atendimento pode ter o atendimento finalizado."
		}
		return "finalizado", n.ResponsavelID, ""
	})
}

func (h *handlers) droneEncerrar(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDoDrone(w, r, "encerrar", func(n sqlccliente.DroneSolicitaco) (string, pgtype.Int8, string) {
		if n.Situacao != "em_atendimento" && n.Situacao != "finalizado" {
			return "", pgtype.Int8{}, "Só uma solicitação em atendimento ou com o atendimento finalizado pode ser encerrada."
		}
		return "encerrada", n.ResponsavelID, ""
	})
}

func (h *handlers) droneReabrir(w http.ResponseWriter, r *http.Request) {
	h.mudarSituacaoDoDrone(w, r, "reabrir", func(n sqlccliente.DroneSolicitaco) (string, pgtype.Int8, string) {
		if n.Situacao != "encerrada" {
			return "", pgtype.Int8{}, "Só uma solicitação encerrada pode ser reaberta."
		}
		return "em_atendimento", pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, ""
	})
}

func (h *handlers) droneExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDrone(w, r, q)
	if !ok {
		return
	}
	h.excluirRegistro(w, r, "drone", n.ID, "/drones?aviso=excluido", "/drones/"+strconv.FormatInt(n.ID, 10))
}

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Diária de trabalho da campanha antirrábica (10.3): equipe do dia, início e fim do trabalho, temperaturas das vacinas e perdas.

var rotulosStatusDiaria = []views.CidadeOpcao{
	{Valor: "aguardando", Rotulo: "Aguardando"},
	{Valor: "executando", Rotulo: "Executando"},
	{Valor: "finalizado", Rotulo: "Finalizado"},
	{Valor: "cancelado", Rotulo: "Cancelado"},
}

// camposDeDiaria são os números do registro da diária, na ordem da tela.
var camposDeDiaria = []struct{ Grupo, Nome, Rotulo string }{
	{"Início de trabalho", "ini_seringas", "Seringas"},
	{"Início de trabalho", "ini_frascos_fechados", "Vacinas frasco fechado"},
	{"Início de trabalho", "ini_frascos_abertos", "Vacinas frasco aberto"},
	{"Início de trabalho", "ini_doses", "Doses de vacina"},
	{"Final de trabalho: seringas", "fim_seringas_usadas", "Seringas usadas"},
	{"Final de trabalho: seringas", "fim_seringas_perdidas", "Perda de seringas"},
	{"Final de trabalho: seringas", "fim_seringas_defeito", "Seringas com defeito"},
	{"Final de trabalho: vacinas", "fim_frascos_vazios", "Frascos vazios / usados"},
	{"Final de trabalho: vacinas", "fim_frascos_cheios", "Frascos cheios / fechados"},
	{"Final de trabalho: vacinas", "fim_frascos_abertos", "Frascos abertos"},
	{"Final de trabalho: vacinas", "fim_vacinas_perdidas", "Perda de vacinas"},
	{"Defeitos, descartes e perdas", "def_seringas_defeito", "Seringas com defeito"},
	{"Defeitos, descartes e perdas", "def_seringas_descartadas", "Seringas descartadas"},
	{"Defeitos, descartes e perdas", "def_vacinas_descartadas", "Vacinas descartadas"},
	{"Defeitos, descartes e perdas", "def_vacinas_perdidas", "Vacinas perdidas"},
}

var horariosDeTemperatura = []string{"08:00", "10:00", "12:00", "14:00", "17:00"}

func valoresDeDiaria(d sqlccliente.CampanhaDiaria) map[string]int32 {
	return map[string]int32{
		"ini_seringas": d.IniSeringas, "ini_frascos_fechados": d.IniFrascosFechados, "ini_frascos_abertos": d.IniFrascosAbertos, "ini_doses": d.IniDoses,
		"fim_seringas_usadas": d.FimSeringasUsadas, "fim_seringas_perdidas": d.FimSeringasPerdidas, "fim_seringas_defeito": d.FimSeringasDefeito,
		"fim_frascos_vazios": d.FimFrascosVazios, "fim_frascos_cheios": d.FimFrascosCheios, "fim_frascos_abertos": d.FimFrascosAbertos,
		"fim_vacinas_perdidas": d.FimVacinasPerdidas, "def_seringas_defeito": d.DefSeringasDefeito, "def_seringas_descartadas": d.DefSeringasDescartadas,
		"def_vacinas_descartadas": d.DefVacinasDescartadas, "def_vacinas_perdidas": d.DefVacinasPerdidas,
	}
}

type temperaturasDiaria struct {
	Recebimento string                       `json:"recebimento"`
	Pontos      map[string]map[string]string `json:"pontos"`
}

func ordemDeDiarias(ordem, dir string) (string, string) {
	switch ordem {
	case "id", "data":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func rotuloDiaria(d sqlccliente.CampanhaDiaria) string {
	return "Diária " + strconv.FormatInt(d.ID, 10)
}

func diariaCaminho(id int64) string { return "/zoonose/campanha/diaria/" + strconv.FormatInt(id, 10) }

func (h *handlers) apenasMinhasDiarias(r *http.Request) bool {
	return h.principal(r).Restrito("campanha_diaria_meu") && meuID(r) != 0
}

func (h *handlers) diariasLista(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	qs := r.URL.Query()
	f := views.DiariasFiltro{Status: qs.Get("status"), Agente: qs.Get("agente"), De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate"))}
	if !validoEm(rotulosStatusDiaria, f.Status) {
		f.Status = ""
	}
	ordem, dir := ordemDeDiarias(qs.Get("ordem"), qs.Get("dir"))
	meu := h.apenasMinhasDiarias(r)
	arg := sqlccliente.ListDiariasParams{Status: f.Status, AgenteID: idOpcional(f.Agente), DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate), SomenteMeu: meu, UsuarioID: meuID(r), Ordem: ordem, Dir: dir}
	pag := paginaDe(r)
	total, err := q.CountDiarias(ctx, sqlccliente.CountDiariasParams{Status: arg.Status, AgenteID: arg.AgenteID, DataDe: arg.DataDe, DataAte: arg.DataAte, SomenteMeu: arg.SomenteMeu, UsuarioID: arg.UsuarioID})
	if err != nil {
		h.falhaInterna(w, r, "contar diárias", err)
		return
	}
	pag.Total = int(total)
	if pag.Page > pag.Pages() {
		pag.Page = pag.Pages()
	}
	arg.Limite, arg.Deslocamento = int32(pag.PerPage), int32((pag.Page-1)*pag.PerPage)
	rows, err := q.ListDiarias(ctx, arg)
	if err != nil {
		h.falhaInterna(w, r, "listar diárias", err)
		return
	}
	d := views.DiariasData{Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: h.principal(r).Pode("campanha_diaria_novo")}
	for _, x := range rows {
		d.Linhas = append(d.Linhas, views.DiariaLinha{ID: x.ID, IDTexto: strconv.FormatInt(x.ID, 10), Status: x.Status, StatusRotulo: rotuloDe(rotulosStatusDiaria, x.Status), Data: dataBR(x.Data), Por: x.Por, Equipe: x.Equipe})
	}
	d.Status = marcarOpcao(rotulosStatusDiaria, f.Status)
	us, err := q.ListUsuariosAtivosParaResponsavel(ctx)
	if err != nil {
		h.falhaInterna(w, r, "listar usuários", err)
		return
	}
	escolhido := arg.AgenteID.Int64
	for _, u := range us {
		d.Agentes = append(d.Agentes, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome, Selecionada: u.ID == escolhido})
	}
	d.Query = url.Values{}
	if ordem != "" {
		d.Query.Set("ordem", ordem)
		d.Query.Set("dir", dir)
	}
	for k, v := range map[string]string{"status": f.Status, "agente": f.Agente, "de": f.De, "ate": f.Ate} {
		if v != "" {
			d.Query.Set(k, v)
		}
	}
	render(w, r, views.Diarias(h.layout(r, "Diária de trabalho", "/zoonose/campanha/diaria"), d))
}

func (h *handlers) diariaNova(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.DiariaNova(h.layout(r, "Nova diária de trabalho", "/zoonose/campanha/diaria"), views.DiariaNovaData{Data: time.Now().In(h.fuso(r)).Format("2006-01-02"), CSRF: csrfToken(r.Context())}))
}

func (h *handlers) diariaCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	d := views.DiariaNovaData{Data: strings.TrimSpace(r.PostFormValue("data")), Anotacao: strings.TrimSpace(r.PostFormValue("anotacao")), CSRF: csrfToken(r.Context())}
	data := dataOpcional(d.Data)
	switch {
	case !data.Valid:
		d.Erro = "Informe a data da diária."
	case len([]rune(d.Anotacao)) > 2000:
		d.Erro = "A anotação passa de 2000 caracteres."
	}
	if d.Erro != "" {
		renderStatus(w, r, http.StatusBadRequest, views.DiariaNova(h.layout(r, "Nova diária de trabalho", "/zoonose/campanha/diaria"), d))
		return
	}
	n, err := q.CreateDiaria(r.Context(), sqlccliente.CreateDiariaParams{Data: data, Anotacao: d.Anotacao, CriadoPor: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}})
	if err != nil {
		h.falhaInterna(w, r, "criar diária", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "diaria.criar", Entidade: "diaria", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDiaria(n), Depois: map[string]any{"data": dataBR(n.Data)}})
	http.Redirect(w, r, diariaCaminho(n.ID)+"?aviso=criada", http.StatusSeeOther)
}

func (h *handlers) carregarDiaria(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries) (sqlccliente.CampanhaDiaria, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.CampanhaDiaria{}, false
	}
	d, err := q.GetDiaria(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return d, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar diária", err)
		return d, false
	}
	if h.apenasMinhasDiarias(r) {
		if ok, err := q.DiariaTemMembro(r.Context(), sqlccliente.DiariaTemMembroParams{DiariaID: id, UsuarioID: meuID(r)}); err != nil {
			h.falhaInterna(w, r, "checar equipe", err)
			return d, false
		} else if !ok {
			http.NotFound(w, r)
			return d, false
		}
	}
	return d, true
}

func (h *handlers) diariaFicha(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDiaria(w, r, q)
	if !ok {
		return
	}
	h.renderDiaria(w, r, q, n, "")
}

func (h *handlers) renderDiaria(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, n sqlccliente.CampanhaDiaria, erro string) {
	pr := h.principal(r)
	aberta := n.Status == "aguardando" || n.Status == "executando"
	d := views.DiariaFichaData{
		ID: n.ID, IDTexto: strconv.FormatInt(n.ID, 10), Data: dataBR(n.Data), DataISO: n.Data.Time.Format("2006-01-02"), Anotacao: n.Anotacao,
		Status: n.Status, StatusRotulo: rotuloDe(rotulosStatusDiaria, n.Status), Aberta: aberta, CSRF: csrfToken(r.Context()), Erro: erro,
		PodeEditar: pr.Pode("campanha_diaria_editar") && aberta, PodeRegistrar: pr.Pode("campanha_diaria_registrar") && aberta,
		PodeFinalizar: pr.Pode("campanha_diaria_finalizar"), PodeDesativar: pr.Pode("campanha_diaria_desativar"),
	}
	switch r.URL.Query().Get("aviso") {
	case "criada":
		d.Aviso = "Diária criada. Monte a equipe e registre o início do trabalho."
	case "salva":
		d.Aviso = "Dados salvos."
	}
	if n.CriadoPor.Valid {
		if u, err := q.GetUsuarioByID(r.Context(), n.CriadoPor.Int64); err == nil {
			d.CriadoPor = u.Nome
		}
	}
	vals := valoresDeDiaria(n)
	var temp temperaturasDiaria
	_ = json.Unmarshal(n.Temperaturas, &temp)
	for _, c := range camposDeDiaria {
		if len(d.Grupos) == 0 || d.Grupos[len(d.Grupos)-1].Titulo != c.Grupo {
			d.Grupos = append(d.Grupos, views.DiariaGrupo{Titulo: c.Grupo})
		}
		g := &d.Grupos[len(d.Grupos)-1]
		g.Campos = append(g.Campos, views.DiariaCampo{Nome: c.Nome, Rotulo: c.Rotulo, Valor: strconv.Itoa(int(vals[c.Nome]))})
	}
	d.Recebimento = temp.Recebimento
	for _, hh := range horariosDeTemperatura {
		d.Temperaturas = append(d.Temperaturas, views.DiariaTemperatura{Horario: hh, Estoque: temp.Pontos[hh]["estoque"], Uso: temp.Pontos[hh]["uso"]})
	}
	equipe, err := q.ListDiariaEquipe(r.Context(), n.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar equipe", err)
		return
	}
	membros := map[int64]bool{}
	for _, e := range equipe {
		membros[e.UsuarioID] = true
		d.Equipe = append(d.Equipe, views.DiariaMembro{ID: e.UsuarioID, Nome: e.Nome, Lider: e.Lider})
	}
	if d.PodeEditar {
		us, err := q.ListUsuariosAtivosParaResponsavel(r.Context())
		if err != nil {
			h.falhaInterna(w, r, "listar usuários", err)
			return
		}
		for _, u := range us {
			if !membros[u.ID] {
				d.Candidatos = append(d.Candidatos, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome})
			}
		}
	}
	cs, err := q.ListCampanhasDaDiaria(r.Context(), pgtype.Int8{Int64: n.ID, Valid: true})
	if err != nil {
		h.falhaInterna(w, r, "listar atendimentos da diária", err)
		return
	}
	loc := h.fuso(r)
	for _, c := range cs {
		d.Atendimentos = append(d.Atendimentos, views.DiariaAtendimento{ID: c.ID, Hora: c.DataHora.Time.In(loc).Format("02/01 15:04"), Endereco: c.Endereco, Vacinados: int(c.VacCachorros + c.VacGatos), NaoVacinados: int(c.NaoCachorros + c.NaoGatos)})
		d.TotalVacinados += int(c.VacCachorros + c.VacGatos)
		d.TotalNaoVacinados += int(c.NaoCachorros + c.NaoGatos)
	}
	d.EstoqueObrigatorio = h.campanhaEstoqueObrigatorio(r, q)
	if d.PodeEditar {
		if d.VacinaLotes, err = h.opcoesDeLotesDaDiaria(r, q, "vacina", n.VacinaLoteID); err == nil {
			d.SeringaLotes, err = h.opcoesDeLotesDaDiaria(r, q, "seringa", n.SeringaLoteID)
		}
		if err != nil {
			h.falhaInterna(w, r, "listar lotes da diária", err)
			return
		}
	}
	baixas, err := q.ListDiariaBaixas(r.Context(), n.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar baixas da diária", err)
		return
	}
	for _, b := range baixas {
		d.Baixas = append(d.Baixas, views.DiariaBaixa{Categoria: b.Categoria, Produto: b.Produto, Lote: b.NumeroLote, Quantidade: qtdTexto(b.Quantidade) + " " + b.UnidadeMedida})
	}
	status := http.StatusOK
	if erro != "" {
		status = http.StatusBadRequest
	}
	renderStatus(w, r, status, views.DiariaFicha(h.layout(r, "Diária "+d.IDTexto, "/zoonose/campanha/diaria"), d))
}

func (h *handlers) diariaSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDiaria(w, r, q)
	if !ok {
		return
	}
	if n.Status != "aguardando" && n.Status != "executando" {
		http.Error(w, "Diária fechada: reabra para editar.", http.StatusConflict)
		return
	}
	data := dataOpcional(strings.TrimSpace(r.PostFormValue("data")))
	anot := strings.TrimSpace(r.PostFormValue("anotacao"))
	if !data.Valid || len([]rune(anot)) > 2000 {
		h.renderDiaria(w, r, q, n, "Informe a data e uma anotação de até 2000 caracteres.")
		return
	}
	if _, err := q.UpdateDiariaDados(r.Context(), sqlccliente.UpdateDiariaDadosParams{ID: n.ID, Data: data, Anotacao: anot}); err != nil {
		h.falhaInterna(w, r, "salvar diária", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "diaria.editar", Entidade: "diaria", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDiaria(n),
		Antes: map[string]any{"data": dataBR(n.Data)}, Depois: map[string]any{"data": dataBR(data)}})
	http.Redirect(w, r, diariaCaminho(n.ID)+"?aviso=salva", http.StatusSeeOther)
}

func (h *handlers) diariaMembro(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDiaria(w, r, q)
	if !ok {
		return
	}
	uid, _ := strconv.ParseInt(r.PostFormValue("usuario"), 10, 64)
	u, err := q.GetUsuarioByID(r.Context(), uid)
	if err != nil || !u.Ativo || (n.Status != "aguardando" && n.Status != "executando") {
		h.renderDiaria(w, r, q, n, "Escolha um usuário ativo para a equipe (a diária precisa estar aberta).")
		return
	}
	lider := r.PostFormValue("lider") == "1"
	if err := q.UpsertDiariaMembro(r.Context(), sqlccliente.UpsertDiariaMembroParams{DiariaID: n.ID, UsuarioID: u.ID, Lider: lider}); err != nil {
		h.falhaInterna(w, r, "adicionar membro", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "diaria.equipe", Entidade: "diaria", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDiaria(n), Depois: map[string]any{"adicionou": u.Nome, "lider": lider}})
	http.Redirect(w, r, diariaCaminho(n.ID)+"?aviso=salva", http.StatusSeeOther)
}

func (h *handlers) diariaMembroRemover(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDiaria(w, r, q)
	if !ok {
		return
	}
	uid, _ := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	if n.Status != "aguardando" && n.Status != "executando" {
		http.Error(w, "Diária fechada: reabra para editar.", http.StatusConflict)
		return
	}
	if c, err := q.RemoveDiariaMembro(r.Context(), sqlccliente.RemoveDiariaMembroParams{DiariaID: n.ID, UsuarioID: uid}); err != nil {
		h.falhaInterna(w, r, "remover membro", err)
		return
	} else if c > 0 {
		h.auditar(r, audit.Evento{Acao: "diaria.equipe", Entidade: "diaria", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDiaria(n), Depois: map[string]any{"removeu": uid}})
	}
	http.Redirect(w, r, diariaCaminho(n.ID)+"?aviso=salva", http.StatusSeeOther)
}

func temperaturaValida(s string) (string, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return "", true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < -40 || v > 60 {
		return "", false
	}
	return strconv.FormatFloat(v, 'f', -1, 64), true
}

// diariaRegistro grava o início, o fim, as perdas e as temperaturas (diária aguardando ou executando).
func (h *handlers) diariaRegistro(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDiaria(w, r, q)
	if !ok {
		return
	}
	if n.Status != "aguardando" && n.Status != "executando" {
		http.Error(w, "Diária fechada: reabra para registrar.", http.StatusConflict)
		return
	}
	vals := map[string]int32{}
	for _, c := range camposDeDiaria {
		txt := strings.TrimSpace(r.PostFormValue(c.Nome))
		v, err := strconv.Atoi(txt)
		if txt == "" {
			v, err = 0, nil
		}
		if err != nil || v < 0 || v > 99999 {
			h.renderDiaria(w, r, q, n, c.Rotulo+": informe um número de 0 a 99999.")
			return
		}
		vals[c.Nome] = int32(v)
	}
	temp := temperaturasDiaria{Recebimento: strings.TrimSpace(r.PostFormValue("recebimento")), Pontos: map[string]map[string]string{}}
	if _, err := time.Parse("15:04", temp.Recebimento); temp.Recebimento != "" && err != nil {
		h.renderDiaria(w, r, q, n, "A hora do recebimento precisa ser no formato HH:MM.")
		return
	}
	for _, hh := range horariosDeTemperatura {
		e, ok1 := temperaturaValida(r.PostFormValue("estoque_" + hh))
		u, ok2 := temperaturaValida(r.PostFormValue("uso_" + hh))
		if !ok1 || !ok2 {
			h.renderDiaria(w, r, q, n, "As temperaturas precisam ser números entre -40 e 60 °C.")
			return
		}
		if e != "" || u != "" {
			temp.Pontos[hh] = map[string]string{"estoque": e, "uso": u}
		}
	}
	bruto, _ := json.Marshal(temp)
	if _, err := q.UpdateDiariaRegistro(r.Context(), sqlccliente.UpdateDiariaRegistroParams{
		ID: n.ID, IniSeringas: vals["ini_seringas"], IniFrascosFechados: vals["ini_frascos_fechados"], IniFrascosAbertos: vals["ini_frascos_abertos"], IniDoses: vals["ini_doses"],
		FimSeringasUsadas: vals["fim_seringas_usadas"], FimSeringasPerdidas: vals["fim_seringas_perdidas"], FimSeringasDefeito: vals["fim_seringas_defeito"],
		FimFrascosVazios: vals["fim_frascos_vazios"], FimFrascosCheios: vals["fim_frascos_cheios"], FimFrascosAbertos: vals["fim_frascos_abertos"],
		FimVacinasPerdidas: vals["fim_vacinas_perdidas"], DefSeringasDefeito: vals["def_seringas_defeito"], DefSeringasDescartadas: vals["def_seringas_descartadas"],
		DefVacinasDescartadas: vals["def_vacinas_descartadas"], DefVacinasPerdidas: vals["def_vacinas_perdidas"], Temperaturas: bruto,
	}); err != nil {
		h.falhaInterna(w, r, "registrar diária", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "diaria.registro", Entidade: "diaria", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDiaria(n),
		Antes: map[string]any{"doses_iniciais": n.IniDoses}, Depois: map[string]any{"doses_iniciais": vals["ini_doses"], "vacinas_perdidas": vals["fim_vacinas_perdidas"]}})
	http.Redirect(w, r, diariaCaminho(n.ID)+"?aviso=salva", http.StatusSeeOther)
}

// mudarStatusDaDiaria é o caminho comum das ações de andamento; regra devolve o status novo ou a mensagem de erro.
func (h *handlers) mudarStatusDaDiaria(w http.ResponseWriter, r *http.Request, acao string, regra func(n sqlccliente.CampanhaDiaria) (string, string)) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDiaria(w, r, q)
	if !ok {
		return
	}
	novo, erro := regra(n)
	if erro != "" {
		h.renderDiaria(w, r, q, n, erro)
		return
	}
	if _, err := q.SetDiariaStatus(r.Context(), sqlccliente.SetDiariaStatusParams{ID: n.ID, Status: novo}); err != nil {
		h.falhaInterna(w, r, "mudar status da diária", err)
		return
	}
	h.auditar(r, audit.Evento{Acao: "diaria." + acao, Entidade: "diaria", EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloDiaria(n),
		Antes: map[string]any{"status": n.Status}, Depois: map[string]any{"status": novo}})
	http.Redirect(w, r, diariaCaminho(n.ID)+"?aviso=salva", http.StatusSeeOther)
}

func (h *handlers) diariaIniciar(w http.ResponseWriter, r *http.Request) {
	h.mudarStatusDaDiaria(w, r, "iniciar", func(n sqlccliente.CampanhaDiaria) (string, string) {
		if n.Status != "aguardando" {
			return "", "Só uma diária aguardando pode ser iniciada."
		}
		return "executando", ""
	})
}

func (h *handlers) diariaCancelar(w http.ResponseWriter, r *http.Request) {
	h.mudarStatusDaDiaria(w, r, "cancelar", func(n sqlccliente.CampanhaDiaria) (string, string) {
		if n.Status != "aguardando" && n.Status != "executando" {
			return "", "Só uma diária aguardando ou em execução pode ser cancelada."
		}
		return "cancelado", ""
	})
}

func (h *handlers) diariaReativar(w http.ResponseWriter, r *http.Request) {
	h.mudarStatusDaDiaria(w, r, "reativar", func(n sqlccliente.CampanhaDiaria) (string, string) {
		if n.Status != "cancelado" {
			return "", "Só uma diária cancelada pode ser reativada."
		}
		return "aguardando", ""
	})
}

func (h *handlers) diariaExcluir(w http.ResponseWriter, r *http.Request) {
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	n, ok := h.carregarDiaria(w, r, q)
	if !ok {
		return
	}
	h.excluirRegistro(w, r, "diaria", n.ID, "/zoonose/campanha/diaria?aviso=excluido", diariaCaminho(n.ID))
}

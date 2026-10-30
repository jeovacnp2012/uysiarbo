package server

import (
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

// Ovitrampas e EDLs (10.1): estações de monitoramento do Aedes instaladas num local. A ovitrampa recebe contagens de ovos;
// a EDL recebe manutenções (nível da água). Os dois tipos usam a mesma tabela e os mesmos handlers.

type armadilhaTipo struct {
	Tipo, Caminho, Plural, Rotulo, RegNome string
	Ovos                                   bool
	Ver, Novo, Editar, Status              string // permissões
	Registrar, RegEditar, RegExcluir       string
	Desativar                              string
}

var tiposDeArmadilha = map[string]armadilhaTipo{
	"ovitrampa": {Tipo: "ovitrampa", Caminho: "/ovitrampas", Plural: "Ovitrampas", Rotulo: "Ovitrampa", RegNome: "Contagem de ovos", Ovos: true,
		Ver: "ovitrampa_visualizar", Novo: "ovitrampa_novo", Editar: "ovitrampa_editar", Status: "ovitrampa_status",
		Registrar: "ovitrampa_ovos", RegEditar: "ovitrampa_ovos", RegExcluir: "ovitrampa_ovos_excluir", Desativar: "ovitrampa_desativar"},
	"edl": {Tipo: "edl", Caminho: "/edls", Plural: "EDLs", Rotulo: "EDL", RegNome: "Manutenção",
		Ver: "edl_visualizar", Novo: "edl_novo", Editar: "edl_editar", Status: "edl_status",
		Registrar: "edl_manutencao", RegEditar: "edl_manutencao_editar", RegExcluir: "edl_manutencao_editar", Desativar: "edl_desativar"},
}

func (t armadilhaTipo) cfg() views.ArmadilhaCfg {
	return views.ArmadilhaCfg{Slug: t.Tipo, Caminho: t.Caminho, Plural: t.Plural, Rotulo: t.Rotulo, Ovos: t.Ovos, RegNome: t.RegNome}
}

func (t armadilhaTipo) statusOpcoes() []views.CidadeOpcao {
	operando := "Operando"
	if t.Ovos {
		operando = "Ativo"
	}
	return []views.CidadeOpcao{
		{Valor: "operando", Rotulo: operando}, {Valor: "encerrado", Rotulo: "Encerrado"}, {Valor: "extraviado", Rotulo: "Extraviado"},
		{Valor: "sem_acesso", Rotulo: "Sem acesso"}, {Valor: "cancelado", Rotulo: "Cancelado"}, {Valor: "retirado", Rotulo: "Retirado"},
	}
}

var niveisDeAgua = opcoesSimples("Seco", "< 50%", "> 50%", "Cheio")

const formatoHoraLocal = "2006-01-02T15:04"

func ordemDeArmadilhas(ordem, dir string) (string, string) {
	switch ordem {
	case "id", "data", "endereco", "status":
		if dir != "desc" {
			dir = "asc"
		}
		return ordem, dir
	}
	return "", ""
}

func rotuloArmadilha(t armadilhaTipo, a sqlccliente.Armadilha) string {
	return t.Rotulo + " " + strconv.FormatInt(a.ID, 10)
}

// horaLocal lê um datetime-local no fuso do cliente.
func horaLocal(s string, loc *time.Location) (time.Time, bool) {
	t, err := time.ParseInLocation(formatoHoraLocal, strings.TrimSpace(s), loc)
	return t, err == nil
}

func coordenadaOpcional(s string, min, max float64) (pgtype.Float8, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return pgtype.Float8{}, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < min || v > max {
		return pgtype.Float8{}, false
	}
	return pgtype.Float8{Float64: v, Valid: true}, true
}

func textoDecimal(f pgtype.Float8) string {
	if !f.Valid {
		return ""
	}
	return strconv.FormatFloat(f.Float64, 'f', -1, 64)
}

func (h *handlers) armadilhaLista(t armadilhaTipo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		qs := r.URL.Query()
		f := views.ArmadilhasFiltro{
			Busca: strings.TrimSpace(qs.Get("busca")), Status: qs.Get("status"), Bairro: qs.Get("bairro"),
			De: strings.TrimSpace(qs.Get("de")), Ate: strings.TrimSpace(qs.Get("ate")),
		}
		if !validoEm(t.statusOpcoes(), f.Status) {
			f.Status = ""
		}
		ordem, dir := ordemDeArmadilhas(qs.Get("ordem"), qs.Get("dir"))
		somente, _ := h.escopoLocais(r)
		pag := paginaDe(r)
		total, err := q.CountArmadilhas(ctx, sqlccliente.CountArmadilhasParams{
			Tipo: t.Tipo, Busca: f.Busca, Status: f.Status, BairroID: idOpcional(f.Bairro), DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate),
			SomenteUsuario: somente.Valid, UsuarioID: somente.Int64,
		})
		if err != nil {
			h.falhaInterna(w, r, "contar armadilhas", err)
			return
		}
		pag.Total = int(total)
		if pag.Page > pag.Pages() {
			pag.Page = pag.Pages()
		}
		rows, err := q.ListArmadilhas(ctx, sqlccliente.ListArmadilhasParams{
			Tipo: t.Tipo, Busca: f.Busca, Status: f.Status, BairroID: idOpcional(f.Bairro), DataDe: dataOpcional(f.De), DataAte: dataOpcional(f.Ate),
			SomenteUsuario: somente.Valid, UsuarioID: somente.Int64, Ordem: ordem, Dir: dir,
			Limite: int32(pag.PerPage), Deslocamento: int32((pag.Page - 1) * pag.PerPage),
		})
		if err != nil {
			h.falhaInterna(w, r, "listar armadilhas", err)
			return
		}
		loc := h.fuso(r)
		d := views.ArmadilhasData{Cfg: t.cfg(), Filtro: f, Ordem: ordem, Dir: dir, Pag: pag, PodeNova: h.principal(r).Pode(t.Novo)}
		for _, x := range rows {
			ultimo := ""
			if x.UltimoEm.Valid {
				ultimo = x.UltimoEm.Time.In(loc).Format("02/01/2006")
				if t.Ovos && x.UltimoOvos >= 0 {
					ultimo += " · " + strconv.Itoa(int(x.UltimoOvos)) + " ovos"
				} else if !t.Ovos && x.UltimoNivel != "" {
					ultimo += " · " + x.UltimoNivel
				}
			}
			d.Linhas = append(d.Linhas, views.ArmadilhaLinha{
				ID: x.ID, IDTexto: strconv.FormatInt(x.ID, 10), Status: x.Status, StatusRotulo: rotuloDe(t.statusOpcoes(), x.Status),
				Data: x.InstaladaEm.Time.In(loc).Format("02/01/2006 15:04"), Endereco: x.Endereco, Bairro: x.Bairro, Agente: x.Agente, UltimoRegistro: ultimo,
			})
		}
		if d.Bairros, err = h.opcoesDeBairros(r, q, idOpcional(f.Bairro).Int64); err != nil {
			h.falhaInterna(w, r, "listar bairros", err)
			return
		}
		d.Status = marcarOpcao(t.statusOpcoes(), f.Status)
		d.Query = url.Values{}
		if ordem != "" {
			d.Query.Set("ordem", ordem)
			d.Query.Set("dir", dir)
		}
		for k, v := range map[string]string{"busca": f.Busca, "status": f.Status, "bairro": f.Bairro, "de": f.De, "ate": f.Ate} {
			if v != "" {
				d.Query.Set(k, v)
			}
		}
		render(w, r, views.Armadilhas(h.layout(r, t.Plural, t.Caminho), d))
	}
}

func (h *handlers) opcoesDeAgentes(r *http.Request, q *sqlccliente.Queries, escolhido int64) ([]views.CidadeOpcao, error) {
	us, err := q.ListUsuariosAtivosParaResponsavel(r.Context())
	if err != nil {
		return nil, err
	}
	out := make([]views.CidadeOpcao, 0, len(us))
	for _, u := range us {
		out = append(out, views.CidadeOpcao{ID: u.ID, Rotulo: u.Nome, Selecionada: u.ID == escolhido})
	}
	return out, nil
}

func (h *handlers) armadilhaNova(t armadilhaTipo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			d := views.ArmadilhaFormData{
				Cfg: t.cfg(), LocalID: l.ID, LocalEndereco: enderecoCompleto(l.Endereco, l.Numero),
				Data: time.Now().In(h.fuso(r)).Format(formatoHoraLocal), CSRF: csrfToken(ctx),
			}
			if l.Latitude.Valid && l.Longitude.Valid {
				d.Latitude, d.Longitude = textoDecimal(l.Latitude), textoDecimal(l.Longitude)
			}
			if d.Agentes, err = h.opcoesDeAgentes(r, q, meuID(r)); err != nil {
				h.falhaInterna(w, r, "listar agentes", err)
				return
			}
			render(w, r, views.ArmadilhaNova(h.layout(r, "Nova "+t.Rotulo, t.Caminho), d))
			return
		}
		e := views.ArmadilhaEscolherLocal{Cfg: t.cfg(), Busca: strings.TrimSpace(r.URL.Query().Get("busca_local"))}
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
		render(w, r, views.ArmadilhaEscolher(h.layout(r, "Nova "+t.Rotulo, t.Caminho), e))
	}
}

// lerDadosDaArmadilha valida o formulário (data, agente, coordenadas, anotação).
func (h *handlers) lerDadosDaArmadilha(r *http.Request, q *sqlccliente.Queries, t armadilhaTipo, d views.ArmadilhaFormData) (views.ArmadilhaFormData, sqlccliente.CreateArmadilhaParams, string) {
	tx := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	d.Data, d.Latitude, d.Longitude, d.Anotacao = tx("data"), tx("latitude"), tx("longitude"), tx("anotacao")
	agente := idOpcional(tx("agente"))
	d.Agentes, _ = h.opcoesDeAgentes(r, q, agente.Int64)
	p := sqlccliente.CreateArmadilhaParams{Tipo: t.Tipo, LocalID: d.LocalID, Anotacao: d.Anotacao, AgenteID: agente}
	msg := ""
	hora, okHora := horaLocal(d.Data, h.fuso(r))
	p.InstaladaEm = pgtype.Timestamptz{Time: hora, Valid: okHora}
	var okLat, okLon bool
	p.Latitude, okLat = coordenadaOpcional(d.Latitude, -90, 90)
	p.Longitude, okLon = coordenadaOpcional(d.Longitude, -180, 180)
	switch {
	case !okHora:
		msg = "Informe a data e a hora da instalação."
	case !okLat || !okLon:
		msg = "Latitude ou longitude inválida."
	case p.Latitude.Valid != p.Longitude.Valid:
		msg = "Informe a latitude e a longitude juntas."
	case len([]rune(d.Anotacao)) > 2000:
		msg = "A anotação passa de 2000 caracteres."
	}
	if agente.Valid {
		if _, err := q.GetUsuarioByID(r.Context(), agente.Int64); err != nil {
			msg = "O agente escolhido não existe."
		}
	}
	return d, p, msg
}

func (h *handlers) armadilhaCriar(t armadilhaTipo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		d, p, msg := h.lerDadosDaArmadilha(r, q, t, views.ArmadilhaFormData{Cfg: t.cfg(), LocalID: l.ID, LocalEndereco: enderecoCompleto(l.Endereco, l.Numero), CSRF: csrfToken(r.Context())})
		if msg != "" {
			d.Erro = msg
			renderStatus(w, r, http.StatusBadRequest, views.ArmadilhaNova(h.layout(r, "Nova "+t.Rotulo, t.Caminho), d))
			return
		}
		p.CriadoPor = pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}
		n, err := q.CreateArmadilha(r.Context(), p)
		if err != nil {
			h.falhaInterna(w, r, "criar armadilha", err)
			return
		}
		h.auditar(r, audit.Evento{Acao: t.Tipo + ".criar", Entidade: t.Tipo, EntidadeID: strconv.FormatInt(n.ID, 10), Rotulo: rotuloArmadilha(t, n),
			Depois: map[string]any{"local": n.LocalID}})
		http.Redirect(w, r, t.Caminho+"/"+strconv.FormatInt(n.ID, 10), http.StatusSeeOther)
	}
}

func (h *handlers) carregarArmadilha(w http.ResponseWriter, r *http.Request, q *sqlccliente.Queries, t armadilhaTipo) (sqlccliente.Armadilha, bool) {
	id, ok := idDaURL(r)
	if !ok {
		http.NotFound(w, r)
		return sqlccliente.Armadilha{}, false
	}
	a, err := q.GetArmadilha(r.Context(), sqlccliente.GetArmadilhaParams{ID: id, Tipo: t.Tipo})
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return a, false
	} else if err != nil {
		h.falhaInterna(w, r, "buscar armadilha", err)
		return a, false
	}
	if l, err := q.GetLocal(r.Context(), a.LocalID); err == nil {
		if ok, err := h.localVisivel(r.Context(), r, q, l); err != nil {
			h.falhaInterna(w, r, "checar acesso ao local", err)
			return a, false
		} else if !ok {
			http.NotFound(w, r)
			return a, false
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		h.falhaInterna(w, r, "buscar local", err)
		return a, false
	}
	return a, true
}

func avisoDaArmadilha(c string) string {
	switch c {
	case "dados":
		return "Dados salvos."
	case "status":
		return "Status alterado."
	case "registro":
		return "Registro salvo."
	case "excluido":
		return "Registro excluído."
	}
	return ""
}

func (h *handlers) armadilhaFicha(t armadilhaTipo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { h.mostrarArmadilha(w, r, t, nil, "") }
}

func (h *handlers) mostrarArmadilha(w http.ResponseWriter, r *http.Request, t armadilhaTipo, dadosForm *views.ArmadilhaFormData, erro string) {
	ctx := r.Context()
	q, _, err := h.cliente(r)
	if err != nil {
		h.falhaInterna(w, r, "pool do cliente", err)
		return
	}
	a, ok := h.carregarArmadilha(w, r, q, t)
	if !ok {
		return
	}
	pr := h.principal(r)
	loc := h.fuso(r)
	d := views.ArmadilhaFichaData{
		Cfg: t.cfg(), ID: a.ID, IDTexto: strconv.FormatInt(a.ID, 10), Status: a.Status, StatusRotulo: rotuloDe(t.statusOpcoes(), a.Status),
		Data: a.InstaladaEm.Time.In(loc).Format("02/01/2006 15:04"), Anotacao: a.Anotacao, LocalID: a.LocalID,
		Niveis: niveisDeAgua, StatusOpcoes: marcarOpcao(t.statusOpcoes(), a.Status), AgoraLocal: time.Now().In(loc).Format(formatoHoraLocal),
		PodeEditar: pr.Pode(t.Editar), PodeStatus: pr.Pode(t.Status), PodeRegistrar: pr.Pode(t.Registrar) && a.Status == "operando",
		PodeRegEditar: pr.Pode(t.RegEditar), PodeRegExcluir: pr.Pode(t.RegExcluir), CSRF: csrfToken(ctx),
		Aviso: avisoDaArmadilha(r.URL.Query().Get("aviso")), Erro: erro,
	}
	if a.Latitude.Valid && a.Longitude.Valid {
		d.Coordenadas = textoDecimal(a.Latitude) + ", " + textoDecimal(a.Longitude)
	}
	if a.RetiradaEm.Valid {
		d.Retirada = a.RetiradaEm.Time.In(loc).Format("02/01/2006 15:04")
	}
	nome := func(id pgtype.Int8) string {
		if id.Valid {
			if u, err := q.GetUsuarioByID(ctx, id.Int64); err == nil {
				return u.Nome
			}
		}
		return ""
	}
	d.Agente, d.RetiradoPor = nome(a.AgenteID), nome(a.RetiradoPorID)
	if l, err := q.GetLocalQualquer(ctx, a.LocalID); err == nil {
		d.LocalEndereco = enderecoCompleto(l.Endereco, l.Numero)
		if b, err := q.GetBairro(ctx, l.BairroID); err == nil {
			d.Bairro = b.Nome
		}
	}
	regs, err := q.ListArmadilhaRegistros(ctx, a.ID)
	if err != nil {
		h.falhaInterna(w, r, "listar registros", err)
		return
	}
	for _, x := range regs {
		linha := views.ArmadilhaRegistroLinha{ID: x.ID, Data: x.RegistradoEm.Time.In(loc).Format("02/01/2006 15:04"), Por: x.Por, Nota: x.Anotacao, Nivel: x.NivelAgua}
		if t.Ovos {
			if x.Ovos.Valid {
				linha.Ovos = strconv.Itoa(int(x.Ovos.Int32))
			}
			linha.Texto = linha.Ovos
		} else {
			linha.Texto = x.NivelAgua
			if linha.Texto == "" {
				linha.Texto = "—"
			}
		}
		d.Registros = append(d.Registros, linha)
	}
	if d.Usuarios, err = h.opcoesDeAgentes(r, q, 0); err != nil {
		h.falhaInterna(w, r, "listar usuários", err)
		return
	}
	if dadosForm != nil {
		d.Dados = *dadosForm
	} else if d.PodeEditar {
		f := views.ArmadilhaFormData{
			Cfg: t.cfg(), ID: a.ID, LocalID: a.LocalID, LocalEndereco: d.LocalEndereco, Data: a.InstaladaEm.Time.In(loc).Format(formatoHoraLocal),
			Latitude: textoDecimal(a.Latitude), Longitude: textoDecimal(a.Longitude), Anotacao: a.Anotacao, CSRF: d.CSRF,
		}
		if f.Agentes, err = h.opcoesDeAgentes(r, q, a.AgenteID.Int64); err != nil {
			h.falhaInterna(w, r, "listar agentes", err)
			return
		}
		d.Dados = f
	}
	render(w, r, views.ArmadilhaFicha(h.layout(r, t.Rotulo+" "+d.IDTexto, t.Caminho), d))
}

func (h *handlers) armadilhaSalvar(t armadilhaTipo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Requisição inválida.", http.StatusBadRequest)
			return
		}
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		antes, ok := h.carregarArmadilha(w, r, q, t)
		if !ok {
			return
		}
		d, p, msg := h.lerDadosDaArmadilha(r, q, t, views.ArmadilhaFormData{Cfg: t.cfg(), ID: antes.ID, LocalID: antes.LocalID, CSRF: csrfToken(r.Context())})
		if msg != "" {
			d.Erro = msg
			w.WriteHeader(http.StatusBadRequest)
			h.mostrarArmadilha(w, r, t, &d, "")
			return
		}
		novo, err := q.UpdateArmadilhaDados(r.Context(), sqlccliente.UpdateArmadilhaDadosParams{
			ID: antes.ID, InstaladaEm: p.InstaladaEm, AgenteID: p.AgenteID, Latitude: p.Latitude, Longitude: p.Longitude, Anotacao: p.Anotacao,
		})
		if err != nil {
			h.falhaInterna(w, r, "salvar armadilha", err)
			return
		}
		ev := audit.Evento{Acao: t.Tipo + ".editar", Entidade: t.Tipo, EntidadeID: strconv.FormatInt(antes.ID, 10), Rotulo: rotuloArmadilha(t, novo)}
		ev.Antes, ev.Depois = audit.Diferenca(
			map[string]any{"anotacao": antes.Anotacao, "latitude": antes.Latitude.Float64, "longitude": antes.Longitude.Float64, "agente": antes.AgenteID.Int64},
			map[string]any{"anotacao": novo.Anotacao, "latitude": novo.Latitude.Float64, "longitude": novo.Longitude.Float64, "agente": novo.AgenteID.Int64})
		h.auditar(r, ev)
		http.Redirect(w, r, t.Caminho+"/"+strconv.FormatInt(antes.ID, 10)+"?aviso=dados", http.StatusSeeOther)
	}
}

func (h *handlers) armadilhaStatus(t armadilhaTipo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Requisição inválida.", http.StatusBadRequest)
			return
		}
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		a, ok := h.carregarArmadilha(w, r, q, t)
		if !ok {
			return
		}
		novo := r.PostFormValue("status")
		if !validoEm(t.statusOpcoes(), novo) {
			h.mostrarArmadilha(w, r, t, nil, "Escolha o status.")
			return
		}
		p := sqlccliente.SetArmadilhaStatusParams{ID: a.ID, Status: novo}
		if novo == "retirado" {
			hora, ok := horaLocal(r.PostFormValue("retirada"), h.fuso(r))
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				h.mostrarArmadilha(w, r, t, nil, "Informe a data da retirada.")
				return
			}
			p.RetiradaEm = pgtype.Timestamptz{Time: hora, Valid: true}
			p.RetiradoPorID = idOpcional(r.PostFormValue("retirado_por"))
		}
		if _, err := q.SetArmadilhaStatus(r.Context(), p); err != nil {
			h.falhaInterna(w, r, "mudar status", err)
			return
		}
		h.auditar(r, audit.Evento{Acao: t.Tipo + ".status", Entidade: t.Tipo, EntidadeID: strconv.FormatInt(a.ID, 10), Rotulo: rotuloArmadilha(t, a),
			Antes: map[string]any{"status": a.Status}, Depois: map[string]any{"status": novo}})
		http.Redirect(w, r, t.Caminho+"/"+strconv.FormatInt(a.ID, 10)+"?aviso=status", http.StatusSeeOther)
	}
}

// lerRegistro valida a contagem de ovos (ovitrampa) ou o nível da água (EDL).
func lerRegistro(r *http.Request, t armadilhaTipo) (ovos pgtype.Int4, nivel, nota, msg string) {
	nota = strings.TrimSpace(r.PostFormValue("anotacao"))
	if len([]rune(nota)) > 2000 {
		return ovos, "", nota, "A anotação passa de 2000 caracteres."
	}
	if t.Ovos {
		n, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("ovos")))
		if err != nil || n < 0 || n > 100000 {
			return ovos, "", nota, "Informe a quantidade de ovos (0 a 100000)."
		}
		return pgtype.Int4{Int32: int32(n), Valid: true}, "", nota, ""
	}
	nivel = strings.TrimSpace(r.PostFormValue("nivel"))
	if nivel != "" && !validoEm(niveisDeAgua, nivel) {
		return ovos, "", nota, "Nível da água inválido."
	}
	return ovos, nivel, nota, ""
}

func (h *handlers) armadilhaRegistroCriar(t armadilhaTipo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Requisição inválida.", http.StatusBadRequest)
			return
		}
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		a, ok := h.carregarArmadilha(w, r, q, t)
		if !ok {
			return
		}
		if a.Status != "operando" {
			w.WriteHeader(http.StatusConflict)
			h.mostrarArmadilha(w, r, t, nil, "Só uma armadilha operando recebe registros.")
			return
		}
		ovos, nivel, nota, msg := lerRegistro(r, t)
		hora, okHora := horaLocal(r.PostFormValue("data"), h.fuso(r))
		if msg == "" && !okHora {
			msg = "Informe a data e a hora."
		}
		if msg != "" {
			w.WriteHeader(http.StatusBadRequest)
			h.mostrarArmadilha(w, r, t, nil, msg)
			return
		}
		reg, err := q.CreateArmadilhaRegistro(r.Context(), sqlccliente.CreateArmadilhaRegistroParams{
			ArmadilhaID: a.ID, Ovos: ovos, NivelAgua: nivel, RegistradoEm: pgtype.Timestamptz{Time: hora, Valid: true},
			PorID: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0}, Anotacao: nota,
		})
		if err != nil {
			h.falhaInterna(w, r, "registrar", err)
			return
		}
		h.auditar(r, audit.Evento{Acao: t.Tipo + ".registro", Entidade: t.Tipo, EntidadeID: strconv.FormatInt(a.ID, 10), Rotulo: rotuloArmadilha(t, a),
			Depois: map[string]any{"registro": reg.ID, "ovos": reg.Ovos.Int32, "nivel_agua": reg.NivelAgua}})
		http.Redirect(w, r, t.Caminho+"/"+strconv.FormatInt(a.ID, 10)+"?aviso=registro#registros", http.StatusSeeOther)
	}
}

func (h *handlers) armadilhaRegistroSalvar(t armadilhaTipo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Requisição inválida.", http.StatusBadRequest)
			return
		}
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		a, ok := h.carregarArmadilha(w, r, q, t)
		if !ok {
			return
		}
		rid, _ := strconv.ParseInt(chi.URLParam(r, "rid"), 10, 64)
		antes, err := q.GetArmadilhaRegistro(r.Context(), sqlccliente.GetArmadilhaRegistroParams{ID: rid, ArmadilhaID: a.ID})
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ovos, nivel, nota, msg := lerRegistro(r, t)
		if msg != "" {
			w.WriteHeader(http.StatusBadRequest)
			h.mostrarArmadilha(w, r, t, nil, msg)
			return
		}
		if _, err := q.UpdateArmadilhaRegistro(r.Context(), sqlccliente.UpdateArmadilhaRegistroParams{ID: rid, ArmadilhaID: a.ID, Ovos: ovos, NivelAgua: nivel, Anotacao: nota}); err != nil {
			h.falhaInterna(w, r, "salvar registro", err)
			return
		}
		h.auditar(r, audit.Evento{Acao: t.Tipo + ".registro_editar", Entidade: t.Tipo, EntidadeID: strconv.FormatInt(a.ID, 10), Rotulo: rotuloArmadilha(t, a),
			Antes: map[string]any{"ovos": antes.Ovos.Int32, "nivel_agua": antes.NivelAgua}, Depois: map[string]any{"ovos": ovos.Int32, "nivel_agua": nivel}})
		http.Redirect(w, r, t.Caminho+"/"+strconv.FormatInt(a.ID, 10)+"?aviso=registro#registros", http.StatusSeeOther)
	}
}

func (h *handlers) armadilhaRegistroExcluir(t armadilhaTipo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		a, ok := h.carregarArmadilha(w, r, q, t)
		if !ok {
			return
		}
		rid, _ := strconv.ParseInt(chi.URLParam(r, "rid"), 10, 64)
		n, err := q.SoftDeleteArmadilhaRegistro(r.Context(), sqlccliente.SoftDeleteArmadilhaRegistroParams{
			ID: rid, ArmadilhaID: a.ID, Por: pgtype.Int8{Int64: meuID(r), Valid: meuID(r) != 0},
		})
		if err != nil {
			h.falhaInterna(w, r, "excluir registro", err)
			return
		}
		if n == 0 {
			http.NotFound(w, r)
			return
		}
		h.auditar(r, audit.Evento{Acao: t.Tipo + ".registro_excluir", Entidade: t.Tipo, EntidadeID: strconv.FormatInt(a.ID, 10), Rotulo: rotuloArmadilha(t, a),
			Depois: map[string]any{"registro": rid}})
		http.Redirect(w, r, t.Caminho+"/"+strconv.FormatInt(a.ID, 10)+"?aviso=excluido#registros", http.StatusSeeOther)
	}
}

func (h *handlers) armadilhaExcluir(t armadilhaTipo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		a, ok := h.carregarArmadilha(w, r, q, t)
		if !ok {
			return
		}
		h.excluirRegistro(w, r, t.Tipo, a.ID, t.Caminho+"?aviso=excluido", t.Caminho+"/"+strconv.FormatInt(a.ID, 10))
	}
}

// registrarRotasDeArmadilha liga as telas de um tipo de armadilha ao roteador.
func (h *handlers) registrarRotasDeArmadilha(r chi.Router, t armadilhaTipo) {
	r.Get(t.Caminho, h.exigir(t.Ver, h.armadilhaLista(t)))
	r.Get(t.Caminho+"/nova", h.exigir(t.Novo, h.armadilhaNova(t)))
	r.Post(t.Caminho, h.exigir(t.Novo, h.armadilhaCriar(t)))
	r.Get(t.Caminho+"/{id}", h.exigir(t.Ver, h.armadilhaFicha(t)))
	r.Post(t.Caminho+"/{id}", h.exigir(t.Editar, h.armadilhaSalvar(t)))
	r.Post(t.Caminho+"/{id}/status", h.exigir(t.Status, h.armadilhaStatus(t)))
	r.Post(t.Caminho+"/{id}/registros", h.exigir(t.Registrar, h.armadilhaRegistroCriar(t)))
	r.Post(t.Caminho+"/{id}/registros/{rid}", h.exigir(t.RegEditar, h.armadilhaRegistroSalvar(t)))
	r.Post(t.Caminho+"/{id}/registros/{rid}/excluir", h.exigir(t.RegExcluir, h.armadilhaRegistroExcluir(t)))
	r.Post(t.Caminho+"/{id}/excluir", h.exigir(t.Desativar, h.armadilhaExcluir(t)))
}

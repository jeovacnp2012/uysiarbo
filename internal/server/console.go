package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/authz"
	"uysiarbo/internal/db/sqlc"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/internal/tenant"
	"uysiarbo/web/views"
)

// somenteMaster libera a tela só para o master, no console da plataforma.
func (h *handlers) somenteMaster(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, _ := authFrom(r.Context())
		if _, cliente := tenant.FromContext(r.Context()); cliente || a.sess == nil || h.tenants == nil {
			http.NotFound(w, r)
			return
		}
		if !a.sess.Principal.Master {
			renderStatus(w, r, http.StatusForbidden, views.SemPermissao(h.layout(r, "Sem permissão", "")))
			return
		}
		next(w, r)
	}
}

// comOrigem prepara o contexto para as camadas de baixo registrarem a auditoria com o ator certo.
func comOrigem(r *http.Request) context.Context {
	return audit.ComOrigem(r.Context(), atorDaSessao(r), ipDe(r), r.UserAgent(), r.Header.Get("X-Request-Id"))
}

// principal devolve as permissões de quem está logado; sem login configurado (testes simples),
// trata como master.
func (h *handlers) principal(r *http.Request) authz.Principal {
	if a, ok := authFrom(r.Context()); ok && a.sess != nil {
		return a.sess.Principal
	}
	return authz.Master()
}

// ------------------------------------------------------------ Painel inicial

func (h *handlers) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pr := h.principal(r)
	d := views.PainelData{Sistema: "indisponível", Master: pr.Master}
	if info, err := h.cat.GetAppInfo(ctx, "sistema"); err == nil {
		d.Sistema = info.Valor
	} else {
		h.log.Warn("ler app_info", "erro", err)
	}
	if a, ok := authFrom(ctx); ok && a.sess != nil {
		d.Usuario = a.sess.Conta.Nome
	}

	if c, ok := tenant.FromContext(ctx); ok {
		d.Cliente, d.Schema = "indisponível", c.SchemaNome
		if pool, err := h.pools.Get(ctx, c.SchemaNome); err != nil {
			h.log.Error("pool do cliente", "cliente", c.Slug, "erro", err)
		} else {
			q := sqlccliente.New(pool)
			if info, err := q.GetClienteInfo(ctx, "nome"); err == nil {
				d.Cliente = info.Valor
			} else {
				h.log.Error("ler cliente_info", "cliente", c.Slug, "erro", err)
			}
			if pr.Pode("auditoria_sistema") {
				d.Ultimos = h.ultimosEventos(ctx, h.fuso(r), q, nil)
			}
		}
		if pr.Pode("bairro_visualizar") {
			d.Atalhos = append(d.Atalhos,
				views.Atalho{Titulo: "Bairros", Descricao: "Bairros, responsáveis e quarteirões.", Href: "/bairros"},
				views.Atalho{Titulo: "Cidades", Descricao: "Os municípios atendidos.", Href: "/cidades"},
			)
		}
		if pr.Pode("local_visualizar") {
			d.Atalhos = append(d.Atalhos, views.Atalho{Titulo: "Locais", Descricao: "Imóveis e pontos visitados, com moradores e habitantes.", Href: "/locais"})
		}
		if pr.Pode("conf_usuario") {
			d.Atalhos = append(d.Atalhos, views.Atalho{Titulo: "Usuários", Descricao: "Cadastre pessoas, escolha o papel e resete senhas.", Href: "/usuarios"})
		}
		if pr.Pode("conf_papel") {
			d.Atalhos = append(d.Atalhos, views.Atalho{Titulo: "Papéis", Descricao: "Defina o que cada papel pode fazer, por bloco.", Href: "/papeis"})
		}
		if pr.Pode("auditoria_sistema") {
			d.Atalhos = append(d.Atalhos, views.Atalho{Titulo: "Auditoria", Descricao: "Histórico de tudo o que foi feito neste ambiente.", Href: "/auditoria"})
		}
		for _, b := range authz.Padrao.Blocos {
			if pr.PodeBloco(authz.Padrao, b.Nome) {
				m := views.Modulo{Nome: b.Nome}
				if pronto, ok := modulosProntos[b.Nome]; ok && pr.Pode(pronto.permissao) {
					m.Href = pronto.href
				}
				d.Modulos = append(d.Modulos, m)
			}
		}
	} else {
		d.Console = true
		if lista, err := h.cat.ListClientes(ctx); err == nil {
			for _, c := range lista {
				if c.Status == "ativo" {
					d.ClientesAtivos++
				} else {
					d.ClientesSuspensos++
				}
			}
		}
		if pr.Master {
			d.Atalhos = append(d.Atalhos,
				views.Atalho{Titulo: "Clientes", Descricao: "Crie prefeituras, escolha o subdomínio, suspenda e abra cada ambiente.", Href: "/clientes"},
				views.Atalho{Titulo: "Usuários da plataforma", Descricao: "Masters que administram todos os clientes.", Href: "/plataforma/usuarios"},
				views.Atalho{Titulo: "Auditoria", Descricao: "Histórico das ações da plataforma e dos clientes.", Href: "/auditoria"},
			)
			if h.pub != nil {
				d.Ultimos = h.ultimosEventos(ctx, h.fuso(r), nil, h.pub)
			}
		}
	}
	d.Atalhos = append(d.Atalhos, views.Atalho{Titulo: "Componentes", Descricao: "Demonstração dos componentes de tela.", Href: "/componentes"})
	render(w, r, views.Home(h.layout(r, "Início", "/"), d))
}

// ultimosEventos devolve as 5 ações mais recentes do histórico (do cliente ou da plataforma).
func (h *handlers) ultimosEventos(ctx context.Context, loc *time.Location, cq *sqlccliente.Queries, pq *sqlc.Queries) []views.AuditoriaLinha {
	regs, _, err := buscarAuditoria(ctx, cq, pq, filtroAuditoria{}, 5, 0)
	if err != nil {
		h.log.Error("últimos eventos", "erro", err)
		return nil
	}
	out := make([]views.AuditoriaLinha, len(regs))
	for i, g := range regs {
		out[i] = linhaAuditoria(g, loc)
	}
	return out
}

// ------------------------------------------------------------------ Auditoria

type registroAuditoria struct {
	ID                                           int64
	Quando                                       time.Time
	AtorTipo, AtorNome, AtorEmail, IP, UserAgent string
	Acao, Entidade, EntidadeID, Rotulo           string
	Antes, Depois, Detalhes                      []byte
}

type filtroAuditoria struct {
	de, ate           time.Time
	ator, acao, busca string
}

func (f filtroAuditoria) ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// buscarAuditoria consulta o histórico do cliente (cq) ou da plataforma (pq): exatamente um é usado.
func buscarAuditoria(ctx context.Context, cq *sqlccliente.Queries, pq *sqlc.Queries, f filtroAuditoria, limite, deslocamento int) ([]registroAuditoria, int, error) {
	var out []registroAuditoria
	var total int
	if cq != nil {
		rows, err := cq.ListAuditoria(ctx, sqlccliente.ListAuditoriaParams{
			De: f.ts(f.de), Ate: f.ts(f.ate), Ator: f.ator, Acao: f.acao, Busca: f.busca, Limite: int32(limite), Deslocamento: int32(deslocamento),
		})
		if err != nil {
			return nil, 0, err
		}
		n, err := cq.CountAuditoria(ctx, sqlccliente.CountAuditoriaParams{De: f.ts(f.de), Ate: f.ts(f.ate), Ator: f.ator, Acao: f.acao, Busca: f.busca})
		if err != nil {
			return nil, 0, err
		}
		total = int(n)
		for _, r := range rows {
			out = append(out, registroAuditoria{r.ID, r.Quando.Time, r.AtorTipo, r.AtorNome, r.AtorEmail, r.Ip, r.UserAgent, r.Acao, r.Entidade, r.EntidadeID, r.Rotulo, r.Antes, r.Depois, r.Detalhes})
		}
		return out, total, nil
	}
	rows, err := pq.ListAuditoriaPlataforma(ctx, sqlc.ListAuditoriaPlataformaParams{
		De: f.ts(f.de), Ate: f.ts(f.ate), Ator: f.ator, Acao: f.acao, Busca: f.busca, Limite: int32(limite), Deslocamento: int32(deslocamento),
	})
	if err != nil {
		return nil, 0, err
	}
	n, err := pq.CountAuditoriaPlataforma(ctx, sqlc.CountAuditoriaPlataformaParams{De: f.ts(f.de), Ate: f.ts(f.ate), Ator: f.ator, Acao: f.acao, Busca: f.busca})
	if err != nil {
		return nil, 0, err
	}
	for _, r := range rows {
		out = append(out, registroAuditoria{r.ID, r.Quando.Time, r.AtorTipo, r.AtorNome, r.AtorEmail, r.Ip, r.UserAgent, r.Acao, r.Entidade, r.EntidadeID, r.Rotulo, r.Antes, r.Depois, r.Detalhes})
	}
	return out, int(n), nil
}

var nomesDeCampo = map[string]string{
	"nome": "Nome", "email": "E-mail", "telefone": "Telefone", "papel": "Papel", "descricao": "Descrição", "ativo": "Ativo",
	"login_web": "Acesso web", "login_tablet": "Acesso tablet", "status": "Situação", "slug": "Identificador",
	"subdominio": "Subdomínio", "dominio": "Domínio próprio", "plano": "Plano", "schema": "Schema", "master": "Master",
	"permissoes_adicionadas": "Permissões acrescentadas", "permissoes_removidas": "Permissões retiradas",
	"total_permissoes": "Total de permissões", "tentativa": "Tentativa nº", "troca_pendente": "Troca de senha pendente",
	"troca_obrigatoria": "Troca obrigatória", "sessoes_encerradas": "Sessões encerradas",
	"metodo": "Método", "caminho": "Caminho",
	"clientes": "Clientes", "com_erro": "Com erro", "cidade": "Cidade", "mutirao": "Mutirão", "classificacao": "Classificação",
	"zona": "Zona", "categoria": "Categoria", "latitude": "Latitude", "longitude": "Longitude", "altitude": "Altitude (m)",
	"distancia_centro": "Distância do centro (km)", "producao": "Entra no relatório de produção",
	"responsaveis_adicionados": "Responsáveis acrescentados", "responsaveis_removidos": "Responsáveis retirados",
	"escopos": "Escopos", "validade": "Validade", "limite_por_minuto": "Chamadas por minuto", "client_id": "Identificador (client_id)", "versao_do_segredo": "Versão do segredo",
	"atualizados": "Atualizados", "ignorados": "Ignorados", "erros": "Erros", "linhas": "Linhas", "a_criar": "A criar", "a_atualizar": "A atualizar", "a_ignorar": "A ignorar", "atualizar_existentes": "Atualizar existentes", "desfeitos": "Desfeitos", "mantidos": "Mantidos",
	"para_a_lixeira": "Enviado para a lixeira", "definitivo": "Exclusão definitiva",
	"alvo": "Onde", "referencia": "Referência", "acrescentados": "Acrescentados", "retirados": "Retirados", "locais": "Locais", "origem": "Origem", "endereco": "Endereço", "numero": "Número",
	"tipo": "Tipo", "lado": "Lado", "sequencia": "Sequência", "bairro": "Bairro", "quarteirao": "Quarteirão",
	"criados": "Criados", "ja_existiam": "Já existiam", "quarteiroes": "Quarteirões", "uf": "UF",
}

func nomeDeCampo(k string) string {
	if n, ok := nomesDeCampo[k]; ok {
		return n
	}
	return k
}

func formatarValor(chave string, v any) string {
	switch x := v.(type) {
	case nil:
		return "—"
	case bool:
		if x {
			return "sim"
		}
		return "não"
	case string:
		if x == "" {
			return "(vazio)"
		}
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case []any:
		if len(x) == 0 {
			return "—"
		}
		partes := make([]string, len(x))
		for i, e := range x {
			s := formatarValor(chave, e)
			if strings.HasPrefix(chave, "permissoes_") {
				if p, ok := authz.Padrao.Permissao(s); ok {
					s = p.Rotulo + " (" + s + ")"
				}
			}
			partes[i] = s
		}
		return strings.Join(partes, "; ")
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func mapaJSON(b []byte) map[string]any {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func linhaAuditoria(g registroAuditoria, loc *time.Location) views.AuditoriaLinha {
	l := views.AuditoriaLinha{
		ID: g.ID, Quando: formatarData(loc, g.Quando), Ator: g.AtorNome, AtorTipo: g.AtorTipo, Acao: audit.Rotulo(g.Acao), Codigo: g.Acao,
		Registro: g.Rotulo, IP: g.IP, Navegador: g.UserAgent,
	}
	if l.Ator == "" {
		l.Ator = g.AtorEmail
	}
	if g.Rotulo == "" {
		l.Registro = g.Entidade
	}
	antes, depois := mapaJSON(g.Antes), mapaJSON(g.Depois)
	chaves := map[string]bool{}
	for k := range antes {
		chaves[k] = true
	}
	for k := range depois {
		chaves[k] = true
	}
	ordenadas := make([]string, 0, len(chaves))
	for k := range chaves {
		ordenadas = append(ordenadas, k)
	}
	sort.Strings(ordenadas)
	for _, k := range ordenadas {
		a, oka := antes[k]
		d, okd := depois[k]
		ta, td := "—", "—"
		if oka {
			ta = formatarValor(k, a)
		}
		if okd {
			td = formatarValor(k, d)
		}
		l.Mudancas = append(l.Mudancas, views.Mudanca{Campo: nomeDeCampo(k), Antes: ta, Depois: td})
	}
	det := mapaJSON(g.Detalhes)
	dk := make([]string, 0, len(det))
	for k := range det {
		dk = append(dk, k)
	}
	sort.Strings(dk)
	for _, k := range dk {
		if v := det[k]; v != nil {
			if lst, ok := v.([]any); ok && len(lst) == 0 {
				continue
			}
			l.Observacoes = append(l.Observacoes, views.Observacao{Campo: nomeDeCampo(k), Valor: formatarValor(k, v)})
		}
	}
	return l
}

func (h *handlers) auditoriaPagina(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, _ := authFrom(ctx)
	_, cliente := tenant.FromContext(ctx)
	switch {
	case a.sess == nil:
		http.NotFound(w, r)
		return
	case cliente && !a.sess.Principal.Pode("auditoria_sistema"):
		renderStatus(w, r, http.StatusForbidden, views.SemPermissao(h.layout(r, "Sem permissão", "")))
		return
	case !cliente && !a.sess.Principal.Master:
		renderStatus(w, r, http.StatusForbidden, views.SemPermissao(h.layout(r, "Sem permissão", "")))
		return
	}

	qs := r.URL.Query()
	filtro := views.AuditoriaFiltro{De: qs.Get("de"), Ate: qs.Get("ate"), Ator: strings.TrimSpace(qs.Get("ator")), Acao: qs.Get("acao"), Busca: strings.TrimSpace(qs.Get("busca"))}
	f := filtroAuditoria{ator: filtro.Ator, acao: filtro.Acao, busca: filtro.Busca}
	if t, err := time.ParseInLocation("2006-01-02", filtro.De, h.fuso(r)); err == nil {
		f.de = t
	}
	if t, err := time.ParseInLocation("2006-01-02", filtro.Ate, h.fuso(r)); err == nil {
		f.ate = t.AddDate(0, 0, 1) // o dia inteiro
	}

	pag := paginaDe(r)
	if n, err := strconv.Atoi(qs.Get("pagina")); err == nil && n > 1 {
		pag.Page = n
	}

	var cq *sqlccliente.Queries
	var pq *sqlc.Queries
	if cliente {
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		cq = q
	} else {
		pq = h.pub
	}
	regs, total, err := buscarAuditoria(ctx, cq, pq, f, pag.PerPage, (pag.Page-1)*pag.PerPage)
	if err != nil {
		h.falhaInterna(w, r, "consultar auditoria", err)
		return
	}
	pag.Total = total
	if pag.Page > pag.Pages() { // página além do fim: mostra a última
		pag.Page = pag.Pages()
		if regs, _, err = buscarAuditoria(ctx, cq, pq, f, pag.PerPage, (pag.Page-1)*pag.PerPage); err != nil {
			h.falhaInterna(w, r, "consultar auditoria", err)
			return
		}
	}

	var codigos []string
	if cq != nil {
		codigos, err = cq.ListAcoesAuditoria(ctx)
	} else {
		codigos, err = pq.ListAcoesAuditoriaPlataforma(ctx)
	}
	if err != nil {
		h.falhaInterna(w, r, "listar ações", err)
		return
	}
	acoes := make([]views.OpcaoAcao, 0, len(codigos))
	for _, c := range codigos {
		acoes = append(acoes, views.OpcaoAcao{Codigo: c, Rotulo: audit.Rotulo(c), Marcada: c == filtro.Acao})
	}
	slices.SortFunc(acoes, func(x, y views.OpcaoAcao) int { return strings.Compare(x.Rotulo, y.Rotulo) })

	linhas := make([]views.AuditoriaLinha, len(regs))
	for i, g := range regs {
		linhas[i] = linhaAuditoria(g, h.fuso(r))
	}
	query := url.Values{}
	for k, v := range map[string]string{"de": filtro.De, "ate": filtro.Ate, "ator": filtro.Ator, "acao": filtro.Acao, "busca": filtro.Busca} {
		if v != "" {
			query.Set(k, v)
		}
	}
	titulo := "Auditoria"
	render(w, r, views.Auditoria(h.layout(r, titulo, "/auditoria"), views.AuditoriaData{
		Console: !cliente, Filtro: filtro, Acoes: acoes, Linhas: linhas, Pag: pag, Query: query,
	}))
}

// ------------------------------------------------------------------- Clientes

var reSubdominio = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// enderecoDoCliente monta o endereço para abrir o ambiente: mesmo esquema e mesma porta da requisição.
func (h *handlers) enderecoDoCliente(r *http.Request, c sqlc.Cliente) (host, link string) {
	host = h.resolver.Host(c)
	esquema := "http"
	if ehHTTPS(r) {
		esquema = "https"
	}
	porta := ""
	if i := strings.LastIndex(r.Host, ":"); i > 0 && !strings.Contains(r.Host[i:], "]") {
		porta = r.Host[i:]
	}
	return host, esquema + "://" + host + porta
}

func (h *handlers) clientesLista(w http.ResponseWriter, r *http.Request) {
	lista, err := h.cat.ListClientes(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar clientes", err)
		return
	}
	linhas := make([]views.ClienteLinha, len(lista))
	for i, c := range lista {
		host, link := h.enderecoDoCliente(r, c)
		linhas[i] = views.ClienteLinha{
			Nome: c.Nome, Slug: c.Slug, Endereco: host, URL: link, Status: c.Status, Schema: c.SchemaNome,
			Cidade: cidadeUF(c.Cidade, c.Uf), Fuso: c.Fuso, Criado: formatarData(h.fuso(r), c.CriadoEm.Time),
		}
	}
	lay := h.layout(r, "Clientes", "/clientes")
	linhas = paginar(r, &lay, linhas)
	render(w, r, views.Clientes(lay, views.ClientesData{Linhas: linhas, CSRF: csrfToken(r.Context())}))
}

func (h *handlers) clienteNovoForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.ClienteForm(h.layout(r, "Novo cliente", "/clientes"), h.formCliente(r, views.ClienteFormData{Plano: "padrao"})))
}

func (h *handlers) clienteCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	in := tenant.Input{
		Nome: strings.TrimSpace(r.PostFormValue("nome")), Slug: strings.ToLower(strings.TrimSpace(r.PostFormValue("slug"))),
		Subdominio: strings.ToLower(strings.TrimSpace(r.PostFormValue("subdominio"))),
		Dominio:    strings.ToLower(strings.TrimSpace(r.PostFormValue("dominio"))), Plano: strings.TrimSpace(r.PostFormValue("plano")),
	}
	var msgLocal string
	in.Cidade, in.UF, in.Fuso, msgLocal = lerLocalizacao(r)
	falha := func(msg string) {
		renderStatus(w, r, http.StatusBadRequest, views.ClienteForm(h.layout(r, "Novo cliente", "/clientes"), h.formCliente(r, views.ClienteFormData{
			Nome: in.Nome, Slug: in.Slug, Subdominio: in.Subdominio, Dominio: in.Dominio, Plano: in.Plano,
			Cidade: in.Cidade, UF: in.UF, Fuso: strings.TrimSpace(r.PostFormValue("fuso")), Erro: msg,
		})))
	}
	switch {
	case in.Nome == "" || len([]rune(in.Nome)) > 120:
		falha("Informe o nome do cliente (até 120 caracteres).")
		return
	case msgLocal != "":
		falha(msgLocal)
		return
	case in.Subdominio == "":
		falha("Escolha o subdomínio de acesso.")
		return
	case !reSubdominio.MatchString(in.Subdominio):
		falha("Subdomínio inválido: use letras minúsculas, números e hífen (sem começar nem terminar com hífen).")
		return
	case in.Dominio != "" && !strings.Contains(in.Dominio, "."):
		falha("O domínio próprio precisa ser um nome completo, como arbo.prefeitura.gov.br.")
		return
	}

	_, err := h.tenants.Provision(comOrigem(r), in)
	naoAuditar(r) // o próprio serviço já registrou "cliente.criar" com quem fez
	switch {
	case errors.Is(err, tenant.ErrJaExiste):
		falha("Já existe um cliente com esse identificador.")
	case errors.Is(err, tenant.ErrSubdominioReservado):
		falha("Este subdomínio é reservado (já tem outro uso no servidor). Escolha outro.")
	case err != nil && violacaoUnica(err):
		falha("Já existe um cliente com esse subdomínio ou domínio.")
	case err != nil && strings.HasPrefix(err.Error(), "identificador inválido"):
		falha("Identificador inválido: use de 3 a 30 letras minúsculas e números, começando por letra.")
	case err != nil:
		h.falhaInterna(w, r, "criar cliente", err)
	default:
		http.Redirect(w, r, "/clientes", http.StatusSeeOther)
	}
}

func (h *handlers) clienteStatus(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	novo := r.PostFormValue("status")
	if novo != "ativo" && novo != "suspenso" {
		http.Error(w, "Situação inválida.", http.StatusBadRequest)
		return
	}
	q := sqlc.New(h.tenants.Pool)
	c, err := q.GetClienteBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar cliente", err)
		return
	}
	if c.Status != "ativo" && c.Status != "suspenso" {
		http.Error(w, "Este cliente não está em uma situação que permita a troca.", http.StatusBadRequest)
		return
	}
	if c.Status == novo {
		naoAuditar(r)
		http.Redirect(w, r, "/clientes", http.StatusSeeOther)
		return
	}
	if _, err := q.UpdateClienteStatus(r.Context(), sqlc.UpdateClienteStatusParams{ID: c.ID, Status: novo}); err != nil {
		h.falhaInterna(w, r, "mudar situação", err)
		return
	}
	h.resolver.Limpar() // vale na hora neste processo
	h.auditar(r, audit.Evento{
		Acao: "cliente.status", Entidade: "cliente", EntidadeID: c.Slug, Rotulo: c.Nome,
		Antes: map[string]any{"status": c.Status}, Depois: map[string]any{"status": novo},
	})
	http.Redirect(w, r, "/clientes", http.StatusSeeOther)
}

// ----------------------------------------------------- Usuários da plataforma

func (h *handlers) mastersLista(w http.ResponseWriter, r *http.Request) {
	lista, err := h.pub.ListPlataformaUsuarios(r.Context())
	if err != nil {
		h.falhaInterna(w, r, "listar usuários da plataforma", err)
		return
	}
	a, _ := authFrom(r.Context())
	linhas := make([]views.MasterLinha, len(lista))
	for i, u := range lista {
		id := idTexto(u.ID)
		linhas[i] = views.MasterLinha{
			ID: id, Nome: u.Nome, Email: u.Email, Master: u.Master, Ativo: u.Ativo, DeveTrocar: u.DeveTrocarSenha,
			UltimoAcesso: formatarData(h.fuso(r), u.UltimoAcesso.Time), Eu: id == a.sess.Conta.ID,
		}
	}
	lay := h.layout(r, "Usuários da plataforma", "/plataforma/usuarios")
	linhas = paginar(r, &lay, linhas)
	render(w, r, views.Masters(lay, views.MastersData{Linhas: linhas, CSRF: csrfToken(r.Context())}))
}

func idTexto(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (h *handlers) masterNovoForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.MasterForm(h.layout(r, "Novo master", "/plataforma/usuarios"), views.MasterFormData{CSRF: csrfToken(r.Context())}))
}

func (h *handlers) masterCriar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requisição inválida.", http.StatusBadRequest)
		return
	}
	nome, email := strings.TrimSpace(r.PostFormValue("nome")), strings.ToLower(strings.TrimSpace(r.PostFormValue("email")))
	falha := func(msg string) {
		renderStatus(w, r, http.StatusBadRequest, views.MasterForm(h.layout(r, "Novo master", "/plataforma/usuarios"), views.MasterFormData{Nome: nome, Email: email, Erro: msg, CSRF: csrfToken(r.Context())}))
	}
	if nome == "" || len([]rune(nome)) > 120 || !strings.Contains(email, "@") || strings.ContainsAny(email, " ,;") {
		falha("Informe o nome completo e um e-mail válido.")
		return
	}
	u, senha, err := h.tenants.CriarMaster(comOrigem(r), nome, email)
	naoAuditar(r)
	if errors.Is(err, tenant.ErrMasterJaExiste) {
		falha("Já existe um usuário da plataforma com esse e-mail.")
		return
	} else if err != nil {
		h.falhaInterna(w, r, "criar master", err)
		return
	}
	h.mostrarSenha(w, r, true, u.Nome, u.Email, senha)
}

func (h *handlers) masterResetarSenha(w http.ResponseWriter, r *http.Request) {
	a, _ := authFrom(r.Context())
	var id pgtype.UUID
	if err := id.Scan(chi.URLParam(r, "id")); err != nil {
		http.NotFound(w, r)
		return
	}
	alvo, err := h.pub.GetPlataformaUsuarioByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		h.falhaInterna(w, r, "buscar usuário da plataforma", err)
		return
	}
	if idTexto(id) == a.sess.Conta.ID {
		http.Error(w, "Para trocar a sua própria senha, use a opção de trocar senha.", http.StatusBadRequest)
		return
	}
	u, senha, err := h.tenants.ResetarSenhaPlataforma(comOrigem(r), alvo.Email)
	naoAuditar(r)
	if err != nil {
		h.falhaInterna(w, r, "resetar senha", err)
		return
	}
	h.mostrarSenha(w, r, false, u.Nome, u.Email, senha)
}

// modulosProntos são os blocos do catálogo que já têm tela. Cada etapa nova acrescenta o seu aqui
// (o painel inicial mostra "disponível" e leva à tela, em vez de "em construção").
var modulosProntos = map[string]struct{ href, permissao string }{
	"Bairros":               {"/bairros", "bairro_visualizar"},
	"Locais":                {"/locais", "local_visualizar"},
	"Mapa":                  {"/mapa", "bairro_visualizar"},
	"Visitas":               {"/visitas", "visita_visualizar"},
	"Laboratório":           {"/laboratorio", "visita_laboratorio_visualizar"},
	"Investigação":          {"/investigacoes", "investigacao_visualizar"},
	"Denúncia":              {"/denuncias", "denuncia_visualizar"},
	"BAAP":                  {"/zoonose/baap", "zoonose_baap_visualizar"},
	"Solicitações de drone": {"/drones", "dronesolicitacao_visualizar"},
	"Notificações":          {"/notificacoes", "notificacao_visualizar"},
	"Infrações":             {"/infracoes", "notificacao_visualizar"},
	"B.I.":                  {"/bi", "bi_visualizar"},
	"Reconhecimento":        {"/reconhecimentos", "reconhecimento_visualizar"},
	"Resumo semanal":        {"/resumosemanal", "resumosemanal_visualizar"},
	"Mobilidade":            {"/mobilidade/historico", "historico_mobilidade"},
	"Auditoria":             {"/auditoria", "auditoria_sistema"},
	"Parâmetros":            {"/ciclos", "param_ciclo"},
	"Configuração":          {"/usuarios", "conf_usuario"},
}

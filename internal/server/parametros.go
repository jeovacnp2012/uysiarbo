package server

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"uysiarbo/internal/audit"
	"uysiarbo/internal/auth"
	"uysiarbo/internal/db/sqlccliente"
	"uysiarbo/web/views"
)

// Os parâmetros do cliente ficam em cliente_info (chave/valor). Cada um é declarado aqui, com tipo, padrão e
// limites, e as telas Geral e Mobilidade são montadas a partir desta lista: parâmetro novo = uma linha aqui.
type parametro struct {
	Chave  string
	Grupo  string // "geral" ou "mobilidade"
	Rotulo string
	Ajuda  string
	Tipo   string // "bool", "int", "cep" ou "senha"
	Padrao string
	Min    int
	Max    int
}

const (
	grupoGeral      = "geral"
	grupoMobilidade = "mobilidade"
	chaveSenhaApp   = "app_senha_limpar_dados" // guarda só o hash da senha
)

var parametros = []parametro{
	{Chave: "visita_tubito_automatico", Grupo: grupoGeral, Tipo: "bool", Padrao: "1", Rotulo: "Numerar os tubitos automaticamente",
		Ajuda: "Desmarcado, o número do tubito é digitado em cada amostra."},
	{Chave: "campanha_estoque", Grupo: grupoGeral, Tipo: "bool", Padrao: "0", Rotulo: "A campanha antirrábica consome o estoque",
		Ajuda: "Quando marcado, vacinas e seringas usadas na campanha saem do estoque."},
	{Chave: "cep_padrao", Grupo: grupoGeral, Tipo: "cep", Rotulo: "CEP padrão dos novos locais",
		Ajuda: "Preenche o CEP ao cadastrar um local (8 dígitos). Deixe vazio para não preencher."},

	{Chave: "app_aviso_precisao_gps", Grupo: grupoMobilidade, Tipo: "int", Padrao: "20", Min: 1, Max: 1000, Rotulo: "Avisar se a precisão do GPS for pior que (metros)",
		Ajuda: "O aplicativo avisa o agente quando o GPS está menos preciso que isso."},
	{Chave: "app_coleta_gps_segundos", Grupo: grupoMobilidade, Tipo: "int", Padrao: "5", Min: 0, Max: 600, Rotulo: "Coletar a posição do agente a cada (segundos)",
		Ajuda: "0 desliga a coleta do rastro."},
	{Chave: "app_digitar_tubito", Grupo: grupoMobilidade, Tipo: "bool", Padrao: "0", Rotulo: "Permitir digitar o número do tubito no aplicativo"},
	{Chave: "app_permite_fechado", Grupo: grupoMobilidade, Tipo: "bool", Padrao: "0", Rotulo: "Permitir DF, PVE e LIRAa com pendência \"fechado\""},
	{Chave: "app_dashboard_grafico", Grupo: grupoMobilidade, Tipo: "bool", Padrao: "1", Rotulo: "Mostrar os gráficos no painel do aplicativo"},
	{Chave: "app_dias_visitas", Grupo: grupoMobilidade, Tipo: "int", Padrao: "30", Min: 0, Max: 3650, Rotulo: "Dias de visitas e fotos que o aplicativo baixa"},
	{Chave: "app_dias_campanhas", Grupo: grupoMobilidade, Tipo: "int", Padrao: "30", Min: 0, Max: 3650, Rotulo: "Dias de campanhas que o aplicativo baixa"},
	{Chave: "app_dias_investigacoes", Grupo: grupoMobilidade, Tipo: "int", Padrao: "30", Min: 0, Max: 3650, Rotulo: "Dias de investigações que o aplicativo baixa"},
	{Chave: "app_dias_anexos", Grupo: grupoMobilidade, Tipo: "int", Padrao: "30", Min: 0, Max: 3650, Rotulo: "Dias de anexos de investigação que o aplicativo baixa"},
	{Chave: chaveSenhaApp, Grupo: grupoMobilidade, Tipo: "senha", Rotulo: "Senha para \"Limpar dados\" no aplicativo",
		Ajuda: "Só quem souber a senha limpa os dados do aparelho. Ela não é mostrada depois de salva."},
}

var reCEPParametro = regexp.MustCompile(`^[0-9]{8}$`)

// valorDoParametro lê o valor salvo, ou o padrão.
func (h *handlers) valorDoParametro(r *http.Request, q *sqlccliente.Queries, p parametro) string {
	if info, err := q.GetClienteInfo(r.Context(), p.Chave); err == nil {
		return info.Valor
	}
	return p.Padrao
}

func parametrosDoGrupo(grupo string) []parametro {
	var out []parametro
	for _, p := range parametros {
		if p.Grupo == grupo {
			out = append(out, p)
		}
	}
	return out
}

var textosDoGrupo = map[string]struct{ titulo, rota, descricao string }{
	grupoGeral:      {"Parâmetros gerais", "/parametros/geral", "Regras que valem para todo o ambiente."},
	grupoMobilidade: {"Mobilidade (aplicativo)", "/parametros/mobilidade", "Como o aplicativo do tablet se comporta."},
}

func (h *handlers) parametrosForm(grupo string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, _, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		d := h.dadosDeParametros(r, q, grupo, nil)
		d.Salvo = r.URL.Query().Get("ok") == "1"
		render(w, r, views.Parametros(h.layout(r, textosDoGrupo[grupo].titulo, textosDoGrupo[grupo].rota), d))
	}
}

// dadosDeParametros monta a tela; enviados (quando não nulo) são os valores digitados, em caso de erro.
func (h *handlers) dadosDeParametros(r *http.Request, q *sqlccliente.Queries, grupo string, enviados map[string]string) views.ParametrosData {
	t := textosDoGrupo[grupo]
	d := views.ParametrosData{Titulo: t.titulo, Descricao: t.descricao, Acao: t.rota, CSRF: csrfToken(r.Context())}
	for _, p := range parametrosDoGrupo(grupo) {
		valor := h.valorDoParametro(r, q, p)
		if v, ok := enviados[p.Chave]; ok {
			valor = v
		}
		c := views.ParametroCampo{Chave: p.Chave, Rotulo: p.Rotulo, Ajuda: p.Ajuda, Tipo: p.Tipo, Valor: valor, Min: p.Min, Max: p.Max}
		switch p.Tipo {
		case "bool":
			c.Marcado = valor == "1"
		case "senha":
			c.Valor = ""
			c.Definida = h.valorDoParametro(r, q, p) != ""
		}
		d.Campos = append(d.Campos, c)
	}
	return d
}

func simNao(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (h *handlers) parametrosSalvar(grupo string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Requisição inválida.", http.StatusBadRequest)
			return
		}
		q, pool, err := h.cliente(r)
		if err != nil {
			h.falhaInterna(w, r, "pool do cliente", err)
			return
		}
		novos := map[string]string{}
		enviados := map[string]string{}
		var erro string
		for _, p := range parametrosDoGrupo(grupo) {
			bruto := strings.TrimSpace(r.PostFormValue(p.Chave))
			enviados[p.Chave] = bruto
			switch p.Tipo {
			case "bool":
				novos[p.Chave] = simNao(r.PostFormValue(p.Chave) == "1")
			case "int":
				n, err := strconv.Atoi(bruto)
				if err != nil || n < p.Min || n > p.Max {
					erro = fmt.Sprintf("%s: informe um número de %d a %d.", p.Rotulo, p.Min, p.Max)
				}
				novos[p.Chave] = strconv.Itoa(n)
			case "cep":
				bruto = strings.NewReplacer("-", "", ".", "").Replace(bruto)
				enviados[p.Chave] = bruto
				if bruto != "" && !reCEPParametro.MatchString(bruto) {
					erro = "O CEP padrão tem 8 dígitos."
				}
				novos[p.Chave] = bruto
			case "senha":
				delete(enviados, p.Chave) // nunca devolvida à tela
				switch {
				case r.PostFormValue(p.Chave+"_remover") == "1":
					novos[p.Chave] = ""
				case bruto != "":
					if len([]rune(bruto)) < 4 || len([]rune(bruto)) > 64 {
						erro = "A senha do aplicativo tem de 4 a 64 caracteres."
						break
					}
					hash, err := auth.Hash(bruto)
					if err != nil {
						h.falhaInterna(w, r, "gerar hash", err)
						return
					}
					novos[p.Chave] = hash
				}
			}
		}
		if erro != "" {
			d := h.dadosDeParametros(r, q, grupo, enviados)
			d.Erro = erro
			renderStatus(w, r, http.StatusBadRequest, views.Parametros(h.layout(r, textosDoGrupo[grupo].titulo, textosDoGrupo[grupo].rota), d))
			return
		}

		tx, err := pool.Begin(r.Context())
		if err != nil {
			h.falhaInterna(w, r, "iniciar transação", err)
			return
		}
		defer tx.Rollback(r.Context())
		qt := sqlccliente.New(tx)
		antes, depois := map[string]any{}, map[string]any{}
		for _, p := range parametrosDoGrupo(grupo) {
			novo, ok := novos[p.Chave]
			if !ok {
				continue
			}
			atual := h.valorDoParametro(r, q, p)
			if novo == atual {
				continue
			}
			if err := qt.SetClienteInfo(r.Context(), sqlccliente.SetClienteInfoParams{Chave: p.Chave, Valor: novo}); err != nil {
				h.falhaInterna(w, r, "salvar parâmetro", err)
				return
			}
			// A auditoria mostra o que mudou, mas nunca a senha: só se foi definida ou removida.
			if p.Tipo == "senha" {
				antes["limpar_dados_protegido"], depois["limpar_dados_protegido"] = situacaoDaSenha(atual), situacaoDaSenha(novo)
				continue
			}
			antes[p.Chave], depois[p.Chave] = atual, novo
		}
		if len(depois) == 0 {
			naoAuditar(r)
		} else {
			h.auditarCom(audit.Cliente{Q: qt}, r, audit.Evento{
				Acao: "parametro." + grupo, Entidade: "parametro", EntidadeID: grupo, Rotulo: textosDoGrupo[grupo].titulo,
				Antes: antes, Depois: depois,
			})
		}
		if err := tx.Commit(r.Context()); err != nil {
			h.falhaInterna(w, r, "confirmar parâmetros", err)
			return
		}
		http.Redirect(w, r, textosDoGrupo[grupo].rota+"?ok=1", http.StatusSeeOther)
	}
}

func situacaoDaSenha(valor string) string {
	if valor != "" {
		return "definida"
	}
	return "sem senha"
}

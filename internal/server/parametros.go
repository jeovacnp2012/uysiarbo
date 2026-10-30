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
	Chave   string
	Grupo   string // "geral" ou "mobilidade"
	Rotulo  string
	Ajuda   string
	Tipo    string // "bool", "int", "cep" ou "senha"
	Padrao  string
	Min     int
	Max     int
	Opcoes  [][2]string // tipo "opcao": valor e rótulo
	SimNao  [2]string   // bool mostrado como select, com os rótulos de Sim e de Não
	SoSe    string      // campo só aparece quando todos os "campo=valor" (separados por ;) valem
	Cartao  bool        // vai para o cartão da direita
	Unidade string
}

const (
	grupoGeral      = "geral"
	grupoMobilidade = "mobilidade"
	grupoContaOvos  = "contaovos"
	chaveSenhaApp   = "app_senha_limpar_dados" // guarda só o hash da senha
)

var parametros = []parametro{
	{Chave: "acesso_local", Grupo: grupoGeral, Tipo: "opcao", Padrao: acessoResponsavel, Rotulo: "Método de controle de acesso aos Locais/Bairros/Quarteirões",
		Opcoes: [][2]string{{acessoResponsavel, "Quando responsável pelo Bairro"}, {acessoParametro, "Pelo Parâmetro — Acesso Local"}}},
	{Chave: chaveSemana, Grupo: grupoGeral, Tipo: "opcao", Padrao: "sinan", Rotulo: "Padrão das semanas",
		Opcoes: [][2]string{{"sinan", "SINAN (habilita o menu Semanas SINAN)"}, {"iso", "ISO 8601"}}},
	{Chave: "visita_tubito_automatico", Grupo: grupoGeral, Tipo: "bool", Padrao: "1", Rotulo: "Tubitos — Numeração Automática",
		SimNao: [2]string{"Sim", "Não"}, Ajuda: "Não: o número do tubito é digitado em cada amostra."},
	{Chave: "campanha_estoque", Grupo: grupoGeral, Tipo: "bool", Padrao: "0", Rotulo: "Zoonose - Campanha Antirrábica - Uso obrigatório do Estoque",
		SimNao: [2]string{"Sim (somente com estoque disponível)", "Não (permite uso sem estoque e opcional do estoque)"}},

	{Chave: "app_aviso_precisao_gps", Grupo: grupoMobilidade, Tipo: "int", Padrao: "20", Min: 1, Max: 1000, Rotulo: "Aviso de precisão do GPS ao salvar a visita (metros)"},
	{Chave: "app_digitar_tubito", Grupo: grupoMobilidade, Tipo: "bool", Padrao: "0", Rotulo: "Permitir digitar o número do tubito?",
		SimNao: [2]string{"Sim", "Não (automático)"}},
	{Chave: "app_coleta_gps_segundos", Grupo: grupoMobilidade, Tipo: "int", Padrao: "5", Min: 0, Max: 600, Rotulo: "Coletar dados do GPS a cada (segundos)",
		Ajuda: "0 desliga a coleta do rastro."},
	{Chave: "app_permite_fechado", Grupo: grupoMobilidade, Tipo: "bool", Padrao: "0", Rotulo: "Permitir visita DF, PVE e LIRAa quando a pendência for \"fechado\"?",
		SimNao: [2]string{"Sim (Permitir)", "Não (mostra um alerta)"}},
	{Chave: "cep_padrao", Grupo: grupoMobilidade, Tipo: "cep", Rotulo: "CEP padrão",
		Ajuda: "Preenche o CEP ao cadastrar um local (8 dígitos). Deixe vazio para não preencher."},
	{Chave: chaveSenhaApp, Grupo: grupoMobilidade, Tipo: "senha", Rotulo: "Senha solicitada ao \"Limpar dados\"",
		Ajuda: "Só quem souber a senha limpa os dados do aparelho. Ela não é mostrada depois de salva."},
	{Chave: "app_dashboard_grafico", Grupo: grupoMobilidade, Tipo: "bool", Padrao: "1", Rotulo: "Mostrar os gráficos do Dashboard do App?",
		SimNao: [2]string{"Sim (Mostrar)", "Não (Ocultar)"},
		Ajuda:  "Ao selecionar \"Não\", os gráficos do Dashboard do App serão ocultados, proporcionando uma interface mais limpa e simplificada para os usuários."},
	{Chave: "app_dias_visitas", Grupo: grupoMobilidade, Tipo: "int", Padrao: "365", Min: 0, Max: 3650, Rotulo: "Fotos das Visitas", Cartao: true, Unidade: "dias"},
	{Chave: "app_dias_campanhas", Grupo: grupoMobilidade, Tipo: "int", Padrao: "365", Min: 0, Max: 3650, Rotulo: "Campanhas de Vacinação", Cartao: true, Unidade: "dias"},
	{Chave: "app_dias_investigacoes", Grupo: grupoMobilidade, Tipo: "int", Padrao: "365", Min: 0, Max: 3650, Rotulo: "Investigações", Cartao: true, Unidade: "dias"},
	{Chave: "app_dias_anexos", Grupo: grupoMobilidade, Tipo: "int", Padrao: "60", Min: 0, Max: 3650, Rotulo: "Anexos das Investigações", Cartao: true, Unidade: "dias",
		Ajuda: "Pode ser diferente das investigações pois anexos ocupam mais espaço de armazenamento."},

	{Chave: "contaovos_ativo", Grupo: grupoContaOvos, Tipo: "bool", Padrao: "0", Rotulo: "Integração ativa", SimNao: [2]string{"Sim", "Não"}},
	{Chave: "contaovos_tipo", Grupo: grupoContaOvos, Tipo: "opcao", Padrao: "publica", Rotulo: "Tipo de integração", SoSe: "contaovos_ativo=1",
		Opcoes: [][2]string{{"publica", "Integração pública"}, {"privada", "Integração privada"}}},
	{Chave: "contaovos_uf", Grupo: grupoContaOvos, Tipo: "texto", Rotulo: "Estado", Max: 2, SoSe: "contaovos_ativo=1", Ajuda: "Sigla do estado, como MT."},
	{Chave: "contaovos_municipio", Grupo: grupoContaOvos, Tipo: "texto", Rotulo: "Município", Max: 80, SoSe: "contaovos_ativo=1"},
	{Chave: "contaovos_chave", Grupo: grupoContaOvos, Tipo: "segredo", Rotulo: "Chave de API (privada)", SoSe: "contaovos_ativo=1;contaovos_tipo=privada",
		Ajuda: "Fornecida pela Conta Ovos. Não é mostrada depois de salva."},
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
	grupoContaOvos:  {"API Conta Ovos", "/parametros/contaovos", "Integração com a Conta Ovos (contagem de ovos das ovitrampas)."},
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
		c := views.ParametroCampo{Chave: p.Chave, Rotulo: p.Rotulo, Ajuda: p.Ajuda, Tipo: p.Tipo, Valor: valor, Min: p.Min, Max: p.Max, SimNao: p.SimNao, SoSe: p.SoSe, Cartao: p.Cartao, Unidade: p.Unidade}
		d.TemCartao = d.TemCartao || p.Cartao
		for _, o := range p.Opcoes {
			c.Opcoes = append(c.Opcoes, views.ParametroOpcao{Valor: o[0], Rotulo: o[1]})
		}
		switch p.Tipo {
		case "bool":
			c.Marcado = valor == "1"
		case "senha", "segredo":
			c.Valor = ""
			c.Definida = h.valorDoParametro(r, q, p) != ""
		}
		d.Campos = append(d.Campos, c)
	}
	if d.TemCartao {
		d.CartaoTitulo, d.CartaoTexto = "Dias máximos para download (App)", "Define quantos dias para trás o App poderá baixar registros de cada módulo."
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
			case "texto":
				if len([]rune(bruto)) > p.Max {
					erro = fmt.Sprintf("%s: no máximo %d caracteres.", p.Rotulo, p.Max)
				}
				if p.Chave == "contaovos_uf" {
					bruto = strings.ToUpper(bruto)
					enviados[p.Chave] = bruto
				}
				novos[p.Chave] = bruto
			case "opcao":
				if _, enviado := r.PostForm[p.Chave]; !enviado {
					continue
				}
				valido := false
				for _, o := range p.Opcoes {
					valido = valido || o[0] == bruto
				}
				if !valido {
					erro = p.Rotulo + ": escolha uma das opções."
				}
				novos[p.Chave] = bruto
			case "segredo":
				delete(enviados, p.Chave) // nunca devolvida à tela
				switch {
				case r.PostFormValue(p.Chave+"_remover") == "1":
					novos[p.Chave] = ""
				case bruto != "":
					if len([]rune(bruto)) > 200 {
						erro = p.Rotulo + ": no máximo 200 caracteres."
						break
					}
					novos[p.Chave] = bruto
				}
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
			if p.Tipo == "segredo" {
				antes["conexao_protegida"], depois["conexao_protegida"] = situacaoDaSenha(atual), situacaoDaSenha(novo)
				continue
			}
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

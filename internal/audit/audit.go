// Package audit registra o histórico de tudo que acontece no sistema: quem fez, o quê, quando, de
// onde, e (quando faz sentido) o antes e o depois. O histórico só cresce: o banco recusa alterar
// ou apagar linhas. Senhas, hashes e tokens nunca são gravados.
package audit

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
)

// Tipos de ator.
const (
	AtorUsuario    = "usuario"    // usuário de um cliente
	AtorPlataforma = "plataforma" // conta da plataforma (o master)
	AtorSistema    = "sistema"    // linha de comando, rotinas automáticas
)

// Ator é quem fez a ação.
type Ator struct {
	Tipo  string
	ID    string
	Nome  string
	Email string
}

// Evento é um fato a registrar.
type Evento struct {
	Ator      Ator
	IP        string
	UserAgent string
	RequestID string

	Acao       string // "entidade.verbo", ex.: "usuario.criar"
	Entidade   string // tipo do registro afetado, ex.: "usuario"
	EntidadeID string
	Rotulo     string // nome legível do registro afetado, ex.: o nome da pessoa

	Antes    map[string]any
	Depois   map[string]any
	Detalhes map[string]any
}

// Registrador grava eventos num histórico (o de um cliente ou o da plataforma).
type Registrador interface {
	Registrar(ctx context.Context, e Evento) error
}

// sensivel reconhece nomes de campos que nunca podem ir para o histórico.
var sensivel = regexp.MustCompile(`(?i)(senha|password|passwd|token|hash|secret|segredo|cookie|csrf|captcha)`)

// Limpar devolve uma cópia sem os campos sensíveis (em qualquer nível).
func Limpar(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if sensivel.MatchString(k) {
			continue
		}
		out[k] = limparValor(v)
	}
	return out
}

func limparValor(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return Limpar(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = limparValor(e)
		}
		return out
	}
	return v
}

// Diferenca devolve só os campos que mudaram, nos dois mapas. Campo que só existe de um lado
// aparece como mudança. Útil para gravar "antes" e "depois" enxutos numa edição.
func Diferenca(antes, depois map[string]any) (a, d map[string]any) {
	a, d = map[string]any{}, map[string]any{}
	chaves := map[string]bool{}
	for k := range antes {
		chaves[k] = true
	}
	for k := range depois {
		chaves[k] = true
	}
	for k := range chaves {
		va, oka := antes[k]
		vd, okd := depois[k]
		if oka && okd && igual(va, vd) {
			continue
		}
		if oka {
			a[k] = va
		}
		if okd {
			d[k] = vd
		}
	}
	return a, d
}

func igual(a, b any) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) == string(jb)
}

// DiferencaLista compara duas listas de textos e devolve o que entrou e o que saiu, ordenados.
func DiferencaLista(antes, depois []string) (entrou, saiu []string) {
	for _, x := range depois {
		if !slices.Contains(antes, x) {
			entrou = append(entrou, x)
		}
	}
	for _, x := range antes {
		if !slices.Contains(depois, x) {
			saiu = append(saiu, x)
		}
	}
	slices.Sort(entrou)
	slices.Sort(saiu)
	return entrou, saiu
}

// JSON serializa um mapa (já limpo) para gravar numa coluna jsonb; nil vira NULL.
func JSON(m map[string]any) []byte {
	if len(m) == 0 {
		return nil
	}
	b, err := json.Marshal(Limpar(m))
	if err != nil {
		return nil
	}
	return b
}

// Rotulos traduz o código de uma ação para o texto mostrado na tela. Código desconhecido aparece
// como está, para nada ficar escondido.
var Rotulos = map[string]string{
	"login.sucesso":          "Entrou no sistema",
	"login.falha":            "Tentativa de entrada com senha errada",
	"login.bloqueio":         "Conta bloqueada por excesso de tentativas",
	"logout":                 "Saiu do sistema",
	"senha.trocar":           "Trocou a própria senha",
	"usuario.criar":          "Criou um usuário",
	"usuario.editar":         "Editou um usuário",
	"usuario.resetar_senha":  "Resetou a senha de um usuário",
	"papel.criar":            "Criou um papel",
	"papel.editar":           "Editou um papel",
	"cliente.criar":          "Criou um cliente",
	"cliente.sincronizar":    "Sincronizou os papéis padrão",
	"cliente.status":         "Mudou a situação de um cliente",
	"cliente.editar":         "Editou os dados de um cliente",
	"cidade.criar":           "Cadastrou uma cidade",
	"cidade.editar":          "Editou uma cidade",
	"bairro.criar":           "Cadastrou um bairro",
	"bairro.editar":          "Editou um bairro",
	"quarteirao.criar":       "Cadastrou quarteirões",
	"quarteirao.status":      "Ativou ou desativou um quarteirão",
	"grupo.criar":            "Criou um grupo de locais",
	"grupo.editar":           "Editou um grupo de locais",
	"grupo.excluir":          "Excluiu um grupo de locais",
	"grupo.clonar":           "Clonou um grupo de locais",
	"grupo.locais_adicionar": "Acrescentou locais a um grupo",
	"grupo.local_remover":    "Retirou um local de um grupo",
	"grupo.limpar":           "Esvaziou um grupo de locais",
	"grupo.exportar":         "Exportou um grupo de locais",
	"local.criar":            "Cadastrou um local",
	"local.editar":           "Editou um local",
	"local.desativar":        "Desativou um local",
	"local.reativar":         "Reativou um local",
	"local.chave_nova":       "Gerou nova chave (QR Code) de um local",
	"local.lote":             "Cadastrou locais em lote",
	"bairro.limite":          "Gravou o limite de um bairro no mapa",
	"bairro.limite_remover":  "Removeu o limite de um bairro do mapa",
	"quarteirao.limites":     "Importou limites de quarteirões no mapa",
	"versao.criar":           "Registrou uma versão nas Novidades",
	"master.criar":           "Criou um usuário master",
	"master.resetar_senha":   "Resetou a senha de um master",
	"requisicao.escrita":     "Gravação no sistema (registro automático)",
}

// Rotulo devolve o texto de uma ação.
func Rotulo(acao string) string {
	if r, ok := Rotulos[acao]; ok {
		return r
	}
	return acao
}

type origemKey struct{}

// ComOrigem guarda no contexto quem está agindo e de onde, para as camadas de baixo (que não conhecem
// a requisição HTTP) registrarem a auditoria com o ator certo.
func ComOrigem(ctx context.Context, ator Ator, ip, userAgent, requestID string) context.Context {
	return context.WithValue(ctx, origemKey{}, Evento{Ator: ator, IP: ip, UserAgent: userAgent, RequestID: requestID})
}

// Preencher completa ator, IP, navegador e requisição do evento com os do contexto, onde estiverem vazios.
func Preencher(ctx context.Context, e *Evento) {
	o, ok := ctx.Value(origemKey{}).(Evento)
	if !ok {
		return
	}
	if e.Ator.Tipo == "" {
		e.Ator = o.Ator
	}
	if e.IP == "" {
		e.IP = o.IP
	}
	if e.UserAgent == "" {
		e.UserAgent = o.UserAgent
	}
	if e.RequestID == "" {
		e.RequestID = o.RequestID
	}
}

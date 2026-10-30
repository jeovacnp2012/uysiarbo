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
	AtorAPI        = "api"        // sistema externo, por uma credencial da API
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
	"login.sucesso":                     "Entrou no sistema",
	"login.falha":                       "Tentativa de entrada com senha errada",
	"login.bloqueio":                    "Conta bloqueada por excesso de tentativas",
	"logout":                            "Saiu do sistema",
	"senha.trocar":                      "Trocou a própria senha",
	"usuario.criar":                     "Criou um usuário",
	"usuario.editar":                    "Editou um usuário",
	"usuario.resetar_senha":             "Resetou a senha de um usuário",
	"papel.criar":                       "Criou um papel",
	"papel.editar":                      "Editou um papel",
	"cliente.criar":                     "Criou um cliente",
	"cliente.sincronizar":               "Sincronizou os papéis padrão",
	"cliente.status":                    "Mudou a situação de um cliente",
	"cliente.editar":                    "Editou os dados de um cliente",
	"cidade.criar":                      "Cadastrou uma cidade",
	"cidade.editar":                     "Editou uma cidade",
	"bairro.criar":                      "Cadastrou um bairro",
	"bairro.editar":                     "Editou um bairro",
	"quarteirao.criar":                  "Cadastrou quarteirões",
	"quarteirao.editar":                 "Editou um quarteirão",
	"quarteirao.status":                 "Ativou ou desativou um quarteirão",
	"cidade.excluir":                    "Enviou uma cidade para a lixeira",
	"cidade.restaurar":                  "Restaurou uma cidade da lixeira",
	"cidade.excluir_definitivo":         "Excluiu uma cidade definitivamente",
	"visita.criar":                      "Registrou uma visita",
	"visita.editar":                     "Editou uma visita",
	"visita.desativar":                  "Desativou uma visita",
	"visita.reativar":                   "Reativou uma visita",
	"visita.excluir":                    "Enviou uma visita para a lixeira",
	"visita.restaurar":                  "Restaurou uma visita da lixeira",
	"amostra.criar":                     "Registrou um tubito",
	"amostra.editar":                    "Corrigiu a coleta de um tubito",
	"amostra.resultado":                 "Registrou o resultado de um tubito",
	"amostra.excluir":                   "Enviou um tubito para a lixeira",
	"amostra.restaurar":                 "Restaurou um tubito da lixeira",
	"amostra.excluir_definitivo":        "Excluiu um tubito definitivamente",
	"visita.excluir_definitivo":         "Excluiu uma visita definitivamente",
	"visita.foto_adicionar":             "Adicionou fotos a uma visita",
	"visita.foto_excluir":               "Excluiu uma foto de uma visita",
	"visita.anotacao":                   "Anotou numa visita",
	"lei.criar":                         "Cadastrou uma lei municipal",
	"lei.editar":                        "Editou uma lei municipal",
	"lei.excluir":                       "Enviou uma lei municipal para a lixeira",
	"lei.restaurar":                     "Restaurou uma lei municipal da lixeira",
	"lei.excluir_definitivo":            "Excluiu uma lei municipal definitivamente",
	"lei.trecho_criar":                  "Cadastrou um trecho de lei",
	"lei.trecho_editar":                 "Editou um trecho de lei",
	"lei_trecho.excluir":                "Enviou um trecho de lei para a lixeira",
	"lei_trecho.restaurar":              "Restaurou um trecho de lei da lixeira",
	"lei_trecho.excluir_definitivo":     "Excluiu um trecho de lei definitivamente",
	"parametro.geral":                   "Alterou os parâmetros gerais",
	"parametro.mobilidade":              "Alterou os parâmetros de mobilidade",
	"parametro.contaovos":               "Alterou a integração com a Conta Ovos",
	"ciclo.criar":                       "Cadastrou um ciclo",
	"ciclo.editar":                      "Editou um ciclo",
	"ciclo.excluir":                     "Enviou um ciclo para a lixeira",
	"ciclo.restaurar":                   "Restaurou um ciclo da lixeira",
	"ciclo.excluir_definitivo":          "Excluiu um ciclo definitivamente",
	"bairro.excluir":                    "Enviou um bairro para a lixeira",
	"bairro.restaurar":                  "Restaurou um bairro da lixeira",
	"bairro.excluir_definitivo":         "Excluiu um bairro definitivamente",
	"quarteirao.excluir":                "Enviou um quarteirão para a lixeira",
	"quarteirao.restaurar":              "Restaurou um quarteirão da lixeira",
	"quarteirao.excluir_definitivo":     "Excluiu um quarteirão definitivamente",
	"local.excluir":                     "Enviou um local para a lixeira",
	"local.restaurar":                   "Restaurou um local da lixeira",
	"local.excluir_definitivo":          "Excluiu um local definitivamente",
	"grupo.restaurar":                   "Restaurou um grupo de locais da lixeira",
	"grupo.excluir_definitivo":          "Excluiu um grupo de locais definitivamente",
	"importacao.mapeamento_salvar":      "Salvou um mapeamento de colunas",
	"mapeamento.excluir":                "Enviou um mapeamento de colunas para a lixeira",
	"mapeamento.restaurar":              "Restaurou um mapeamento de colunas da lixeira",
	"mapeamento.excluir_definitivo":     "Excluiu um mapeamento de colunas definitivamente",
	"importacao.analisar":               "Enviou uma planilha para importação",
	"importacao.iniciar":                "Iniciou uma importação",
	"importacao.retomar":                "Retomou uma importação",
	"importacao.interromper":            "Interrompeu uma importação",
	"importacao.concluir":               "Importação concluída",
	"importacao.cancelar":               "Descartou uma importação antes de gravar",
	"importacao.desfazer":               "Desfez uma importação",
	"api.credencial_criar":              "Criou uma credencial de API",
	"api.credencial_editar":             "Editou uma credencial de API",
	"api.credencial_rotacionar":         "Gerou novo segredo de uma credencial de API",
	"api.credencial_ativar":             "Reativou uma credencial de API",
	"api.credencial_desativar":          "Desativou uma credencial de API",
	"parametro.semana":                  "Mudou o tipo de semana epidemiológica",
	"acesso.metodo":                     "Mudou o método de controle de acesso aos locais",
	"acesso.criar":                      "Concedeu acesso a locais",
	"acesso.remover":                    "Retirou um acesso a locais",
	"grupo.criar":                       "Criou um grupo de locais",
	"grupo.editar":                      "Editou um grupo de locais",
	"grupo.excluir":                     "Excluiu um grupo de locais",
	"grupo.clonar":                      "Clonou um grupo de locais",
	"grupo.locais_adicionar":            "Acrescentou locais a um grupo",
	"grupo.local_remover":               "Retirou um local de um grupo",
	"grupo.limpar":                      "Esvaziou um grupo de locais",
	"grupo.exportar":                    "Exportou um grupo de locais",
	"doenca.criar":                      "Cadastrou uma doença",
	"doenca.editar":                     "Alterou uma doença",
	"entrevista.criar":                  "Criou uma entrevista",
	"entrevista.editar":                 "Alterou uma entrevista",
	"denuncia.criar":                    "Registrou uma denúncia",
	"denuncia.editar":                   "Alterou os dados de uma denúncia",
	"denuncia.atender":                  "Assumiu o atendimento de uma denúncia",
	"denuncia.encerrar":                 "Encerrou uma denúncia",
	"denuncia.cancelar":                 "Cancelou uma denúncia",
	"denuncia.reabrir":                  "Reabriu uma denúncia",
	"denuncia.local":                    "Vinculou um local a uma denúncia",
	"denuncia.visita":                   "Vinculou uma visita a uma denúncia",
	"denuncia.excluir":                  "Enviou uma denúncia para a lixeira",
	"denuncia.restaurar":                "Restaurou uma denúncia da lixeira",
	"denuncia.excluir_definitivo":       "Excluiu uma denúncia definitivamente",
	"notificacao.criar":                 "Registrou uma notificação",
	"notificacao.editar":                "Alterou os dados de uma notificação",
	"notificacao.resolver":              "Resolveu uma notificação",
	"notificacao.cancelar":              "Cancelou uma notificação",
	"notificacao.reabrir":               "Reabriu uma notificação",
	"notificacao.excluir":               "Enviou uma notificação para a lixeira",
	"notificacao.restaurar":             "Restaurou uma notificação da lixeira",
	"notificacao.excluir_definitivo":    "Excluiu uma notificação definitivamente",
	"infracao.criar":                    "Gerou um auto de infração",
	"infracao.editar":                   "Alterou os dados de um auto de infração",
	"infracao.pagar":                    "Marcou um auto de infração como pago",
	"infracao.cancelar":                 "Cancelou um auto de infração",
	"infracao.excluir":                  "Enviou um auto de infração para a lixeira",
	"infracao.restaurar":                "Restaurou um auto de infração da lixeira",
	"infracao.excluir_definitivo":       "Excluiu um auto de infração definitivamente",
	"investigacao.criar":                "Abriu uma investigação",
	"investigacao.editar":               "Alterou os dados de uma investigação",
	"investigacao.atender":              "Assumiu o atendimento de uma investigação",
	"investigacao.resultado":            "Definiu o resultado de uma investigação",
	"investigacao.encerrar":             "Encerrou uma investigação",
	"investigacao.reabrir":              "Reabriu uma investigação",
	"investigacao.local":                "Vinculou um local a uma investigação",
	"investigacao.entrevista":           "Registrou a entrevista de uma investigação",
	"investigacao.excluir":              "Enviou uma investigação para a lixeira",
	"investigacao.restaurar":            "Restaurou uma investigação da lixeira",
	"investigacao.excluir_definitivo":   "Excluiu uma investigação definitivamente",
	"caso.comentar":                     "Comentou em um caso",
	"caso.encaminhar":                   "Encaminhou um caso",
	"caso.anexar":                       "Anexou arquivos a um caso",
	"caso.anexo_excluir":                "Excluiu o anexo de um caso",
	"relatorio.baixar":                  "Baixou um relatório",
	"reconhecimento.criar":              "Cadastrou um reconhecimento geográfico",
	"reconhecimento.editar":             "Alterou um reconhecimento geográfico",
	"reconhecimento.excluir":            "Enviou um reconhecimento geográfico para a lixeira",
	"reconhecimento.restaurar":          "Restaurou um reconhecimento geográfico da lixeira",
	"reconhecimento.excluir_definitivo": "Excluiu um reconhecimento geográfico definitivamente",
	"resumosemanal.gerar":               "Gerou um resumo semanal",
	"resumosemanal.excluir":             "Enviou um resumo semanal para a lixeira",
	"resumosemanal.restaurar":           "Restaurou um resumo semanal da lixeira",
	"resumosemanal.excluir_definitivo":  "Excluiu um resumo semanal definitivamente",
	"local.criar":                       "Cadastrou um local",
	"local.editar":                      "Editou um local",
	"local.desativar":                   "Desativou um local",
	"local.reativar":                    "Reativou um local",
	"local.chave_nova":                  "Gerou nova chave (QR Code) de um local",
	"local.lote":                        "Cadastrou locais em lote",
	"bairro.limite":                     "Gravou o limite de um bairro no mapa",
	"bairro.limite_remover":             "Removeu o limite de um bairro do mapa",
	"quarteirao.limites":                "Importou limites de quarteirões no mapa",
	"versao.criar":                      "Registrou uma versão nas Novidades",
	"master.criar":                      "Criou um usuário master",
	"master.resetar_senha":              "Resetou a senha de um master",
	"requisicao.escrita":                "Gravação no sistema (registro automático)",
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

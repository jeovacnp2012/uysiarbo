package audit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLimparRemoveSegredosEmQualquerNivel(t *testing.T) {
	in := map[string]any{
		"nome": "Ana", "senha_hash": "$argon2id$...", "Senha": "x", "token": "t", "csrf": "c",
		"aninhado":             map[string]any{"password": "p", "ok": 1, "lista": []any{map[string]any{"secret": "s", "v": 2}}},
		"g-recaptcha-response": "r",
	}
	out := Limpar(in)
	b, _ := json.Marshal(out)
	for _, proibido := range []string{"argon2id", "senha", "Senha", "token", "csrf", "password", "secret", "recaptcha"} {
		if strings.Contains(strings.ToLower(string(b)), strings.ToLower(proibido)) {
			t.Errorf("o histórico não pode conter %q: %s", proibido, b)
		}
	}
	if out["nome"] != "Ana" || out["aninhado"].(map[string]any)["ok"] != 1 {
		t.Errorf("campos normais deveriam ficar: %s", b)
	}
	if in["senha_hash"] == nil {
		t.Error("Limpar não pode alterar o original")
	}
	if Limpar(nil) != nil {
		t.Error("nil continua nil")
	}
}

func TestDiferencaGuardaSoOQueMudou(t *testing.T) {
	a, d := Diferenca(
		map[string]any{"nome": "Ana", "ativo": true, "papel": "Agente", "so_antes": 1},
		map[string]any{"nome": "Ana Lima", "ativo": true, "papel": "Agente", "so_depois": 2},
	)
	if len(a) != 2 || a["nome"] != "Ana" || a["so_antes"] != 1 {
		t.Errorf("antes = %v", a)
	}
	if len(d) != 2 || d["nome"] != "Ana Lima" || d["so_depois"] != 2 {
		t.Errorf("depois = %v", d)
	}
	if a, d := Diferenca(map[string]any{"x": 1}, map[string]any{"x": 1}); len(a)+len(d) != 0 {
		t.Errorf("sem mudança deveria dar vazio: %v %v", a, d)
	}
}

func TestDiferencaLista(t *testing.T) {
	entrou, saiu := DiferencaLista([]string{"a", "b", "c"}, []string{"c", "d", "b0"})
	if strings.Join(entrou, ",") != "b0,d" || strings.Join(saiu, ",") != "a,b" {
		t.Errorf("entrou=%v saiu=%v", entrou, saiu)
	}
}

func TestJSONEhLimpoEVazioViraNulo(t *testing.T) {
	if JSON(nil) != nil || JSON(map[string]any{}) != nil {
		t.Error("vazio deve virar NULL")
	}
	if b := string(JSON(map[string]any{"a": 1, "senha": "x"})); b != `{"a":1}` {
		t.Errorf("JSON = %s", b)
	}
}

func TestRotulos(t *testing.T) {
	if Rotulo("usuario.criar") != "Criou um usuário" {
		t.Error("rótulo conhecido")
	}
	if Rotulo("coisa.nova") != "coisa.nova" {
		t.Error("ação desconhecida deve aparecer como está")
	}
}

// Toda ação que o sistema registra precisa ter rótulo em português; senão a tela mostraria o código cru.
func TestRotulosCobremAsAcoesConhecidas(t *testing.T) {
	for _, acao := range []string{
		"login.sucesso", "login.falha", "login.bloqueio", "logout", "senha.trocar",
		"usuario.criar", "usuario.editar", "usuario.resetar_senha", "papel.criar", "papel.editar",
		"cliente.criar", "cliente.sincronizar", "cliente.status", "cliente.editar", "master.criar", "master.resetar_senha",
		"versao.criar", "requisicao.escrita", "cidade.criar", "cidade.editar", "bairro.criar", "bairro.editar",
		"quarteirao.criar", "quarteirao.status",
	} {
		if Rotulo(acao) == acao {
			t.Errorf("a ação %q não tem rótulo em português", acao)
		}
	}
}

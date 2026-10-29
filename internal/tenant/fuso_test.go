package tenant

import (
	"testing"
	"time"
)

func TestTodoEstadoTemFusoValido(t *testing.T) {
	ufs := UFs()
	if len(ufs) != 27 {
		t.Fatalf("estados = %d, quer 27 (26 + DF)", len(ufs))
	}
	for _, uf := range ufs {
		f := FusoDaUF(uf)
		if !FusoValido(f) {
			t.Errorf("%s: fuso %q inválido", uf, f)
		}
		if !UFValida(uf) {
			t.Errorf("%s deveria ser válida", uf)
		}
	}
}

func TestFusoDaUF(t *testing.T) {
	casos := map[string]string{
		"MT": "America/Cuiaba", " mt ": "America/Cuiaba", "SP": "America/Sao_Paulo", "AC": "America/Rio_Branco",
		"AM": "America/Manaus", "MS": "America/Campo_Grande", "BA": "America/Bahia", "PE": "America/Recife",
		"": FusoPadrao, "XX": FusoPadrao,
	}
	for uf, quer := range casos {
		if got := FusoDaUF(uf); got != quer {
			t.Errorf("FusoDaUF(%q) = %q, quer %q", uf, got, quer)
		}
	}
	if UFValida("XX") || UFValida("") || UFValida("mt") {
		t.Error("só siglas maiúsculas de estados brasileiros são válidas")
	}
}

func TestFusoValido(t *testing.T) {
	for _, f := range FusosBrasil {
		if !FusoValido(f.Nome) || f.Descricao == "" {
			t.Errorf("fuso %q da lista deveria ser válido e ter descrição", f.Nome)
		}
	}
	for _, ruim := range []string{"", "UTC", "Europe/Paris", "America/Nova_York", "../etc/passwd", "America/Sao_Paulo "} {
		if FusoValido(ruim) {
			t.Errorf("%q não deveria ser aceito", ruim)
		}
	}
}

// Cuiabá está 1 hora atrás de Brasília e Rio Branco 2: o mesmo instante tem horas diferentes por cliente.
func TestHorariosDiferemPorFuso(t *testing.T) {
	instante := time.Date(2026, 10, 29, 15, 0, 0, 0, time.UTC)
	horas := map[string]string{"America/Sao_Paulo": "12:00", "America/Cuiaba": "11:00", "America/Rio_Branco": "10:00", "America/Noronha": "13:00"}
	for nome, quer := range horas {
		if got := instante.In(Local(nome)).Format("15:04"); got != quer {
			t.Errorf("%s: %s, quer %s", nome, got, quer)
		}
	}
	if Local("fuso-que-nao-existe") == nil {
		t.Error("Local nunca devolve nil")
	}
	if got := instante.In(Local("fuso-que-nao-existe")).Format("15:04"); got != "11:00" {
		t.Errorf("fuso inválido deveria cair no do console (Cuiabá): %s", got)
	}
}

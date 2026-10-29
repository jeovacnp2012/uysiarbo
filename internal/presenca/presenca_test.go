package presenca

import (
	"sync"
	"testing"
	"time"
)

type relogio struct{ t time.Time }

func (r *relogio) agora() time.Time { return r.t }

func novo(t *testing.T) (*Registro, *relogio) {
	t.Helper()
	rel := &relogio{t: time.Date(2026, 10, 29, 14, 0, 0, 0, time.UTC)}
	return &Registro{TTL: 45 * time.Second, Agora: rel.agora}, rel
}

var (
	ana   = Pessoa{Tipo: "usuario", ID: "1", Nome: "Ana Souza"}
	bruno = Pessoa{Tipo: "usuario", ID: "2", Nome: "Bruno Lima"}
)

func TestOutrosNaoIncluiAPropriaJanela(t *testing.T) {
	r, _ := novo(t)
	r.Sinal("c1", "bairro", "7", "abaA", ana, Ver)
	if got := r.Outros("c1", "bairro", "7", "abaA"); len(got) != 0 {
		t.Errorf("ninguém além de mim: %+v", got)
	}
	r.Sinal("c1", "bairro", "7", "abaB", bruno, Editar)
	got := r.Outros("c1", "bairro", "7", "abaA")
	if len(got) != 1 || got[0].Pessoa.Nome != "Bruno Lima" || got[0].Modo != Editar {
		t.Errorf("Ana deveria ver o Bruno editando: %+v", got)
	}
	got = r.Outros("c1", "bairro", "7", "abaB")
	if len(got) != 1 || got[0].Pessoa.Nome != "Ana Souza" || got[0].Modo != Ver {
		t.Errorf("Bruno deveria ver a Ana visualizando: %+v", got)
	}
}

func TestMesmaPessoaEmOutraJanelaApareceComoOutro(t *testing.T) {
	r, _ := novo(t)
	r.Sinal("c1", "bairro", "7", "abaA", ana, Editar)
	r.Sinal("c1", "bairro", "7", "abaB", ana, Ver)
	got := r.Outros("c1", "bairro", "7", "abaA")
	if len(got) != 1 || got[0].Pessoa.ID != "1" || got[0].Aba != "abaB" {
		t.Errorf("a outra janela da mesma pessoa deveria aparecer: %+v", got)
	}
}

func TestEscoposERegistrosNaoSeMisturam(t *testing.T) {
	r, _ := novo(t)
	r.Sinal("c1", "bairro", "7", "a", ana, Ver)
	for nome, got := range map[string][]Presenca{
		"outro cliente":  r.Outros("c2", "bairro", "7", "x"),
		"outro registro": r.Outros("c1", "bairro", "8", "x"),
		"outro recurso":  r.Outros("c1", "cidade", "7", "x"),
	} {
		if len(got) != 0 {
			t.Errorf("%s: não deveria ver ninguém: %+v", nome, got)
		}
	}
	if got := r.Outros("c1", "bairro", "7", "x"); len(got) != 1 {
		t.Errorf("mesmo cliente, recurso e id: %+v", got)
	}
}

func TestPresencaExpiraSemSinalERenovaComSinal(t *testing.T) {
	r, rel := novo(t)
	r.Sinal("c1", "bairro", "7", "a", ana, Ver)
	rel.t = rel.t.Add(30 * time.Second)
	r.Sinal("c1", "bairro", "7", "a", ana, Ver) // renova
	rel.t = rel.t.Add(30 * time.Second)         // 60 s desde o primeiro sinal, 30 s desde o último
	if got := r.Outros("c1", "bairro", "7", "x"); len(got) != 1 {
		t.Errorf("com sinal recente ainda deveria estar: %+v", got)
	}
	rel.t = rel.t.Add(20 * time.Second) // 50 s sem sinal
	if got := r.Outros("c1", "bairro", "7", "x"); len(got) != 0 {
		t.Errorf("sem sinal além do TTL deveria sair: %+v", got)
	}
}

func TestDesdeFicaEMudaSoQuandoOModoMuda(t *testing.T) {
	r, rel := novo(t)
	inicio := rel.t
	r.Sinal("c1", "bairro", "7", "a", ana, Ver)
	rel.t = rel.t.Add(10 * time.Second)
	r.Sinal("c1", "bairro", "7", "a", ana, Ver)
	if got := r.Outros("c1", "bairro", "7", "x"); !got[0].Desde.Equal(inicio) {
		t.Errorf("o 'desde' deveria ficar o do primeiro sinal: %v", got[0].Desde)
	}
	rel.t = rel.t.Add(10 * time.Second)
	r.Sinal("c1", "bairro", "7", "a", ana, Editar)
	got := r.Outros("c1", "bairro", "7", "x")
	if got[0].Modo != Editar || !got[0].Desde.Equal(rel.t) {
		t.Errorf("ao passar a editar, conta desde agora: %+v", got[0])
	}
}

func TestOrdemPeloMaisAntigoESair(t *testing.T) {
	r, rel := novo(t)
	r.Sinal("c1", "bairro", "7", "b", bruno, Editar)
	rel.t = rel.t.Add(5 * time.Second)
	r.Sinal("c1", "bairro", "7", "a", ana, Ver)
	got := r.Outros("c1", "bairro", "7", "x")
	if len(got) != 2 || got[0].Pessoa.Nome != "Bruno Lima" || got[1].Pessoa.Nome != "Ana Souza" {
		t.Errorf("ordem: %+v", got)
	}
	r.Sair("c1", "bairro", "7", "b")
	if got := r.Outros("c1", "bairro", "7", "x"); len(got) != 1 || got[0].Pessoa.Nome != "Ana Souza" {
		t.Errorf("depois de sair: %+v", got)
	}
	r.Sair("c1", "bairro", "7", "a")
	r.Sair("c1", "bairro", "7", "nao-existe") // não deve quebrar
	if len(r.m) != 0 {
		t.Errorf("o mapa deveria esvaziar: %v", r.m)
	}
}

func TestUsoConcorrente(t *testing.T) {
	r := &Registro{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			aba := string(rune('a' + i%26))
			for j := 0; j < 100; j++ {
				r.Sinal("c", "bairro", "1", aba, ana, Ver)
				_ = r.Outros("c", "bairro", "1", aba)
			}
			r.Sair("c", "bairro", "1", aba)
		}(i)
	}
	wg.Wait()
}

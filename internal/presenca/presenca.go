// Package presenca controla quem está com um registro aberto neste momento (vendo ou editando), para
// avisar quem abrir o mesmo registro. É memória do servidor: não vai para o banco e some sozinha
// quando a pessoa fecha a tela ou fica sem dar sinal.
package presenca

import (
	"sort"
	"sync"
	"time"
)

// Modo é o que a pessoa está fazendo no registro.
type Modo string

const (
	Ver    Modo = "ver"    // visualizando
	Editar Modo = "editar" // com a tela de edição aberta
)

// Pessoa é quem está no registro.
type Pessoa struct {
	Tipo string // usuario ou plataforma
	ID   string
	Nome string
}

// Presenca é uma pessoa, em uma janela, num registro.
type Presenca struct {
	Pessoa Pessoa
	Modo   Modo
	Aba    string // identifica a janela/aba (a mesma pessoa pode ter várias)
	Desde  time.Time
	Ultimo time.Time
}

type chave struct{ escopo, recurso, id string }

// Registro guarda as presenças. O zero é utilizável (TTL de 45 s).
type Registro struct {
	TTL   time.Duration
	Agora func() time.Time

	mu      sync.Mutex
	m       map[chave]map[string]*Presenca
	chamada int
}

func (r *Registro) agora() time.Time {
	if r.Agora != nil {
		return r.Agora()
	}
	return time.Now()
}

func (r *Registro) ttl() time.Duration {
	if r.TTL > 0 {
		return r.TTL
	}
	return 45 * time.Second
}

// Sinal registra (ou renova) que a pessoa está no registro nesta janela. escopo separa os clientes
// entre si (e a plataforma); o mesmo recurso e id em escopos diferentes não se misturam.
func (r *Registro) Sinal(escopo, recurso, id, aba string, p Pessoa, modo Modo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	agora := r.agora()
	if r.m == nil {
		r.m = map[chave]map[string]*Presenca{}
	}
	k := chave{escopo, recurso, id}
	abas := r.m[k]
	if abas == nil {
		abas = map[string]*Presenca{}
		r.m[k] = abas
	}
	if cur, ok := abas[aba]; ok && cur.Pessoa.ID == p.ID && cur.Pessoa.Tipo == p.Tipo && agora.Sub(cur.Ultimo) < r.ttl() {
		if cur.Modo != modo { // mudou o que está fazendo: conta desde agora
			cur.Modo, cur.Desde = modo, agora
		}
		cur.Ultimo = agora
	} else {
		abas[aba] = &Presenca{Pessoa: p, Modo: modo, Aba: aba, Desde: agora, Ultimo: agora}
	}
	r.chamada++
	if r.chamada%200 == 0 {
		r.limparTudo(agora)
	}
}

// Outros devolve quem mais está no registro (todas as janelas, menos a que perguntou), do mais antigo
// ao mais novo. Quem está há mais tempo sem dar sinal que o TTL já saiu.
func (r *Registro) Outros(escopo, recurso, id, aba string) []Presenca {
	r.mu.Lock()
	defer r.mu.Unlock()
	agora := r.agora()
	k := chave{escopo, recurso, id}
	var out []Presenca
	for a, p := range r.m[k] {
		if agora.Sub(p.Ultimo) >= r.ttl() {
			delete(r.m[k], a)
			continue
		}
		if a != aba {
			out = append(out, *p)
		}
	}
	if len(r.m[k]) == 0 {
		delete(r.m, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Desde.Equal(out[j].Desde) {
			return out[i].Desde.Before(out[j].Desde)
		}
		return out[i].Aba < out[j].Aba
	})
	return out
}

// Sair tira a janela do registro (ao fechar a tela).
func (r *Registro) Sair(escopo, recurso, id, aba string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := chave{escopo, recurso, id}
	delete(r.m[k], aba)
	if len(r.m[k]) == 0 {
		delete(r.m, k)
	}
}

func (r *Registro) limparTudo(agora time.Time) {
	for k, abas := range r.m {
		for a, p := range abas {
			if agora.Sub(p.Ultimo) >= r.ttl() {
				delete(abas, a)
			}
		}
		if len(abas) == 0 {
			delete(r.m, k)
		}
	}
}

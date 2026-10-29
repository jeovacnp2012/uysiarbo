package geocode

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func servidorFalso(t *testing.T) (*Gratuito, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var viacep, nominatim atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/78450000/json/", func(w http.ResponseWriter, r *http.Request) {
		viacep.Add(1)
		_, _ = w.Write([]byte(`{"cep":"78450-000","logradouro":"Avenida das Palmeiras","bairro":"Centro","localidade":"Nova Mutum","uf":"MT"}`))
	})
	mux.HandleFunc("/ws/00000000/json/", func(w http.ResponseWriter, r *http.Request) {
		viacep.Add(1)
		_, _ = w.Write([]byte(`{"erro":"true"}`))
	})
	mux.HandleFunc("/ws/50000000/json/", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "falhou", 500) })
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		nominatim.Add(1)
		if !strings.Contains(r.Header.Get("User-Agent"), "contato@exemplo.com") {
			t.Errorf("o Nominatim exige User-Agent com contato: %q", r.Header.Get("User-Agent"))
		}
		q := r.URL.Query().Get("q")
		switch {
		case strings.Contains(q, "Rua Perdida"):
			_, _ = w.Write([]byte(`[]`))
		case strings.Contains(q, "Rua da Praça, 10"): // com número
			_, _ = w.Write([]byte(`[{"lat":"-13.8247","lon":"-56.0813"}]`))
		case strings.Contains(q, "Rua da Praça"): // só a rua
			_, _ = w.Write([]byte(`[{"lat":"-13.8200","lon":"-56.0800"}]`))
		case strings.Contains(q, "Rua Sem Número, 7"):
			_, _ = w.Write([]byte(`[]`))
		case strings.Contains(q, "Rua Sem Número"):
			_, _ = w.Write([]byte(`[{"lat":"-13.7","lon":"-56.1"}]`))
		default:
			http.Error(w, "x", 500)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	g := &Gratuito{ViaCEPURL: srv.URL, NominatimURL: srv.URL, Contato: "contato@exemplo.com"}
	g.iniciar()
	g.dormir = func(context.Context, time.Duration) error { return nil }
	return g, &viacep, &nominatim
}

func TestPorCEP(t *testing.T) {
	g, viacep, _ := servidorFalso(t)
	e, err := g.PorCEP(t.Context(), "78450-000")
	if err != nil || e.Logradouro != "Avenida das Palmeiras" || e.Bairro != "Centro" || e.Cidade != "Nova Mutum" || e.UF != "MT" {
		t.Fatalf("PorCEP: %+v %v", e, err)
	}
	if _, err := g.PorCEP(t.Context(), "78450000"); err != nil || viacep.Load() != 1 {
		t.Errorf("a segunda busca deveria sair do cache (chamadas=%d, err=%v)", viacep.Load(), err)
	}
	if _, err := g.PorCEP(t.Context(), "00000-000"); !errors.Is(err, ErrNaoEncontrado) {
		t.Errorf("CEP inexistente: %v", err)
	}
	if _, err := g.PorCEP(t.Context(), "00000000"); !errors.Is(err, ErrNaoEncontrado) || viacep.Load() != 2 {
		t.Errorf("a falta também é guardada (chamadas=%d, err=%v)", viacep.Load(), err)
	}
	for _, ruim := range []string{"", "123", "abcdefgh", "1234567890"} {
		if _, err := g.PorCEP(t.Context(), ruim); !errors.Is(err, ErrNaoEncontrado) {
			t.Errorf("CEP %q: %v", ruim, err)
		}
	}
	if _, err := g.PorCEP(t.Context(), "50000-000"); !errors.Is(err, ErrIndisponivel) {
		t.Errorf("erro do serviço: %v", err)
	}
}

func TestLocalizar(t *testing.T) {
	g, _, nominatim := servidorFalso(t)
	c := Consulta{Logradouro: "Rua da Praça", Numero: "10", Bairro: "Centro", Cidade: "Nova Mutum", UF: "MT"}
	p, err := g.Localizar(t.Context(), c)
	if err != nil || p.Precisao != "endereco" || p.Latitude != -13.8247 || p.Longitude != -56.0813 {
		t.Fatalf("com número: %+v %v", p, err)
	}
	// Mesmo endereço de novo: cache, sem nova chamada.
	n := nominatim.Load()
	if _, err := g.Localizar(t.Context(), c); err != nil || nominatim.Load() != n {
		t.Errorf("deveria sair do cache (chamadas %d -> %d, err=%v)", n, nominatim.Load(), err)
	}

	// Número que o mapa não conhece: cai para a rua, com a precisão avisando.
	p, err = g.Localizar(t.Context(), Consulta{Logradouro: "Rua Sem Número", Numero: "7", Cidade: "Nova Mutum", UF: "MT"})
	if err != nil || p.Precisao != "rua" || p.Latitude != -13.7 {
		t.Errorf("fallback para a rua: %+v %v", p, err)
	}
	// S/N já busca só a rua.
	p, err = g.Localizar(t.Context(), Consulta{Logradouro: "Rua da Praça", Numero: "S/N", Cidade: "Nova Mutum"})
	if err != nil || p.Precisao != "rua" {
		t.Errorf("S/N: %+v %v", p, err)
	}
	if _, err := g.Localizar(t.Context(), Consulta{Logradouro: "Rua Perdida", Numero: "1", Cidade: "Nova Mutum"}); !errors.Is(err, ErrNaoEncontrado) {
		t.Errorf("rua desconhecida: %v", err)
	}
	if _, err := g.Localizar(t.Context(), Consulta{Logradouro: "", Cidade: "Nova Mutum"}); !errors.Is(err, ErrNaoEncontrado) {
		t.Errorf("sem rua: %v", err)
	}
	if _, err := g.Localizar(t.Context(), Consulta{Logradouro: "Qualquer", Cidade: "Nova Mutum"}); !errors.Is(err, ErrIndisponivel) {
		t.Errorf("erro do serviço: %v", err)
	}
}

func TestUmaChamadaPorSegundoAoNominatim(t *testing.T) {
	g := &Gratuito{}
	g.iniciar()
	agora := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	g.agora = func() time.Time { return agora }
	var dormiu []time.Duration
	g.dormir = func(_ context.Context, d time.Duration) error { dormiu = append(dormiu, d); return nil }

	for i := 0; i < 3; i++ {
		if err := g.esperarVez(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	// Sem o relógio andar: a 1ª passa direto, a 2ª espera 1,1 s e a 3ª espera 2,2 s (cada uma reserva a sua vez).
	if len(dormiu) != 2 || dormiu[0] != 1100*time.Millisecond || dormiu[1] != 2200*time.Millisecond {
		t.Errorf("esperas = %v", dormiu)
	}
	agora = agora.Add(time.Hour)
	dormiu = nil
	_ = g.esperarVez(t.Context())
	if len(dormiu) != 0 {
		t.Errorf("depois de muito tempo não deveria esperar: %v", dormiu)
	}
}

func TestDesligado(t *testing.T) {
	var d Desligado
	if _, err := d.PorCEP(t.Context(), "78450000"); !errors.Is(err, ErrDesligado) {
		t.Error(err)
	}
	if _, err := d.Localizar(t.Context(), Consulta{}); !errors.Is(err, ErrDesligado) {
		t.Error(err)
	}
}

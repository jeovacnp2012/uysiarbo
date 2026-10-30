// Package carga faz um teste de carga simples: várias conexões pedindo a mesma URL (GET) por um tempo,
// com o resumo de vazão, erros e percentis de latência. Serve para páginas públicas (/healthz, /entrar);
// para telas que exigem login, passe o cookie de sessão.
package carga

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Resultado resume uma execução.
type Resultado struct {
	Total, Erros int
	PorStatus    map[int]int
	Duracao      time.Duration
	Latencias    []time.Duration // ordenadas
}

// Opcoes configura a execução.
type Opcoes struct {
	URL           string
	Conexoes      int
	Duracao       time.Duration
	Cookie        string // valor do cabeçalho Cookie, opcional
	Host          string // sobrescreve o Host (para testar um cliente específico pelo IP)
	TimeoutPorReq time.Duration
}

// Executar dispara as requisições até acabar o tempo (ou o contexto).
func Executar(ctx context.Context, o Opcoes) (Resultado, error) {
	if o.Conexoes < 1 || o.Conexoes > 500 {
		return Resultado{}, fmt.Errorf("conexões deve ficar entre 1 e 500")
	}
	if o.Duracao < time.Second {
		return Resultado{}, fmt.Errorf("a duração mínima é 1s")
	}
	if o.TimeoutPorReq == 0 {
		o.TimeoutPorReq = 30 * time.Second
	}
	cli := &http.Client{Timeout: o.TimeoutPorReq, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ctx, cancela := context.WithTimeout(ctx, o.Duracao)
	defer cancela()

	var mu sync.Mutex
	res := Resultado{PorStatus: map[int]int{}}
	var wg sync.WaitGroup
	inicio := time.Now()
	for i := 0; i < o.Conexoes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.URL, nil)
				if err != nil {
					return
				}
				if o.Cookie != "" {
					req.Header.Set("Cookie", o.Cookie)
				}
				if o.Host != "" {
					req.Host = o.Host
				}
				t0 := time.Now()
				resp, err := cli.Do(req)
				dur := time.Since(t0)
				if ctx.Err() != nil && err != nil {
					return // a requisição interrompida pelo fim do teste não conta
				}
				mu.Lock()
				res.Total++
				if err != nil {
					res.Erros++
				} else {
					res.PorStatus[resp.StatusCode]++
					if resp.StatusCode >= 500 {
						res.Erros++
					}
					res.Latencias = append(res.Latencias, dur)
				}
				mu.Unlock()
				if err == nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}
		}()
	}
	wg.Wait()
	res.Duracao = time.Since(inicio)
	sort.Slice(res.Latencias, func(i, j int) bool { return res.Latencias[i] < res.Latencias[j] })
	return res, nil
}

// Percentil devolve a latência do percentil p (0 a 100) de uma lista ordenada.
func Percentil(ord []time.Duration, p float64) time.Duration {
	if len(ord) == 0 {
		return 0
	}
	i := int(float64(len(ord))*p/100+0.5) - 1
	i = max(0, min(i, len(ord)-1))
	return ord[i]
}

// Resumo escreve o relatório legível.
func (r Resultado) Resumo() string {
	vazao := 0.0
	if r.Duracao > 0 {
		vazao = float64(r.Total) / r.Duracao.Seconds()
	}
	s := fmt.Sprintf("Requisições: %d em %.1fs (%.1f/s), erros: %d\n", r.Total, r.Duracao.Seconds(), vazao, r.Erros)
	codigos := make([]int, 0, len(r.PorStatus))
	for c := range r.PorStatus {
		codigos = append(codigos, c)
	}
	sort.Ints(codigos)
	for _, c := range codigos {
		s += fmt.Sprintf("  HTTP %d: %d\n", c, r.PorStatus[c])
	}
	s += fmt.Sprintf("Latência: p50 %s  p90 %s  p99 %s  máx %s\n",
		Percentil(r.Latencias, 50).Round(time.Millisecond), Percentil(r.Latencias, 90).Round(time.Millisecond),
		Percentil(r.Latencias, 99).Round(time.Millisecond), Percentil(r.Latencias, 100).Round(time.Millisecond))
	return s
}

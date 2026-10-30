// Package metrics guarda contadores simples em memória e os escreve no formato de texto do Prometheus,
// sem dependências externas.
package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Buckets são os limites (em segundos) do histograma de duração das requisições.
var Buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

type chaveReq struct {
	Metodo, Rota string
	Status       int
}

type histograma struct {
	cont  []uint64 // por bucket (não cumulativo)
	soma  float64
	total uint64
}

// Registro reúne as métricas de HTTP do processo.
type Registro struct {
	inicio   time.Time
	versao   string
	emVoo    atomic.Int64
	mu       sync.Mutex
	reqs     map[chaveReq]uint64
	duracoes map[string]*histograma // por rota
}

// Novo cria um registro; versao aparece em uysiarbo_build_info.
func Novo(versao string) *Registro {
	return &Registro{inicio: time.Now(), versao: versao, reqs: map[chaveReq]uint64{}, duracoes: map[string]*histograma{}}
}

// Entrou e Saiu contam as requisições em andamento.
func (r *Registro) Entrou() { r.emVoo.Add(1) }
func (r *Registro) Saiu()   { r.emVoo.Add(-1) }

// Observar registra uma requisição concluída. rota deve ser o padrão da rota (poucos valores
// diferentes), nunca o caminho cru.
func (r *Registro) Observar(metodo, rota string, status int, dur time.Duration) {
	seg := dur.Seconds()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs[chaveReq{metodo, rota, status}]++
	h := r.duracoes[rota]
	if h == nil {
		h = &histograma{cont: make([]uint64, len(Buckets)+1)}
		r.duracoes[rota] = h
	}
	i := sort.SearchFloat64s(Buckets, seg)
	h.cont[i]++
	h.soma += seg
	h.total++
}

// Pool descreve a situação do pool de conexões do banco.
type Pool struct{ Abertas, Ociosas, EmUso, Maximo int32 }

func esc(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s)
}

// Escrever grava as métricas no formato de texto do Prometheus.
func (r *Registro) Escrever(w io.Writer, pool *Pool) {
	fmt.Fprintf(w, "# HELP uysiarbo_build_info Versão do sistema.\n# TYPE uysiarbo_build_info gauge\nuysiarbo_build_info{versao=\"%s\"} 1\n", esc(r.versao))
	fmt.Fprintf(w, "# HELP uysiarbo_uptime_seconds Tempo desde que o processo iniciou.\n# TYPE uysiarbo_uptime_seconds gauge\nuysiarbo_uptime_seconds %.0f\n", time.Since(r.inicio).Seconds())
	fmt.Fprintf(w, "# HELP uysiarbo_http_in_flight Requisições em andamento.\n# TYPE uysiarbo_http_in_flight gauge\nuysiarbo_http_in_flight %d\n", r.emVoo.Load())
	if pool != nil {
		fmt.Fprintf(w, "# HELP uysiarbo_db_conexoes Conexões do pool do banco.\n# TYPE uysiarbo_db_conexoes gauge\n")
		fmt.Fprintf(w, "uysiarbo_db_conexoes{estado=\"abertas\"} %d\nuysiarbo_db_conexoes{estado=\"ociosas\"} %d\nuysiarbo_db_conexoes{estado=\"em_uso\"} %d\nuysiarbo_db_conexoes{estado=\"maximo\"} %d\n",
			pool.Abertas, pool.Ociosas, pool.EmUso, pool.Maximo)
	}

	r.mu.Lock()
	reqs := make([]chaveReq, 0, len(r.reqs))
	vals := make(map[chaveReq]uint64, len(r.reqs))
	for k, v := range r.reqs {
		reqs = append(reqs, k)
		vals[k] = v
	}
	rotas := make([]string, 0, len(r.duracoes))
	hs := make(map[string]histograma, len(r.duracoes))
	for k, h := range r.duracoes {
		rotas = append(rotas, k)
		hs[k] = histograma{cont: append([]uint64(nil), h.cont...), soma: h.soma, total: h.total}
	}
	r.mu.Unlock()

	sort.Slice(reqs, func(i, j int) bool {
		a, b := reqs[i], reqs[j]
		if a.Rota != b.Rota {
			return a.Rota < b.Rota
		}
		if a.Metodo != b.Metodo {
			return a.Metodo < b.Metodo
		}
		return a.Status < b.Status
	})
	fmt.Fprint(w, "# HELP uysiarbo_http_requests_total Requisições atendidas.\n# TYPE uysiarbo_http_requests_total counter\n")
	for _, k := range reqs {
		fmt.Fprintf(w, "uysiarbo_http_requests_total{metodo=\"%s\",rota=\"%s\",status=\"%d\"} %d\n", esc(k.Metodo), esc(k.Rota), k.Status, vals[k])
	}
	sort.Strings(rotas)
	fmt.Fprint(w, "# HELP uysiarbo_http_request_duration_seconds Duração das requisições.\n# TYPE uysiarbo_http_request_duration_seconds histogram\n")
	for _, rota := range rotas {
		h := hs[rota]
		var acum uint64
		for i, le := range Buckets {
			acum += h.cont[i]
			fmt.Fprintf(w, "uysiarbo_http_request_duration_seconds_bucket{rota=\"%s\",le=\"%g\"} %d\n", esc(rota), le, acum)
		}
		fmt.Fprintf(w, "uysiarbo_http_request_duration_seconds_bucket{rota=\"%s\",le=\"+Inf\"} %d\n", esc(rota), h.total)
		fmt.Fprintf(w, "uysiarbo_http_request_duration_seconds_sum{rota=\"%s\"} %g\nuysiarbo_http_request_duration_seconds_count{rota=\"%s\"} %d\n", esc(rota), h.soma, esc(rota), h.total)
	}
}

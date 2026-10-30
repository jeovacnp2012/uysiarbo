package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestEscreverFormatoPrometheus(t *testing.T) {
	r := Novo("1.2.3")
	r.Entrou()
	r.Observar("GET", "/locais/{id}", 200, 30*time.Millisecond)
	r.Observar("GET", "/locais/{id}", 200, 3*time.Second)
	r.Observar("POST", "/entrar", 401, 5*time.Millisecond)

	var sb strings.Builder
	r.Escrever(&sb, &Pool{Abertas: 3, Ociosas: 2, EmUso: 1, Maximo: 10})
	s := sb.String()
	for _, quer := range []string{
		`uysiarbo_build_info{versao="1.2.3"} 1`,
		`uysiarbo_http_in_flight 1`,
		`uysiarbo_db_conexoes{estado="em_uso"} 1`,
		`uysiarbo_http_requests_total{metodo="GET",rota="/locais/{id}",status="200"} 2`,
		`uysiarbo_http_requests_total{metodo="POST",rota="/entrar",status="401"} 1`,
		`uysiarbo_http_request_duration_seconds_bucket{rota="/locais/{id}",le="0.05"} 1`,
		`uysiarbo_http_request_duration_seconds_bucket{rota="/locais/{id}",le="2.5"} 1`,
		`uysiarbo_http_request_duration_seconds_bucket{rota="/locais/{id}",le="5"} 2`,
		`uysiarbo_http_request_duration_seconds_bucket{rota="/locais/{id}",le="+Inf"} 2`,
		`uysiarbo_http_request_duration_seconds_count{rota="/locais/{id}"} 2`,
	} {
		if !strings.Contains(s, quer) {
			t.Errorf("faltou %q na saída:\n%s", quer, s)
		}
	}
	r.Saiu()
}

func TestEscapaRotulos(t *testing.T) {
	r := Novo(`a"b`)
	var sb strings.Builder
	r.Escrever(&sb, nil)
	if !strings.Contains(sb.String(), `versao="a\"b"`) {
		t.Fatalf("aspas deveriam ser escapadas:\n%s", sb.String())
	}
}

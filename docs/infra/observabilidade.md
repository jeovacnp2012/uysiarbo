# Observabilidade (13.4)

## Logs
O servidor escreve uma linha por requisição (`requisição`: método, caminho, status, bytes, `duration_ms`, `request_id`). Em produção (`APP_ENV=production`) o formato é JSON, uma linha por evento, pronto para `journalctl`/Loki/ELK. O `request_id` também volta no cabeçalho `X-Request-Id` e nas mensagens de erro, então uma queixa do usuário se liga à linha do log. Requisições com **1 s ou mais** geram também um aviso `requisição lenta` (nível WARN). Falhas internas aparecem como `ERROR`; falha ao gravar auditoria sai como `ERRO AO GRAVAR AUDITORIA`.

## Saúde
- `GET /healthz`: o processo está de pé (use no `Restart=` do systemd e no balanceador).
- `GET /readyz`: o banco responde (use para liberar tráfego depois de uma atualização).

## Métricas (Prometheus)
Defina `METRICS_TOKEN` no `.env` (um valor longo e aleatório). Sem ele a rota `/metrics` não existe (404). Com ele:
```
curl -H "Authorization: Bearer $METRICS_TOKEN" https://uysiarbo.inovarsistemas.biz/metrics
```
Séries principais:
- `uysiarbo_http_requests_total{metodo,rota,status}`: requisições; a `rota` é o padrão (`/locais/{id}`), nunca o id.
- `uysiarbo_http_request_duration_seconds` (histograma por rota).
- `uysiarbo_http_in_flight`: requisições em andamento.
- `uysiarbo_db_conexoes{estado=abertas|ociosas|em_uso|maximo}`: pool do banco.
- `uysiarbo_build_info{versao}` e `uysiarbo_uptime_seconds`.

Configuração do Prometheus (trecho):
```
scrape_configs:
  - job_name: uysiarbo
    scheme: https
    metrics_path: /metrics
    authorization: { credentials: "<METRICS_TOKEN>" }
    static_configs: [{ targets: ["uysiarbo.inovarsistemas.biz"] }]
```

## Alertas sugeridos (regras do Prometheus/Alertmanager)
```
- alert: UysiArboForaDoAr
  expr: up{job="uysiarbo"} == 0
  for: 2m
- alert: UysiArboErros5xx
  expr: sum(rate(uysiarbo_http_requests_total{status=~"5.."}[5m])) / sum(rate(uysiarbo_http_requests_total[5m])) > 0.02
  for: 5m
- alert: UysiArboLento
  expr: histogram_quantile(0.95, sum by (le) (rate(uysiarbo_http_request_duration_seconds_bucket[5m]))) > 2
  for: 10m
- alert: UysiArboPoolDoBancoCheio
  expr: uysiarbo_db_conexoes{estado="em_uso"} >= uysiarbo_db_conexoes{estado="maximo"} * 0.9
  for: 5m
- alert: UysiArboBackupAtrasado   # exige o arquivo de backup monitorado por node_exporter/textfile; ou confira `uysiarbo backup listar`
```
Sem Prometheus, o mínimo útil: um monitor externo (UptimeRobot ou similar) em `/readyz` e `journalctl -u uysiarbo -p err` olhado com regularidade.

## Teste de carga
```
uysiarbo carga -url http://127.0.0.1:8090/healthz -c 20 -d 30s
uysiarbo carga -url https://uysiarbonm.uysiarbo.local/entrar -c 10 -d 20s
uysiarbo carga -url https://servidor/locais -c 10 -d 20s -cookie "uysi_sessao=<valor do cookie de uma sessão de teste>"
```
Mostra requisições por segundo, erros (HTTP 5xx e falhas de conexão) e latência p50/p90/p99. Rode em homologação ou fora do horário de uso; use um usuário de teste para páginas com login. Acompanhe `uysiarbo_db_conexoes` durante o teste: se `em_uso` chegar ao máximo, o pool é o gargalo.

## Medições (2026-10-30, máquina de desenvolvimento, banco local)
`/healthz` com 20 conexões por 10 s: cerca de 1.090 req/s, p50 14 ms, p99 60 ms, 0 erros. `/readyz` (com consulta ao banco): cerca de 800 req/s, p99 87 ms, 0 erros. Páginas com login e consultas pesadas (Locais com 39 mil registros, relatórios) ainda não foram medidas: faça com `-cookie` em homologação antes de ir à produção.

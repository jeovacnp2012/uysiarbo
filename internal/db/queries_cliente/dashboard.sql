-- name: DashboardResumo :one
SELECT count(*)::int AS visitas,
       count(DISTINCT v.local_id)::int AS locais,
       count(*) FILTER (WHERE v.achou_deposito)::int AS com_deposito,
       count(*) FILTER (WHERE v.pendencia = 'fechado')::int AS fechadas,
       count(*) FILTER (WHERE v.pendencia = 'recusado')::int AS recusadas,
       count(*) FILTER (WHERE v.pendencia = 'tentativa')::int AS tentativas,
       count(*) FILTER (WHERE v.coletou_amostra)::int AS com_amostra,
       count(*) FILTER (WHERE v.fez_tratamento)::int AS com_tratamento,
       count(DISTINCT v.agente_id)::int AS agentes
FROM visitas v
WHERE v.deleted_at IS NULL AND v.ativo
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint);

-- name: DashboardPorSemana :many
SELECT v.semana::int AS semana, count(*)::int AS visitas,
       count(*) FILTER (WHERE v.achou_deposito)::int AS com_deposito
FROM visitas v
WHERE v.deleted_at IS NULL AND v.ativo AND v.semana IS NOT NULL
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY v.semana ORDER BY v.semana;

-- name: DashboardPorAgente :many
SELECT u.nome AS agente, count(*)::int AS visitas,
       count(*) FILTER (WHERE v.achou_deposito)::int AS com_deposito,
       count(*) FILTER (WHERE v.pendencia <> 'nenhuma')::int AS pendentes
FROM visitas v JOIN usuarios u ON u.id = v.agente_id
WHERE v.deleted_at IS NULL AND v.ativo
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY u.id, u.nome ORDER BY count(*) DESC, u.nome LIMIT 10;

-- name: DashboardAmostrasPorStatus :many
SELECT a.status, count(*)::int AS total
FROM amostras a JOIN visitas v ON v.id = a.visita_id
WHERE a.deleted_at IS NULL AND v.deleted_at IS NULL AND v.ativo
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY a.status ORDER BY a.status;

-- name: DashboardCicloAtual :one
SELECT id, nome FROM ciclos WHERE deleted_at IS NULL
ORDER BY (CURRENT_DATE BETWEEN data_inicio AND data_final) DESC, data_inicio DESC, id DESC LIMIT 1;

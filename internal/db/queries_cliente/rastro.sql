-- name: ListRastroGPS :many
SELECT v.id, v.agente_id, u.nome AS agente, v.data, v.hora_entrada, v.latitude, v.longitude,
       COALESCE(v.precisao_gps, 0)::float8 AS precisao, v.achou_deposito, v.pendencia,
       l.endereco, l.numero
FROM visitas v
JOIN usuarios u ON u.id = v.agente_id
JOIN locais l ON l.id = v.local_id
WHERE v.deleted_at IS NULL AND v.ativo AND v.latitude IS NOT NULL AND v.longitude IS NOT NULL
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR v.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR v.data <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
ORDER BY v.agente_id, v.data, v.hora_entrada NULLS LAST, v.id
LIMIT 5001;

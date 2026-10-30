-- B.I. Epidemiológico (7.3): investigações por doença, bairro e dia de início. Confirmado = resultado
-- positivo; descartado = negativo; em investigação = resultado ainda aguardando.

-- name: EpiPorDoenca :many
SELECT d.nome AS doenca,
       count(*)::int AS total,
       count(*) FILTER (WHERE i.resultado = 'positivo')::int AS confirmados,
       count(*) FILTER (WHERE i.resultado = 'negativo')::int AS descartados,
       count(*) FILTER (WHERE i.resultado = 'inconclusivo')::int AS inconclusivos,
       count(*) FILTER (WHERE i.resultado = 'aguardando')::int AS em_investigacao
FROM investigacoes i JOIN doencas d ON d.id = i.doenca_id
WHERE i.deleted_at IS NULL
  AND (sqlc.narg(data_de)::date IS NULL OR i.data_inicio >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR i.data_inicio <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(doenca_id)::bigint IS NULL OR i.doenca_id = sqlc.narg(doenca_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint)
GROUP BY d.nome
ORDER BY total DESC, d.nome;

-- name: EpiPorBairro :many
SELECT COALESCE(b.nome, '')::text AS bairro,
       count(*)::int AS total,
       count(*) FILTER (WHERE i.resultado = 'positivo')::int AS confirmados
FROM investigacoes i
LEFT JOIN locais l ON l.id = i.local_id
LEFT JOIN bairros b ON b.id = l.bairro_id
WHERE i.deleted_at IS NULL
  AND (sqlc.narg(data_de)::date IS NULL OR i.data_inicio >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR i.data_inicio <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(doenca_id)::bigint IS NULL OR i.doenca_id = sqlc.narg(doenca_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint)
GROUP BY b.nome
ORDER BY total DESC, b.nome;

-- name: EpiPorDia :many
SELECT i.data_inicio::date AS dia,
       count(*)::int AS total,
       count(*) FILTER (WHERE i.resultado = 'positivo')::int AS confirmados
FROM investigacoes i
WHERE i.deleted_at IS NULL AND i.data_inicio IS NOT NULL
  AND (sqlc.narg(data_de)::date IS NULL OR i.data_inicio >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR i.data_inicio <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(doenca_id)::bigint IS NULL OR i.doenca_id = sqlc.narg(doenca_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint)
GROUP BY i.data_inicio
ORDER BY i.data_inicio;

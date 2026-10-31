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
       count(*) FILTER (WHERE i.resultado = 'positivo')::int AS confirmados,
       count(*) FILTER (WHERE i.resultado = 'negativo')::int AS descartados,
       count(*) FILTER (WHERE i.resultado = 'inconclusivo')::int AS inconclusivos
FROM investigacoes i
WHERE i.deleted_at IS NULL AND i.data_inicio IS NOT NULL
  AND (sqlc.narg(data_de)::date IS NULL OR i.data_inicio >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR i.data_inicio <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(doenca_id)::bigint IS NULL OR i.doenca_id = sqlc.narg(doenca_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint)
GROUP BY i.data_inicio
ORDER BY i.data_inicio;

-- name: EpiPorMes :many
-- Investigações por ano e mês de início, para o histórico de vários anos.
SELECT extract(year FROM i.data_inicio)::int AS ano, extract(month FROM i.data_inicio)::int AS mes,
       count(*)::int AS total,
       count(*) FILTER (WHERE i.resultado = 'positivo')::int AS confirmados
FROM investigacoes i
WHERE i.deleted_at IS NULL AND i.data_inicio IS NOT NULL
  AND i.data_inicio >= sqlc.arg(desde)::date AND i.data_inicio <= sqlc.arg(ate)::date
  AND (sqlc.narg(doenca_id)::bigint IS NULL OR i.doenca_id = sqlc.narg(doenca_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint)
GROUP BY 1, 2
ORDER BY 1, 2;

-- name: EpiPorAnoMesmoPeriodo :many
-- Total e confirmados por ano até o mesmo dia do ano (mes*100+dia), para comparar os anos de forma justa.
SELECT extract(year FROM i.data_inicio)::int AS ano,
       count(*)::int AS total,
       count(*) FILTER (WHERE i.resultado = 'positivo')::int AS confirmados,
       count(*) FILTER (WHERE i.resultado = 'negativo')::int AS descartados
FROM investigacoes i
WHERE i.deleted_at IS NULL AND i.data_inicio IS NOT NULL
  AND i.data_inicio >= sqlc.arg(desde)::date AND i.data_inicio <= sqlc.arg(ate)::date
  AND (extract(month FROM i.data_inicio)::int * 100 + extract(day FROM i.data_inicio)::int) <= sqlc.arg(mes_dia)::int
  AND (sqlc.narg(doenca_id)::bigint IS NULL OR i.doenca_id = sqlc.narg(doenca_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint)
GROUP BY 1
ORDER BY 1;

-- name: EpiOportunidade :one
-- Situação e tempo de encerramento (data de início até o encerramento) das investigações do período.
SELECT count(*)::int AS total,
       count(*) FILTER (WHERE i.situacao = 'encerrada')::int AS encerradas,
       count(*) FILTER (WHERE i.situacao = 'em_atendimento')::int AS em_atendimento,
       count(*) FILTER (WHERE i.situacao = 'aguardando')::int AS aguardando,
       count(*) FILTER (WHERE i.situacao = 'encerrada' AND i.encerrada_em IS NOT NULL AND i.data_inicio IS NOT NULL
                          AND i.encerrada_em::date - i.data_inicio <= 7)::int AS ate_7,
       count(*) FILTER (WHERE i.situacao = 'encerrada' AND i.encerrada_em IS NOT NULL AND i.data_inicio IS NOT NULL
                          AND i.encerrada_em::date - i.data_inicio <= 30)::int AS ate_30,
       count(*) FILTER (WHERE i.situacao = 'encerrada' AND i.encerrada_em IS NOT NULL AND i.data_inicio IS NOT NULL
                          AND i.encerrada_em::date >= i.data_inicio)::int AS com_tempo,
       COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY (i.encerrada_em::date - i.data_inicio))
                FILTER (WHERE i.situacao = 'encerrada' AND i.encerrada_em IS NOT NULL AND i.data_inicio IS NOT NULL
                          AND i.encerrada_em::date >= i.data_inicio), 0)::float8 AS mediana_dias
FROM investigacoes i
WHERE i.deleted_at IS NULL
  AND (sqlc.narg(data_de)::date IS NULL OR i.data_inicio >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR i.data_inicio <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(doenca_id)::bigint IS NULL OR i.doenca_id = sqlc.narg(doenca_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint);

-- name: CampoPorCiclo :many
-- Trabalho de campo por ciclo (ciclos que começam entre os anos informados): imóveis visitados, fechados,
-- recuperados (fechado e depois trabalhado no mesmo ciclo), fechados sem recuperação e imóveis positivos.
WITH vl AS (
    SELECT v.ciclo_id, v.local_id,
           bool_or(v.pendencia = 'fechado') AS teve_fechado,
           bool_or(v.pendencia = 'nenhuma') AS trabalhado,
           count(*) AS visitas
    FROM visitas v JOIN ciclos c ON c.id = v.ciclo_id JOIN locais l ON l.id = v.local_id
    WHERE v.deleted_at IS NULL AND v.ativo AND c.deleted_at IS NULL AND l.deleted_at IS NULL
      AND extract(year FROM c.data_inicio) BETWEEN sqlc.arg(ano_de)::int AND sqlc.arg(ano_ate)::int
      AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
      AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, l.bairro_id))
    GROUP BY v.ciclo_id, v.local_id
), pos AS (
    SELECT v.ciclo_id, count(DISTINCT v.local_id) AS positivos
    FROM visitas v JOIN amostras a ON a.visita_id = v.id
    WHERE v.deleted_at IS NULL AND v.ativo AND v.pendencia = 'nenhuma' AND a.deleted_at IS NULL
      AND a.status = 'concluido' AND (a.total_aegypti + a.total_albopictus) > 0
    GROUP BY v.ciclo_id
)
SELECT c.id AS ciclo_id, c.nome AS ciclo, extract(year FROM c.data_inicio)::int AS ano, c.data_inicio,
       COALESCE(sum(vl.visitas), 0)::int AS visitas,
       count(vl.local_id)::int AS imoveis,
       count(*) FILTER (WHERE vl.teve_fechado)::int AS fechados,
       count(*) FILTER (WHERE vl.teve_fechado AND vl.trabalhado)::int AS recuperados,
       count(*) FILTER (WHERE vl.teve_fechado AND NOT vl.trabalhado)::int AS fechados_sem_rec,
       COALESCE(max(pos.positivos), 0)::int AS positivos
FROM ciclos c
JOIN vl ON vl.ciclo_id = c.id
LEFT JOIN pos ON pos.ciclo_id = c.id
GROUP BY c.id, c.nome, c.data_inicio
ORDER BY c.data_inicio;

-- name: CampoFechadosPorBairro :many
-- Por bairro, nos ciclos que começam no ano: imóveis visitados e fechados sem recuperação.
WITH vl AS (
    SELECT v.ciclo_id, v.local_id, l.bairro_id,
           bool_or(v.pendencia = 'fechado') AS teve_fechado,
           bool_or(v.pendencia = 'nenhuma') AS trabalhado
    FROM visitas v JOIN ciclos c ON c.id = v.ciclo_id JOIN locais l ON l.id = v.local_id
    WHERE v.deleted_at IS NULL AND v.ativo AND c.deleted_at IS NULL AND l.deleted_at IS NULL
      AND extract(year FROM c.data_inicio) = sqlc.arg(ano)::int
      AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
      AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, l.bairro_id))
    GROUP BY v.ciclo_id, v.local_id, l.bairro_id
)
SELECT b.nome AS bairro, count(*)::int AS imoveis,
       count(*) FILTER (WHERE vl.teve_fechado AND NOT vl.trabalhado)::int AS fechados_sem_rec
FROM vl JOIN bairros b ON b.id = vl.bairro_id
GROUP BY b.id, b.nome
ORDER BY b.nome;

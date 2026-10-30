-- Resumo Semanal no formato da ficha do PNCD: uma semana, um bairro, uma atividade.
-- Imóvel trabalhado = local com visita sem pendência; recuperados = visitas do tipo "recuperacao".

-- name: PncdLocalidade :one
SELECT b.nome AS bairro, c.nome AS cidade, c.ibge
FROM bairros b JOIN cidades c ON c.id = b.cidade_id WHERE b.id = $1;

-- name: PncdResumo :one
WITH v AS (
    SELECT v.*, l.tipo AS ltipo, l.quarteirao_id
    FROM visitas v JOIN locais l ON l.id = v.local_id
    WHERE v.deleted_at IS NULL AND v.ativo AND v.ciclo_id = @ciclo_id AND v.semana = @semana
      AND l.bairro_id = @bairro_id AND v.atividade_id = @atividade_id
      AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
), ok AS (SELECT * FROM v WHERE pendencia = 'nenhuma')
SELECT
  (SELECT count(DISTINCT quarteirao_id) FROM ok)::int AS quarteiroes,
  (SELECT count(DISTINCT local_id) FROM ok WHERE ltipo = 'residencial')::int AS im_residencia,
  (SELECT count(DISTINCT local_id) FROM ok WHERE ltipo = 'comercio')::int AS im_comercio,
  (SELECT count(DISTINCT local_id) FROM ok WHERE ltipo = 'terreno_baldio')::int AS im_tb,
  (SELECT count(DISTINCT local_id) FROM ok WHERE ltipo = 'ponto_estrategico')::int AS im_pe,
  (SELECT count(DISTINCT local_id) FROM ok WHERE ltipo = 'outro')::int AS im_outro,
  (SELECT count(DISTINCT local_id) FROM ok)::int AS im_total,
  (SELECT count(DISTINCT local_id) FROM ok WHERE l1_tratados > 0 OR l2_tratados > 0)::int AS trat_focal,
  (SELECT count(DISTINCT local_id) FROM ok WHERE adulticida_cargas > 0)::int AS trat_perifocal,
  (SELECT count(*) FROM v WHERE pendencia = 'recusado')::int AS recusadas,
  (SELECT count(*) FROM v WHERE pendencia = 'fechado')::int AS fechados,
  (SELECT count(*) FROM v WHERE tipo_visita = 'recuperacao' AND pendencia = 'nenhuma')::int AS recuperados,
  (SELECT COALESCE(sum(dep_a1), 0) FROM ok)::int AS dep_a1, (SELECT COALESCE(sum(dep_a2), 0) FROM ok)::int AS dep_a2,
  (SELECT COALESCE(sum(dep_b), 0) FROM ok)::int AS dep_b, (SELECT COALESCE(sum(dep_c), 0) FROM ok)::int AS dep_c,
  (SELECT COALESCE(sum(dep_d1), 0) FROM ok)::int AS dep_d1, (SELECT COALESCE(sum(dep_d2), 0) FROM ok)::int AS dep_d2,
  (SELECT COALESCE(sum(dep_e), 0) FROM ok)::int AS dep_e,
  (SELECT COALESCE(sum(total_eliminado), 0) FROM ok)::int AS eliminados,
  (SELECT COALESCE(sum(l1_gramas), 0) FROM ok)::float8 AS l1_gramas, (SELECT COALESCE(sum(l1_tratados), 0) FROM ok)::int AS l1_tratados,
  (SELECT COALESCE(sum(l2_gramas), 0) FROM ok)::float8 AS l2_gramas, (SELECT COALESCE(sum(l2_tratados), 0) FROM ok)::int AS l2_tratados,
  (SELECT COALESCE(max(adulticida_tipo), '') FROM ok WHERE adulticida_cargas > 0)::text AS adulticida_tipo,
  (SELECT COALESCE(sum(adulticida_cargas), 0) FROM ok)::int AS adulticida_cargas,
  (SELECT count(DISTINCT agente_id) FROM v)::int AS agentes,
  (SELECT count(DISTINCT (agente_id, data)) FROM v)::int AS dias,
  (SELECT count(*) FROM amostras a JOIN v ON v.id = a.visita_id WHERE a.deleted_at IS NULL)::int AS amostras;

-- name: PncdConcluido :one
SELECT NOT EXISTS (
    SELECT 1 FROM locais l
    WHERE l.bairro_id = @bairro_id AND l.ativo AND l.deleted_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM visitas v WHERE v.local_id = l.id AND v.ciclo_id = @ciclo_id AND v.deleted_at IS NULL AND v.ativo AND v.pendencia = 'nenhuma')
)::bool AS concluido;

-- name: PncdLabDepositos :many
SELECT a.deposito::text AS deposito,
       count(*) FILTER (WHERE a.total_aegypti > 0)::int AS aegypti,
       count(*) FILTER (WHERE a.total_albopictus > 0)::int AS albopictus
FROM amostras a JOIN visitas v ON v.id = a.visita_id JOIN locais l ON l.id = v.local_id
WHERE a.deleted_at IS NULL AND a.status = 'concluido' AND v.deleted_at IS NULL AND v.ativo AND v.ciclo_id = @ciclo_id AND v.semana = @semana
  AND l.bairro_id = @bairro_id AND v.atividade_id = @atividade_id
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY a.deposito;

-- name: PncdLabImoveis :many
SELECT l.tipo::text AS tipo,
       count(DISTINCT l.id) FILTER (WHERE a.total_aegypti > 0)::int AS aegypti,
       count(DISTINCT l.id) FILTER (WHERE a.total_albopictus > 0)::int AS albopictus,
       COALESCE(sum(a.total_aegypti), 0)::int AS larvas_aegypti, COALESCE(sum(a.total_albopictus), 0)::int AS larvas_albopictus
FROM amostras a JOIN visitas v ON v.id = a.visita_id JOIN locais l ON l.id = v.local_id
WHERE a.deleted_at IS NULL AND a.status = 'concluido' AND v.deleted_at IS NULL AND v.ativo AND v.ciclo_id = @ciclo_id AND v.semana = @semana
  AND l.bairro_id = @bairro_id AND v.atividade_id = @atividade_id
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY l.tipo;

-- name: PncdLabQuarteiroes :many
SELECT q.nome::text AS quarteirao,
       bool_or(a.total_aegypti > 0)::bool AS aegypti, bool_or(a.total_albopictus > 0)::bool AS albopictus
FROM amostras a JOIN visitas v ON v.id = a.visita_id JOIN locais l ON l.id = v.local_id JOIN quarteiroes q ON q.id = l.quarteirao_id
WHERE a.deleted_at IS NULL AND a.status = 'concluido' AND v.deleted_at IS NULL AND v.ativo AND v.ciclo_id = @ciclo_id AND v.semana = @semana
  AND l.bairro_id = @bairro_id AND v.atividade_id = @atividade_id
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY q.id, q.nome HAVING bool_or(a.total_aegypti > 0) OR bool_or(a.total_albopictus > 0)
ORDER BY q.nome;

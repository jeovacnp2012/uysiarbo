-- B.I. (7.2). Imóvel pesquisado = local com visita sem pendência; positivo = local com tubito concluído
-- com larvas/pupas de Aedes (aegypti ou albopictus); recipiente positivo = tubito positivo.
-- name: BiPorBairro :many
WITH pesq AS (
    SELECT v.id AS visita_id, v.local_id, l.bairro_id
    FROM visitas v JOIN locais l ON l.id = v.local_id
    WHERE v.deleted_at IS NULL AND v.ativo AND v.pendencia = 'nenhuma' AND l.deleted_at IS NULL
      AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
      AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
      AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
      AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
      AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
      AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, l.bairro_id))
), pos AS (
    SELECT p.bairro_id, p.local_id, count(*) AS tubitos
    FROM pesq p JOIN amostras a ON a.visita_id = p.visita_id
    WHERE a.deleted_at IS NULL AND a.status = 'concluido' AND (a.total_aegypti + a.total_albopictus) > 0
    GROUP BY p.bairro_id, p.local_id
), por_bairro AS (
    SELECT bairro_id, count(DISTINCT local_id) AS pesquisados FROM pesq GROUP BY bairro_id
), pos_bairro AS (
    SELECT bairro_id, count(*) AS imoveis_positivos, sum(tubitos) AS recipientes FROM pos GROUP BY bairro_id
)
SELECT b.id AS bairro_id, b.nome AS bairro,
       (SELECT count(*) FROM locais l WHERE l.bairro_id = b.id AND l.ativo AND l.deleted_at IS NULL)::int AS locais,
       pb.pesquisados::int AS pesquisados,
       COALESCE(pos.imoveis_positivos, 0)::int AS positivos,
       COALESCE(pos.recipientes, 0)::int AS recipientes
FROM por_bairro pb JOIN bairros b ON b.id = pb.bairro_id
LEFT JOIN pos_bairro pos ON pos.bairro_id = pb.bairro_id
ORDER BY b.nome;

-- name: BiVisitasPorBairro :many
-- Visitas e depósitos por bairro (todas as visitas do filtro; os depósitos só das sem pendência).
SELECT b.nome AS bairro, count(*)::int AS visitas,
       COALESCE(sum(v.dep_a1) FILTER (WHERE v.pendencia = 'nenhuma'), 0)::int AS a1,
       COALESCE(sum(v.dep_a2) FILTER (WHERE v.pendencia = 'nenhuma'), 0)::int AS a2,
       COALESCE(sum(v.dep_b)  FILTER (WHERE v.pendencia = 'nenhuma'), 0)::int AS b,
       COALESCE(sum(v.dep_c)  FILTER (WHERE v.pendencia = 'nenhuma'), 0)::int AS c,
       COALESCE(sum(v.dep_d1) FILTER (WHERE v.pendencia = 'nenhuma'), 0)::int AS d1,
       COALESCE(sum(v.dep_d2) FILTER (WHERE v.pendencia = 'nenhuma'), 0)::int AS d2,
       COALESCE(sum(v.dep_e)  FILTER (WHERE v.pendencia = 'nenhuma'), 0)::int AS e
FROM visitas v JOIN locais l ON l.id = v.local_id JOIN bairros b ON b.id = l.bairro_id
WHERE v.deleted_at IS NULL AND v.ativo AND l.deleted_at IS NULL
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, l.bairro_id))
GROUP BY b.id, b.nome
ORDER BY b.nome;

-- name: BiTotais :one
-- Quarteirões e locais (total e visitados), imóveis fechados e tubitos positivos/negativos do filtro.
WITH vis AS (
    SELECT v.id, v.local_id, v.pendencia, l.quarteirao_id
    FROM visitas v JOIN locais l ON l.id = v.local_id
    WHERE v.deleted_at IS NULL AND v.ativo AND l.deleted_at IS NULL
      AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
      AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
      AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
      AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
      AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
      AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, l.bairro_id))
)
SELECT
    (SELECT count(*) FROM quarteiroes q JOIN bairros b ON b.id = q.bairro_id
      WHERE q.ativo AND b.deleted_at IS NULL
        AND (sqlc.narg(bairro_id)::bigint IS NULL OR q.bairro_id = sqlc.narg(bairro_id)::bigint)
        AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, q.bairro_id)))::int AS quarteiroes,
    (SELECT count(DISTINCT quarteirao_id) FROM vis WHERE pendencia = 'nenhuma')::int AS quarteiroes_visitados,
    (SELECT count(*) FROM locais l
      WHERE l.ativo AND l.deleted_at IS NULL
        AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
        AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, l.bairro_id)))::int AS locais,
    (SELECT count(DISTINCT local_id) FROM vis WHERE pendencia = 'nenhuma')::int AS locais_visitados,
    (SELECT count(DISTINCT local_id) FROM vis WHERE pendencia = 'fechado')::int AS fechados,
    (SELECT count(*) FROM vis JOIN amostras a ON a.visita_id = vis.id
      WHERE a.deleted_at IS NULL AND a.status = 'concluido' AND (a.total_aegypti + a.total_albopictus) > 0)::int AS amostras_positivas,
    (SELECT count(*) FROM vis JOIN amostras a ON a.visita_id = vis.id
      WHERE a.deleted_at IS NULL AND a.status = 'concluido' AND (a.total_aegypti + a.total_albopictus) = 0)::int AS amostras_negativas;

-- name: BiPositivosPorTipo :many
-- Locais positivos (tubito concluído com Aedes) por tipo de local.
SELECT l.tipo, count(DISTINCT l.id)::int AS locais
FROM visitas v JOIN locais l ON l.id = v.local_id JOIN amostras a ON a.visita_id = v.id
WHERE v.deleted_at IS NULL AND v.ativo AND v.pendencia = 'nenhuma' AND l.deleted_at IS NULL
  AND a.deleted_at IS NULL AND a.status = 'concluido' AND (a.total_aegypti + a.total_albopictus) > 0
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, l.bairro_id))
GROUP BY l.tipo
ORDER BY locais DESC, l.tipo;

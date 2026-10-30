-- ---------------------------------------------------------------- Campanha antirrábica (10.2)

-- name: CreateCampanha :one
INSERT INTO campanhas (local_id, data_hora, situacao, vac_cachorros, vac_gatos, nao_cachorros, nao_gatos, descricao, aplicador_id, criado_por)
VALUES (sqlc.arg(local_id), sqlc.arg(data_hora), sqlc.arg(situacao), sqlc.arg(vac_cachorros), sqlc.arg(vac_gatos), sqlc.arg(nao_cachorros),
        sqlc.arg(nao_gatos), sqlc.arg(descricao), sqlc.narg(aplicador_id), sqlc.narg(criado_por))
RETURNING *;

-- name: GetCampanha :one
SELECT * FROM campanhas WHERE id = $1 AND deleted_at IS NULL;

-- name: GetCampanhaQualquer :one
SELECT * FROM campanhas WHERE id = $1;

-- name: UpdateCampanha :execrows
UPDATE campanhas SET data_hora = sqlc.arg(data_hora), situacao = sqlc.arg(situacao), vac_cachorros = sqlc.arg(vac_cachorros),
       vac_gatos = sqlc.arg(vac_gatos), nao_cachorros = sqlc.arg(nao_cachorros), nao_gatos = sqlc.arg(nao_gatos),
       descricao = sqlc.arg(descricao), aplicador_id = sqlc.narg(aplicador_id), atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountCampanhas :one
SELECT count(*)::int
FROM campanhas c
JOIN locais l ON l.id = c.local_id
LEFT JOIN usuarios a ON a.id = c.aplicador_id
WHERE c.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR c.id::text = sqlc.arg(busca)::text OR c.id_externo = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(l.endereco || ' ' || l.numero || ' ' || COALESCE(a.nome, '') || ' ' || c.descricao)) > 0)
  AND (sqlc.arg(situacao)::text = '' OR c.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(local_id)::bigint IS NULL OR c.local_id = sqlc.narg(local_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR c.data_hora >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR c.data_hora < sqlc.narg(data_ate)::date + 1)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id));

-- name: ListCampanhas :many
SELECT c.id, c.data_hora, c.situacao, c.usar_estoque, c.vac_cachorros, c.vac_gatos, c.nao_cachorros, c.nao_gatos, c.local_id,
       (l.endereco || ', ' || l.numero)::text AS endereco, COALESCE(b.nome, '')::text AS bairro, COALESCE(a.nome, '')::text AS aplicador
FROM campanhas c
JOIN locais l ON l.id = c.local_id
LEFT JOIN bairros b ON b.id = l.bairro_id
LEFT JOIN usuarios a ON a.id = c.aplicador_id
WHERE c.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR c.id::text = sqlc.arg(busca)::text OR c.id_externo = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(l.endereco || ' ' || l.numero || ' ' || COALESCE(a.nome, '') || ' ' || c.descricao)) > 0)
  AND (sqlc.arg(situacao)::text = '' OR c.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(local_id)::bigint IS NULL OR c.local_id = sqlc.narg(local_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR c.data_hora >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR c.data_hora < sqlc.narg(data_ate)::date + 1)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id))
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN c.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN c.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN c.data_hora END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN c.data_hora END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'asc' THEN lower(l.endereco) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'desc' THEN lower(l.endereco) END DESC,
  c.data_hora DESC, c.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- ---- exclusão lógica

-- name: SoftDeleteCampanha :execrows
UPDATE campanhas SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreCampanha :execrows
UPDATE campanhas SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteCampanha :execrows
DELETE FROM campanhas WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListCampanhasExcluidas :many
SELECT c.id, (l.endereco || ', ' || l.numero)::text AS contexto, c.deleted_at, COALESCE(u.nome, '')::text AS por
FROM campanhas c JOIN locais l ON l.id = c.local_id LEFT JOIN usuarios u ON u.id = c.deleted_by
WHERE c.deleted_at IS NOT NULL ORDER BY c.deleted_at DESC LIMIT 500;

-- ---- vínculos do local

-- name: CountCampanhasAtivasDoLocal :one
SELECT count(*)::int FROM campanhas WHERE local_id = $1 AND deleted_at IS NULL;

-- name: CountCampanhasDoLocalQualquer :one
SELECT count(*)::int FROM campanhas WHERE local_id = $1;

-- name: ListCampanhasDaFichaDoLocal :many
SELECT c.id, c.data_hora, c.situacao, c.usar_estoque, c.vac_cachorros, c.vac_gatos, c.nao_cachorros, c.nao_gatos, COALESCE(a.nome, '')::text AS aplicador
FROM campanhas c LEFT JOIN usuarios a ON a.id = c.aplicador_id
WHERE c.local_id = $1 AND c.deleted_at IS NULL ORDER BY c.data_hora DESC, c.id DESC LIMIT 20;

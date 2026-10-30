-- ---------------------------------------------------------------- Solicitações de drone (9.5)

-- name: CreateDrone :one
INSERT INTO drone_solicitacoes (local_id, visita_id, descricao, data_abertura, criado_por)
VALUES (sqlc.arg(local_id), sqlc.narg(visita_id), sqlc.arg(descricao), sqlc.arg(data_abertura), sqlc.narg(criado_por))
RETURNING *;

-- name: GetDrone :one
SELECT * FROM drone_solicitacoes WHERE id = $1 AND deleted_at IS NULL;

-- name: GetDroneQualquer :one
SELECT * FROM drone_solicitacoes WHERE id = $1;

-- name: SetDroneVisita :execrows
UPDATE drone_solicitacoes SET visita_id = sqlc.narg(visita_id), atualizado_em = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: SetDroneSituacao :execrows
UPDATE drone_solicitacoes SET situacao = sqlc.arg(situacao)::text, responsavel_id = sqlc.narg(responsavel_id),
       encerrada_em = CASE WHEN sqlc.arg(situacao)::text = 'encerrada' THEN now() ELSE NULL END, atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountDrones :one
SELECT count(*)::int
FROM drone_solicitacoes d
JOIN locais l ON l.id = d.local_id
LEFT JOIN usuarios c ON c.id = d.criado_por
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR d.id::text = sqlc.arg(busca)::text OR d.id_externo = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(d.descricao || ' ' || l.endereco || ' ' || l.numero || ' ' || COALESCE(c.nome, ''))) > 0)
  AND (sqlc.arg(situacao)::text = '' OR d.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR d.data_abertura >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR d.data_abertura <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id));

-- name: ListDrones :many
SELECT d.id, d.descricao, d.data_abertura, d.situacao, d.local_id,
       (l.endereco || ', ' || l.numero)::text AS endereco,
       COALESCE(b.nome, '')::text AS bairro, COALESCE(c.nome, '')::text AS aberto_por, COALESCE(u.nome, '')::text AS responsavel
FROM drone_solicitacoes d
JOIN locais l ON l.id = d.local_id
LEFT JOIN bairros b ON b.id = l.bairro_id
LEFT JOIN usuarios c ON c.id = d.criado_por
LEFT JOIN usuarios u ON u.id = d.responsavel_id
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR d.id::text = sqlc.arg(busca)::text OR d.id_externo = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(d.descricao || ' ' || l.endereco || ' ' || l.numero || ' ' || COALESCE(c.nome, ''))) > 0)
  AND (sqlc.arg(situacao)::text = '' OR d.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR d.data_abertura >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR d.data_abertura <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id))
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN d.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN d.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN d.data_abertura END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN d.data_abertura END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'asc' THEN lower(l.endereco) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'desc' THEN lower(l.endereco) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'asc' THEN d.situacao END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'desc' THEN d.situacao END DESC,
  d.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- ---- exclusão lógica

-- name: SoftDeleteDrone :execrows
UPDATE drone_solicitacoes SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreDrone :execrows
UPDATE drone_solicitacoes SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteDrone :execrows
DELETE FROM drone_solicitacoes WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListDronesExcluidos :many
SELECT d.id, d.descricao, d.deleted_at, COALESCE(u.nome, '')::text AS por
FROM drone_solicitacoes d LEFT JOIN usuarios u ON u.id = d.deleted_by
WHERE d.deleted_at IS NOT NULL ORDER BY d.deleted_at DESC LIMIT 500;

-- ---- vínculos do local

-- name: CountDronesAtivosDoLocal :one
SELECT count(*)::int FROM drone_solicitacoes WHERE local_id = $1 AND deleted_at IS NULL;

-- name: CountDronesDoLocalQualquer :one
SELECT count(*)::int FROM drone_solicitacoes WHERE local_id = $1;

-- name: ListDronesDoLocal :many
SELECT id, descricao, data_abertura, situacao FROM drone_solicitacoes
WHERE local_id = $1 AND deleted_at IS NULL ORDER BY id DESC LIMIT 20;

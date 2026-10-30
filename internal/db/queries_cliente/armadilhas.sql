-- ---------------------------------------------------------------- Ovitrampas e EDLs (10.1)

-- name: CreateArmadilha :one
INSERT INTO armadilhas (tipo, local_id, instalada_em, agente_id, latitude, longitude, anotacao, criado_por)
VALUES (sqlc.arg(tipo), sqlc.arg(local_id), sqlc.arg(instalada_em), sqlc.narg(agente_id), sqlc.narg(latitude), sqlc.narg(longitude),
        sqlc.arg(anotacao), sqlc.narg(criado_por))
RETURNING *;

-- name: GetArmadilha :one
SELECT * FROM armadilhas WHERE id = $1 AND tipo = $2 AND deleted_at IS NULL;

-- name: GetArmadilhaQualquer :one
SELECT * FROM armadilhas WHERE id = $1;

-- name: UpdateArmadilhaDados :one
UPDATE armadilhas SET instalada_em = sqlc.arg(instalada_em), agente_id = sqlc.narg(agente_id), latitude = sqlc.narg(latitude),
       longitude = sqlc.narg(longitude), anotacao = sqlc.arg(anotacao), atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL RETURNING *;

-- name: SetArmadilhaStatus :execrows
UPDATE armadilhas SET status = sqlc.arg(status)::text, retirada_em = sqlc.narg(retirada_em), retirado_por_id = sqlc.narg(retirado_por_id),
       atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountArmadilhas :one
SELECT count(*)::int
FROM armadilhas a
JOIN locais l ON l.id = a.local_id
WHERE a.deleted_at IS NULL AND a.tipo = sqlc.arg(tipo)::text
  AND (sqlc.arg(busca)::text = '' OR a.id::text = sqlc.arg(busca)::text OR a.id_externo = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(l.endereco || ' ' || l.numero || ' ' || a.anotacao)) > 0)
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR a.instalada_em >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR a.instalada_em < sqlc.narg(data_ate)::date + 1)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id));

-- name: ListArmadilhas :many
SELECT a.id, a.status, a.instalada_em, (l.endereco || ', ' || l.numero)::text AS endereco, COALESCE(b.nome, '')::text AS bairro,
       COALESCE(u.nome, '')::text AS agente,
       (SELECT r.registrado_em FROM armadilha_registros r WHERE r.armadilha_id = a.id AND r.deleted_at IS NULL ORDER BY r.registrado_em DESC LIMIT 1)::timestamptz AS ultimo_em,
       COALESCE((SELECT r.ovos FROM armadilha_registros r WHERE r.armadilha_id = a.id AND r.deleted_at IS NULL ORDER BY r.registrado_em DESC LIMIT 1), -1)::int AS ultimo_ovos,
       COALESCE((SELECT r.nivel_agua FROM armadilha_registros r WHERE r.armadilha_id = a.id AND r.deleted_at IS NULL ORDER BY r.registrado_em DESC LIMIT 1), '')::text AS ultimo_nivel
FROM armadilhas a
JOIN locais l ON l.id = a.local_id
LEFT JOIN bairros b ON b.id = l.bairro_id
LEFT JOIN usuarios u ON u.id = a.agente_id
WHERE a.deleted_at IS NULL AND a.tipo = sqlc.arg(tipo)::text
  AND (sqlc.arg(busca)::text = '' OR a.id::text = sqlc.arg(busca)::text OR a.id_externo = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(l.endereco || ' ' || l.numero || ' ' || a.anotacao)) > 0)
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR a.instalada_em >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR a.instalada_em < sqlc.narg(data_ate)::date + 1)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id))
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN a.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN a.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN a.instalada_em END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN a.instalada_em END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'asc' THEN lower(l.endereco) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'desc' THEN lower(l.endereco) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'status' AND sqlc.arg(dir)::text = 'asc' THEN a.status END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'status' AND sqlc.arg(dir)::text = 'desc' THEN a.status END DESC,
  a.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- ---- contagens e manutenções

-- name: CreateArmadilhaRegistro :one
INSERT INTO armadilha_registros (armadilha_id, ovos, nivel_agua, registrado_em, por_id, anotacao)
VALUES (sqlc.arg(armadilha_id), sqlc.narg(ovos), sqlc.arg(nivel_agua), sqlc.arg(registrado_em), sqlc.narg(por_id), sqlc.arg(anotacao))
RETURNING *;

-- name: GetArmadilhaRegistro :one
SELECT * FROM armadilha_registros WHERE id = $1 AND armadilha_id = $2 AND deleted_at IS NULL;

-- name: UpdateArmadilhaRegistro :execrows
UPDATE armadilha_registros SET ovos = sqlc.narg(ovos), nivel_agua = sqlc.arg(nivel_agua), anotacao = sqlc.arg(anotacao)
WHERE id = sqlc.arg(id) AND armadilha_id = sqlc.arg(armadilha_id) AND deleted_at IS NULL;

-- name: SoftDeleteArmadilhaRegistro :execrows
UPDATE armadilha_registros SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND armadilha_id = $2 AND deleted_at IS NULL;

-- name: ListArmadilhaRegistros :many
SELECT r.id, r.ovos, r.nivel_agua, r.registrado_em, r.anotacao, COALESCE(u.nome, '')::text AS por
FROM armadilha_registros r LEFT JOIN usuarios u ON u.id = r.por_id
WHERE r.armadilha_id = $1 AND r.deleted_at IS NULL ORDER BY r.registrado_em DESC, r.id DESC;

-- ---- exclusão lógica

-- name: SoftDeleteArmadilha :execrows
UPDATE armadilhas SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreArmadilha :execrows
UPDATE armadilhas SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteArmadilhaRegistros :exec
DELETE FROM armadilha_registros WHERE armadilha_id = $1;

-- name: ForceDeleteArmadilha :execrows
DELETE FROM armadilhas WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListArmadilhasExcluidas :many
SELECT a.id, (l.endereco || ', ' || l.numero)::text AS endereco, a.deleted_at, COALESCE(u.nome, '')::text AS por
FROM armadilhas a JOIN locais l ON l.id = a.local_id LEFT JOIN usuarios u ON u.id = a.deleted_by
WHERE a.deleted_at IS NOT NULL AND a.tipo = $1 ORDER BY a.deleted_at DESC LIMIT 500;

-- ---- vínculos do local

-- name: CountArmadilhasAtivasDoLocal :one
SELECT count(*)::int FROM armadilhas WHERE local_id = $1 AND deleted_at IS NULL;

-- name: CountArmadilhasDoLocalQualquer :one
SELECT count(*)::int FROM armadilhas WHERE local_id = $1;

-- name: ListArmadilhasDoLocal :many
SELECT id, tipo, status, instalada_em FROM armadilhas WHERE local_id = $1 AND deleted_at IS NULL ORDER BY id DESC LIMIT 20;

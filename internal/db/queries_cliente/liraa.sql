-- ---------------------------------------------------------------- LIRAa (10.5)

-- name: CreateLiraaGrupo :one
INSERT INTO liraa_grupos (nome, ativo) VALUES (sqlc.arg(nome), sqlc.arg(ativo)) RETURNING *;

-- name: GetLiraaGrupo :one
SELECT * FROM liraa_grupos WHERE id = $1 AND deleted_at IS NULL;

-- name: GetLiraaGrupoQualquer :one
SELECT * FROM liraa_grupos WHERE id = $1;

-- name: UpdateLiraaGrupo :execrows
UPDATE liraa_grupos SET nome = sqlc.arg(nome), ativo = sqlc.arg(ativo) WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListLiraaGrupos :many
SELECT g.id, g.nome, g.ativo, (SELECT count(*) FROM liraa_grupo_bairros i WHERE i.grupo_id = g.id)::int AS bairros
FROM liraa_grupos g WHERE g.deleted_at IS NULL ORDER BY lower(g.nome), g.id;

-- name: ListLiraaGrupoBairros :many
SELECT b.id, b.nome FROM liraa_grupo_bairros i JOIN bairros b ON b.id = i.bairro_id WHERE i.grupo_id = $1 ORDER BY lower(b.nome);

-- name: AddLiraaGrupoBairro :exec
INSERT INTO liraa_grupo_bairros (grupo_id, bairro_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: RemoveLiraaGrupoBairro :execrows
DELETE FROM liraa_grupo_bairros WHERE grupo_id = $1 AND bairro_id = $2;

-- name: SoftDeleteLiraaGrupo :execrows
UPDATE liraa_grupos SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreLiraaGrupo :execrows
UPDATE liraa_grupos SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteLiraaGrupo :execrows
DELETE FROM liraa_grupos WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListLiraaGruposExcluidos :many
SELECT g.id, g.nome, g.deleted_at, COALESCE(u.nome, '')::text AS por
FROM liraa_grupos g LEFT JOIN usuarios u ON u.id = g.deleted_by WHERE g.deleted_at IS NOT NULL ORDER BY g.deleted_at DESC LIMIT 500;

-- ---- agenda

-- name: CreateLiraaAgenda :one
INSERT INTO liraa_agenda (ciclo_id, semana, bairro_id, quarteirao_id, usuario_id, situacao, criado_por)
VALUES (sqlc.arg(ciclo_id), sqlc.arg(semana), sqlc.arg(bairro_id), sqlc.arg(quarteirao_id), sqlc.narg(usuario_id), sqlc.arg(situacao), sqlc.narg(criado_por))
RETURNING *;

-- name: GetLiraaAgenda :one
SELECT * FROM liraa_agenda WHERE id = $1 AND deleted_at IS NULL;

-- name: GetLiraaAgendaQualquer :one
SELECT * FROM liraa_agenda WHERE id = $1;

-- name: UpdateLiraaAgenda :execrows
UPDATE liraa_agenda SET ciclo_id = sqlc.arg(ciclo_id), semana = sqlc.arg(semana), bairro_id = sqlc.arg(bairro_id), quarteirao_id = sqlc.arg(quarteirao_id),
       usuario_id = sqlc.narg(usuario_id), situacao = sqlc.arg(situacao), atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountLiraaAgenda :one
SELECT count(*)::int FROM liraa_agenda a
WHERE a.deleted_at IS NULL
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR a.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(semana)::int IS NULL OR a.semana = sqlc.narg(semana)::int)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR a.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR a.usuario_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.arg(situacao)::text = '' OR a.situacao = sqlc.arg(situacao)::text);

-- name: ListLiraaAgenda :many
SELECT a.id, a.semana, a.situacao, c.nome AS ciclo, b.nome AS bairro, q.nome AS quarteirao, COALESCE(u.nome, '')::text AS agente
FROM liraa_agenda a
JOIN ciclos c ON c.id = a.ciclo_id JOIN bairros b ON b.id = a.bairro_id JOIN quarteiroes q ON q.id = a.quarteirao_id
LEFT JOIN usuarios u ON u.id = a.usuario_id
WHERE a.deleted_at IS NULL
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR a.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(semana)::int IS NULL OR a.semana = sqlc.narg(semana)::int)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR a.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR a.usuario_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.arg(situacao)::text = '' OR a.situacao = sqlc.arg(situacao)::text)
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN a.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN a.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'semana' AND sqlc.arg(dir)::text = 'asc' THEN a.semana END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'semana' AND sqlc.arg(dir)::text = 'desc' THEN a.semana END DESC,
  a.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: SoftDeleteLiraaAgenda :execrows
UPDATE liraa_agenda SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreLiraaAgenda :execrows
UPDATE liraa_agenda SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteLiraaAgenda :execrows
DELETE FROM liraa_agenda WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListLiraaAgendaExcluidas :many
SELECT a.id, (b.nome || ' · quarteirão ' || q.nome || ' · semana ' || a.semana)::text AS contexto, a.deleted_at, COALESCE(u.nome, '')::text AS por
FROM liraa_agenda a JOIN bairros b ON b.id = a.bairro_id JOIN quarteiroes q ON q.id = a.quarteirao_id LEFT JOIN usuarios u ON u.id = a.deleted_by
WHERE a.deleted_at IS NOT NULL ORDER BY a.deleted_at DESC LIMIT 500;

-- name: CountLiraaAgendaDoBairroQualquer :one
SELECT count(*)::int FROM liraa_agenda WHERE bairro_id = $1;

-- name: CountLiraaAgendaDoCicloQualquer :one
SELECT count(*)::int FROM liraa_agenda WHERE ciclo_id = $1;

-- name: CountLiraaGruposDoBairro :one
SELECT count(*)::int FROM liraa_grupo_bairros i WHERE i.bairro_id = $1;

-- name: CountLiraaAgendaDoQuarteiraoQualquer :one
SELECT count(*)::int FROM liraa_agenda WHERE quarteirao_id = $1;

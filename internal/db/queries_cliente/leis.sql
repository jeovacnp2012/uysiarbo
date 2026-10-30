-- ---------------------------------------------------------------- Leis municipais

-- name: ListLeis :many
SELECT l.*, (SELECT count(*) FROM leis_trechos t WHERE t.lei_id = l.id AND t.deleted_at IS NULL)::int AS total_trechos
FROM leis_municipais l
WHERE l.deleted_at IS NULL
ORDER BY l.ano DESC, lower(l.numero);

-- name: GetLei :one
SELECT * FROM leis_municipais WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateLei :one
INSERT INTO leis_municipais (numero, ano, nome, publicacao, ativo, ementa, preambulo, link_download) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: UpdateLei :one
UPDATE leis_municipais SET numero = $2, ano = $3, nome = $4, publicacao = $5, ativo = $6, ementa = $7, preambulo = $8, link_download = $9, atualizado_em = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- ---------------------------------------------------------------- Trechos

-- name: ListTrechosDaLei :many
SELECT * FROM leis_trechos WHERE lei_id = $1 AND deleted_at IS NULL ORDER BY id;

-- name: GetTrecho :one
SELECT * FROM leis_trechos WHERE id = $1 AND lei_id = $2 AND deleted_at IS NULL;

-- name: CreateTrecho :one
INSERT INTO leis_trechos (lei_id, artigo, texto) VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateTrecho :one
UPDATE leis_trechos SET artigo = $3, texto = $4, atualizado_em = now()
WHERE id = $1 AND lei_id = $2 AND deleted_at IS NULL
RETURNING *;

-- ---------------------------------------------------------------- Exclusão lógica

-- name: SoftDeleteLei :execrows
UPDATE leis_municipais SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now() WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: RestoreLei :execrows
UPDATE leis_municipais SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteLei :execrows
DELETE FROM leis_municipais WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: GetLeiQualquer :one
SELECT * FROM leis_municipais WHERE id = $1;

-- name: ContarTrechosDaLei :one
SELECT count(*) FILTER (WHERE deleted_at IS NULL)::int AS ativos, count(*)::int AS total FROM leis_trechos WHERE lei_id = $1;

-- name: ListLeisExcluidas :many
SELECT l.id, ('Lei ' || l.numero || '/' || l.ano)::text AS rotulo, l.nome AS contexto, l.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM leis_municipais l LEFT JOIN usuarios u ON u.id = l.deleted_by
WHERE l.deleted_at IS NOT NULL ORDER BY l.deleted_at DESC LIMIT 500;

-- name: SoftDeleteTrecho :execrows
UPDATE leis_trechos SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now() WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: RestoreTrecho :execrows
UPDATE leis_trechos SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteTrecho :execrows
DELETE FROM leis_trechos WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: GetTrechoQualquer :one
SELECT * FROM leis_trechos WHERE id = $1;

-- name: ListTrechosExcluidos :many
SELECT t.id, (CASE WHEN t.artigo <> '' THEN t.artigo ELSE left(t.texto, 40) END)::text AS rotulo,
       ('Lei ' || l.numero || '/' || l.ano)::text AS contexto, t.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM leis_trechos t JOIN leis_municipais l ON l.id = t.lei_id LEFT JOIN usuarios u ON u.id = t.deleted_by
WHERE t.deleted_at IS NOT NULL ORDER BY t.deleted_at DESC LIMIT 500;

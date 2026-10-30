-- ---------------------------------------------------------------- Ficha de caso (9.1)

-- name: CreateCasoEvento :one
INSERT INTO caso_eventos (tipo, caso_id, evento, texto, situacao_de, situacao_para, destino_id, usuario_id)
VALUES (sqlc.arg(tipo), sqlc.arg(caso_id), sqlc.arg(evento), sqlc.arg(texto), sqlc.narg(situacao_de), sqlc.narg(situacao_para),
        sqlc.narg(destino_id), sqlc.narg(usuario_id))
RETURNING id;

-- name: ListCasoEventos :many
SELECT e.id, e.evento, e.texto, e.situacao_de, e.situacao_para, e.criado_em,
       COALESCE(u.nome, '') AS autor, COALESCE(d.nome, '') AS destino
FROM caso_eventos e
LEFT JOIN usuarios u ON u.id = e.usuario_id
LEFT JOIN usuarios d ON d.id = e.destino_id
WHERE e.tipo = $1 AND e.caso_id = $2
ORDER BY e.id DESC;

-- name: ListCasoAnexos :many
SELECT a.id, a.nome, a.tipo_midia, length(a.conteudo)::int AS tamanho, a.criado_em, COALESCE(u.nome, '') AS autor
FROM caso_anexos a
LEFT JOIN usuarios u ON u.id = a.criado_por
WHERE a.tipo = $1 AND a.caso_id = $2 AND a.deleted_at IS NULL
ORDER BY a.id;

-- name: CountCasoAnexos :one
SELECT count(*)::int FROM caso_anexos WHERE tipo = $1 AND caso_id = $2 AND deleted_at IS NULL;

-- name: CreateCasoAnexo :one
INSERT INTO caso_anexos (tipo, caso_id, nome, tipo_midia, conteudo, criado_por)
VALUES ($1, $2, $3, $4, $5, sqlc.narg(criado_por))
RETURNING id;

-- name: GetCasoAnexo :one
SELECT id, nome, tipo_midia, conteudo FROM caso_anexos WHERE id = $1 AND tipo = $2 AND caso_id = $3 AND deleted_at IS NULL;

-- name: SoftDeleteCasoAnexo :execrows
UPDATE caso_anexos SET deleted_at = now(), deleted_by = sqlc.narg(por)
WHERE id = $1 AND tipo = $2 AND caso_id = $3 AND deleted_at IS NULL;

-- Quem está com o caso agora: o destino do último encaminhamento.
-- name: GetCasoResponsavel :one
SELECT COALESCE(d.nome, '') AS nome, e.criado_em
FROM caso_eventos e
JOIN usuarios d ON d.id = e.destino_id
WHERE e.tipo = $1 AND e.caso_id = $2 AND e.evento = 'encaminhamento'
ORDER BY e.id DESC LIMIT 1;

-- name: ForceDeleteCasoEventos :exec
DELETE FROM caso_eventos WHERE tipo = $1 AND caso_id = $2;

-- name: ForceDeleteCasoAnexos :exec
DELETE FROM caso_anexos WHERE tipo = $1 AND caso_id = $2;

-- ---------------------------------------------------------------- Patrimônio (11.3)

-- name: CreatePatrimonio :one
INSERT INTO patrimonios (lote_id, numero_patrimonio, numero_serie, nome, responsavel_id, data_entrada, observacao)
VALUES (sqlc.narg(lote_id), sqlc.arg(numero_patrimonio), sqlc.arg(numero_serie), sqlc.arg(nome), sqlc.narg(responsavel_id), sqlc.narg(data_entrada), sqlc.arg(observacao))
RETURNING *;

-- name: GetPatrimonio :one
SELECT * FROM patrimonios WHERE id = $1 AND deleted_at IS NULL;

-- name: GetPatrimonioParaAtualizar :one
SELECT * FROM patrimonios WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: GetPatrimonioQualquer :one
SELECT * FROM patrimonios WHERE id = $1;

-- name: UpdatePatrimonioDados :execrows
UPDATE patrimonios SET lote_id = sqlc.narg(lote_id), numero_patrimonio = sqlc.arg(numero_patrimonio), numero_serie = sqlc.arg(numero_serie),
       nome = sqlc.arg(nome), data_entrada = sqlc.narg(data_entrada), observacao = sqlc.arg(observacao)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SetPatrimonioResponsavel :exec
UPDATE patrimonios SET responsavel_id = sqlc.narg(responsavel_id) WHERE id = sqlc.arg(id);

-- name: SetPatrimonioStatus :exec
UPDATE patrimonios SET status = sqlc.arg(status) WHERE id = sqlc.arg(id);

-- name: CountPatrimonios :one
SELECT count(*)::int FROM patrimonios p
WHERE p.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(p.numero_patrimonio || ' ' || p.nome || ' ' || p.numero_serie)) > 0)
  AND (sqlc.arg(status)::text = '' OR p.status = sqlc.arg(status)::text)
  AND (sqlc.narg(responsavel_id)::bigint IS NULL OR p.responsavel_id = sqlc.narg(responsavel_id)::bigint);

-- name: ListPatrimonios :many
SELECT p.id, p.numero_patrimonio, p.numero_serie, p.nome, p.status, p.data_entrada, COALESCE(u.nome, '')::text AS responsavel
FROM patrimonios p LEFT JOIN usuarios u ON u.id = p.responsavel_id
WHERE p.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(p.numero_patrimonio || ' ' || p.nome || ' ' || p.numero_serie)) > 0)
  AND (sqlc.arg(status)::text = '' OR p.status = sqlc.arg(status)::text)
  AND (sqlc.narg(responsavel_id)::bigint IS NULL OR p.responsavel_id = sqlc.narg(responsavel_id)::bigint)
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN p.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN p.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'nome' AND sqlc.arg(dir)::text = 'desc' THEN lower(p.nome) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'nome' AND sqlc.arg(dir)::text = 'asc' THEN lower(p.nome) END ASC,
  p.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: SoftDeletePatrimonio :execrows
UPDATE patrimonios SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestorePatrimonio :execrows
UPDATE patrimonios SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeletePatrimonio :execrows
DELETE FROM patrimonios WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListPatrimoniosExcluidos :many
SELECT p.id, (p.numero_patrimonio || ' — ' || p.nome)::text AS nome, p.deleted_at, COALESCE(u.nome, '')::text AS por
FROM patrimonios p LEFT JOIN usuarios u ON u.id = p.deleted_by WHERE p.deleted_at IS NOT NULL ORDER BY p.deleted_at DESC LIMIT 500;

-- name: CreatePatrimonioEvento :exec
INSERT INTO patrimonio_eventos (patrimonio_id, tipo, descricao, usuario_id)
VALUES (sqlc.arg(patrimonio_id), sqlc.arg(tipo), sqlc.arg(descricao), sqlc.narg(usuario_id));

-- name: ListPatrimonioEventos :many
SELECT e.id, e.tipo, e.descricao, e.criado_em, COALESCE(u.nome, '')::text AS por
FROM patrimonio_eventos e LEFT JOIN usuarios u ON u.id = e.usuario_id WHERE e.patrimonio_id = $1 ORDER BY e.id DESC LIMIT 300;

-- name: ListEstoqueLotesParaPatrimonio :many
SELECT l.id, (p.nome || ' — lote ' || l.numero_lote)::text AS rotulo
FROM estoque_lotes l JOIN estoque_produtos p ON p.id = l.produto_id
WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL AND p.tipo = 'equipamento' ORDER BY lower(p.nome), l.id;

-- name: CountPatrimoniosDoLote :one
SELECT count(*)::int FROM patrimonios WHERE lote_id = $1;

-- ---------------------------------------------------------------- API: território (leitura e id_externo)

-- name: ListBairrosApi :many
SELECT * FROM bairros
WHERE id > sqlc.arg(depois)::bigint
  AND (sqlc.narg(cidade_id)::bigint IS NULL OR cidade_id = sqlc.narg(cidade_id)::bigint)
  AND (sqlc.arg(status)::text = '' OR status = sqlc.arg(status)::text)
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(nome)) > 0)
  AND (sqlc.arg(id_externo)::text = '' OR id_externo = sqlc.arg(id_externo)::text)
  AND (sqlc.narg(atualizado_desde)::timestamptz IS NULL OR atualizado_em >= sqlc.narg(atualizado_desde)::timestamptz)
ORDER BY id LIMIT sqlc.arg(limite)::int;

-- name: ListQuarteiroesApi :many
SELECT * FROM quarteiroes
WHERE id > sqlc.arg(depois)::bigint
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(ativo)::boolean IS NULL OR ativo = sqlc.narg(ativo)::boolean)
  AND (sqlc.arg(id_externo)::text = '' OR id_externo = sqlc.arg(id_externo)::text)
  AND (sqlc.narg(atualizado_desde)::timestamptz IS NULL OR atualizado_em >= sqlc.narg(atualizado_desde)::timestamptz)
ORDER BY id LIMIT sqlc.arg(limite)::int;

-- name: ListLocaisApi :many
SELECT * FROM locais
WHERE id > sqlc.arg(depois)::bigint
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(quarteirao_id)::bigint IS NULL OR quarteirao_id = sqlc.narg(quarteirao_id)::bigint)
  AND (sqlc.arg(tipo)::text = '' OR tipo = sqlc.arg(tipo)::text)
  AND (sqlc.narg(ativo)::boolean IS NULL OR ativo = sqlc.narg(ativo)::boolean)
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(endereco || ' ' || numero)) > 0)
  AND (sqlc.arg(id_externo)::text = '' OR id_externo = sqlc.arg(id_externo)::text)
  AND (sqlc.narg(atualizado_desde)::timestamptz IS NULL OR atualizado_em >= sqlc.narg(atualizado_desde)::timestamptz)
ORDER BY id LIMIT sqlc.arg(limite)::int;

-- name: GetBairroPorIdExterno :one
SELECT * FROM bairros WHERE id_externo = $1;

-- name: GetQuarteiraoPorIdExterno :one
SELECT * FROM quarteiroes WHERE id_externo = $1;

-- name: GetLocalPorIdExterno :one
SELECT * FROM locais WHERE id_externo = $1;

-- name: SetBairroIdExterno :one
UPDATE bairros SET id_externo = $2 WHERE id = $1 RETURNING *;

-- name: SetQuarteiraoIdExterno :one
UPDATE quarteiroes SET id_externo = $2 WHERE id = $1 RETURNING *;

-- name: SetLocalIdExterno :one
UPDATE locais SET id_externo = $2 WHERE id = $1 RETURNING *;

-- name: CreateQuarteiraoApi :one
INSERT INTO quarteiroes (bairro_id, nome, ativo, id_externo) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateQuarteirao :one
UPDATE quarteiroes SET nome = $2, ativo = $3, atualizado_em = now() WHERE id = $1 RETURNING *;

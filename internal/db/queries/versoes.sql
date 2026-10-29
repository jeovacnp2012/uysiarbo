-- name: ListVersoes :many
SELECT * FROM versoes ORDER BY id DESC;

-- name: ListVersaoItens :many
SELECT * FROM versao_itens ORDER BY versao_id, ordem, id;

-- name: GetUltimaVersao :one
SELECT * FROM versoes ORDER BY id DESC LIMIT 1;

-- name: GetVersaoByNumero :one
SELECT * FROM versoes WHERE versao = $1;

-- name: CreateVersao :one
INSERT INTO versoes (versao, titulo, publicada_em, criado_por) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: AddVersaoItem :exec
INSERT INTO versao_itens (versao_id, tipo, texto, ordem) VALUES ($1, $2, $3, $4);

-- name: SetPlataformaNovidadeVista :exec
UPDATE plataforma_usuarios SET novidade_vista_id = GREATEST(novidade_vista_id, $2) WHERE id = $1;

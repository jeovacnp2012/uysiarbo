
-- ---------------------------------------------------------------- Semanas epidemiológicas por ano

-- name: ListSemanasAnos :many
SELECT * FROM semanas_anos ORDER BY ano DESC;

-- name: GetSemanaAno :one
SELECT * FROM semanas_anos WHERE ano = $1;

-- name: CreateSemanaAno :one
INSERT INTO semanas_anos (ano, primeiro_dia, ultimo_dia, semanas) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateSemanaAno :one
UPDATE semanas_anos SET primeiro_dia = $2, ultimo_dia = $3, semanas = $4, atualizado_em = now() WHERE ano = $1 RETURNING *;

-- name: DeleteSemanaAno :execrows
DELETE FROM semanas_anos WHERE ano = $1;

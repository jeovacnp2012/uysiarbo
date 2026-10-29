-- name: CreateCliente :one
INSERT INTO clientes (nome, slug, schema_nome, subdominio, dominio, plano, cidade, uf, fuso)
VALUES (sqlc.arg(nome)::text, sqlc.arg(slug)::text, 'c_' || sqlc.arg(slug)::text, sqlc.arg(subdominio)::text,
        sqlc.narg(dominio)::text, sqlc.arg(plano)::text, sqlc.arg(cidade)::text, sqlc.arg(uf)::text,
        COALESCE(NULLIF(sqlc.arg(fuso)::text, ''), 'America/Sao_Paulo'))
RETURNING *;

-- name: GetClienteByID :one
SELECT * FROM clientes WHERE id = $1;

-- name: GetClienteBySubdominio :one
SELECT * FROM clientes WHERE subdominio = $1;

-- name: GetClienteByDominio :one
SELECT * FROM clientes WHERE dominio = $1;

-- name: ListClientes :many
SELECT * FROM clientes ORDER BY nome;

-- name: UpdateClienteStatus :one
UPDATE clientes SET status = $2, atualizado_em = now() WHERE id = $1
RETURNING *;

-- name: GetClienteBySlug :one
SELECT * FROM clientes WHERE slug = $1;

-- name: UpdateClienteDados :one
UPDATE clientes SET nome = $2, cidade = $3, uf = $4, fuso = $5, plano = $6, atualizado_em = now()
WHERE id = $1
RETURNING *;

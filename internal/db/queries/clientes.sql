-- name: CreateCliente :one
INSERT INTO clientes (nome, slug, schema_nome, subdominio, dominio, plano)
VALUES ($1, $2, 'c_' || $2, $3, $4, $5)
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

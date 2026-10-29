-- name: CreateUsuario :one
INSERT INTO usuarios (nome, email, telefone, senha_hash, papel_id, login_web, login_tablet, deve_trocar_senha)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetUsuarioByEmail :one
SELECT * FROM usuarios WHERE lower(email) = lower($1);

-- name: ListUsuarios :many
SELECT * FROM usuarios ORDER BY nome;

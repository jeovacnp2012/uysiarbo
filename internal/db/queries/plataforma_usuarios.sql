-- name: CreatePlataformaUsuario :one
INSERT INTO plataforma_usuarios (nome, email, senha_hash, master, deve_trocar_senha)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPlataformaUsuarioByEmail :one
SELECT * FROM plataforma_usuarios WHERE lower(email) = lower($1);

-- name: ListPlataformaUsuarios :many
SELECT * FROM plataforma_usuarios ORDER BY nome;

-- name: SetPlataformaUsuarioSenha :one
UPDATE plataforma_usuarios
SET senha_hash = $2, deve_trocar_senha = $3, atualizado_em = now()
WHERE id = $1
RETURNING *;

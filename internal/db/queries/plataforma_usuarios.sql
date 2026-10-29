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
SET senha_hash = $2, deve_trocar_senha = $3, tentativas_falhas = 0, bloqueado_ate = NULL, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: GetPlataformaUsuarioByID :one
SELECT * FROM plataforma_usuarios WHERE id = $1;

-- name: RegistrarFalhaPlataforma :exec
UPDATE plataforma_usuarios
SET tentativas_falhas = tentativas_falhas + 1,
    bloqueado_ate = CASE WHEN tentativas_falhas + 1 >= sqlc.arg(limite)::int
                         THEN now() + make_interval(secs => sqlc.arg(bloqueio_segundos)::int)
                         ELSE bloqueado_ate END
WHERE id = sqlc.arg(id);

-- name: RegistrarSucessoPlataforma :exec
UPDATE plataforma_usuarios
SET tentativas_falhas = 0, bloqueado_ate = NULL, ultimo_acesso = now()
WHERE id = $1;

-- name: CreateSessaoPlataforma :exec
INSERT INTO sessoes_plataforma (token_hash, usuario_id, expira_em, ip, user_agent)
VALUES ($1, $2, $3, $4, $5);

-- name: GetSessaoPlataforma :one
SELECT * FROM sessoes_plataforma WHERE token_hash = $1;

-- name: TouchSessaoPlataforma :exec
UPDATE sessoes_plataforma SET ultimo_uso = now(), expira_em = $2 WHERE token_hash = $1;

-- name: DeleteSessaoPlataforma :exec
DELETE FROM sessoes_plataforma WHERE token_hash = $1;

-- name: DeleteSessoesDoUsuarioPlataforma :exec
DELETE FROM sessoes_plataforma WHERE usuario_id = $1 AND token_hash IS DISTINCT FROM $2;

-- name: DeleteSessoesPlataformaExpiradas :exec
DELETE FROM sessoes_plataforma WHERE expira_em < now();

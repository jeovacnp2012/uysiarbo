-- name: CreateUsuario :one
INSERT INTO usuarios (nome, email, telefone, senha_hash, papel_id, login_web, login_tablet, deve_trocar_senha)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetUsuarioByEmail :one
SELECT * FROM usuarios WHERE lower(email) = lower($1);

-- name: ListUsuarios :many
SELECT * FROM usuarios ORDER BY nome;

-- name: GetUsuarioByID :one
SELECT * FROM usuarios WHERE id = $1;

-- name: SetUsuarioSenha :exec
UPDATE usuarios
SET senha_hash = $2, deve_trocar_senha = $3, tentativas_falhas = 0, bloqueado_ate = NULL, atualizado_em = now()
WHERE id = $1;

-- name: RegistrarFalhaUsuario :exec
UPDATE usuarios
SET tentativas_falhas = tentativas_falhas + 1,
    bloqueado_ate = CASE WHEN tentativas_falhas + 1 >= sqlc.arg(limite)::int
                         THEN now() + make_interval(secs => sqlc.arg(bloqueio_segundos)::int)
                         ELSE bloqueado_ate END
WHERE id = sqlc.arg(id);

-- name: RegistrarSucessoUsuario :exec
UPDATE usuarios
SET tentativas_falhas = 0, bloqueado_ate = NULL, ultimo_acesso = now()
WHERE id = $1;

-- name: CreateSessaoUsuario :exec
INSERT INTO sessoes (token_hash, usuario_id, expira_em, ip, user_agent) VALUES ($1, $2, $3, $4, $5);

-- name: CreateSessaoMaster :exec
INSERT INTO sessoes (token_hash, master_id, expira_em, ip, user_agent) VALUES ($1, $2, $3, $4, $5);

-- name: GetSessao :one
SELECT * FROM sessoes WHERE token_hash = $1;

-- name: TouchSessao :exec
UPDATE sessoes SET ultimo_uso = now(), expira_em = $2 WHERE token_hash = $1;

-- name: DeleteSessao :exec
DELETE FROM sessoes WHERE token_hash = $1;

-- name: DeleteSessoesDoUsuario :exec
DELETE FROM sessoes WHERE usuario_id = $1 AND token_hash IS DISTINCT FROM $2;

-- name: DeleteSessoesDoMaster :exec
DELETE FROM sessoes WHERE master_id = $1 AND token_hash IS DISTINCT FROM $2;

-- name: DeleteSessoesExpiradas :exec
DELETE FROM sessoes WHERE expira_em < now();

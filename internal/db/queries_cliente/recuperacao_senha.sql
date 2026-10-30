-- name: CreateRecuperacaoSenha :exec
INSERT INTO recuperacoes_senha (token_hash, usuario_id, expira_em, ip)
VALUES ($1, $2, $3, $4);

-- name: CountRecuperacoesRecentes :one
SELECT count(*) FROM recuperacoes_senha WHERE usuario_id = $1 AND criado_em > $2;

-- name: GetRecuperacaoSenhaParaUsar :one
SELECT * FROM recuperacoes_senha WHERE token_hash = $1 FOR UPDATE;

-- name: UsarRecuperacaoSenha :exec
UPDATE recuperacoes_senha SET usado_em = now() WHERE token_hash = $1;

-- name: InvalidarRecuperacoesDoUsuario :exec
UPDATE recuperacoes_senha SET usado_em = now() WHERE usuario_id = $1 AND usado_em IS NULL;

-- name: DeleteRecuperacoesAntigas :exec
DELETE FROM recuperacoes_senha WHERE expira_em < now() - interval '7 days';

-- name: GetAppInfo :one
SELECT chave, valor, criado_em FROM app_info WHERE chave = $1;

-- name: SetAppInfo :exec
INSERT INTO app_info (chave, valor) VALUES ($1, $2)
ON CONFLICT (chave) DO UPDATE SET valor = EXCLUDED.valor;

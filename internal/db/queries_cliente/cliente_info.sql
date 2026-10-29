-- name: GetClienteInfo :one
SELECT chave, valor, criado_em FROM cliente_info WHERE chave = $1;

-- name: SetClienteInfo :exec
INSERT INTO cliente_info (chave, valor) VALUES ($1, $2)
ON CONFLICT (chave) DO UPDATE SET valor = EXCLUDED.valor;

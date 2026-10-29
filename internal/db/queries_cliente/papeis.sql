-- name: CreatePapel :one
INSERT INTO papeis (nome, descricao) VALUES ($1, $2) RETURNING *;

-- name: GetPapelByNome :one
SELECT * FROM papeis WHERE lower(nome) = lower($1);

-- name: ListPapeis :many
SELECT * FROM papeis ORDER BY nome;

-- name: AddPermissoesAoPapel :exec
INSERT INTO papel_permissoes (papel_id, permissao)
SELECT sqlc.arg(papel_id)::bigint, unnest(sqlc.arg(permissoes)::text[])
ON CONFLICT DO NOTHING;

-- name: ClearPermissoesDoPapel :exec
DELETE FROM papel_permissoes WHERE papel_id = $1;

-- name: ListPermissoesDoPapel :many
SELECT permissao FROM papel_permissoes WHERE papel_id = $1 ORDER BY permissao;

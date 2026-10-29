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

-- name: ListPapeisResumo :many
SELECT p.id, p.nome, p.descricao, p.ativo,
       (SELECT count(*) FROM papel_permissoes pp WHERE pp.papel_id = p.id)::int AS total_permissoes,
       (SELECT count(*) FROM usuarios u WHERE u.papel_id = p.id)::int AS total_usuarios
FROM papeis p
ORDER BY lower(p.nome);

-- name: GetPapel :one
SELECT * FROM papeis WHERE id = $1;

-- name: UpdatePapel :one
UPDATE papeis SET nome = $2, descricao = $3, ativo = $4, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: RemovePermissoesDoPapel :exec
DELETE FROM papel_permissoes WHERE papel_id = sqlc.arg(papel_id) AND permissao = ANY(sqlc.arg(permissoes)::text[]);

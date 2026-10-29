-- ---------------------------------------------------------------- Cidades

-- name: ListCidades :many
SELECT c.*, (SELECT count(*) FROM bairros b WHERE b.cidade_id = c.id)::int AS total_bairros
FROM cidades c
ORDER BY lower(c.nome), c.uf;

-- name: ListCidadesAtivas :many
SELECT * FROM cidades WHERE ativo ORDER BY lower(nome), uf;

-- name: GetCidade :one
SELECT * FROM cidades WHERE id = $1;

-- name: CreateCidade :one
INSERT INTO cidades (nome, uf, ibge) VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateCidade :one
UPDATE cidades SET nome = $2, uf = $3, ibge = $4, ativo = $5, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- ---------------------------------------------------------------- Bairros

-- name: ListBairros :many
SELECT b.id, b.cidade_id, b.nome, b.mutirao, b.status, b.categoria, b.classificacao,
       c.nome AS cidade_nome, c.uf AS cidade_uf,
       (SELECT count(*) FROM quarteiroes q WHERE q.bairro_id = b.id)::int AS total_quarteiroes,
       COALESCE((SELECT string_agg(u.nome, ', ' ORDER BY u.nome)
                 FROM bairro_responsaveis r JOIN usuarios u ON u.id = r.usuario_id
                 WHERE r.bairro_id = b.id), '')::text AS responsaveis
FROM bairros b
JOIN cidades c ON c.id = b.cidade_id
WHERE (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(b.nome)) > 0)
  AND (sqlc.narg(cidade_id)::bigint IS NULL OR b.cidade_id = sqlc.narg(cidade_id)::bigint)
  AND (sqlc.arg(status)::text = '' OR b.status = sqlc.arg(status)::text)
  AND (sqlc.arg(categoria)::text = '' OR b.categoria = sqlc.arg(categoria)::text)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR EXISTS (
        SELECT 1 FROM bairro_responsaveis r WHERE r.bairro_id = b.id AND r.usuario_id = sqlc.narg(somente_usuario)::bigint))
ORDER BY lower(b.nome)
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: CountBairros :one
SELECT count(*)::int
FROM bairros b
WHERE (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(b.nome)) > 0)
  AND (sqlc.narg(cidade_id)::bigint IS NULL OR b.cidade_id = sqlc.narg(cidade_id)::bigint)
  AND (sqlc.arg(status)::text = '' OR b.status = sqlc.arg(status)::text)
  AND (sqlc.arg(categoria)::text = '' OR b.categoria = sqlc.arg(categoria)::text)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR EXISTS (
        SELECT 1 FROM bairro_responsaveis r WHERE r.bairro_id = b.id AND r.usuario_id = sqlc.narg(somente_usuario)::bigint));

-- name: GetBairro :one
SELECT * FROM bairros WHERE id = $1;

-- name: CreateBairro :one
INSERT INTO bairros (cidade_id, nome, mutirao, status, ibge, classificacao, zona, categoria,
                     latitude, longitude, altitude, distancia_centro, producao)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: UpdateBairro :one
UPDATE bairros
SET cidade_id = $2, nome = $3, mutirao = $4, status = $5, ibge = $6, classificacao = $7, zona = $8,
    categoria = $9, latitude = $10, longitude = $11, altitude = $12, distancia_centro = $13,
    producao = $14, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: ListResponsaveisDoBairro :many
SELECT usuario_id FROM bairro_responsaveis WHERE bairro_id = $1 ORDER BY usuario_id;

-- name: ClearResponsaveisDoBairro :exec
DELETE FROM bairro_responsaveis WHERE bairro_id = $1;

-- name: AddResponsaveisAoBairro :exec
INSERT INTO bairro_responsaveis (bairro_id, usuario_id)
SELECT sqlc.arg(bairro_id)::bigint, unnest(sqlc.arg(usuarios)::bigint[])
ON CONFLICT DO NOTHING;

-- name: ListUsuariosAtivosParaResponsavel :many
SELECT id, nome, email FROM usuarios WHERE ativo ORDER BY lower(nome);

-- ------------------------------------------------------------ Quarteirões

-- name: ListQuarteiroesDoBairro :many
SELECT * FROM quarteiroes WHERE bairro_id = $1
ORDER BY NULLIF(regexp_replace(nome, '\D', '', 'g'), '')::bigint NULLS LAST, lower(nome);

-- name: CreateQuarteirao :execrows
INSERT INTO quarteiroes (bairro_id, nome) VALUES ($1, $2)
ON CONFLICT (bairro_id, lower(nome)) DO NOTHING;

-- name: GetQuarteirao :one
SELECT * FROM quarteiroes WHERE id = $1;

-- name: SetQuarteiraoAtivo :execrows
UPDATE quarteiroes SET ativo = $3 WHERE id = $1 AND bairro_id = $2;

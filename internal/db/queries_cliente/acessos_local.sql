-- ---------------------------------------------------------------- Controle de acesso aos locais

-- name: ListAcessosLocal :many
SELECT a.id, a.usuario_id, u.nome AS usuario_nome, a.tipo, a.referencia_id, a.descricao, a.criado_em,
       CASE a.tipo
         WHEN 'bairro' THEN COALESCE((SELECT b.nome FROM bairros b WHERE b.id = a.referencia_id), '(removido)')
         WHEN 'quarteirao' THEN COALESCE((SELECT 'Quarteirão ' || q.nome || ' · ' || b.nome FROM quarteiroes q JOIN bairros b ON b.id = q.bairro_id WHERE q.id = a.referencia_id), '(removido)')
         ELSE COALESCE((SELECT l.endereco || ', ' || l.numero || ' · ' || b.nome FROM locais l JOIN bairros b ON b.id = l.bairro_id WHERE l.id = a.referencia_id), '(removido)')
       END::text AS referencia_nome
FROM acessos_local a JOIN usuarios u ON u.id = a.usuario_id
WHERE (sqlc.narg(usuario_id)::bigint IS NULL OR a.usuario_id = sqlc.narg(usuario_id)::bigint)
ORDER BY lower(u.nome), a.tipo, a.id
LIMIT 500;

-- name: GetAcessoLocal :one
SELECT * FROM acessos_local WHERE id = $1;

-- name: CreateAcessoLocal :execrows
INSERT INTO acessos_local (usuario_id, tipo, referencia_id, descricao, criado_por)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING;

-- name: DeleteAcessoLocal :execrows
DELETE FROM acessos_local WHERE id = $1;

-- name: ListAcessosDoLocal :many
SELECT a.id, a.usuario_id, u.nome AS usuario_nome, a.tipo, a.criado_em
FROM acessos_local a JOIN usuarios u ON u.id = a.usuario_id
WHERE (a.tipo = 'local' AND a.referencia_id = sqlc.arg(local)::bigint)
   OR (a.tipo = 'quarteirao' AND a.referencia_id = sqlc.arg(quarteirao)::bigint)
   OR (a.tipo = 'bairro' AND a.referencia_id = sqlc.arg(bairro)::bigint)
ORDER BY lower(u.nome), a.tipo;

-- ---------------------------------------------------------------- Leishmaniose visceral canina (10.4)

-- name: CreateLvc :one
INSERT INTO lvc_testes (local_id, data, nome, raca, pelagem, idade, sexo, proprietario, proprietario_telefone, proprietario_cpf, observacao,
                        dpp, pedido_reteste, data_pedido, data_pedido_recebido, resultado_reteste, situacao_cachorro, data_morte, criado_por)
VALUES (sqlc.arg(local_id), sqlc.arg(data), sqlc.arg(nome), sqlc.arg(raca), sqlc.arg(pelagem), sqlc.arg(idade), sqlc.arg(sexo), sqlc.arg(proprietario),
        sqlc.arg(proprietario_telefone), sqlc.arg(proprietario_cpf), sqlc.arg(observacao), sqlc.arg(dpp), sqlc.arg(pedido_reteste),
        sqlc.narg(data_pedido), sqlc.narg(data_pedido_recebido), sqlc.arg(resultado_reteste), sqlc.arg(situacao_cachorro), sqlc.narg(data_morte), sqlc.narg(criado_por))
RETURNING *;

-- name: GetLvc :one
SELECT * FROM lvc_testes WHERE id = $1 AND deleted_at IS NULL;

-- name: GetLvcQualquer :one
SELECT * FROM lvc_testes WHERE id = $1;

-- name: UpdateLvc :execrows
UPDATE lvc_testes SET data = sqlc.arg(data), nome = sqlc.arg(nome), raca = sqlc.arg(raca), pelagem = sqlc.arg(pelagem), idade = sqlc.arg(idade),
       sexo = sqlc.arg(sexo), proprietario = sqlc.arg(proprietario), proprietario_telefone = sqlc.arg(proprietario_telefone),
       proprietario_cpf = sqlc.arg(proprietario_cpf), observacao = sqlc.arg(observacao), dpp = sqlc.arg(dpp), pedido_reteste = sqlc.arg(pedido_reteste),
       data_pedido = sqlc.narg(data_pedido), data_pedido_recebido = sqlc.narg(data_pedido_recebido), resultado_reteste = sqlc.arg(resultado_reteste),
       situacao_cachorro = sqlc.arg(situacao_cachorro), data_morte = sqlc.narg(data_morte), atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountLvc :one
SELECT count(*)::int
FROM lvc_testes t JOIN locais l ON l.id = t.local_id
WHERE t.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR t.id::text = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(t.nome || ' ' || t.proprietario || ' ' || l.endereco || ' ' || l.numero)) > 0)
  AND (sqlc.arg(dpp)::text = '' OR t.dpp = sqlc.arg(dpp)::text)
  AND (sqlc.arg(situacao)::text = '' OR t.situacao_cachorro = sqlc.arg(situacao)::text)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR t.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR t.data <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id));

-- name: ListLvc :many
SELECT t.id, t.data, t.nome, t.proprietario, t.dpp, t.pedido_reteste, t.resultado_reteste, t.situacao_cachorro,
       (l.endereco || ', ' || l.numero)::text AS endereco, COALESCE(u.nome, '')::text AS por
FROM lvc_testes t JOIN locais l ON l.id = t.local_id LEFT JOIN usuarios u ON u.id = t.criado_por
WHERE t.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR t.id::text = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(t.nome || ' ' || t.proprietario || ' ' || l.endereco || ' ' || l.numero)) > 0)
  AND (sqlc.arg(dpp)::text = '' OR t.dpp = sqlc.arg(dpp)::text)
  AND (sqlc.arg(situacao)::text = '' OR t.situacao_cachorro = sqlc.arg(situacao)::text)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR t.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR t.data <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id))
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN t.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN t.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN t.data END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN t.data END DESC,
  t.data DESC, t.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- ---- exclusão lógica

-- name: SoftDeleteLvc :execrows
UPDATE lvc_testes SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreLvc :execrows
UPDATE lvc_testes SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteLvc :execrows
DELETE FROM lvc_testes WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListLvcExcluidos :many
SELECT t.id, (l.endereco || ', ' || l.numero)::text AS contexto, t.deleted_at, COALESCE(u.nome, '')::text AS por
FROM lvc_testes t JOIN locais l ON l.id = t.local_id LEFT JOIN usuarios u ON u.id = t.deleted_by
WHERE t.deleted_at IS NOT NULL ORDER BY t.deleted_at DESC LIMIT 500;

-- ---- vínculos do local

-- name: CountLvcAtivosDoLocal :one
SELECT count(*)::int FROM lvc_testes WHERE local_id = $1 AND deleted_at IS NULL;

-- name: CountLvcDoLocalQualquer :one
SELECT count(*)::int FROM lvc_testes WHERE local_id = $1;

-- name: ListLvcDaFichaDoLocal :many
SELECT id, data, nome, dpp, resultado_reteste, situacao_cachorro, pedido_reteste
FROM lvc_testes WHERE local_id = $1 AND deleted_at IS NULL ORDER BY data DESC, id DESC LIMIT 20;

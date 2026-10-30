-- ---------------------------------------------------------------- Retiradas e balanço (11.2)

-- name: CreateEstoqueRetirada :one
INSERT INTO estoque_retiradas (tipo, usuario_id, data_saida, data_retorno_prevista, observacao, criado_por)
VALUES (sqlc.arg(tipo), sqlc.arg(usuario_id), sqlc.arg(data_saida), sqlc.narg(data_retorno_prevista), sqlc.arg(observacao), sqlc.narg(criado_por))
RETURNING *;

-- name: GetEstoqueRetirada :one
SELECT * FROM estoque_retiradas WHERE id = $1 AND deleted_at IS NULL;

-- name: GetEstoqueRetiradaParaAtualizar :one
SELECT * FROM estoque_retiradas WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: GetEstoqueRetiradaQualquer :one
SELECT * FROM estoque_retiradas WHERE id = $1;

-- name: UpdateEstoqueRetiradaDados :execrows
UPDATE estoque_retiradas SET usuario_id = sqlc.arg(usuario_id), data_retorno_prevista = sqlc.narg(data_retorno_prevista), observacao = sqlc.arg(observacao)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND status = 'aberta';

-- name: SetEstoqueRetiradaStatus :exec
UPDATE estoque_retiradas SET status = sqlc.arg(status), data_retorno_real = sqlc.narg(data_retorno_real) WHERE id = sqlc.arg(id);

-- name: CountEstoqueRetiradas :one
SELECT count(*)::int FROM estoque_retiradas r
WHERE r.deleted_at IS NULL
  AND (sqlc.arg(status)::text = '' OR r.status = sqlc.arg(status)::text)
  AND (sqlc.arg(tipo)::text = '' OR r.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.narg(usuario_id)::bigint IS NULL OR r.usuario_id = sqlc.narg(usuario_id)::bigint);

-- name: ListEstoqueRetiradas :many
SELECT r.id, r.tipo, r.status, r.data_saida, r.data_retorno_prevista, r.data_retorno_real, u.nome AS responsavel,
       (SELECT count(*) FROM estoque_retirada_itens i WHERE i.retirada_id = r.id)::int AS itens
FROM estoque_retiradas r JOIN usuarios u ON u.id = r.usuario_id
WHERE r.deleted_at IS NULL
  AND (sqlc.arg(status)::text = '' OR r.status = sqlc.arg(status)::text)
  AND (sqlc.arg(tipo)::text = '' OR r.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.narg(usuario_id)::bigint IS NULL OR r.usuario_id = sqlc.narg(usuario_id)::bigint)
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN r.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'saida' AND sqlc.arg(dir)::text = 'asc' THEN r.data_saida END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'saida' AND sqlc.arg(dir)::text = 'desc' THEN r.data_saida END DESC,
  r.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: SoftDeleteEstoqueRetirada :execrows
UPDATE estoque_retiradas SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreEstoqueRetirada :execrows
UPDATE estoque_retiradas SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteEstoqueRetirada :execrows
DELETE FROM estoque_retiradas WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListEstoqueRetiradasExcluidas :many
SELECT r.id, (u.nome || ' — ' || r.tipo)::text AS contexto, r.deleted_at, COALESCE(x.nome, '')::text AS por
FROM estoque_retiradas r JOIN usuarios u ON u.id = r.usuario_id LEFT JOIN usuarios x ON x.id = r.deleted_by
WHERE r.deleted_at IS NOT NULL ORDER BY r.deleted_at DESC LIMIT 500;

-- name: ListEstoqueRetiradaItens :many
SELECT i.id, i.lote_id, i.retirado, i.usado, i.devolvido, i.perda, l.numero_lote, p.nome AS produto, p.unidade_medida
FROM estoque_retirada_itens i JOIN estoque_lotes l ON l.id = i.lote_id JOIN estoque_produtos p ON p.id = l.produto_id
WHERE i.retirada_id = $1 ORDER BY i.id;

-- name: GetEstoqueRetiradaItem :one
SELECT * FROM estoque_retirada_itens WHERE id = $1 AND retirada_id = $2;

-- name: CreateEstoqueRetiradaItem :one
INSERT INTO estoque_retirada_itens (retirada_id, lote_id, retirado) VALUES ($1, $2, $3) RETURNING *;

-- name: DeleteEstoqueRetiradaItem :exec
DELETE FROM estoque_retirada_itens WHERE id = $1;

-- name: SetEstoqueRetiradaItemFinal :exec
UPDATE estoque_retirada_itens SET usado = sqlc.arg(usado), devolvido = sqlc.arg(devolvido), perda = sqlc.arg(perda) WHERE id = sqlc.arg(id);

-- name: ListEstoqueLotesDisponiveis :many
SELECT l.id, (p.nome || ' — lote ' || l.numero_lote || ' (saldo ' || trim(trailing '.' from trim(trailing '0' from l.saldo::text)) || ' ' || p.unidade_medida || ')')::text AS rotulo
FROM estoque_lotes l JOIN estoque_produtos p ON p.id = l.produto_id
WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL AND p.ativo AND l.saldo > 0
  AND (sqlc.arg(tipo)::text = 'campo' AND p.saida_permitida IN ('campo', 'campo_permanente')
       OR sqlc.arg(tipo)::text = 'permanente' AND p.saida_permitida IN ('permanente', 'campo_permanente'))
ORDER BY lower(p.nome), l.data_validade NULLS LAST, l.id;

-- name: CountEstoqueItensDoLote :one
SELECT (count(*) FILTER (WHERE r.status = 'aberta' AND r.deleted_at IS NULL))::int AS abertos, count(*)::int AS total
FROM estoque_retirada_itens i JOIN estoque_retiradas r ON r.id = i.retirada_id WHERE i.lote_id = $1;

-- name: CountEstoqueRetiradasDoUsuarioAbertas :one
SELECT count(*)::int FROM estoque_retiradas WHERE usuario_id = $1 AND status = 'aberta' AND deleted_at IS NULL;

-- name: CountEstoqueBalanco :one
SELECT count(*)::int FROM estoque_produtos p
WHERE p.deleted_at IS NULL
  AND (sqlc.narg(unidade_id)::bigint IS NULL OR p.unidade_id = sqlc.narg(unidade_id)::bigint)
  AND (sqlc.arg(tipo)::text = '' OR p.tipo = sqlc.arg(tipo)::text);

-- name: ListEstoqueBalanco :many
SELECT p.id, p.nome, p.tipo, p.unidade_medida, u.nome AS unidade,
       COALESCE((SELECT sum(l.saldo) FROM estoque_lotes l WHERE l.produto_id = p.id AND l.deleted_at IS NULL), 0)::numeric AS em_estoque,
       COALESCE((SELECT sum(i.retirado - i.usado - i.devolvido - i.perda)
                 FROM estoque_retirada_itens i JOIN estoque_lotes l ON l.id = i.lote_id JOIN estoque_retiradas r ON r.id = i.retirada_id
                 WHERE l.produto_id = p.id AND r.status = 'aberta' AND r.deleted_at IS NULL), 0)::numeric AS em_retirada,
       COALESCE((SELECT sum(l.saldo) FROM estoque_lotes l WHERE l.produto_id = p.id AND l.deleted_at IS NULL AND l.data_validade < sqlc.arg(hoje)::date), 0)::numeric AS vencido,
       (SELECT min(l.data_validade) FROM estoque_lotes l WHERE l.produto_id = p.id AND l.deleted_at IS NULL AND l.saldo > 0 AND l.data_validade >= sqlc.arg(hoje)::date)::date AS proximo_vencimento
FROM estoque_produtos p JOIN estoque_unidades u ON u.id = p.unidade_id
WHERE p.deleted_at IS NULL
  AND (sqlc.narg(unidade_id)::bigint IS NULL OR p.unidade_id = sqlc.narg(unidade_id)::bigint)
  AND (sqlc.arg(tipo)::text = '' OR p.tipo = sqlc.arg(tipo)::text)
ORDER BY lower(u.nome), lower(p.nome), p.id
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

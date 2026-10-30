-- ---------------------------------------------------------------- Estoque (11.1)

-- ---- unidades

-- name: CreateEstoqueUnidade :one
INSERT INTO estoque_unidades (nome, descricao, ativo) VALUES (sqlc.arg(nome), sqlc.arg(descricao), sqlc.arg(ativo)) RETURNING *;

-- name: GetEstoqueUnidade :one
SELECT * FROM estoque_unidades WHERE id = $1 AND deleted_at IS NULL;

-- name: GetEstoqueUnidadeQualquer :one
SELECT * FROM estoque_unidades WHERE id = $1;

-- name: UpdateEstoqueUnidade :execrows
UPDATE estoque_unidades SET nome = sqlc.arg(nome), descricao = sqlc.arg(descricao), ativo = sqlc.arg(ativo) WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListEstoqueUnidades :many
SELECT u.id, u.nome, u.descricao, u.ativo, (SELECT count(*) FROM estoque_produtos p WHERE p.unidade_id = u.id AND p.deleted_at IS NULL)::int AS produtos
FROM estoque_unidades u WHERE u.deleted_at IS NULL ORDER BY lower(u.nome), u.id;

-- name: ListEstoqueUnidadesAtivas :many
SELECT id, nome FROM estoque_unidades WHERE deleted_at IS NULL AND ativo ORDER BY lower(nome);

-- name: SoftDeleteEstoqueUnidade :execrows
UPDATE estoque_unidades SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreEstoqueUnidade :execrows
UPDATE estoque_unidades SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteEstoqueUnidade :execrows
DELETE FROM estoque_unidades WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListEstoqueUnidadesExcluidas :many
SELECT x.id, x.nome, x.deleted_at, COALESCE(u.nome, '')::text AS por
FROM estoque_unidades x LEFT JOIN usuarios u ON u.id = x.deleted_by WHERE x.deleted_at IS NOT NULL ORDER BY x.deleted_at DESC LIMIT 500;

-- name: CountEstoqueProdutosDaUnidade :one
SELECT (count(*) FILTER (WHERE deleted_at IS NULL))::int AS ativos, count(*)::int AS total FROM estoque_produtos WHERE unidade_id = $1;

-- ---- produtos

-- name: CreateEstoqueProduto :one
INSERT INTO estoque_produtos (unidade_id, nome, tipo, unidade_medida, saida_permitida, fracionavel, doses_por_unidade, unidade_fracao, descricao,
                              selecionavel_diaria, categoria_diaria, ativo)
VALUES (sqlc.arg(unidade_id), sqlc.arg(nome), sqlc.arg(tipo), sqlc.arg(unidade_medida), sqlc.arg(saida_permitida), sqlc.arg(fracionavel),
        sqlc.narg(doses_por_unidade), sqlc.narg(unidade_fracao), sqlc.arg(descricao), sqlc.arg(selecionavel_diaria), sqlc.narg(categoria_diaria), sqlc.arg(ativo))
RETURNING *;

-- name: GetEstoqueProduto :one
SELECT * FROM estoque_produtos WHERE id = $1 AND deleted_at IS NULL;

-- name: GetEstoqueProdutoQualquer :one
SELECT * FROM estoque_produtos WHERE id = $1;

-- name: UpdateEstoqueProduto :execrows
UPDATE estoque_produtos SET unidade_id = sqlc.arg(unidade_id), nome = sqlc.arg(nome), tipo = sqlc.arg(tipo), unidade_medida = sqlc.arg(unidade_medida),
       saida_permitida = sqlc.arg(saida_permitida), fracionavel = sqlc.arg(fracionavel), doses_por_unidade = sqlc.narg(doses_por_unidade),
       unidade_fracao = sqlc.narg(unidade_fracao), descricao = sqlc.arg(descricao), selecionavel_diaria = sqlc.arg(selecionavel_diaria),
       categoria_diaria = sqlc.narg(categoria_diaria), ativo = sqlc.arg(ativo)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountEstoqueProdutos :one
SELECT count(*)::int FROM estoque_produtos p
WHERE p.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(p.nome)) > 0)
  AND (sqlc.arg(tipo)::text = '' OR p.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.narg(unidade_id)::bigint IS NULL OR p.unidade_id = sqlc.narg(unidade_id)::bigint)
  AND (sqlc.arg(situacao)::text = '' OR (sqlc.arg(situacao)::text = 'ativo') = p.ativo);

-- name: ListEstoqueProdutos :many
SELECT p.id, p.nome, p.tipo, p.unidade_medida, p.fracionavel, p.ativo, u.nome AS unidade,
       COALESCE((SELECT sum(l.saldo) FROM estoque_lotes l WHERE l.produto_id = p.id AND l.deleted_at IS NULL), 0)::numeric AS saldo
FROM estoque_produtos p JOIN estoque_unidades u ON u.id = p.unidade_id
WHERE p.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(p.nome)) > 0)
  AND (sqlc.arg(tipo)::text = '' OR p.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.narg(unidade_id)::bigint IS NULL OR p.unidade_id = sqlc.narg(unidade_id)::bigint)
  AND (sqlc.arg(situacao)::text = '' OR (sqlc.arg(situacao)::text = 'ativo') = p.ativo)
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'nome' AND sqlc.arg(dir)::text = 'desc' THEN lower(p.nome) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN p.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN p.id END DESC,
  lower(p.nome), p.id
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: ListEstoqueProdutosAtivos :many
SELECT p.id, (p.nome || ' (' || p.unidade_medida || ')')::text AS nome FROM estoque_produtos p WHERE p.deleted_at IS NULL AND p.ativo ORDER BY lower(p.nome);

-- name: SoftDeleteEstoqueProduto :execrows
UPDATE estoque_produtos SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreEstoqueProduto :execrows
UPDATE estoque_produtos SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteEstoqueProduto :execrows
DELETE FROM estoque_produtos WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListEstoqueProdutosExcluidos :many
SELECT x.id, x.nome, x.deleted_at, COALESCE(u.nome, '')::text AS por
FROM estoque_produtos x LEFT JOIN usuarios u ON u.id = x.deleted_by WHERE x.deleted_at IS NOT NULL ORDER BY x.deleted_at DESC LIMIT 500;

-- name: CountEstoqueLotesDoProduto :one
SELECT (count(*) FILTER (WHERE deleted_at IS NULL))::int AS ativos, count(*)::int AS total FROM estoque_lotes WHERE produto_id = $1;

-- ---- lotes

-- name: CreateEstoqueLote :one
INSERT INTO estoque_lotes (produto_id, numero_lote, fabricante, data_fabricacao, data_validade, quantidade, saldo, custo_unitario, nota_fiscal, fornecedor, observacao)
VALUES (sqlc.arg(produto_id), sqlc.arg(numero_lote), sqlc.arg(fabricante), sqlc.narg(data_fabricacao), sqlc.narg(data_validade), sqlc.arg(quantidade),
        sqlc.arg(quantidade), sqlc.narg(custo_unitario), sqlc.arg(nota_fiscal), sqlc.arg(fornecedor), sqlc.arg(observacao))
RETURNING *;

-- name: GetEstoqueLote :one
SELECT * FROM estoque_lotes WHERE id = $1 AND deleted_at IS NULL;

-- name: GetEstoqueLoteParaAjuste :one
SELECT * FROM estoque_lotes WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: GetEstoqueLoteQualquer :one
SELECT * FROM estoque_lotes WHERE id = $1;

-- name: UpdateEstoqueLote :execrows
UPDATE estoque_lotes SET fabricante = sqlc.arg(fabricante), data_fabricacao = sqlc.narg(data_fabricacao), data_validade = sqlc.narg(data_validade),
       custo_unitario = sqlc.narg(custo_unitario), nota_fiscal = sqlc.arg(nota_fiscal), fornecedor = sqlc.arg(fornecedor), observacao = sqlc.arg(observacao)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SetEstoqueLoteSaldo :exec
UPDATE estoque_lotes SET saldo = sqlc.arg(saldo) WHERE id = sqlc.arg(id);

-- name: CountEstoqueLotes :one
SELECT count(*)::int FROM estoque_lotes l JOIN estoque_produtos p ON p.id = l.produto_id
WHERE l.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(l.numero_lote || ' ' || p.nome || ' ' || l.fornecedor || ' ' || l.nota_fiscal)) > 0)
  AND (sqlc.narg(produto_id)::bigint IS NULL OR l.produto_id = sqlc.narg(produto_id)::bigint)
  AND (sqlc.arg(situacao)::text = ''
       OR (sqlc.arg(situacao)::text = 'com_saldo' AND l.saldo > 0)
       OR (sqlc.arg(situacao)::text = 'zerado' AND l.saldo = 0)
       OR (sqlc.arg(situacao)::text = 'vencido' AND l.data_validade < sqlc.arg(hoje)::date)
       OR (sqlc.arg(situacao)::text = 'a_vencer' AND l.data_validade >= sqlc.arg(hoje)::date AND l.data_validade <= sqlc.arg(hoje)::date + 30));

-- name: ListEstoqueLotes :many
SELECT l.id, l.numero_lote, l.data_validade, l.quantidade, l.saldo, l.fornecedor, p.nome AS produto, p.unidade_medida
FROM estoque_lotes l JOIN estoque_produtos p ON p.id = l.produto_id
WHERE l.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(l.numero_lote || ' ' || p.nome || ' ' || l.fornecedor || ' ' || l.nota_fiscal)) > 0)
  AND (sqlc.narg(produto_id)::bigint IS NULL OR l.produto_id = sqlc.narg(produto_id)::bigint)
  AND (sqlc.arg(situacao)::text = ''
       OR (sqlc.arg(situacao)::text = 'com_saldo' AND l.saldo > 0)
       OR (sqlc.arg(situacao)::text = 'zerado' AND l.saldo = 0)
       OR (sqlc.arg(situacao)::text = 'vencido' AND l.data_validade < sqlc.arg(hoje)::date)
       OR (sqlc.arg(situacao)::text = 'a_vencer' AND l.data_validade >= sqlc.arg(hoje)::date AND l.data_validade <= sqlc.arg(hoje)::date + 30))
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN l.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN l.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'validade' AND sqlc.arg(dir)::text = 'asc' THEN l.data_validade END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'validade' AND sqlc.arg(dir)::text = 'desc' THEN l.data_validade END DESC,
  l.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: SoftDeleteEstoqueLote :execrows
UPDATE estoque_lotes SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreEstoqueLote :execrows
UPDATE estoque_lotes SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteEstoqueLote :execrows
DELETE FROM estoque_lotes WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListEstoqueLotesExcluidos :many
SELECT l.id, (p.nome || ' — lote ' || l.numero_lote)::text AS contexto, l.deleted_at, COALESCE(u.nome, '')::text AS por
FROM estoque_lotes l JOIN estoque_produtos p ON p.id = l.produto_id LEFT JOIN usuarios u ON u.id = l.deleted_by
WHERE l.deleted_at IS NOT NULL ORDER BY l.deleted_at DESC LIMIT 500;

-- ---- movimentações

-- name: CreateEstoqueMovimentacao :exec
INSERT INTO estoque_movimentacoes (lote_id, tipo, quantidade, saldo_apos, usuario_id, descricao, origem)
VALUES (sqlc.arg(lote_id), sqlc.arg(tipo), sqlc.arg(quantidade), sqlc.arg(saldo_apos), sqlc.narg(usuario_id), sqlc.arg(descricao), sqlc.arg(origem));

-- name: ListEstoqueMovimentacoes :many
SELECT m.id, m.tipo, m.quantidade, m.saldo_apos, m.descricao, m.origem, m.criado_em, COALESCE(u.nome, '')::text AS por
FROM estoque_movimentacoes m LEFT JOIN usuarios u ON u.id = m.usuario_id WHERE m.lote_id = $1 ORDER BY m.id DESC LIMIT 200;

-- name: SumEstoqueSaldoDoProduto :one
SELECT COALESCE(sum(saldo), 0)::numeric FROM estoque_lotes WHERE produto_id = $1 AND deleted_at IS NULL;

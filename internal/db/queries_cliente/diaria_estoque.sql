-- ---------------------------------------------------------------- Diária x estoque (11.4)

-- name: SetDiariaLotes :execrows
UPDATE campanha_diarias SET vacina_lote_id = sqlc.narg(vacina_lote_id), seringa_lote_id = sqlc.narg(seringa_lote_id), atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListEstoqueLotesDaDiaria :many
SELECT l.id, (p.nome || ' — lote ' || l.numero_lote || ' (saldo ' || trim(trailing '.' from trim(trailing '0' from l.saldo::text)) || ' ' || p.unidade_medida || ')')::text AS rotulo
FROM estoque_lotes l JOIN estoque_produtos p ON p.id = l.produto_id
WHERE l.deleted_at IS NULL AND p.deleted_at IS NULL AND p.ativo AND p.selecionavel_diaria AND p.categoria_diaria = sqlc.arg(categoria)::text
  AND (l.saldo > 0 OR l.id = sqlc.narg(atual)::bigint)
ORDER BY lower(p.nome), l.data_validade NULLS LAST, l.id;

-- name: GetEstoqueLoteDaCategoria :one
SELECT l.* FROM estoque_lotes l JOIN estoque_produtos p ON p.id = l.produto_id
WHERE l.id = sqlc.arg(id) AND l.deleted_at IS NULL AND p.ativo AND p.selecionavel_diaria AND p.categoria_diaria = sqlc.arg(categoria)::text;

-- name: CreateDiariaBaixa :exec
INSERT INTO campanha_diaria_baixas (diaria_id, lote_id, categoria, quantidade) VALUES ($1, $2, $3, $4);

-- name: ListDiariaBaixas :many
SELECT b.id, b.lote_id, b.categoria, b.quantidade, l.numero_lote, p.nome AS produto, p.unidade_medida
FROM campanha_diaria_baixas b JOIN estoque_lotes l ON l.id = b.lote_id JOIN estoque_produtos p ON p.id = l.produto_id
WHERE b.diaria_id = $1 ORDER BY b.id;

-- name: DeleteDiariaBaixas :exec
DELETE FROM campanha_diaria_baixas WHERE diaria_id = $1;

-- name: CountDiariasDoLote :one
SELECT ((SELECT count(*) FROM campanha_diarias d WHERE d.vacina_lote_id = $1 OR d.seringa_lote_id = $1)
      + (SELECT count(*) FROM campanha_diaria_baixas b WHERE b.lote_id = $1))::int;

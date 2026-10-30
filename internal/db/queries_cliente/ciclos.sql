-- ---------------------------------------------------------------- Atividades (catálogo fixo)

-- name: ListAtividades :many
SELECT * FROM atividades ORDER BY id;

-- ---------------------------------------------------------------- Ciclos

-- name: ListCiclos :many
SELECT c.*,
       COALESCE((SELECT string_agg(a.sigla, ', ' ORDER BY a.id)
                 FROM ciclo_atividades ca JOIN atividades a ON a.id = ca.atividade_id
                 WHERE ca.ciclo_id = c.id), '')::text AS atividades
FROM ciclos c
WHERE c.deleted_at IS NULL
ORDER BY c.data_inicio DESC, c.id DESC;

-- name: GetCiclo :one
SELECT * FROM ciclos WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateCiclo :one
INSERT INTO ciclos (nome, data_inicio, data_final, situacao, permite_fora_periodo, painel_publico, painel_publico_texto)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateCiclo :one
UPDATE ciclos
SET nome = $2, data_inicio = $3, data_final = $4, situacao = $5, permite_fora_periodo = $6,
    painel_publico = $7, painel_publico_texto = $8, atualizado_em = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- Ciclos (não excluídos) cujo período cruza o intervalo dado, para avisar de sobreposição.
-- name: ListCiclosQueCruzam :many
SELECT id, nome FROM ciclos
WHERE deleted_at IS NULL AND id <> sqlc.arg(ignorar)::bigint
  AND data_inicio <= sqlc.arg(fim)::date AND data_final >= sqlc.arg(inicio)::date
ORDER BY data_inicio;

-- name: ListCiclosParaSincronizar :many
SELECT id, nome FROM ciclos WHERE deleted_at IS NULL AND id <> sqlc.arg(ignorar)::bigint ORDER BY data_inicio DESC, id DESC;

-- name: ListAtividadesDoCiclo :many
SELECT atividade_id FROM ciclo_atividades WHERE ciclo_id = $1 ORDER BY atividade_id;

-- name: ClearAtividadesDoCiclo :exec
DELETE FROM ciclo_atividades WHERE ciclo_id = $1;

-- name: AddAtividadesAoCiclo :exec
INSERT INTO ciclo_atividades (ciclo_id, atividade_id)
SELECT sqlc.arg(ciclo)::bigint, unnest(sqlc.arg(atividades)::smallint[])
ON CONFLICT DO NOTHING;

-- name: ListAtividadesDoPainel :many
SELECT atividade_id FROM ciclo_painel_atividades WHERE ciclo_id = $1 ORDER BY atividade_id;

-- name: ClearAtividadesDoPainel :exec
DELETE FROM ciclo_painel_atividades WHERE ciclo_id = $1;

-- name: AddAtividadesAoPainel :exec
INSERT INTO ciclo_painel_atividades (ciclo_id, atividade_id)
SELECT sqlc.arg(ciclo)::bigint, unnest(sqlc.arg(atividades)::smallint[])
ON CONFLICT DO NOTHING;

-- name: ListCiclosSincronizados :many
SELECT ciclo_destino_id FROM ciclo_sincronizar WHERE ciclo_id = $1 ORDER BY ciclo_destino_id;

-- name: ClearCiclosSincronizados :exec
DELETE FROM ciclo_sincronizar WHERE ciclo_id = $1;

-- name: AddCiclosSincronizados :exec
INSERT INTO ciclo_sincronizar (ciclo_id, ciclo_destino_id)
SELECT sqlc.arg(ciclo)::bigint, d FROM unnest(sqlc.arg(destinos)::bigint[]) AS d
JOIN ciclos c ON c.id = d AND c.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------- Exclusão lógica

-- name: SoftDeleteCiclo :execrows
UPDATE ciclos SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now() WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: RestoreCiclo :execrows
UPDATE ciclos SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteCiclo :execrows
DELETE FROM ciclos WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: GetCicloQualquer :one
SELECT * FROM ciclos WHERE id = $1;

-- Vínculos que impedem a exclusão definitiva: outros ciclos que sincronizam com este (contando os excluídos).
-- name: ContarVinculosDoCiclo :one
SELECT count(*)::int FROM ciclo_sincronizar WHERE ciclo_destino_id = $1;

-- name: ListCiclosExcluidos :many
SELECT c.id, c.nome AS rotulo, (to_char(c.data_inicio, 'DD/MM/YYYY') || ' a ' || to_char(c.data_final, 'DD/MM/YYYY'))::text AS contexto,
       c.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM ciclos c LEFT JOIN usuarios u ON u.id = c.deleted_by
WHERE c.deleted_at IS NOT NULL ORDER BY c.deleted_at DESC LIMIT 500;

-- ---------------------------------------------------------------- Sincronização do tablet (6.4)

-- name: CreateAmostraSync :one
INSERT INTO amostras (visita_id, numero_tubito, deposito, larvas, pupas, observacao, criado_por, id_externo)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetAmostraPorIdExterno :one
SELECT * FROM amostras WHERE id_externo = $1;

-- name: MarcarVisitaSincronizada :exec
UPDATE visitas SET data_sincronismo = now() WHERE id = $1;

-- name: CreateSincronismo :one
INSERT INTO sincronismos (credencial_id, dispositivo, enviado_em, visitas, criadas, existentes, conflitos, rejeitadas, amostras)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, recebido_em;

-- name: ListVisitasApi :many
SELECT * FROM visitas
WHERE (deleted_at IS NULL OR sqlc.arg(incluir_excluidos)::boolean) AND id > sqlc.arg(depois)::bigint
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(local_id)::bigint IS NULL OR local_id = sqlc.narg(local_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR agente_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.arg(id_externo)::text = '' OR id_externo = sqlc.arg(id_externo)::text)
  AND (sqlc.narg(atualizado_desde)::timestamptz IS NULL OR atualizado_em >= sqlc.narg(atualizado_desde)::timestamptz)
ORDER BY id LIMIT sqlc.arg(limite)::int;

-- name: ListAmostrasDasVisitas :many
SELECT * FROM amostras WHERE visita_id = ANY(sqlc.arg(ids)::bigint[]) AND deleted_at IS NULL ORDER BY id;

-- Serializa a numeração automática dos tubitos até o fim da transação.
-- name: BloquearNumeracaoTubito :exec
SELECT pg_advisory_xact_lock(7001);

-- name: CountSincronismos :one
SELECT count(*)::int FROM sincronismos WHERE dispositivo = $1;

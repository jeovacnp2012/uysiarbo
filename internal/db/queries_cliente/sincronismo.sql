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

-- ---------------------------------------------------------------- Histórico de mobilidade (6.6)

-- name: CountSincronismosFiltrados :one
SELECT count(*)::int FROM sincronismos s
WHERE (sqlc.arg(dispositivo)::text = '' OR position(lower(sqlc.arg(dispositivo)::text) in lower(s.dispositivo)) > 0)
  AND (sqlc.narg(credencial_id)::bigint IS NULL OR s.credencial_id = sqlc.narg(credencial_id)::bigint)
  AND (sqlc.narg(de)::timestamptz IS NULL OR s.recebido_em >= sqlc.narg(de)::timestamptz)
  AND (sqlc.narg(ate)::timestamptz IS NULL OR s.recebido_em < sqlc.narg(ate)::timestamptz)
  AND (sqlc.arg(problemas)::boolean = false OR s.conflitos > 0 OR s.rejeitadas > 0);

-- name: ListSincronismos :many
SELECT s.id, s.dispositivo, s.enviado_em, s.recebido_em, s.visitas, s.criadas, s.existentes, s.conflitos, s.rejeitadas, s.amostras,
       COALESCE(c.nome, '')::text AS credencial_nome
FROM sincronismos s LEFT JOIN api_credenciais c ON c.id = s.credencial_id
WHERE (sqlc.arg(dispositivo)::text = '' OR position(lower(sqlc.arg(dispositivo)::text) in lower(s.dispositivo)) > 0)
  AND (sqlc.narg(credencial_id)::bigint IS NULL OR s.credencial_id = sqlc.narg(credencial_id)::bigint)
  AND (sqlc.narg(de)::timestamptz IS NULL OR s.recebido_em >= sqlc.narg(de)::timestamptz)
  AND (sqlc.narg(ate)::timestamptz IS NULL OR s.recebido_em < sqlc.narg(ate)::timestamptz)
  AND (sqlc.arg(problemas)::boolean = false OR s.conflitos > 0 OR s.rejeitadas > 0)
ORDER BY s.recebido_em DESC, s.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

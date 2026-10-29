-- ---------------------------------------------------------------- API: credenciais

-- name: ListApiCredenciais :many
SELECT c.*,
       (SELECT count(*) FROM api_chamadas x WHERE x.credencial_id = c.id AND x.quando > now() - interval '24 hours')::int AS chamadas_24h
FROM api_credenciais c ORDER BY lower(c.nome);

-- name: GetApiCredencial :one
SELECT * FROM api_credenciais WHERE id = $1;

-- name: GetApiCredencialPorClientID :one
SELECT * FROM api_credenciais WHERE client_id = $1;

-- name: CreateApiCredencial :one
INSERT INTO api_credenciais (nome, descricao, client_id, segredo_hash, escopos, valida_ate, limite_por_minuto, criado_por)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateApiCredencial :one
UPDATE api_credenciais
SET nome = $2, descricao = $3, escopos = $4, valida_ate = $5, limite_por_minuto = $6, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: SetApiCredencialAtivo :execrows
UPDATE api_credenciais SET ativo = $2, atualizado_em = now() WHERE id = $1;

-- name: RotacionarApiCredencial :one
UPDATE api_credenciais
SET segredo_hash = $2, versao_segredo = versao_segredo + 1, rotacionado_em = now(), atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: TocarApiCredencial :exec
UPDATE api_credenciais SET ultimo_uso_em = now() WHERE id = $1;

-- ---------------------------------------------------------------- API: chamadas

-- name: InsertApiChamada :exec
INSERT INTO api_chamadas (credencial_id, metodo, caminho, status, duracao_ms, ip, request_id)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListApiChamadas :many
SELECT * FROM api_chamadas WHERE credencial_id = $1 ORDER BY quando DESC, id DESC LIMIT $2;

-- name: ResumoApiChamadas :one
SELECT count(*)::int AS total,
       count(*) FILTER (WHERE status >= 400)::int AS com_erro,
       COALESCE(round(avg(duracao_ms)), 0)::int AS media_ms
FROM api_chamadas WHERE credencial_id = $1 AND quando > $2;

-- ---------------------------------------------------------------- API: idempotência

-- name: ReservarIdempotencia :execrows
INSERT INTO api_idempotencia (credencial_id, chave, hash_pedido) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: GetIdempotencia :one
SELECT * FROM api_idempotencia WHERE credencial_id = $1 AND chave = $2;

-- name: ConcluirIdempotencia :exec
UPDATE api_idempotencia SET status = $3, resposta = $4 WHERE credencial_id = $1 AND chave = $2;

-- name: LiberarIdempotencia :exec
DELETE FROM api_idempotencia WHERE credencial_id = $1 AND chave = $2 AND status = 0;

-- name: LimparIdempotenciaAntiga :execrows
DELETE FROM api_idempotencia WHERE criado_em < now() - interval '24 hours';

-- ---------------------------------------------------------------- API: cidades (recurso simples da Etapa 2)

-- name: ListCidadesApi :many
SELECT * FROM cidades WHERE id > $1 AND deleted_at IS NULL ORDER BY id LIMIT $2;

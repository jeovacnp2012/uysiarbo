-- ---------------------------------------------------------------- Amostras (tubitos)

-- name: CreateAmostra :one
INSERT INTO amostras (visita_id, numero_tubito, deposito, larvas, pupas, observacao, criado_por)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAmostra :one
SELECT * FROM amostras WHERE id = $1 AND deleted_at IS NULL;

-- name: GetAmostraQualquer :one
SELECT * FROM amostras WHERE id = $1;

-- name: ListAmostrasDaVisita :many
SELECT * FROM amostras WHERE visita_id = $1 AND deleted_at IS NULL ORDER BY id;

-- name: UpdateAmostraColeta :one
UPDATE amostras SET deposito = $2, larvas = $3, pupas = $4, observacao = $5, atualizado_em = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateAmostraResultado :one
UPDATE amostras SET status = $2, total_aegypti = $3, total_albopictus = $4, total_negativo = $5,
    resultado_por = $6, resultado_em = now(), atualizado_em = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- Próximo número automático: maior tubito numérico (inclusive de amostras excluídas) + 1.
-- name: ProximoNumeroTubito :one
SELECT (COALESCE(max(numero_tubito::bigint), 0) + 1)::text AS numero FROM amostras WHERE numero_tubito ~ '^[0-9]{1,15}$';

-- name: SoftDeleteAmostra :execrows
UPDATE amostras SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now()
WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: RestoreAmostra :execrows
UPDATE amostras SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteAmostra :execrows
DELETE FROM amostras WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListAmostrasExcluidas :many
SELECT a.id, ('Tubito ' || a.numero_tubito)::text AS rotulo, ('Visita ' || a.visita_id)::text AS contexto,
       a.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM amostras a LEFT JOIN usuarios u ON u.id = a.deleted_by
WHERE a.deleted_at IS NOT NULL ORDER BY a.deleted_at DESC LIMIT 500;

-- Amostras da visita (contando as excluídas) para o vínculo da exclusão definitiva.
-- name: ContarAmostrasDaVisita :one
SELECT count(*) FILTER (WHERE deleted_at IS NULL)::int AS ativas, count(*)::int AS total FROM amostras WHERE visita_id = $1;

-- ---------------------------------------------------------------- Listagem do laboratório

-- name: CountAmostras :one
SELECT count(*)::int
FROM amostras a
JOIN visitas v ON v.id = a.visita_id
JOIN locais l ON l.id = v.local_id
WHERE a.deleted_at IS NULL AND v.deleted_at IS NULL
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
  AND (sqlc.arg(tubito)::text = '' OR position(lower(sqlc.arg(tubito)::text) in lower(a.numero_tubito)) > 0)
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR v.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR v.data <= sqlc.narg(data_ate)::date)
  AND (sqlc.arg(endereco)::text = '' OR position(lower(sqlc.arg(endereco)::text) in lower(l.endereco)) > 0)
  AND (sqlc.arg(numero)::text = '' OR position(lower(sqlc.arg(numero)::text) in lower(l.numero)) > 0)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(quarteirao_id)::bigint IS NULL OR l.quarteirao_id = sqlc.narg(quarteirao_id)::bigint)
  AND (sqlc.arg(lado)::text = '' OR lower(l.lado) = lower(sqlc.arg(lado)::text))
  AND (sqlc.arg(id_texto)::text = '' OR v.id::text = sqlc.arg(id_texto)::text OR v.id_externo = sqlc.arg(id_texto)::text)
  AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
  AND (sqlc.narg(semana)::int IS NULL OR v.semana = sqlc.narg(semana)::int);

-- name: ListAmostras :many
SELECT a.id, a.visita_id, a.numero_tubito, a.deposito, a.larvas, a.pupas, a.status, a.total_aegypti, a.total_albopictus, a.total_negativo,
       v.data, l.endereco, l.numero, c.nome AS ciclo_nome, u.nome AS agente_nome
FROM amostras a
JOIN visitas v ON v.id = a.visita_id
JOIN locais l ON l.id = v.local_id
JOIN ciclos c ON c.id = v.ciclo_id
JOIN usuarios u ON u.id = v.agente_id
WHERE a.deleted_at IS NULL AND v.deleted_at IS NULL
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
  AND (sqlc.arg(tubito)::text = '' OR position(lower(sqlc.arg(tubito)::text) in lower(a.numero_tubito)) > 0)
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR v.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR v.data <= sqlc.narg(data_ate)::date)
  AND (sqlc.arg(endereco)::text = '' OR position(lower(sqlc.arg(endereco)::text) in lower(l.endereco)) > 0)
  AND (sqlc.arg(numero)::text = '' OR position(lower(sqlc.arg(numero)::text) in lower(l.numero)) > 0)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(quarteirao_id)::bigint IS NULL OR l.quarteirao_id = sqlc.narg(quarteirao_id)::bigint)
  AND (sqlc.arg(lado)::text = '' OR lower(l.lado) = lower(sqlc.arg(lado)::text))
  AND (sqlc.arg(id_texto)::text = '' OR v.id::text = sqlc.arg(id_texto)::text OR v.id_externo = sqlc.arg(id_texto)::text)
  AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
  AND (sqlc.narg(semana)::int IS NULL OR v.semana = sqlc.narg(semana)::int)
ORDER BY v.data DESC, a.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- Selo do menu: tubitos ainda em análise (quem vê só as próprias visitas conta só as suas).
-- name: CountAmostrasEmAnalise :one
SELECT count(*)::int FROM amostras a JOIN visitas v ON v.id = a.visita_id
WHERE a.deleted_at IS NULL AND v.deleted_at IS NULL AND a.status = 'em_analise'
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint);

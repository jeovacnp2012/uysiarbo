-- name: InsertAuditoriaPlataforma :exec
INSERT INTO auditoria_plataforma (ator_tipo, ator_id, ator_nome, ator_email, ip, user_agent, request_id,
    acao, entidade, entidade_id, rotulo, antes, depois, detalhes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: ListAuditoriaPlataforma :many
SELECT * FROM auditoria_plataforma
WHERE (sqlc.narg(de)::timestamptz IS NULL OR quando >= sqlc.narg(de))
  AND (sqlc.narg(ate)::timestamptz IS NULL OR quando < sqlc.narg(ate))
  AND (sqlc.arg(ator)::text = '' OR position(lower(sqlc.arg(ator)::text) in lower(ator_nome || ' ' || ator_email)) > 0)
  AND (sqlc.arg(acao)::text = '' OR acao = sqlc.arg(acao)::text OR acao LIKE sqlc.arg(acao)::text || '.%')
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(rotulo || ' ' || entidade_id)) > 0)
ORDER BY quando DESC, id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: CountAuditoriaPlataforma :one
SELECT count(*)::int FROM auditoria_plataforma
WHERE (sqlc.narg(de)::timestamptz IS NULL OR quando >= sqlc.narg(de))
  AND (sqlc.narg(ate)::timestamptz IS NULL OR quando < sqlc.narg(ate))
  AND (sqlc.arg(ator)::text = '' OR position(lower(sqlc.arg(ator)::text) in lower(ator_nome || ' ' || ator_email)) > 0)
  AND (sqlc.arg(acao)::text = '' OR acao = sqlc.arg(acao)::text OR acao LIKE sqlc.arg(acao)::text || '.%')
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(rotulo || ' ' || entidade_id)) > 0);

-- name: ListAcoesAuditoriaPlataforma :many
SELECT DISTINCT acao FROM auditoria_plataforma ORDER BY acao;

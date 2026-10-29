-- ---------------------------------------------------------------- Exclusão lógica, lixeira e exclusão definitiva

-- name: SoftDeleteCidade :execrows
UPDATE cidades SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now() WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: SoftDeleteBairro :execrows
UPDATE bairros SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now() WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: SoftDeleteQuarteirao :execrows
UPDATE quarteiroes SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now() WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: SoftDeleteLocal :execrows
UPDATE locais SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now() WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: SoftDeleteGrupo :execrows
UPDATE local_grupos SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now() WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: RestoreCidade :execrows
UPDATE cidades SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: RestoreBairro :execrows
UPDATE bairros SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: RestoreQuarteirao :execrows
UPDATE quarteiroes SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: RestoreLocal :execrows
UPDATE locais SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: RestoreGrupo :execrows
UPDATE local_grupos SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- Exclusão definitiva: só do que já está na lixeira (a checagem de vínculos é feita antes, no servidor).
-- name: ForceDeleteCidade :execrows
DELETE FROM cidades WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteBairro :execrows
DELETE FROM bairros WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteQuarteirao :execrows
DELETE FROM quarteiroes WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteLocal :execrows
DELETE FROM locais WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteGrupo :execrows
DELETE FROM local_grupos WHERE id = $1 AND deleted_at IS NOT NULL;

-- ---- Filhos ATIVOS (impedem a exclusão lógica do pai) e vínculos totais (impedem a exclusão definitiva)

-- name: ContarFilhosDaCidade :one
SELECT count(*) FILTER (WHERE deleted_at IS NULL)::int AS ativos, count(*)::int AS total FROM bairros WHERE cidade_id = $1;

-- name: ContarFilhosDoBairro :one
SELECT (SELECT count(*) FROM quarteiroes q1 WHERE q1.bairro_id = sqlc.arg(bairro)::bigint AND q1.deleted_at IS NULL)::int AS quarteiroes_ativos,
       (SELECT count(*) FROM quarteiroes q2 WHERE q2.bairro_id = sqlc.arg(bairro)::bigint)::int AS quarteiroes_total,
       (SELECT count(*) FROM locais l1 WHERE l1.bairro_id = sqlc.arg(bairro)::bigint AND l1.deleted_at IS NULL)::int AS locais_ativos,
       (SELECT count(*) FROM locais l2 WHERE l2.bairro_id = sqlc.arg(bairro)::bigint)::int AS locais_total;

-- name: ContarFilhosDoQuarteirao :one
SELECT count(*) FILTER (WHERE deleted_at IS NULL)::int AS ativos, count(*)::int AS total FROM locais WHERE quarteirao_id = $1;

-- name: ContarVinculosDoLocal :one
SELECT (SELECT count(*) FROM local_grupo_itens gi WHERE gi.local_id = sqlc.arg(local)::bigint)::int AS grupos,
       (SELECT count(*) FROM acessos_local al WHERE al.tipo = 'local' AND al.referencia_id = sqlc.arg(local)::bigint)::int AS acessos;

-- ---- Lixeira (lista dos excluídos de cada tipo, do mais recente para o mais antigo)

-- name: ListCidadesExcluidas :many
SELECT c.id, (c.nome || '/' || c.uf)::text AS rotulo, ''::text AS contexto, c.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM cidades c LEFT JOIN usuarios u ON u.id = c.deleted_by
WHERE c.deleted_at IS NOT NULL ORDER BY c.deleted_at DESC LIMIT 500;

-- name: ListBairrosExcluidos :many
SELECT b.id, b.nome AS rotulo, (c.nome || '/' || c.uf)::text AS contexto, b.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM bairros b JOIN cidades c ON c.id = b.cidade_id LEFT JOIN usuarios u ON u.id = b.deleted_by
WHERE b.deleted_at IS NOT NULL ORDER BY b.deleted_at DESC LIMIT 500;

-- name: ListQuarteiroesExcluidos :many
SELECT q.id, ('Quarteirão ' || q.nome)::text AS rotulo, b.nome AS contexto, q.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM quarteiroes q JOIN bairros b ON b.id = q.bairro_id LEFT JOIN usuarios u ON u.id = q.deleted_by
WHERE q.deleted_at IS NOT NULL ORDER BY q.deleted_at DESC LIMIT 500;

-- name: ListLocaisExcluidos :many
SELECT l.id, (l.endereco || ', ' || l.numero)::text AS rotulo, b.nome AS contexto, l.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM locais l JOIN bairros b ON b.id = l.bairro_id LEFT JOIN usuarios u ON u.id = l.deleted_by
WHERE l.deleted_at IS NOT NULL ORDER BY l.deleted_at DESC LIMIT 500;

-- name: ListGruposExcluidos :many
SELECT g.id, g.nome AS rotulo, ''::text AS contexto, g.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM local_grupos g LEFT JOIN usuarios u ON u.id = g.deleted_by
WHERE g.deleted_at IS NOT NULL ORDER BY g.deleted_at DESC LIMIT 500;

-- ---- Leitura de um registro mesmo excluído (restaurar e excluir definitivamente)

-- name: GetCidadeQualquer :one
SELECT * FROM cidades WHERE id = $1;

-- name: GetBairroQualquer :one
SELECT * FROM bairros WHERE id = $1;

-- name: GetQuarteiraoQualquer :one
SELECT * FROM quarteiroes WHERE id = $1;

-- name: GetLocalQualquer :one
SELECT * FROM locais WHERE id = $1;

-- name: GetGrupoQualquer :one
SELECT * FROM local_grupos WHERE id = $1;

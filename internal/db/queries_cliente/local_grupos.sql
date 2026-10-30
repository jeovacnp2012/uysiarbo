-- ---------------------------------------------------------------- Grupos de Locais
-- Todas as consultas de itens respeitam o escopo do papel (apenas atribuídos, só ponto estratégico,
-- ver desativados): quem tem escopo menor nunca vê, conta, copia nem remove o que está fora dele.

-- name: ListGrupos :many
SELECT g.id, g.nome, g.descricao, g.criado_em, g.principal_id,
       pl.endereco AS principal_endereco, pl.numero AS principal_numero, pl.lado AS principal_lado,
       pl.sequencia AS principal_sequencia, pl.complemento AS principal_complemento,
       pb.nome AS principal_bairro, pq.nome AS principal_quarteirao,
       (SELECT count(*) FROM local_grupo_itens i JOIN locais l ON l.id = i.local_id
         WHERE i.grupo_id = g.id AND l.deleted_at IS NULL
           AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
           AND (NOT sqlc.arg(somente_pe)::boolean OR l.tipo = 'ponto_estrategico')
           AND (sqlc.arg(ver_desativados)::boolean OR l.ativo))::int AS total_locais
FROM local_grupos g
LEFT JOIN locais pl ON pl.id = g.principal_id AND pl.deleted_at IS NULL
LEFT JOIN bairros pb ON pb.id = pl.bairro_id
LEFT JOIN quarteiroes pq ON pq.id = pl.quarteirao_id
WHERE g.deleted_at IS NULL AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(g.nome || ' ' || g.descricao || ' ' || COALESCE(pl.endereco || ' ' || pl.numero, ''))) > 0)
ORDER BY lower(g.nome);

-- name: GetGrupo :one
SELECT * FROM local_grupos WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateGrupo :one
INSERT INTO local_grupos (nome, descricao, criado_por) VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateGrupo :one
UPDATE local_grupos SET nome = $2, descricao = $3, atualizado_em = now() WHERE id = $1 RETURNING *;

-- name: CountLocaisDoGrupo :one
SELECT count(*)::int FROM local_grupo_itens i JOIN locais l ON l.id = i.local_id JOIN bairros b ON b.id = l.bairro_id
WHERE i.grupo_id = sqlc.arg(grupo_id)::bigint AND l.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(l.endereco || ' ' || l.numero || ' ' || l.referencia || ' ' || b.nome)) > 0)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
  AND (NOT sqlc.arg(somente_pe)::boolean OR l.tipo = 'ponto_estrategico')
  AND (sqlc.arg(ver_desativados)::boolean OR l.ativo);

-- name: ListLocaisDoGrupo :many
SELECT l.id, l.tipo, l.endereco, l.numero, l.lado, l.sequencia, l.complemento, l.proprietario, l.ativo, l.latitude, l.longitude,
       l.qr_chave, b.nome AS bairro_nome, q.nome AS quarteirao_nome
FROM local_grupo_itens i
JOIN locais l ON l.id = i.local_id
JOIN bairros b ON b.id = l.bairro_id
JOIN quarteiroes q ON q.id = l.quarteirao_id
WHERE i.grupo_id = sqlc.arg(grupo_id)::bigint AND l.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(l.endereco || ' ' || l.numero || ' ' || l.referencia || ' ' || b.nome)) > 0)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
  AND (NOT sqlc.arg(somente_pe)::boolean OR l.tipo = 'ponto_estrategico')
  AND (sqlc.arg(ver_desativados)::boolean OR l.ativo)
ORDER BY lower(b.nome), lower(l.endereco), l.numero, l.id
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: AddLocaisAoGrupoPorFiltro :execrows
INSERT INTO local_grupo_itens (grupo_id, local_id)
SELECT sqlc.arg(grupo_id)::bigint, l.id FROM locais l
WHERE l.ativo AND l.deleted_at IS NULL
  AND l.bairro_id = sqlc.arg(bairro_id)::bigint
  AND (sqlc.narg(quarteirao_id)::bigint IS NULL OR l.quarteirao_id = sqlc.narg(quarteirao_id)::bigint)
  AND (sqlc.arg(tipo)::text = '' OR l.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
  AND (NOT sqlc.arg(somente_pe)::boolean OR l.tipo = 'ponto_estrategico')
ON CONFLICT DO NOTHING;

-- name: EnsureGrupoPrincipal :exec
-- O principal é sempre um local do grupo: sem principal (ou com o principal retirado), o primeiro local do grupo
-- passa a ser o principal; grupo vazio fica sem principal.
UPDATE local_grupos g SET principal_id = (
    SELECT i.local_id FROM local_grupo_itens i WHERE i.grupo_id = g.id ORDER BY i.adicionado_em, i.local_id LIMIT 1)
WHERE g.id = $1 AND (g.principal_id IS NULL OR NOT EXISTS (
    SELECT 1 FROM local_grupo_itens i WHERE i.grupo_id = g.id AND i.local_id = g.principal_id));

-- name: SetGrupoPrincipal :exec
UPDATE local_grupos SET principal_id = $2, atualizado_em = now() WHERE id = $1;

-- name: AddLocalAoGrupo :execrows
INSERT INTO local_grupo_itens (grupo_id, local_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: RemoveLocalDoGrupo :execrows
DELETE FROM local_grupo_itens WHERE grupo_id = $1 AND local_id = $2;

-- name: ClearLocaisDoGrupo :execrows
DELETE FROM local_grupo_itens i USING locais l
WHERE i.grupo_id = sqlc.arg(grupo_id)::bigint AND l.id = i.local_id
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
  AND (NOT sqlc.arg(somente_pe)::boolean OR l.tipo = 'ponto_estrategico')
  AND (sqlc.arg(ver_desativados)::boolean OR l.ativo);

-- name: CopyLocaisDoGrupo :execrows
INSERT INTO local_grupo_itens (grupo_id, local_id)
SELECT sqlc.arg(destino)::bigint, l.id FROM local_grupo_itens i JOIN locais l ON l.id = i.local_id
WHERE i.grupo_id = sqlc.arg(origem)::bigint AND l.deleted_at IS NULL
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
  AND (NOT sqlc.arg(somente_pe)::boolean OR l.tipo = 'ponto_estrategico')
  AND (sqlc.arg(ver_desativados)::boolean OR l.ativo)
ON CONFLICT DO NOTHING;

-- name: ListGruposDoLocal :many
SELECT g.id, g.nome FROM local_grupos g JOIN local_grupo_itens i ON i.grupo_id = g.id
WHERE i.local_id = $1 AND g.deleted_at IS NULL ORDER BY lower(g.nome);

-- name: ListGruposParaEscolher :many
SELECT id, nome FROM local_grupos WHERE deleted_at IS NULL ORDER BY lower(nome);

-- name: ListAuditoriaDaEntidade :many
SELECT * FROM auditoria WHERE entidade = $1 AND entidade_id = $2 ORDER BY quando DESC, id DESC LIMIT $3;

-- name: NomeDeGrupoExiste :one
SELECT EXISTS (SELECT 1 FROM local_grupos WHERE lower(nome) = lower($1) AND deleted_at IS NULL);

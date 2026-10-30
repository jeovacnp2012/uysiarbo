-- ---------------------------------------------------------------- Entrevistas e doenças (9.2)

-- name: ListEntrevistaGrupos :many
SELECT g.*, (SELECT count(*)::int FROM doencas d WHERE d.grupo_id = g.id) AS total_doencas
FROM entrevista_grupos g ORDER BY lower(g.titulo);

-- name: GetEntrevistaGrupo :one
SELECT * FROM entrevista_grupos WHERE id = $1;

-- name: CreateEntrevistaGrupo :one
INSERT INTO entrevista_grupos (codigo, titulo, ativo, campos) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateEntrevistaGrupo :one
UPDATE entrevista_grupos SET titulo = $2, ativo = $3, campos = $4, atualizado_em = now() WHERE id = $1 RETURNING *;

-- name: ListDoencas :many
SELECT d.*, COALESCE(g.titulo, '') AS grupo_titulo
FROM doencas d LEFT JOIN entrevista_grupos g ON g.id = d.grupo_id
ORDER BY lower(d.nome);

-- name: GetDoenca :one
SELECT * FROM doencas WHERE id = $1;

-- name: CreateDoenca :one
INSERT INTO doencas (nome, grupo_id, ativo) VALUES ($1, sqlc.narg(grupo_id), $2) RETURNING *;

-- name: UpdateDoenca :one
UPDATE doencas SET nome = $2, grupo_id = sqlc.narg(grupo_id), ativo = $3, atualizado_em = now() WHERE id = $1 RETURNING *;

-- ---------------------------------------------------------------- Investigações

-- name: CreateInvestigacao :one
INSERT INTO investigacoes (id_externo, doenca_id, local_id, endereco_texto, paciente, telefone, data_inicio, observacao, grupo_id, criado_por)
VALUES (sqlc.narg(id_externo), sqlc.arg(doenca_id), sqlc.narg(local_id), sqlc.arg(endereco_texto), sqlc.arg(paciente), sqlc.arg(telefone),
        sqlc.narg(data_inicio), sqlc.arg(observacao), sqlc.narg(grupo_id), sqlc.narg(criado_por))
RETURNING *;

-- name: GetInvestigacao :one
SELECT * FROM investigacoes WHERE id = $1 AND deleted_at IS NULL;

-- name: GetInvestigacaoQualquer :one
SELECT * FROM investigacoes WHERE id = $1;

-- name: UpdateInvestigacaoDados :one
UPDATE investigacoes SET doenca_id = $2, endereco_texto = $3, paciente = $4, telefone = $5, data_inicio = sqlc.narg(data_inicio),
       observacao = $6, grupo_id = sqlc.narg(grupo_id), atualizado_em = now()
WHERE id = $1 AND deleted_at IS NULL RETURNING *;

-- name: SetInvestigacaoLocal :execrows
UPDATE investigacoes SET local_id = sqlc.narg(local_id), atualizado_em = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: SetInvestigacaoEntrevista :execrows
UPDATE investigacoes SET entrevista = $2, grupo_id = sqlc.narg(grupo_id), atualizado_em = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: SetInvestigacaoSituacao :execrows
UPDATE investigacoes SET situacao = sqlc.arg(situacao)::text, resultado = sqlc.arg(resultado)::text, responsavel_id = sqlc.narg(responsavel_id),
       encerrada_em = CASE WHEN sqlc.arg(situacao)::text = 'encerrada' THEN now() ELSE NULL END, atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountInvestigacoes :one
SELECT count(*)::int
FROM investigacoes i
JOIN doencas d ON d.id = i.doenca_id
LEFT JOIN locais l ON l.id = i.local_id
WHERE i.deleted_at IS NULL
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint)
  AND (sqlc.arg(busca)::text = '' OR i.id::text = sqlc.arg(busca)::text OR i.id_externo = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(i.paciente || ' ' || i.endereco_texto || ' ' || COALESCE(l.endereco, '') || ' ' || i.telefone)) > 0)
  AND (sqlc.narg(doenca_id)::bigint IS NULL OR i.doenca_id = sqlc.narg(doenca_id)::bigint)
  AND (sqlc.arg(situacao)::text = '' OR i.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.arg(resultado)::text = '' OR i.resultado = sqlc.arg(resultado)::text)
  AND (sqlc.narg(data_de)::date IS NULL OR i.data_inicio >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR i.data_inicio <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(responsavel_id)::bigint IS NULL OR i.responsavel_id = sqlc.narg(responsavel_id)::bigint);

-- name: ListInvestigacoes :many
SELECT i.id, i.id_externo, i.paciente, i.telefone, i.data_inicio, i.situacao, i.resultado, i.criado_em,
       d.nome AS doenca_nome, COALESCE(l.endereco || ', ' || l.numero, i.endereco_texto)::text AS endereco,
       COALESCE(u.nome, '')::text AS responsavel, (i.local_id IS NOT NULL)::boolean AS tem_local
FROM investigacoes i
JOIN doencas d ON d.id = i.doenca_id
LEFT JOIN locais l ON l.id = i.local_id
LEFT JOIN usuarios u ON u.id = i.responsavel_id
WHERE i.deleted_at IS NULL
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint)
  AND (sqlc.arg(busca)::text = '' OR i.id::text = sqlc.arg(busca)::text OR i.id_externo = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(i.paciente || ' ' || i.endereco_texto || ' ' || COALESCE(l.endereco, '') || ' ' || i.telefone)) > 0)
  AND (sqlc.narg(doenca_id)::bigint IS NULL OR i.doenca_id = sqlc.narg(doenca_id)::bigint)
  AND (sqlc.arg(situacao)::text = '' OR i.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.arg(resultado)::text = '' OR i.resultado = sqlc.arg(resultado)::text)
  AND (sqlc.narg(data_de)::date IS NULL OR i.data_inicio >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR i.data_inicio <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(responsavel_id)::bigint IS NULL OR i.responsavel_id = sqlc.narg(responsavel_id)::bigint)
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN i.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN i.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'paciente' AND sqlc.arg(dir)::text = 'asc' THEN lower(i.paciente) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'paciente' AND sqlc.arg(dir)::text = 'desc' THEN lower(i.paciente) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'doenca' AND sqlc.arg(dir)::text = 'asc' THEN lower(d.nome) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'doenca' AND sqlc.arg(dir)::text = 'desc' THEN lower(d.nome) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'asc' THEN lower(COALESCE(l.endereco, i.endereco_texto)) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'desc' THEN lower(COALESCE(l.endereco, i.endereco_texto)) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'asc' THEN i.situacao END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'desc' THEN i.situacao END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'resultado' AND sqlc.arg(dir)::text = 'asc' THEN i.resultado END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'resultado' AND sqlc.arg(dir)::text = 'desc' THEN i.resultado END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN i.data_inicio END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN i.data_inicio END DESC,
  i.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- Busca de locais para vincular à investigação (até 15).
-- name: BuscarLocaisParaVinculo :many
SELECT l.id, l.endereco, l.numero, b.nome AS bairro_nome
FROM locais l JOIN bairros b ON b.id = l.bairro_id
WHERE l.deleted_at IS NULL AND l.ativo
  AND position(lower(sqlc.arg(busca)::text) in lower(l.endereco || ' ' || l.numero || ' ' || b.nome)) > 0
ORDER BY lower(l.endereco), l.numero
LIMIT 15;

-- ---- exclusão lógica

-- name: SoftDeleteInvestigacao :execrows
UPDATE investigacoes SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreInvestigacao :execrows
UPDATE investigacoes SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteInvestigacao :execrows
DELETE FROM investigacoes WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListInvestigacoesExcluidas :many
SELECT i.id, i.paciente, d.nome AS doenca_nome, i.deleted_at, COALESCE(u.nome, '')::text AS por
FROM investigacoes i JOIN doencas d ON d.id = i.doenca_id LEFT JOIN usuarios u ON u.id = i.deleted_by
WHERE i.deleted_at IS NOT NULL ORDER BY i.deleted_at DESC LIMIT 500;

-- name: ContarDoencasDoGrupo :one
SELECT count(*)::int FROM doencas WHERE grupo_id = $1;

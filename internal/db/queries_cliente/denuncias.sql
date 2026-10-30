-- ---------------------------------------------------------------- Denúncias (9.3)

-- name: CreateDenuncia :one
INSERT INTO denuncias (anonima, cidadao_nome, cidadao_telefone, cidadao_cpf, tipo, titulo, descricao, data_denuncia, origem,
                       endereco_texto, bairro_id, quarteirao_id, criado_por)
VALUES (sqlc.arg(anonima), sqlc.arg(cidadao_nome), sqlc.arg(cidadao_telefone), sqlc.arg(cidadao_cpf), sqlc.arg(tipo), sqlc.arg(titulo),
        sqlc.arg(descricao), sqlc.arg(data_denuncia), sqlc.arg(origem), sqlc.arg(endereco_texto), sqlc.narg(bairro_id),
        sqlc.narg(quarteirao_id), sqlc.narg(criado_por))
RETURNING *;

-- name: GetDenuncia :one
SELECT * FROM denuncias WHERE id = $1 AND deleted_at IS NULL;

-- name: GetDenunciaQualquer :one
SELECT * FROM denuncias WHERE id = $1;

-- name: UpdateDenunciaDados :one
UPDATE denuncias SET anonima = sqlc.arg(anonima), cidadao_nome = sqlc.arg(cidadao_nome), cidadao_telefone = sqlc.arg(cidadao_telefone),
       cidadao_cpf = sqlc.arg(cidadao_cpf), tipo = sqlc.arg(tipo), titulo = sqlc.arg(titulo), descricao = sqlc.arg(descricao),
       data_denuncia = sqlc.arg(data_denuncia), origem = sqlc.arg(origem), endereco_texto = sqlc.arg(endereco_texto),
       bairro_id = sqlc.narg(bairro_id), quarteirao_id = sqlc.narg(quarteirao_id), atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL RETURNING *;

-- name: SetDenunciaLocal :execrows
UPDATE denuncias SET local_id = sqlc.narg(local_id), atualizado_em = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: SetDenunciaVisita :execrows
UPDATE denuncias SET visita_id = sqlc.narg(visita_id), atualizado_em = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: SetDenunciaSituacao :execrows
UPDATE denuncias SET situacao = sqlc.arg(situacao)::text, responsavel_id = sqlc.narg(responsavel_id),
       encerrada_em = CASE WHEN sqlc.arg(situacao)::text IN ('encerrada', 'cancelada') THEN now() ELSE NULL END, atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountDenuncias :one
SELECT count(*)::int
FROM denuncias d
LEFT JOIN locais l ON l.id = d.local_id
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR d.id::text = sqlc.arg(busca)::text OR d.id_externo = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(d.titulo || ' ' || d.cidadao_nome || ' ' || d.cidadao_telefone || ' ' || d.endereco_texto || ' ' || COALESCE(l.endereco, ''))) > 0)
  AND (sqlc.arg(tipo)::text = '' OR d.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.arg(situacao)::text = '' OR d.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.arg(origem)::text = '' OR d.origem = sqlc.arg(origem)::text)
  AND (sqlc.arg(anonima)::text = '' OR d.anonima = (sqlc.arg(anonima)::text = 'sim'))
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR d.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR d.data_denuncia >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR d.data_denuncia <= sqlc.narg(data_ate)::date);

-- name: ListDenuncias :many
SELECT d.id, d.titulo, d.tipo, d.anonima, d.cidadao_nome, d.cidadao_telefone, d.data_denuncia, d.origem, d.situacao,
       COALESCE(l.endereco || ', ' || l.numero, d.endereco_texto)::text AS endereco,
       COALESCE(b.nome, '')::text AS bairro, COALESCE(u.nome, '')::text AS responsavel
FROM denuncias d
LEFT JOIN locais l ON l.id = d.local_id
LEFT JOIN bairros b ON b.id = d.bairro_id
LEFT JOIN usuarios u ON u.id = d.responsavel_id
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR d.id::text = sqlc.arg(busca)::text OR d.id_externo = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(d.titulo || ' ' || d.cidadao_nome || ' ' || d.cidadao_telefone || ' ' || d.endereco_texto || ' ' || COALESCE(l.endereco, ''))) > 0)
  AND (sqlc.arg(tipo)::text = '' OR d.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.arg(situacao)::text = '' OR d.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.arg(origem)::text = '' OR d.origem = sqlc.arg(origem)::text)
  AND (sqlc.arg(anonima)::text = '' OR d.anonima = (sqlc.arg(anonima)::text = 'sim'))
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR d.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR d.data_denuncia >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR d.data_denuncia <= sqlc.narg(data_ate)::date)
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN d.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN d.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'titulo' AND sqlc.arg(dir)::text = 'asc' THEN lower(d.titulo) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'titulo' AND sqlc.arg(dir)::text = 'desc' THEN lower(d.titulo) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'tipo' AND sqlc.arg(dir)::text = 'asc' THEN lower(d.tipo) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'tipo' AND sqlc.arg(dir)::text = 'desc' THEN lower(d.tipo) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN d.data_denuncia END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN d.data_denuncia END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'asc' THEN lower(COALESCE(l.endereco, d.endereco_texto)) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'desc' THEN lower(COALESCE(l.endereco, d.endereco_texto)) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'asc' THEN d.situacao END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'desc' THEN d.situacao END DESC,
  d.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- Denúncias anteriores do mesmo telefone (para reconhecer o cidadão).
-- name: ListDenunciasDoTelefone :many
SELECT id, titulo, data_denuncia, situacao, cidadao_nome FROM denuncias
WHERE deleted_at IS NULL AND cidadao_telefone = sqlc.arg(telefone)::text AND cidadao_telefone <> ''
ORDER BY id DESC LIMIT 5;

-- ---- exclusão lógica

-- name: SoftDeleteDenuncia :execrows
UPDATE denuncias SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreDenuncia :execrows
UPDATE denuncias SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteDenuncia :execrows
DELETE FROM denuncias WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListDenunciasExcluidas :many
SELECT d.id, d.titulo, d.tipo, d.deleted_at, COALESCE(u.nome, '')::text AS por
FROM denuncias d LEFT JOIN usuarios u ON u.id = d.deleted_by
WHERE d.deleted_at IS NOT NULL ORDER BY d.deleted_at DESC LIMIT 500;

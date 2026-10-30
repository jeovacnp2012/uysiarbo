-- ---------------------------------------------------------------- BAAP: animais peçonhentos (9.5)

-- name: CreateBaap :one
INSERT INTO baap_notificacoes (sinam, data_notificado, paciente, nome_mae, data_nascimento, telefone, telefone2, animal, animal_outro,
                               especie, local_picada, situacao_paciente, atuacao_vga, observacao, endereco_texto, numero, referencia,
                               bairro_id, criado_por)
VALUES (sqlc.arg(sinam), sqlc.arg(data_notificado), sqlc.arg(paciente), sqlc.arg(nome_mae), sqlc.narg(data_nascimento), sqlc.arg(telefone),
        sqlc.arg(telefone2), sqlc.arg(animal), sqlc.arg(animal_outro), sqlc.arg(especie), sqlc.arg(local_picada), sqlc.arg(situacao_paciente),
        sqlc.arg(atuacao_vga), sqlc.arg(observacao), sqlc.arg(endereco_texto), sqlc.arg(numero), sqlc.arg(referencia), sqlc.narg(bairro_id),
        sqlc.narg(criado_por))
RETURNING *;

-- name: GetBaap :one
SELECT * FROM baap_notificacoes WHERE id = $1 AND deleted_at IS NULL;

-- name: GetBaapQualquer :one
SELECT * FROM baap_notificacoes WHERE id = $1;

-- name: UpdateBaapDados :one
UPDATE baap_notificacoes SET sinam = sqlc.arg(sinam), data_notificado = sqlc.arg(data_notificado), paciente = sqlc.arg(paciente),
       nome_mae = sqlc.arg(nome_mae), data_nascimento = sqlc.narg(data_nascimento), telefone = sqlc.arg(telefone),
       telefone2 = sqlc.arg(telefone2), animal = sqlc.arg(animal), animal_outro = sqlc.arg(animal_outro), especie = sqlc.arg(especie),
       local_picada = sqlc.arg(local_picada), situacao_paciente = sqlc.arg(situacao_paciente), atuacao_vga = sqlc.arg(atuacao_vga),
       observacao = sqlc.arg(observacao), endereco_texto = sqlc.arg(endereco_texto), numero = sqlc.arg(numero),
       referencia = sqlc.arg(referencia), bairro_id = sqlc.narg(bairro_id), atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL RETURNING *;

-- name: SetBaapLocal :execrows
UPDATE baap_notificacoes SET local_id = sqlc.narg(local_id), atualizado_em = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: SetBaapVisita :execrows
UPDATE baap_notificacoes SET visita_id = sqlc.narg(visita_id), atualizado_em = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: SetBaapSituacao :execrows
UPDATE baap_notificacoes SET situacao = sqlc.arg(situacao)::text, responsavel_id = sqlc.narg(responsavel_id),
       encerrada_em = CASE WHEN sqlc.arg(situacao)::text = 'encerrada' THEN now() ELSE NULL END, atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountBaap :one
SELECT count(*)::int
FROM baap_notificacoes d
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR d.id::text = sqlc.arg(busca)::text OR d.sinam = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(d.paciente || ' ' || d.nome_mae || ' ' || d.telefone || ' ' || d.endereco_texto || ' ' || d.especie)) > 0)
  AND (sqlc.arg(situacao)::text = '' OR d.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.arg(animal)::text = '' OR d.animal = sqlc.arg(animal)::text)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR d.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR d.data_notificado >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR d.data_notificado <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_minhas)::boolean OR d.criado_por = sqlc.arg(usuario_id)::bigint OR d.responsavel_id = sqlc.arg(usuario_id)::bigint);

-- name: ListBaap :many
SELECT d.id, d.sinam, d.paciente, d.animal, d.animal_outro, d.especie, d.data_notificado, d.situacao,
       COALESCE(NULLIF(btrim(d.endereco_texto || ' ' || d.numero), ''), '')::text AS endereco,
       COALESCE(b.nome, '')::text AS bairro, COALESCE(u.nome, '')::text AS responsavel
FROM baap_notificacoes d
LEFT JOIN bairros b ON b.id = d.bairro_id
LEFT JOIN usuarios u ON u.id = d.responsavel_id
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR d.id::text = sqlc.arg(busca)::text OR d.sinam = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(d.paciente || ' ' || d.nome_mae || ' ' || d.telefone || ' ' || d.endereco_texto || ' ' || d.especie)) > 0)
  AND (sqlc.arg(situacao)::text = '' OR d.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.arg(animal)::text = '' OR d.animal = sqlc.arg(animal)::text)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR d.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR d.data_notificado >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR d.data_notificado <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_minhas)::boolean OR d.criado_por = sqlc.arg(usuario_id)::bigint OR d.responsavel_id = sqlc.arg(usuario_id)::bigint)
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN d.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN d.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'paciente' AND sqlc.arg(dir)::text = 'asc' THEN lower(d.paciente) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'paciente' AND sqlc.arg(dir)::text = 'desc' THEN lower(d.paciente) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'animal' AND sqlc.arg(dir)::text = 'asc' THEN d.animal END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'animal' AND sqlc.arg(dir)::text = 'desc' THEN d.animal END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN d.data_notificado END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN d.data_notificado END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'asc' THEN d.situacao END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'desc' THEN d.situacao END DESC,
  d.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- ---- exclusão lógica

-- name: SoftDeleteBaap :execrows
UPDATE baap_notificacoes SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreBaap :execrows
UPDATE baap_notificacoes SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteBaap :execrows
DELETE FROM baap_notificacoes WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListBaapExcluidos :many
SELECT d.id, d.paciente, d.animal, d.deleted_at, COALESCE(u.nome, '')::text AS por
FROM baap_notificacoes d LEFT JOIN usuarios u ON u.id = d.deleted_by
WHERE d.deleted_at IS NOT NULL ORDER BY d.deleted_at DESC LIMIT 500;

-- ---- vínculos do local

-- name: CountBaapAtivosDoLocal :one
SELECT count(*)::int FROM baap_notificacoes WHERE local_id = $1 AND deleted_at IS NULL;

-- name: CountBaapDoLocalQualquer :one
SELECT count(*)::int FROM baap_notificacoes WHERE local_id = $1;

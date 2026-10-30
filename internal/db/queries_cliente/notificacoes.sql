-- ---------------------------------------------------------------- Numeração (9.4)

-- Próximo número do ano, de forma atômica.
-- name: ProximoNumero :one
INSERT INTO numeracoes (tipo, ano, ultimo) VALUES (sqlc.arg(tipo)::text, sqlc.arg(ano)::int, 1)
ON CONFLICT (tipo, ano) DO UPDATE SET ultimo = numeracoes.ultimo + 1
RETURNING ultimo;

-- ---------------------------------------------------------------- Notificações

-- name: CreateNotificacao :one
INSERT INTO notificacoes (numero, local_id, visita_id, lei_trecho_id, motivo, descricao, data_notificacao, prazo, criado_por)
VALUES (sqlc.arg(numero), sqlc.arg(local_id), sqlc.narg(visita_id), sqlc.narg(lei_trecho_id), sqlc.arg(motivo), sqlc.arg(descricao),
        sqlc.arg(data_notificacao), sqlc.narg(prazo), sqlc.narg(criado_por))
RETURNING *;

-- name: GetNotificacao :one
SELECT * FROM notificacoes WHERE id = $1 AND deleted_at IS NULL;

-- name: GetNotificacaoQualquer :one
SELECT * FROM notificacoes WHERE id = $1;

-- name: UpdateNotificacaoDados :one
UPDATE notificacoes SET visita_id = sqlc.narg(visita_id), lei_trecho_id = sqlc.narg(lei_trecho_id), motivo = sqlc.arg(motivo),
       descricao = sqlc.arg(descricao), data_notificacao = sqlc.arg(data_notificacao), prazo = sqlc.narg(prazo), atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL RETURNING *;

-- name: SetNotificacaoSituacao :execrows
UPDATE notificacoes SET situacao = sqlc.arg(situacao)::text,
       resolvida_em = CASE WHEN sqlc.arg(situacao)::text = 'resolvida' THEN now() ELSE NULL END, atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountNotificacoes :one
SELECT count(*)::int
FROM notificacoes n
JOIN locais l ON l.id = n.local_id
WHERE n.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR n.id::text = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(n.numero || ' ' || n.motivo || ' ' || l.endereco || ' ' || l.numero)) > 0)
  AND (sqlc.arg(situacao)::text = '' OR n.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.arg(vencidas)::text <> 'sim' OR (n.situacao = 'aberta' AND n.prazo < sqlc.arg(hoje)::date))
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR n.data_notificacao >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR n.data_notificacao <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id));

-- name: ListNotificacoes :many
SELECT n.id, n.numero, n.motivo, n.data_notificacao, n.prazo, n.situacao,
       (l.endereco || ', ' || l.numero)::text AS endereco, b.nome AS bairro,
       COALESCE(a.numero, '')::text AS auto_numero
FROM notificacoes n
JOIN locais l ON l.id = n.local_id
JOIN bairros b ON b.id = l.bairro_id
LEFT JOIN autos_infracao a ON a.notificacao_id = n.id AND a.situacao <> 'cancelado' AND a.deleted_at IS NULL
WHERE n.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR n.id::text = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(n.numero || ' ' || n.motivo || ' ' || l.endereco || ' ' || l.numero)) > 0)
  AND (sqlc.arg(situacao)::text = '' OR n.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.arg(vencidas)::text <> 'sim' OR (n.situacao = 'aberta' AND n.prazo < sqlc.arg(hoje)::date))
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR n.data_notificacao >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR n.data_notificacao <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id))
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'numero' AND sqlc.arg(dir)::text = 'asc' THEN n.numero END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'numero' AND sqlc.arg(dir)::text = 'desc' THEN n.numero END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'motivo' AND sqlc.arg(dir)::text = 'asc' THEN lower(n.motivo) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'motivo' AND sqlc.arg(dir)::text = 'desc' THEN lower(n.motivo) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN n.data_notificacao END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN n.data_notificacao END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'prazo' AND sqlc.arg(dir)::text = 'asc' THEN n.prazo END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'prazo' AND sqlc.arg(dir)::text = 'desc' THEN n.prazo END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'asc' THEN lower(l.endereco) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'desc' THEN lower(l.endereco) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'asc' THEN n.situacao END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'desc' THEN n.situacao END DESC,
  n.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- Notificações de um local (ficha do local).
-- name: ListNotificacoesDoLocal :many
SELECT id, numero, motivo, data_notificacao, prazo, situacao FROM notificacoes
WHERE local_id = $1 AND deleted_at IS NULL ORDER BY id DESC LIMIT 20;

-- name: CountNotificacoesAtivasDoLocal :one
SELECT count(*)::int FROM notificacoes WHERE local_id = $1 AND deleted_at IS NULL;

-- name: CountNotificacoesDoLocalQualquer :one
SELECT count(*)::int FROM notificacoes WHERE local_id = $1;

-- name: CountNotificacoesDoTrecho :one
SELECT count(*)::int FROM notificacoes WHERE lei_trecho_id = $1;

-- name: CountNotificacoesAtivasDoTrecho :one
SELECT count(*)::int FROM notificacoes WHERE lei_trecho_id = $1 AND deleted_at IS NULL;

-- name: CountNotificacoesDaVisita :one
SELECT count(*)::int FROM notificacoes WHERE visita_id = $1;

-- name: SoftDeleteNotificacao :execrows
UPDATE notificacoes SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreNotificacao :execrows
UPDATE notificacoes SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteNotificacao :execrows
DELETE FROM notificacoes WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListNotificacoesExcluidas :many
SELECT n.id, n.numero, n.motivo, n.deleted_at, COALESCE(u.nome, '')::text AS por
FROM notificacoes n LEFT JOIN usuarios u ON u.id = n.deleted_by
WHERE n.deleted_at IS NOT NULL ORDER BY n.deleted_at DESC LIMIT 500;

-- Trechos de lei ativos de todas as leis ativas (para escolher o enquadramento).
-- name: ListTrechosParaEscolha :many
SELECT t.id, t.artigo, m.numero AS lei_numero, m.ano AS lei_ano
FROM leis_trechos t JOIN leis_municipais m ON m.id = t.lei_id
WHERE t.deleted_at IS NULL AND m.deleted_at IS NULL
ORDER BY m.ano DESC, m.numero, t.id;

-- ---------------------------------------------------------------- Autos de infração

-- name: CreateAuto :one
INSERT INTO autos_infracao (numero, notificacao_id, local_id, lei_trecho_id, descricao, valor_centavos, data_auto, vencimento, criado_por)
VALUES (sqlc.arg(numero), sqlc.arg(notificacao_id), sqlc.arg(local_id), sqlc.narg(lei_trecho_id), sqlc.arg(descricao),
        sqlc.arg(valor_centavos), sqlc.arg(data_auto), sqlc.narg(vencimento), sqlc.narg(criado_por))
RETURNING *;

-- name: GetAuto :one
SELECT * FROM autos_infracao WHERE id = $1 AND deleted_at IS NULL;

-- name: GetAutoQualquer :one
SELECT * FROM autos_infracao WHERE id = $1;

-- Auto vivo (aberto ou pago) de uma notificação.
-- name: GetAutoDaNotificacao :one
SELECT * FROM autos_infracao WHERE notificacao_id = $1 AND situacao <> 'cancelado' AND deleted_at IS NULL;

-- name: ListAutosDaNotificacao :many
SELECT id, numero, situacao, valor_centavos, data_auto FROM autos_infracao
WHERE notificacao_id = $1 AND deleted_at IS NULL ORDER BY id DESC;

-- name: UpdateAutoDados :one
UPDATE autos_infracao SET lei_trecho_id = sqlc.narg(lei_trecho_id), descricao = sqlc.arg(descricao), valor_centavos = sqlc.arg(valor_centavos),
       data_auto = sqlc.arg(data_auto), vencimento = sqlc.narg(vencimento), atualizado_em = now()
WHERE id = sqlc.arg(id) AND situacao = 'aberto' AND deleted_at IS NULL RETURNING *;

-- name: SetAutoSituacao :execrows
UPDATE autos_infracao SET situacao = sqlc.arg(situacao)::text, pago_em = sqlc.narg(pago_em), atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountAutos :one
SELECT count(*)::int
FROM autos_infracao a
JOIN locais l ON l.id = a.local_id
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR a.id::text = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(a.numero || ' ' || a.descricao || ' ' || l.endereco || ' ' || l.numero)) > 0)
  AND (sqlc.arg(situacao)::text = '' OR a.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.arg(vencidos)::text <> 'sim' OR (a.situacao = 'aberto' AND a.vencimento < sqlc.arg(hoje)::date))
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR a.data_auto >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR a.data_auto <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id));

-- name: ListAutos :many
SELECT a.id, a.numero, a.notificacao_id, n.numero AS notificacao_numero, a.valor_centavos, a.data_auto, a.vencimento, a.situacao, a.pago_em,
       (l.endereco || ', ' || l.numero)::text AS endereco, b.nome AS bairro
FROM autos_infracao a
JOIN notificacoes n ON n.id = a.notificacao_id
JOIN locais l ON l.id = a.local_id
JOIN bairros b ON b.id = l.bairro_id
WHERE a.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR a.id::text = sqlc.arg(busca)::text
       OR position(lower(sqlc.arg(busca)::text) in lower(a.numero || ' ' || a.descricao || ' ' || l.endereco || ' ' || l.numero)) > 0)
  AND (sqlc.arg(situacao)::text = '' OR a.situacao = sqlc.arg(situacao)::text)
  AND (sqlc.arg(vencidos)::text <> 'sim' OR (a.situacao = 'aberto' AND a.vencimento < sqlc.arg(hoje)::date))
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR a.data_auto >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR a.data_auto <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_usuario)::boolean OR usuario_ve_local(sqlc.arg(usuario_id)::bigint, l.id, l.bairro_id, l.quarteirao_id))
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'numero' AND sqlc.arg(dir)::text = 'asc' THEN a.numero END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'numero' AND sqlc.arg(dir)::text = 'desc' THEN a.numero END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN a.data_auto END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN a.data_auto END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'vencimento' AND sqlc.arg(dir)::text = 'asc' THEN a.vencimento END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'vencimento' AND sqlc.arg(dir)::text = 'desc' THEN a.vencimento END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'valor' AND sqlc.arg(dir)::text = 'asc' THEN a.valor_centavos END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'valor' AND sqlc.arg(dir)::text = 'desc' THEN a.valor_centavos END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'asc' THEN lower(l.endereco) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'desc' THEN lower(l.endereco) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'asc' THEN a.situacao END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'situacao' AND sqlc.arg(dir)::text = 'desc' THEN a.situacao END DESC,
  a.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: ListAutosDoLocal :many
SELECT id, numero, situacao, valor_centavos, data_auto FROM autos_infracao
WHERE local_id = $1 AND deleted_at IS NULL ORDER BY id DESC LIMIT 20;

-- name: CountAutosDoLocalQualquer :one
SELECT count(*)::int FROM autos_infracao WHERE local_id = $1;

-- name: CountAutosDoTrecho :one
SELECT count(*)::int FROM autos_infracao WHERE lei_trecho_id = $1;

-- name: CountAutosVivosDaNotificacao :one
SELECT count(*)::int FROM autos_infracao WHERE notificacao_id = $1 AND deleted_at IS NULL;

-- name: CountAutosDaNotificacaoQualquer :one
SELECT count(*)::int FROM autos_infracao WHERE notificacao_id = $1;

-- name: SoftDeleteAuto :execrows
UPDATE autos_infracao SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreAuto :execrows
UPDATE autos_infracao SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteAuto :execrows
DELETE FROM autos_infracao WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListAutosExcluidos :many
SELECT a.id, a.numero, a.deleted_at, COALESCE(u.nome, '')::text AS por
FROM autos_infracao a LEFT JOIN usuarios u ON u.id = a.deleted_by
WHERE a.deleted_at IS NOT NULL ORDER BY a.deleted_at DESC LIMIT 500;

-- name: ListNotificacoesDaFichaDoLocal :many
SELECT n.id, n.numero, n.situacao, n.data_notificacao, u.nome AS por_nome,
       EXISTS (SELECT 1 FROM autos_infracao a WHERE a.notificacao_id = n.id AND a.deleted_at IS NULL) AS autuado
FROM notificacoes n LEFT JOIN usuarios u ON u.id = n.criado_por
WHERE n.local_id = $1 AND n.deleted_at IS NULL ORDER BY n.id DESC LIMIT 20;

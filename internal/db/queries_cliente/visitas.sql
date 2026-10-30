-- ---------------------------------------------------------------- Visitas

-- name: CreateVisita :one
INSERT INTO visitas (
    id_externo, local_id, ciclo_id, atividade_id, agente_id, data, hora_entrada, tipo_visita, pendencia, semana, semana_ano,
    anotacao, assinatura, achou_deposito, dep_a1, dep_a2, dep_b, dep_c, dep_d1, dep_d2, dep_e, total_eliminado,
    coletou_amostra, fez_tratamento, l1_gramas, l1_tratados, l2_gramas, l2_tratados, adulticida_tipo, adulticida_cargas,
    tratamento_eliminados, bri_volume, latitude, longitude, precisao_gps, data_tablet, criado_por
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
    $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22,
    $23, $24, $25, $26, $27, $28, $29, $30,
    $31, $32, $33, $34, $35, $36, $37
)
RETURNING *;

-- name: UpdateVisita :one
UPDATE visitas SET
    ciclo_id = $2, atividade_id = $3, agente_id = $4, data = $5, hora_entrada = $6, tipo_visita = $7, pendencia = $8,
    semana = $9, semana_ano = $10, anotacao = $11, assinatura = $12, achou_deposito = $13,
    dep_a1 = $14, dep_a2 = $15, dep_b = $16, dep_c = $17, dep_d1 = $18, dep_d2 = $19, dep_e = $20, total_eliminado = $21,
    coletou_amostra = $22, fez_tratamento = $23, l1_gramas = $24, l1_tratados = $25, l2_gramas = $26, l2_tratados = $27,
    adulticida_tipo = $28, adulticida_cargas = $29, tratamento_eliminados = $30, bri_volume = $31,
    latitude = $32, longitude = $33, precisao_gps = $34, atualizado_em = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: GetVisita :one
SELECT * FROM visitas WHERE id = $1 AND deleted_at IS NULL;

-- name: GetVisitaQualquer :one
SELECT * FROM visitas WHERE id = $1;

-- name: GetVisitaPorIDExterno :one
SELECT * FROM visitas WHERE id_externo = $1;

-- name: SetVisitaAtiva :execrows
UPDATE visitas SET ativo = $2, atualizado_em = now() WHERE id = $1 AND deleted_at IS NULL AND ativo <> $2;

-- Últimas visitas (a listagem completa, com filtros, é da 6.2).
-- name: ListVisitasRecentes :many
SELECT v.id, v.data, v.ativo, v.tipo_visita, v.pendencia, v.semana, v.semana_ano,
       l.endereco, l.numero, c.nome AS ciclo_nome, a.sigla AS atividade_sigla, u.nome AS agente_nome,
       v.achou_deposito, v.coletou_amostra, v.fez_tratamento
FROM visitas v
JOIN locais l ON l.id = v.local_id
JOIN ciclos c ON c.id = v.ciclo_id
JOIN atividades a ON a.id = v.atividade_id
JOIN usuarios u ON u.id = v.agente_id
WHERE v.deleted_at IS NULL AND (v.ativo OR sqlc.arg(ver_desativadas)::boolean)
  AND (sqlc.narg(local_id)::bigint IS NULL OR v.local_id = sqlc.narg(local_id)::bigint)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
ORDER BY v.data DESC, v.id DESC
LIMIT 5000;

-- Listagem com os filtros da tela (6.2).
-- name: CountVisitas :one
SELECT count(*)::int
FROM visitas v
JOIN locais l ON l.id = v.local_id
WHERE v.deleted_at IS NULL AND (v.ativo OR sqlc.arg(ver_desativadas)::boolean)
  AND (sqlc.narg(local_id)::bigint IS NULL OR v.local_id = sqlc.narg(local_id)::bigint)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
  AND (sqlc.arg(id_texto)::text = '' OR v.id::text = sqlc.arg(id_texto)::text OR v.id_externo = sqlc.arg(id_texto)::text)
  AND (sqlc.arg(endereco)::text = '' OR position(lower(sqlc.arg(endereco)::text) in lower(l.endereco)) > 0)
  AND (sqlc.arg(numero)::text = '' OR position(lower(sqlc.arg(numero)::text) in lower(l.numero)) > 0)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(quarteirao_id)::bigint IS NULL OR l.quarteirao_id = sqlc.narg(quarteirao_id)::bigint)
  AND (sqlc.arg(lado)::text = '' OR lower(l.lado) = lower(sqlc.arg(lado)::text))
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
  AND (sqlc.narg(semana)::int IS NULL OR v.semana = sqlc.narg(semana)::int)
  AND (sqlc.narg(data_de)::date IS NULL OR v.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR v.data <= sqlc.narg(data_ate)::date)
  AND (sqlc.arg(pendencia)::text = '' OR v.pendencia = sqlc.arg(pendencia)::text)
  AND (sqlc.arg(deposito)::text = '' OR v.achou_deposito = (sqlc.arg(deposito)::text = 'sim'))
  AND (sqlc.arg(amostra)::text = '' OR v.coletou_amostra = (sqlc.arg(amostra)::text = 'sim'))
  AND (sqlc.arg(tratamento)::text = '' OR v.fez_tratamento = (sqlc.arg(tratamento)::text = 'sim'))
  AND (sqlc.arg(foto)::text = '' OR EXISTS (SELECT 1 FROM visita_fotos f WHERE f.visita_id = v.id AND f.deleted_at IS NULL) = (sqlc.arg(foto)::text = 'sim'))
  AND (sqlc.arg(situacao)::text = '' OR v.ativo = (sqlc.arg(situacao)::text = 'ativa'));

-- name: ListVisitas :many
SELECT v.id, v.data, v.ativo, v.tipo_visita, v.pendencia, v.semana, v.semana_ano,
       l.endereco, l.numero, c.nome AS ciclo_nome, a.sigla AS atividade_sigla, u.nome AS agente_nome,
       v.achou_deposito, v.coletou_amostra, v.fez_tratamento
FROM visitas v
JOIN locais l ON l.id = v.local_id
JOIN ciclos c ON c.id = v.ciclo_id
JOIN atividades a ON a.id = v.atividade_id
JOIN usuarios u ON u.id = v.agente_id
WHERE v.deleted_at IS NULL AND (v.ativo OR sqlc.arg(ver_desativadas)::boolean)
  AND (sqlc.narg(local_id)::bigint IS NULL OR v.local_id = sqlc.narg(local_id)::bigint)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
  AND (sqlc.arg(id_texto)::text = '' OR v.id::text = sqlc.arg(id_texto)::text OR v.id_externo = sqlc.arg(id_texto)::text)
  AND (sqlc.arg(endereco)::text = '' OR position(lower(sqlc.arg(endereco)::text) in lower(l.endereco)) > 0)
  AND (sqlc.arg(numero)::text = '' OR position(lower(sqlc.arg(numero)::text) in lower(l.numero)) > 0)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(quarteirao_id)::bigint IS NULL OR l.quarteirao_id = sqlc.narg(quarteirao_id)::bigint)
  AND (sqlc.arg(lado)::text = '' OR lower(l.lado) = lower(sqlc.arg(lado)::text))
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
  AND (sqlc.narg(semana)::int IS NULL OR v.semana = sqlc.narg(semana)::int)
  AND (sqlc.narg(data_de)::date IS NULL OR v.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR v.data <= sqlc.narg(data_ate)::date)
  AND (sqlc.arg(pendencia)::text = '' OR v.pendencia = sqlc.arg(pendencia)::text)
  AND (sqlc.arg(deposito)::text = '' OR v.achou_deposito = (sqlc.arg(deposito)::text = 'sim'))
  AND (sqlc.arg(amostra)::text = '' OR v.coletou_amostra = (sqlc.arg(amostra)::text = 'sim'))
  AND (sqlc.arg(tratamento)::text = '' OR v.fez_tratamento = (sqlc.arg(tratamento)::text = 'sim'))
  AND (sqlc.arg(foto)::text = '' OR EXISTS (SELECT 1 FROM visita_fotos f WHERE f.visita_id = v.id AND f.deleted_at IS NULL) = (sqlc.arg(foto)::text = 'sim'))
  AND (sqlc.arg(situacao)::text = '' OR v.ativo = (sqlc.arg(situacao)::text = 'ativa'))
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN COALESCE(v.id_externo, v.id::text) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN COALESCE(v.id_externo, v.id::text) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'asc' THEN lower(l.endereco) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'endereco' AND sqlc.arg(dir)::text = 'desc' THEN lower(l.endereco) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN v.data END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN v.data END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'ciclo' AND sqlc.arg(dir)::text = 'asc' THEN c.nome END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'ciclo' AND sqlc.arg(dir)::text = 'desc' THEN c.nome END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'semana' AND sqlc.arg(dir)::text = 'asc' THEN v.semana END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'semana' AND sqlc.arg(dir)::text = 'desc' THEN v.semana END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'atividade' AND sqlc.arg(dir)::text = 'asc' THEN a.sigla END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'atividade' AND sqlc.arg(dir)::text = 'desc' THEN a.sigla END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'pendencia' AND sqlc.arg(dir)::text = 'asc' THEN v.pendencia END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'pendencia' AND sqlc.arg(dir)::text = 'desc' THEN v.pendencia END DESC,
  v.data DESC, v.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- Ficha do local (6.5): últimas visitas e resumo dos últimos 12 meses.
-- name: ListVisitasDoLocal :many
SELECT v.id, v.data, v.pendencia, v.semana, v.semana_ano, c.nome AS ciclo_nome, a.sigla AS atividade_sigla, u.nome AS agente_nome,
       v.achou_deposito, v.coletou_amostra, v.fez_tratamento
FROM visitas v
JOIN ciclos c ON c.id = v.ciclo_id
JOIN atividades a ON a.id = v.atividade_id
JOIN usuarios u ON u.id = v.agente_id
WHERE v.local_id = sqlc.arg(local_id)::bigint AND v.deleted_at IS NULL AND v.ativo
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
ORDER BY v.data DESC, v.id DESC
LIMIT 10;

-- name: ResumoDoLocal12Meses :one
SELECT count(*)::int AS visitas,
       count(*) FILTER (WHERE v.pendencia = 'nenhuma')::int AS trabalhadas,
       count(*) FILTER (WHERE v.achou_deposito)::int AS com_deposito,
       (max(v.data) FILTER (WHERE v.achou_deposito))::date AS ultimo_deposito,
       (SELECT count(*) FROM amostras am JOIN visitas v2 ON v2.id = am.visita_id
         WHERE v2.local_id = sqlc.arg(local_id)::bigint AND v2.deleted_at IS NULL AND v2.ativo AND am.deleted_at IS NULL
           AND v2.data > sqlc.arg(hoje)::date - interval '12 months' AND v2.data <= sqlc.arg(hoje)::date
           AND (am.total_aegypti + am.total_albopictus) > 0)::int AS amostras_positivas
FROM visitas v
WHERE v.local_id = sqlc.arg(local_id)::bigint AND v.deleted_at IS NULL AND v.ativo
  AND v.data > sqlc.arg(hoje)::date - interval '12 months' AND v.data <= sqlc.arg(hoje)::date;

-- ---------------------------------------------------------------- Fotos

-- name: CreateVisitaFoto :one
INSERT INTO visita_fotos (visita_id, nome, tipo_midia, conteudo, criado_por)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: ListFotosDaVisita :many
SELECT id, nome, tipo_midia, length(conteudo)::int AS tamanho FROM visita_fotos WHERE visita_id = $1 AND deleted_at IS NULL ORDER BY id;

-- name: GetFotoDaVisita :one
SELECT * FROM visita_fotos WHERE id = $1 AND visita_id = $2 AND deleted_at IS NULL;

-- name: CountFotosDaVisita :one
SELECT count(*)::int FROM visita_fotos WHERE visita_id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteVisitaFoto :execrows
UPDATE visita_fotos SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint WHERE id = sqlc.arg(id)::bigint AND visita_id = sqlc.arg(visita)::bigint AND deleted_at IS NULL;

-- ---------------------------------------------------------------- Anotações internas

-- name: CreateVisitaAnotacao :one
INSERT INTO visita_anotacoes (visita_id, usuario_id, texto) VALUES ($1, $2, $3) RETURNING id;

-- name: ListAnotacoesDaVisita :many
SELECT a.id, a.texto, a.criado_em, COALESCE(u.nome, '')::text AS autor
FROM visita_anotacoes a LEFT JOIN usuarios u ON u.id = a.usuario_id
WHERE a.visita_id = $1 ORDER BY a.id;

-- ---------------------------------------------------------------- Exclusão lógica

-- name: SoftDeleteVisita :execrows
UPDATE visitas SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now() WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: RestoreVisita :execrows
UPDATE visitas SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteVisita :execrows
WITH f AS (DELETE FROM visita_fotos x WHERE x.visita_id = sqlc.arg(id)::bigint AND EXISTS (SELECT 1 FROM visitas v WHERE v.id = sqlc.arg(id)::bigint AND v.deleted_at IS NOT NULL)),
     a AS (DELETE FROM visita_anotacoes y WHERE y.visita_id = sqlc.arg(id)::bigint AND EXISTS (SELECT 1 FROM visitas v WHERE v.id = sqlc.arg(id)::bigint AND v.deleted_at IS NOT NULL))
DELETE FROM visitas WHERE visitas.id = sqlc.arg(id)::bigint AND visitas.deleted_at IS NOT NULL;

-- name: ListVisitasExcluidas :many
SELECT v.id, ('Visita ' || v.id)::text AS rotulo, (to_char(v.data, 'DD/MM/YYYY') || ' — ' || l.endereco || ', ' || l.numero)::text AS contexto,
       v.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM visitas v JOIN locais l ON l.id = v.local_id LEFT JOIN usuarios u ON u.id = v.deleted_by
WHERE v.deleted_at IS NOT NULL ORDER BY v.deleted_at DESC LIMIT 500;

-- Visitas (contando as excluídas) que impedem excluir de vez o local, o ciclo, o agente.
-- name: ContarVisitasDoLocal :one
SELECT count(*) FILTER (WHERE deleted_at IS NULL)::int AS ativas, count(*)::int AS total FROM visitas WHERE local_id = $1;

-- name: ContarVisitasDoCiclo :one
SELECT count(*) FILTER (WHERE deleted_at IS NULL)::int AS ativas, count(*)::int AS total FROM visitas WHERE ciclo_id = $1;

-- Histórico de alterações da ficha da visita (6.6): a visita e os tubitos dela.
-- name: ListAuditoriaDaVisita :many
SELECT * FROM auditoria
WHERE (entidade = 'visita' AND entidade_id = sqlc.arg(visita)::text)
   OR (entidade = 'amostra' AND detalhes->>'visita' = sqlc.arg(visita)::text)
ORDER BY quando DESC, id DESC LIMIT sqlc.arg(limite)::int;

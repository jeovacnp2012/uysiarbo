-- name: ListReconhecimentos :many
SELECT r.id, b.nome AS bairro, c.nome AS ciclo, r.data_inicio, r.data_fim,
       (COALESCE(r.imoveis_residenciais,0) + COALESCE(r.imoveis_comerciais,0) + COALESCE(r.imoveis_outros,0))::int AS imoveis,
       COALESCE(r.habitantes, 0)::int AS habitantes
FROM reconhecimentos r JOIN bairros b ON b.id = r.bairro_id JOIN ciclos c ON c.id = r.ciclo_id
WHERE r.deleted_at IS NULL
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR r.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR r.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, r.bairro_id))
ORDER BY c.data_inicio DESC, b.nome
LIMIT 5000;

-- name: GetReconhecimento :one
SELECT * FROM reconhecimentos WHERE id = $1 AND deleted_at IS NULL;

-- name: GetReconhecimentoQualquer :one
SELECT r.id, r.deleted_at, b.nome AS bairro_nome, c.nome AS ciclo_nome FROM reconhecimentos r
JOIN bairros b ON b.id = r.bairro_id JOIN ciclos c ON c.id = r.ciclo_id WHERE r.id = $1;

-- name: ExisteReconhecimento :one
SELECT EXISTS (SELECT 1 FROM reconhecimentos WHERE bairro_id = $1 AND ciclo_id = $2 AND deleted_at IS NULL AND id <> $3);

-- name: CreateReconhecimento :one
INSERT INTO reconhecimentos (bairro_id, ciclo_id, data_inicio, data_fim, imoveis_residenciais, imoveis_comerciais, imoveis_outros,
    habitantes, quarteiroes, pontos_estrategicos, armadilhas, terrenos_baldios, caes, gatos, roedores,
    latitude, longitude, altitude, distancia_centro, infraestrutura, riscos, observacao, criado_por)
VALUES (@bairro_id, @ciclo_id, @data_inicio, @data_fim, @imoveis_residenciais, @imoveis_comerciais, @imoveis_outros,
    @habitantes, @quarteiroes, @pontos_estrategicos, @armadilhas, @terrenos_baldios, @caes, @gatos, @roedores,
    @latitude, @longitude, @altitude, @distancia_centro, @infraestrutura, @riscos, @observacao, @criado_por)
RETURNING *;

-- name: UpdateReconhecimento :one
UPDATE reconhecimentos SET data_inicio = @data_inicio, data_fim = @data_fim, imoveis_residenciais = @imoveis_residenciais,
    imoveis_comerciais = @imoveis_comerciais, imoveis_outros = @imoveis_outros, habitantes = @habitantes, quarteiroes = @quarteiroes,
    pontos_estrategicos = @pontos_estrategicos, armadilhas = @armadilhas, terrenos_baldios = @terrenos_baldios,
    caes = @caes, gatos = @gatos, roedores = @roedores, latitude = @latitude, longitude = @longitude, altitude = @altitude,
    distancia_centro = @distancia_centro, infraestrutura = @infraestrutura, riscos = @riscos, observacao = @observacao,
    atualizado_em = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteReconhecimento :execrows
UPDATE reconhecimentos SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint, atualizado_em = now()
WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: RestoreReconhecimento :execrows
UPDATE reconhecimentos SET deleted_at = NULL, deleted_by = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteReconhecimento :execrows
DELETE FROM reconhecimentos WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListReconhecimentosExcluidos :many
SELECT r.id, ('Reconhecimento ' || b.nome)::text AS rotulo, c.nome::text AS contexto, r.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM reconhecimentos r JOIN bairros b ON b.id = r.bairro_id JOIN ciclos c ON c.id = r.ciclo_id LEFT JOIN usuarios u ON u.id = r.deleted_by
WHERE r.deleted_at IS NOT NULL ORDER BY r.deleted_at DESC LIMIT 500;

-- name: ListResumosSemanais :many
SELECT s.id, c.nome AS ciclo, s.semana, COALESCE(b.nome, 'Todos os bairros')::text AS bairro,
       COALESCE(a.sigla, 'Todas')::text AS atividade, s.gerado_em
FROM resumos_semanais s JOIN ciclos c ON c.id = s.ciclo_id
LEFT JOIN bairros b ON b.id = s.bairro_id LEFT JOIN atividades a ON a.id = s.atividade_id
WHERE s.deleted_at IS NULL
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR s.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR (s.bairro_id IS NOT NULL AND usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, s.bairro_id)))
ORDER BY s.gerado_em DESC LIMIT 5000;

-- name: GetResumoSemanal :one
SELECT s.id, s.ciclo_id, s.semana, s.bairro_id, s.dados, s.gerado_em, c.nome AS ciclo_nome,
       COALESCE(b.nome, 'Todos os bairros')::text AS bairro_nome,
       COALESCE(a.sigla || ' — ' || a.nome, 'Todas as atividades')::text AS atividade_nome,
       COALESCE(u.nome, '')::text AS gerado_por_nome
FROM resumos_semanais s JOIN ciclos c ON c.id = s.ciclo_id
LEFT JOIN bairros b ON b.id = s.bairro_id LEFT JOIN atividades a ON a.id = s.atividade_id LEFT JOIN usuarios u ON u.id = s.gerado_por
WHERE s.id = $1 AND s.deleted_at IS NULL;

-- name: GetResumoSemanalQualquer :one
SELECT s.id, s.deleted_at, s.semana, c.nome AS ciclo_nome FROM resumos_semanais s JOIN ciclos c ON c.id = s.ciclo_id WHERE s.id = $1;

-- name: CreateResumoSemanal :one
INSERT INTO resumos_semanais (ciclo_id, semana, bairro_id, atividade_id, dados, gerado_por)
VALUES (@ciclo_id, @semana, @bairro_id, @atividade_id, @dados, @gerado_por) RETURNING id;

-- name: SoftDeleteResumoSemanal :execrows
UPDATE resumos_semanais SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint
WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: RestoreResumoSemanal :execrows
UPDATE resumos_semanais SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteResumoSemanal :execrows
DELETE FROM resumos_semanais WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListResumosSemanaisExcluidos :many
SELECT s.id, ('Resumo semanal — semana ' || s.semana)::text AS rotulo, c.nome::text AS contexto, s.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM resumos_semanais s JOIN ciclos c ON c.id = s.ciclo_id LEFT JOIN usuarios u ON u.id = s.deleted_by
WHERE s.deleted_at IS NOT NULL ORDER BY s.deleted_at DESC LIMIT 500;

-- name: CalcularResumoSemanal :one
SELECT count(*)::int AS visitas, count(DISTINCT v.local_id)::int AS locais,
       count(*) FILTER (WHERE v.pendencia = 'fechado')::int AS fechadas,
       count(*) FILTER (WHERE v.pendencia = 'recusado')::int AS recusadas,
       count(*) FILTER (WHERE v.pendencia = 'tentativa')::int AS tentativas,
       count(*) FILTER (WHERE v.achou_deposito)::int AS com_deposito,
       count(*) FILTER (WHERE v.coletou_amostra)::int AS com_amostra,
       count(*) FILTER (WHERE v.fez_tratamento)::int AS com_tratamento,
       COALESCE(sum(v.dep_a1), 0)::int AS dep_a1, COALESCE(sum(v.dep_a2), 0)::int AS dep_a2, COALESCE(sum(v.dep_b), 0)::int AS dep_b,
       COALESCE(sum(v.dep_c), 0)::int AS dep_c, COALESCE(sum(v.dep_d1), 0)::int AS dep_d1, COALESCE(sum(v.dep_d2), 0)::int AS dep_d2,
       COALESCE(sum(v.dep_e), 0)::int AS dep_e, COALESCE(sum(v.total_eliminado), 0)::int AS eliminados,
       COALESCE(sum(v.l1_gramas), 0)::float8 AS l1_gramas, COALESCE(sum(v.l1_tratados), 0)::int AS l1_tratados,
       COALESCE(sum(v.l2_gramas), 0)::float8 AS l2_gramas, COALESCE(sum(v.l2_tratados), 0)::int AS l2_tratados,
       COALESCE(sum(v.adulticida_cargas), 0)::int AS cargas,
       COALESCE(sum((SELECT count(*) FROM amostras a WHERE a.visita_id = v.id AND a.deleted_at IS NULL)), 0)::int AS tubitos,
       COALESCE(sum((SELECT count(*) FROM amostras a WHERE a.visita_id = v.id AND a.deleted_at IS NULL AND a.status = 'concluido'
                     AND a.total_aegypti + a.total_albopictus > 0)), 0)::int AS tubitos_positivos
FROM visitas v JOIN locais l ON l.id = v.local_id
WHERE v.deleted_at IS NULL AND v.ativo AND v.ciclo_id = @ciclo_id AND v.semana = @semana
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint);

-- name: ContarFichasDoBairro :one
SELECT (SELECT count(*) FROM reconhecimentos WHERE bairro_id = sqlc.arg(id)::bigint)::int AS reconhecimentos,
       (SELECT count(*) FROM resumos_semanais WHERE bairro_id = sqlc.arg(id)::bigint)::int AS resumos;

-- name: ContarFichasDoCiclo :one
SELECT (SELECT count(*) FROM reconhecimentos WHERE ciclo_id = sqlc.arg(id)::bigint)::int AS reconhecimentos,
       (SELECT count(*) FROM resumos_semanais WHERE ciclo_id = sqlc.arg(id)::bigint)::int AS resumos;

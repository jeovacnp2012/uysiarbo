-- Consultas dos relatórios do sistema (8.2). Cada uma devolve linhas já agregadas para o motor de relatórios.

-- name: RelLocalVisita :many
SELECT b.nome AS bairro, q.nome AS quarteirao, l.endereco, l.numero, l.lado,
       count(v.id)::int AS visitas, max(v.data)::date AS ultima,
       count(v.id) FILTER (WHERE v.achou_deposito)::int AS com_deposito
FROM locais l
JOIN bairros b ON b.id = l.bairro_id
JOIN quarteiroes q ON q.id = l.quarteirao_id
LEFT JOIN visitas v ON v.local_id = l.id AND v.deleted_at IS NULL AND v.ativo
     AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
     AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
     AND (sqlc.arg(pendencia)::text = '' OR v.pendencia = sqlc.arg(pendencia)::text)
     AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
WHERE l.deleted_at IS NULL AND l.ativo
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
GROUP BY b.nome, q.nome, l.id
HAVING (sqlc.arg(situacao)::text = '' OR (sqlc.arg(situacao)::text = 'sim') = (count(v.id) > 0))
ORDER BY b.nome, q.nome, l.endereco, l.numero
LIMIT 20001;

-- name: RelProducao :many
SELECT u.nome AS agente, count(*)::int AS visitas, count(DISTINCT v.local_id)::int AS locais,
       count(*) FILTER (WHERE v.pendencia = 'fechado')::int AS fechadas,
       count(*) FILTER (WHERE v.pendencia = 'recusado')::int AS recusadas,
       count(*) FILTER (WHERE v.pendencia = 'tentativa')::int AS tentativas,
       count(*) FILTER (WHERE v.achou_deposito)::int AS com_deposito,
       count(*) FILTER (WHERE v.coletou_amostra)::int AS com_amostra,
       count(*) FILTER (WHERE v.fez_tratamento)::int AS com_tratamento,
       count(DISTINCT v.data)::int AS dias
FROM visitas v JOIN usuarios u ON u.id = v.agente_id
WHERE v.deleted_at IS NULL AND v.ativo
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR v.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR v.data <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY u.id, u.nome ORDER BY u.nome;

-- name: RelAntivetorial :many
SELECT a.sigla, a.nome AS atividade, count(*)::int AS visitas, count(DISTINCT v.local_id)::int AS locais,
       COALESCE(sum(COALESCE(v.dep_a1,0) + COALESCE(v.dep_a2,0) + COALESCE(v.dep_b,0) + COALESCE(v.dep_c,0)
                    + COALESCE(v.dep_d1,0) + COALESCE(v.dep_d2,0) + COALESCE(v.dep_e,0)), 0)::int AS depositos,
       COALESCE(sum(v.total_eliminado), 0)::int AS eliminados,
       COALESCE(sum(v.l1_gramas), 0)::float8 AS l1_gramas, COALESCE(sum(v.l1_tratados), 0)::int AS l1_tratados,
       COALESCE(sum(v.l2_gramas), 0)::float8 AS l2_gramas, COALESCE(sum(v.l2_tratados), 0)::int AS l2_tratados,
       COALESCE(sum(v.adulticida_cargas), 0)::int AS cargas
FROM visitas v JOIN atividades a ON a.id = v.atividade_id
WHERE v.deleted_at IS NULL AND v.ativo
  AND (sqlc.narg(atividade_id)::bigint IS NULL OR v.atividade_id = sqlc.narg(atividade_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR v.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR v.data <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY a.id, a.sigla, a.nome ORDER BY a.id;

-- name: RelDiaTrabalhado :many
SELECT u.nome AS agente, v.data, count(*)::int AS visitas, count(DISTINCT v.local_id)::int AS locais,
       min(v.hora_entrada)::time AS primeira, max(v.hora_entrada)::time AS ultima
FROM visitas v JOIN usuarios u ON u.id = v.agente_id
WHERE v.deleted_at IS NULL AND v.ativo
  AND (sqlc.narg(agente_id)::bigint IS NULL OR v.agente_id = sqlc.narg(agente_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR v.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR v.data <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY u.id, u.nome, v.data ORDER BY u.nome, v.data
LIMIT 20001;

-- name: RelLaboratorioTotal :many
SELECT b.nome AS bairro, count(*)::int AS tubitos,
       count(*) FILTER (WHERE a.status = 'em_analise')::int AS em_analise,
       count(*) FILTER (WHERE a.status = 'concluido')::int AS concluidos,
       count(*) FILTER (WHERE a.status = 'concluido' AND a.total_aegypti + a.total_albopictus > 0)::int AS positivos,
       COALESCE(sum(a.total_aegypti), 0)::int AS aegypti, COALESCE(sum(a.total_albopictus), 0)::int AS albopictus,
       COALESCE(sum(a.total_negativo), 0)::int AS negativo
FROM amostras a
JOIN visitas v ON v.id = a.visita_id
JOIN locais l ON l.id = v.local_id
JOIN bairros b ON b.id = l.bairro_id
WHERE a.deleted_at IS NULL AND v.deleted_at IS NULL AND v.ativo
  AND (sqlc.narg(data_de)::date IS NULL OR v.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR v.data <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY b.id, b.nome ORDER BY b.nome;

-- name: RelAnimais :many
SELECT b.nome AS bairro, count(*)::int AS locais,
       COALESCE(sum(l.humanos), 0)::int AS humanos, COALESCE(sum(l.cachorros), 0)::int AS cachorros,
       COALESCE(sum(l.gatos), 0)::int AS gatos, COALESCE(sum(l.roedores), 0)::int AS roedores
FROM locais l JOIN bairros b ON b.id = l.bairro_id
WHERE l.deleted_at IS NULL AND l.ativo
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
GROUP BY b.id, b.nome ORDER BY b.nome;

-- name: RelBRI :many
SELECT v.id, v.data, l.endereco, l.numero, b.nome AS bairro, q.nome AS quarteirao, u.nome AS agente,
       c.nome AS ciclo, v.bri_volume::float8 AS volume
FROM visitas v
JOIN locais l ON l.id = v.local_id
JOIN bairros b ON b.id = l.bairro_id
JOIN quarteiroes q ON q.id = l.quarteirao_id
JOIN usuarios u ON u.id = v.agente_id
JOIN ciclos c ON c.id = v.ciclo_id
WHERE v.deleted_at IS NULL AND v.ativo AND v.bri_volume IS NOT NULL AND v.bri_volume > 0
  AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(data_de)::date IS NULL OR v.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR v.data <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
ORDER BY v.data, v.id
LIMIT 20001;

-- name: RelLocalIncompleto :many
SELECT b.nome AS bairro, q.nome AS quarteirao, l.id, l.endereco, l.numero, l.tipo,
       concat_ws('; ',
           CASE WHEN l.latitude IS NULL THEN 'sem coordenada' END,
           CASE WHEN l.cep = '' THEN 'sem CEP' END,
           CASE WHEN l.tipo = 'residencial' AND l.proprietario = '' THEN 'sem proprietário' END,
           CASE WHEN l.tipo = 'residencial' AND l.humanos IS NULL THEN 'sem moradores' END) AS faltas
FROM locais l
JOIN bairros b ON b.id = l.bairro_id
JOIN quarteiroes q ON q.id = l.quarteirao_id
WHERE l.deleted_at IS NULL AND l.ativo
  AND (l.latitude IS NULL OR l.cep = ''
       OR (l.tipo = 'residencial' AND (l.proprietario = '' OR l.humanos IS NULL)))
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.arg(tipo)::text = '' OR l.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
ORDER BY b.nome, q.nome, l.endereco, l.numero
LIMIT 20001;

-- name: RelInvestigacaoTotal :many
SELECT d.nome AS doenca, count(*)::int AS total,
       count(*) FILTER (WHERE i.situacao = 'aguardando')::int AS aguardando,
       count(*) FILTER (WHERE i.situacao = 'em_atendimento')::int AS em_atendimento,
       count(*) FILTER (WHERE i.situacao = 'encerrada')::int AS encerradas,
       count(*) FILTER (WHERE i.resultado = 'positivo')::int AS positivos,
       count(*) FILTER (WHERE i.resultado = 'negativo')::int AS negativos,
       count(*) FILTER (WHERE i.resultado = 'inconclusivo')::int AS inconclusivos,
       count(*) FILTER (WHERE i.resultado = 'aguardando')::int AS sem_resultado
FROM investigacoes i JOIN doencas d ON d.id = i.doenca_id
WHERE i.deleted_at IS NULL
  AND (sqlc.narg(data_de)::date IS NULL OR COALESCE(i.data_inicio, i.criado_em::date) >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR COALESCE(i.data_inicio, i.criado_em::date) <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint)
GROUP BY d.id, d.nome ORDER BY count(*) DESC, d.nome;

-- name: RelInvestigacaoSinan :many
SELECT i.id, COALESCE(i.id_externo, '')::text AS id_origem, d.nome AS agravo, i.data_inicio, i.paciente, i.telefone,
       COALESCE(NULLIF(btrim(l.endereco || ', ' || l.numero), ','), '')::text AS logradouro_local,
       COALESCE(b.nome, '')::text AS bairro_local, i.endereco_texto, i.situacao, i.resultado, i.encerrada_em,
       COALESCE(u.nome, '')::text AS responsavel, i.observacao
FROM investigacoes i
JOIN doencas d ON d.id = i.doenca_id
LEFT JOIN locais l ON l.id = i.local_id
LEFT JOIN bairros b ON b.id = l.bairro_id
LEFT JOIN usuarios u ON u.id = i.responsavel_id
WHERE i.deleted_at IS NULL
  AND (sqlc.narg(data_de)::date IS NULL OR COALESCE(i.data_inicio, i.criado_em::date) >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR COALESCE(i.data_inicio, i.criado_em::date) <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR i.criado_por = sqlc.narg(somente_usuario)::bigint OR i.responsavel_id = sqlc.narg(somente_usuario)::bigint)
ORDER BY COALESCE(i.data_inicio, i.criado_em::date), i.id
LIMIT 20001;

-- name: RelCampanha :many
-- Campanha antirrábica por bairro: atendimentos, animais vacinados e não vacinados, recusas e fechadas.
SELECT b.nome AS bairro, count(*)::int AS atendimentos,
       COALESCE(sum(c.vac_cachorros), 0)::int AS vac_cachorros, COALESCE(sum(c.vac_gatos), 0)::int AS vac_gatos,
       COALESCE(sum(c.nao_cachorros), 0)::int AS nao_cachorros, COALESCE(sum(c.nao_gatos), 0)::int AS nao_gatos,
       count(*) FILTER (WHERE c.situacao = 'recusa')::int AS recusas,
       count(*) FILTER (WHERE c.situacao IN ('fechada', 'responsavel_ausente'))::int AS fechadas
FROM campanhas c
JOIN locais l ON l.id = c.local_id
JOIN bairros b ON b.id = l.bairro_id
WHERE c.deleted_at IS NULL
  AND (sqlc.narg(data_de)::date IS NULL OR (c.data_hora AT TIME ZONE sqlc.arg(fuso)::text)::date >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR (c.data_hora AT TIME ZONE sqlc.arg(fuso)::text)::date <= sqlc.narg(data_ate)::date)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
GROUP BY b.id, b.nome
ORDER BY b.nome;

-- name: RelLiraa :many
-- LIRAa por bairro: agenda de quarteirões (programados e encerrados) e o boletim de campo e laboratório das
-- visitas da atividade LIRAa no ciclo. Imóvel pesquisado = visita sem pendência; positivo = tubito concluído com Aedes.
WITH ativ AS (SELECT id FROM atividades WHERE sigla = 'LIRAa'),
v AS (
    SELECT v.id, v.local_id, v.pendencia, v.achou_deposito, l.bairro_id,
           COALESCE(v.dep_a1,0)+COALESCE(v.dep_a2,0)+COALESCE(v.dep_b,0)+COALESCE(v.dep_c,0)+COALESCE(v.dep_d1,0)+COALESCE(v.dep_d2,0)+COALESCE(v.dep_e,0) AS depositos
    FROM visitas v JOIN locais l ON l.id = v.local_id
    WHERE v.deleted_at IS NULL AND v.ativo AND v.atividade_id IN (SELECT id FROM ativ)
      AND (sqlc.narg(ciclo_id)::bigint IS NULL OR v.ciclo_id = sqlc.narg(ciclo_id)::bigint)
      AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
      AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
),
camp AS (
    SELECT bairro_id,
           count(DISTINCT local_id) FILTER (WHERE pendencia = 'nenhuma')::int AS pesquisados,
           count(*) FILTER (WHERE pendencia <> 'nenhuma')::int AS pendencias,
           COALESCE(sum(depositos) FILTER (WHERE pendencia = 'nenhuma'), 0)::int AS depositos
    FROM v GROUP BY bairro_id
),
lab AS (
    SELECT v.bairro_id, count(DISTINCT v.local_id)::int AS positivos, count(*)::int AS recipientes
    FROM v JOIN amostras a ON a.visita_id = v.id
    WHERE v.pendencia = 'nenhuma' AND a.deleted_at IS NULL AND a.status = 'concluido' AND (a.total_aegypti + a.total_albopictus) > 0
    GROUP BY v.bairro_id
),
ag AS (
    SELECT g.bairro_id,
           count(*) FILTER (WHERE g.situacao <> 'desativado')::int AS programados,
           count(*) FILTER (WHERE g.situacao = 'encerrado')::int AS encerrados
    FROM liraa_agenda g
    WHERE g.deleted_at IS NULL
      AND (sqlc.narg(ciclo_id)::bigint IS NULL OR g.ciclo_id = sqlc.narg(ciclo_id)::bigint)
      AND (sqlc.narg(bairro_id)::bigint IS NULL OR g.bairro_id = sqlc.narg(bairro_id)::bigint)
    GROUP BY g.bairro_id
)
SELECT b.nome AS bairro,
       COALESCE(ag.programados, 0)::int AS programados, COALESCE(ag.encerrados, 0)::int AS encerrados,
       COALESCE(camp.pesquisados, 0)::int AS pesquisados, COALESCE(camp.pendencias, 0)::int AS pendencias,
       COALESCE(camp.depositos, 0)::int AS depositos,
       COALESCE(lab.positivos, 0)::int AS positivos, COALESCE(lab.recipientes, 0)::int AS recipientes
FROM bairros b
LEFT JOIN camp ON camp.bairro_id = b.id
LEFT JOIN lab ON lab.bairro_id = b.id
LEFT JOIN ag ON ag.bairro_id = b.id
WHERE b.deleted_at IS NULL
  AND (camp.bairro_id IS NOT NULL OR ag.bairro_id IS NOT NULL)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_bairro(sqlc.narg(somente_usuario)::bigint, b.id))
ORDER BY b.nome;

-- name: RelCriticidade :many
-- Locais críticos: com visitas com depósito encontrado nos últimos 12 meses (a partir do mínimo informado) e
-- tubitos positivos; os mais graves primeiro.
SELECT b.nome AS bairro, q.nome AS quarteirao, l.endereco, l.numero, l.tipo,
       count(v.id) FILTER (WHERE v.achou_deposito)::int AS com_deposito,
       COALESCE(sum(pos.tubitos), 0)::int AS positivos,
       max(v.data) FILTER (WHERE v.achou_deposito)::date AS ultima
FROM locais l
JOIN bairros b ON b.id = l.bairro_id
JOIN quarteiroes q ON q.id = l.quarteirao_id
JOIN visitas v ON v.local_id = l.id AND v.deleted_at IS NULL AND v.ativo AND v.data >= (CURRENT_DATE - interval '12 months')
LEFT JOIN LATERAL (
    SELECT count(*) AS tubitos FROM amostras a
    WHERE a.visita_id = v.id AND a.deleted_at IS NULL AND a.status = 'concluido' AND (a.total_aegypti + a.total_albopictus) > 0
) pos ON true
WHERE l.deleted_at IS NULL AND l.ativo
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
  AND (sqlc.narg(somente_agente)::bigint IS NULL OR v.agente_id = sqlc.narg(somente_agente)::bigint)
GROUP BY l.id, b.nome, q.nome
HAVING count(v.id) FILTER (WHERE v.achou_deposito) >= sqlc.arg(minimo)::int
ORDER BY positivos DESC, com_deposito DESC, b.nome, q.nome, l.endereco, l.numero
LIMIT 20001;

-- name: RelLocaisParaPapel :many
-- Lista de locais ativos, por quarteirão, para imprimir e preencher a mão em campo.
SELECT b.nome AS bairro, q.nome AS quarteirao, l.endereco, l.numero, l.tipo
FROM locais l
JOIN bairros b ON b.id = l.bairro_id
JOIN quarteiroes q ON q.id = l.quarteirao_id
WHERE l.deleted_at IS NULL AND l.ativo
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.arg(tipo)::text = '' OR l.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR usuario_ve_local(sqlc.narg(somente_usuario)::bigint, l.id, l.bairro_id, l.quarteirao_id))
ORDER BY b.nome, q.nome, l.endereco, l.numero
LIMIT 20001;

-- ---------------------------------------------------------------- Importações

-- name: CreateImportacao :one
INSERT INTO importacoes (tipo, arquivo_nome, arquivo_hash, atualizar_existentes, status, total_linhas, prev_criar, prev_atualizar,
                         prev_ignorar, prev_erro, colunas_ignoradas, responsavel_id, responsavel_nome, cabecalho)
VALUES ($1, $2, $3, $4, 'analisada', $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetImportacao :one
SELECT * FROM importacoes WHERE id = $1;

-- name: ListImportacoes :many
SELECT * FROM importacoes ORDER BY id DESC LIMIT 100;

-- name: SetImportacaoStatus :exec
UPDATE importacoes SET status = $2, mensagem = $3 WHERE id = $1;

-- name: IniciarImportacao :execrows
UPDATE importacoes SET status = 'executando', iniciado_em = COALESCE(iniciado_em, now()), mensagem = ''
WHERE id = $1 AND status IN ('analisada', 'interrompida');

-- name: AtualizarProgressoImportacao :exec
UPDATE importacoes SET processadas = $2, criados = $3, atualizados = $4, ignorados = $5, erros = $6 WHERE id = $1;

-- name: ConcluirImportacao :exec
UPDATE importacoes SET status = $2, concluido_em = now(), mensagem = $3 WHERE id = $1;

-- name: DesfazerImportacaoConcluida :exec
UPDATE importacoes SET status = 'desfeita', desfeito_em = now(), desfeitos = $2, mensagem = $3 WHERE id = $1;

-- name: ContarImportacoesEmExecucao :one
SELECT count(*)::int FROM importacoes WHERE status = 'executando';

-- name: InsertImportacaoLinha :exec
INSERT INTO importacao_linhas (importacao_id, linha, acao, mensagem, alvo_id, dados, antes, original)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListImportacaoLinhas :many
SELECT * FROM importacao_linhas
WHERE importacao_id = sqlc.arg(importacao_id)::bigint
  AND (sqlc.arg(acao)::text = '' OR acao = sqlc.arg(acao)::text)
ORDER BY linha
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: CountImportacaoLinhas :one
SELECT count(*)::int FROM importacao_linhas
WHERE importacao_id = sqlc.arg(importacao_id)::bigint AND (sqlc.arg(acao)::text = '' OR acao = sqlc.arg(acao)::text);

-- name: ListImportacaoLinhasPendentes :many
SELECT * FROM importacao_linhas
WHERE importacao_id = $1 AND resultado IS NULL AND acao <> 'erro'
ORDER BY linha
LIMIT $2;

-- name: SetImportacaoLinhaResultado :exec
UPDATE importacao_linhas SET resultado = $3, resultado_msg = $4, registro_id = $5 WHERE importacao_id = $1 AND linha = $2;

-- name: ListImportacaoLinhasParaDesfazer :many
SELECT * FROM importacao_linhas
WHERE importacao_id = $1 AND resultado IN ('criado', 'atualizado')
ORDER BY linha DESC;

-- name: ResumoImportacaoLinhas :many
SELECT COALESCE(resultado, 'pendente')::text AS resultado, count(*)::int AS total
FROM importacao_linhas WHERE importacao_id = $1 GROUP BY 1;

-- ---------------------------------------------------------------- Importações: buscas auxiliares

-- name: ListCidadesParaImportacao :many
SELECT id, nome, uf, ativo FROM cidades WHERE deleted_at IS NULL;

-- name: ListBairrosParaImportacao :many
SELECT id, cidade_id, nome, id_externo FROM bairros WHERE deleted_at IS NULL;

-- name: ListQuarteiroesParaImportacao :many
SELECT id, bairro_id, nome, ativo, id_externo FROM quarteiroes WHERE deleted_at IS NULL;

-- name: ListLocaisParaImportacao :many
SELECT id, quarteirao_id, lower(endereco)::text AS endereco, lower(numero)::text AS numero, lower(lado)::text AS lado,
       COALESCE(sequencia, 0)::int AS sequencia, COALESCE(complemento, 0)::int AS complemento, id_externo
FROM locais WHERE deleted_at IS NULL;

-- name: MarcarLinhasComErroDaImportacao :execrows
UPDATE importacao_linhas SET resultado = 'erro', resultado_msg = mensagem
WHERE importacao_id = $1 AND acao = 'erro' AND resultado IS NULL;

-- name: ListImportacaoLinhasRelatorio :many
SELECT linha, acao, mensagem, resultado, resultado_msg, registro_id, original
FROM importacao_linhas WHERE importacao_id = $1 AND (sqlc.arg(so_erros)::boolean = false OR acao = 'erro' OR resultado = 'erro')
ORDER BY linha;

-- name: ContarImportacaoLinhasAFazer :one
SELECT count(*)::int FROM importacao_linhas WHERE importacao_id = $1 AND resultado IS NULL AND acao IN ('criar', 'atualizar');

-- ---------------------------------------------------------------- Conectores e mapeamentos

-- name: CreateImportacaoParaMapear :one
INSERT INTO importacoes (tipo, conector, arquivo_nome, arquivo_hash, atualizar_existentes, status, total_linhas, colunas_ignoradas,
                         responsavel_id, responsavel_nome, cabecalho, arquivo, origem_info)
VALUES ($1, $2, $3, $4, $5, 'mapeando', $6, '', $7, $8, $9, $10, $11)
RETURNING *;

-- name: FinalizarAnaliseDaImportacao :one
UPDATE importacoes
SET status = 'analisada', arquivo = NULL, total_linhas = $2, prev_criar = $3, prev_atualizar = $4, prev_ignorar = $5, prev_erro = $6, colunas_ignoradas = $7
WHERE id = $1 AND status = 'mapeando'
RETURNING *;

-- name: ListMapeamentosDeImportacao :many
SELECT * FROM importacao_mapeamentos WHERE conector = $1 AND tipo = $2 AND deleted_at IS NULL ORDER BY lower(nome);

-- name: ListTodosMapeamentosDeImportacao :many
SELECT * FROM importacao_mapeamentos WHERE deleted_at IS NULL ORDER BY conector, tipo, lower(nome);

-- name: GetMapeamentoDeImportacao :one
SELECT * FROM importacao_mapeamentos WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateMapeamentoDeImportacao :one
INSERT INTO importacao_mapeamentos (nome, conector, tipo, mapa, criado_por) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: SoftDeleteMapeamento :execrows
UPDATE importacao_mapeamentos SET deleted_at = now(), deleted_by = sqlc.narg(por)::bigint WHERE id = sqlc.arg(id)::bigint AND deleted_at IS NULL;

-- name: RestoreMapeamento :execrows
UPDATE importacao_mapeamentos SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteMapeamento :execrows
DELETE FROM importacao_mapeamentos WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: GetMapeamentoQualquer :one
SELECT * FROM importacao_mapeamentos WHERE id = $1;

-- name: ListMapeamentosExcluidos :many
SELECT m.id, m.nome AS rotulo, (m.conector || ' · ' || m.tipo)::text AS contexto, m.deleted_at, COALESCE(u.nome, '')::text AS excluido_por
FROM importacao_mapeamentos m LEFT JOIN usuarios u ON u.id = m.deleted_by
WHERE m.deleted_at IS NOT NULL ORDER BY m.deleted_at DESC LIMIT 500;

-- name: CancelarImportacao :exec
UPDATE importacoes SET status = 'cancelada', arquivo = NULL, mensagem = $2 WHERE id = $1 AND status IN ('analisada', 'mapeando');

-- name: ListImportacaoLinhasApi :many
SELECT linha, acao, mensagem, resultado, resultado_msg, registro_id FROM importacao_linhas
WHERE importacao_id = sqlc.arg(importacao_id)::bigint AND linha > sqlc.arg(depois)::int
  AND (sqlc.arg(acao)::text = '' OR acao = sqlc.arg(acao)::text)
ORDER BY linha LIMIT sqlc.arg(limite)::int;

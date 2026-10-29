-- ---------------------------------------------------------------- Locais

-- name: ListLocais :many
SELECT l.id, l.tipo, l.endereco, l.numero, l.lado, l.sequencia, l.complemento, l.proprietario, l.ativo,
       l.humanos, b.nome AS bairro_nome, q.nome AS quarteirao_nome
FROM locais l
JOIN bairros b ON b.id = l.bairro_id
JOIN quarteiroes q ON q.id = l.quarteirao_id
WHERE (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(l.endereco || ' ' || l.numero || ' ' || l.proprietario)) > 0)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.arg(tipo)::text = '' OR l.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.arg(situacao)::text = '' OR l.ativo = (sqlc.arg(situacao)::text = 'ativo'))
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR EXISTS (
        SELECT 1 FROM bairro_responsaveis r WHERE r.bairro_id = l.bairro_id AND r.usuario_id = sqlc.narg(somente_usuario)::bigint))
  AND (NOT sqlc.arg(somente_pe)::boolean OR l.tipo = 'ponto_estrategico')
  AND (sqlc.arg(ver_desativados)::boolean OR l.ativo)
ORDER BY lower(b.nome), lower(l.endereco), l.numero, l.id
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: CountLocais :one
SELECT count(*)::int
FROM locais l
WHERE (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(l.endereco || ' ' || l.numero || ' ' || l.proprietario)) > 0)
  AND (sqlc.narg(bairro_id)::bigint IS NULL OR l.bairro_id = sqlc.narg(bairro_id)::bigint)
  AND (sqlc.arg(tipo)::text = '' OR l.tipo = sqlc.arg(tipo)::text)
  AND (sqlc.arg(situacao)::text = '' OR l.ativo = (sqlc.arg(situacao)::text = 'ativo'))
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR EXISTS (
        SELECT 1 FROM bairro_responsaveis r WHERE r.bairro_id = l.bairro_id AND r.usuario_id = sqlc.narg(somente_usuario)::bigint))
  AND (NOT sqlc.arg(somente_pe)::boolean OR l.tipo = 'ponto_estrategico')
  AND (sqlc.arg(ver_desativados)::boolean OR l.ativo);

-- name: GetLocal :one
SELECT * FROM locais WHERE id = $1;

-- name: CreateLocal :one
INSERT INTO locais (tipo, cep, endereco, numero, referencia, bairro_id, quarteirao_id, lado, sequencia, complemento,
                    latitude, longitude, proprietario, proprietario_telefone, proprietario_cpf, proprietario_whatsapp,
                    proprietario_email, aceita_whatsapp, aceita_email, inquilino, inquilino_telefone,
                    humanos, cachorros, gatos, roedores, criado_por)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)
RETURNING *;

-- name: UpdateLocal :one
UPDATE locais
SET tipo = $2, cep = $3, endereco = $4, numero = $5, referencia = $6, bairro_id = $7, quarteirao_id = $8, lado = $9,
    sequencia = $10, complemento = $11, latitude = $12, longitude = $13, proprietario = $14, proprietario_telefone = $15,
    proprietario_cpf = $16, proprietario_whatsapp = $17, proprietario_email = $18, aceita_whatsapp = $19,
    aceita_email = $20, inquilino = $21, inquilino_telefone = $22, humanos = $23, cachorros = $24, gatos = $25,
    roedores = $26, atualizado_em = now()
WHERE id = $1
RETURNING *;

-- name: ListQuarteiroesAtivosDoBairro :many
SELECT id, nome FROM quarteiroes WHERE bairro_id = $1 AND ativo
ORDER BY NULLIF(regexp_replace(nome, '\D', '', 'g'), '')::bigint NULLS LAST, lower(nome);

-- name: ListBairrosAtivosParaLocal :many
SELECT b.id, b.nome, c.nome AS cidade_nome, c.uf AS cidade_uf
FROM bairros b JOIN cidades c ON c.id = b.cidade_id
WHERE b.status = 'ativo'
  AND (sqlc.narg(somente_usuario)::bigint IS NULL OR EXISTS (
        SELECT 1 FROM bairro_responsaveis r WHERE r.bairro_id = b.id AND r.usuario_id = sqlc.narg(somente_usuario)::bigint))
ORDER BY lower(b.nome);

-- name: SetLocalAtivo :execrows
UPDATE locais SET ativo = $2, atualizado_em = now() WHERE id = $1;

-- name: CreateLocalLote :execrows
INSERT INTO locais (tipo, endereco, numero, bairro_id, quarteirao_id, lado, sequencia, criado_por)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT DO NOTHING;

-- name: NovaChaveDoLocal :one
UPDATE locais SET qr_chave = substr(replace(gen_random_uuid()::text, '-', ''), 1, 20), atualizado_em = now()
WHERE id = $1
RETURNING qr_chave;

-- name: GetLocalPorChave :one
SELECT * FROM locais WHERE qr_chave = $1;

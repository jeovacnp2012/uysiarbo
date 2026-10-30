-- ---------------------------------------------------------------- Comunicações (12.1)

-- name: CreateComunicacao :one
INSERT INTO comunicacoes (canal, destinatario, destinatario_nome, assunto, corpo, origem, origem_id, criado_por)
VALUES (sqlc.arg(canal), sqlc.arg(destinatario), sqlc.arg(destinatario_nome), sqlc.arg(assunto), sqlc.arg(corpo), sqlc.arg(origem), sqlc.narg(origem_id), sqlc.narg(criado_por))
RETURNING *;

-- name: GetComunicacao :one
SELECT * FROM comunicacoes WHERE id = $1;

-- name: GetComunicacaoParaAtualizar :one
SELECT * FROM comunicacoes WHERE id = $1 FOR UPDATE;

-- name: CountComunicacoes :one
SELECT count(*)::int FROM comunicacoes c
WHERE (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(c.destinatario || ' ' || c.destinatario_nome || ' ' || c.assunto || ' ' || c.corpo)) > 0)
  AND (sqlc.arg(canal)::text = '' OR c.canal = sqlc.arg(canal)::text)
  AND (sqlc.arg(status)::text = '' OR c.status = sqlc.arg(status)::text);

-- name: ListComunicacoes :many
SELECT c.id, c.canal, c.destinatario, c.destinatario_nome, c.assunto, c.corpo, c.status, c.tentativas, c.max_tentativas, c.criado_em
FROM comunicacoes c
WHERE (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(c.destinatario || ' ' || c.destinatario_nome || ' ' || c.assunto || ' ' || c.corpo)) > 0)
  AND (sqlc.arg(canal)::text = '' OR c.canal = sqlc.arg(canal)::text)
  AND (sqlc.arg(status)::text = '' OR c.status = sqlc.arg(status)::text)
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN c.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN c.id END DESC,
  c.id DESC
LIMIT sqlc.arg(limite) OFFSET sqlc.arg(deslocamento);

-- name: ListComunicacoesVencidas :many
SELECT id FROM comunicacoes WHERE status = 'fila' AND proxima_tentativa <= now() ORDER BY proxima_tentativa, id LIMIT $1;

-- name: SetComunicacaoEnviada :exec
UPDATE comunicacoes SET status = 'enviada', tentativas = tentativas + 1, provedor = sqlc.arg(provedor), id_provedor = sqlc.arg(id_provedor),
       enviada_em = now(), ultimo_erro = '', atualizado_em = now()
WHERE id = sqlc.arg(id);

-- name: SetComunicacaoFalha :one
UPDATE comunicacoes SET tentativas = tentativas + 1, ultimo_erro = sqlc.arg(erro), provedor = sqlc.arg(provedor),
       status = CASE WHEN tentativas + 1 >= max_tentativas THEN 'falhou' ELSE 'fila' END,
       proxima_tentativa = now() + (interval '5 minutes' * (tentativas + 1)), atualizado_em = now()
WHERE id = sqlc.arg(id)
RETURNING status;

-- name: SetComunicacaoNaFila :exec
UPDATE comunicacoes SET status = 'fila', tentativas = 0, ultimo_erro = '', proxima_tentativa = now(), atualizado_em = now() WHERE id = $1;

-- name: SetComunicacaoCancelada :exec
UPDATE comunicacoes SET status = 'cancelada', atualizado_em = now() WHERE id = $1;

-- name: SetComunicacaoEntregue :exec
UPDATE comunicacoes SET status = CASE WHEN status = 'visualizada' THEN status ELSE 'entregue' END, entregue_em = COALESCE(entregue_em, now()), atualizado_em = now() WHERE id = $1;

-- name: SetComunicacaoVisualizada :exec
UPDATE comunicacoes SET status = 'visualizada', entregue_em = COALESCE(entregue_em, now()), visualizada_em = COALESCE(visualizada_em, now()), atualizado_em = now() WHERE id = $1;

-- name: CreateComunicacaoEvento :exec
INSERT INTO comunicacao_eventos (comunicacao_id, tipo, descricao, usuario_id) VALUES (sqlc.arg(comunicacao_id), sqlc.arg(tipo), sqlc.arg(descricao), sqlc.narg(usuario_id));

-- name: ListComunicacaoEventos :many
SELECT e.id, e.tipo, e.descricao, e.criado_em, COALESCE(u.nome, '')::text AS por
FROM comunicacao_eventos e LEFT JOIN usuarios u ON u.id = e.usuario_id
WHERE e.comunicacao_id = $1 ORDER BY e.id;

-- name: CountComunicacoesEmFila :one
SELECT count(*)::int FROM comunicacoes WHERE status = 'fila';

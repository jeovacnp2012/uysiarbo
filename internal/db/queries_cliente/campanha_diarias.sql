-- ---------------------------------------------------------------- Diária de trabalho da campanha (10.3)

-- name: CreateDiaria :one
INSERT INTO campanha_diarias (data, anotacao, criado_por) VALUES (sqlc.arg(data), sqlc.arg(anotacao), sqlc.narg(criado_por)) RETURNING *;

-- name: GetDiaria :one
SELECT * FROM campanha_diarias WHERE id = $1 AND deleted_at IS NULL;

-- name: GetDiariaQualquer :one
SELECT * FROM campanha_diarias WHERE id = $1;

-- name: UpdateDiariaDados :execrows
UPDATE campanha_diarias SET data = sqlc.arg(data), anotacao = sqlc.arg(anotacao), atualizado_em = now() WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SetDiariaStatus :execrows
UPDATE campanha_diarias SET status = sqlc.arg(status), atualizado_em = now() WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: UpdateDiariaRegistro :execrows
UPDATE campanha_diarias SET
    ini_seringas = sqlc.arg(ini_seringas), ini_frascos_fechados = sqlc.arg(ini_frascos_fechados), ini_frascos_abertos = sqlc.arg(ini_frascos_abertos), ini_doses = sqlc.arg(ini_doses),
    fim_seringas_usadas = sqlc.arg(fim_seringas_usadas), fim_seringas_perdidas = sqlc.arg(fim_seringas_perdidas), fim_seringas_defeito = sqlc.arg(fim_seringas_defeito),
    fim_frascos_vazios = sqlc.arg(fim_frascos_vazios), fim_frascos_cheios = sqlc.arg(fim_frascos_cheios), fim_frascos_abertos = sqlc.arg(fim_frascos_abertos),
    fim_vacinas_perdidas = sqlc.arg(fim_vacinas_perdidas),
    def_seringas_defeito = sqlc.arg(def_seringas_defeito), def_seringas_descartadas = sqlc.arg(def_seringas_descartadas),
    def_vacinas_descartadas = sqlc.arg(def_vacinas_descartadas), def_vacinas_perdidas = sqlc.arg(def_vacinas_perdidas),
    temperaturas = sqlc.arg(temperaturas)::jsonb, atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountDiarias :one
SELECT count(*)::int FROM campanha_diarias d
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(status)::text = '' OR d.status = sqlc.arg(status)::text)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR EXISTS (SELECT 1 FROM campanha_diaria_equipe e WHERE e.diaria_id = d.id AND e.usuario_id = sqlc.narg(agente_id)::bigint))
  AND (sqlc.narg(data_de)::date IS NULL OR d.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR d.data <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_meu)::boolean OR EXISTS (SELECT 1 FROM campanha_diaria_equipe e WHERE e.diaria_id = d.id AND e.usuario_id = sqlc.arg(usuario_id)::bigint));

-- name: ListDiarias :many
SELECT d.id, d.data, d.status, COALESCE(c.nome, '')::text AS por,
       COALESCE((SELECT string_agg(u.nome, ', ' ORDER BY e.lider DESC, u.nome) FROM campanha_diaria_equipe e JOIN usuarios u ON u.id = e.usuario_id WHERE e.diaria_id = d.id), '')::text AS equipe
FROM campanha_diarias d LEFT JOIN usuarios c ON c.id = d.criado_por
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(status)::text = '' OR d.status = sqlc.arg(status)::text)
  AND (sqlc.narg(agente_id)::bigint IS NULL OR EXISTS (SELECT 1 FROM campanha_diaria_equipe e WHERE e.diaria_id = d.id AND e.usuario_id = sqlc.narg(agente_id)::bigint))
  AND (sqlc.narg(data_de)::date IS NULL OR d.data >= sqlc.narg(data_de)::date)
  AND (sqlc.narg(data_ate)::date IS NULL OR d.data <= sqlc.narg(data_ate)::date)
  AND (NOT sqlc.arg(somente_meu)::boolean OR EXISTS (SELECT 1 FROM campanha_diaria_equipe e WHERE e.diaria_id = d.id AND e.usuario_id = sqlc.arg(usuario_id)::bigint))
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN d.id END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'desc' THEN d.id END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'asc' THEN d.data END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'data' AND sqlc.arg(dir)::text = 'desc' THEN d.data END DESC,
  d.data DESC, d.id DESC
LIMIT sqlc.arg(limite)::int OFFSET sqlc.arg(deslocamento)::int;

-- name: ListDiariaEquipe :many
SELECT e.usuario_id, e.lider, u.nome FROM campanha_diaria_equipe e JOIN usuarios u ON u.id = e.usuario_id WHERE e.diaria_id = $1 ORDER BY e.lider DESC, u.nome;

-- name: UpsertDiariaMembro :exec
INSERT INTO campanha_diaria_equipe (diaria_id, usuario_id, lider) VALUES ($1, $2, $3)
ON CONFLICT (diaria_id, usuario_id) DO UPDATE SET lider = EXCLUDED.lider;

-- name: RemoveDiariaMembro :execrows
DELETE FROM campanha_diaria_equipe WHERE diaria_id = $1 AND usuario_id = $2;

-- name: DiariaTemMembro :one
SELECT EXISTS (SELECT 1 FROM campanha_diaria_equipe WHERE diaria_id = $1 AND usuario_id = $2);

-- name: ListDiariasAbertas :many
SELECT id, data FROM campanha_diarias WHERE deleted_at IS NULL AND status IN ('aguardando', 'executando') ORDER BY data DESC, id DESC LIMIT 200;

-- name: ListCampanhasDaDiaria :many
SELECT c.id, c.data_hora, c.vac_cachorros, c.vac_gatos, c.nao_cachorros, c.nao_gatos, (l.endereco || ', ' || l.numero)::text AS endereco
FROM campanhas c JOIN locais l ON l.id = c.local_id
WHERE c.diaria_id = $1 AND c.deleted_at IS NULL ORDER BY c.data_hora, c.id;

-- name: SetCampanhaDiaria :execrows
UPDATE campanhas SET diaria_id = sqlc.narg(diaria_id) WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- ---- exclusão lógica

-- name: SoftDeleteDiaria :execrows
UPDATE campanha_diarias SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreDiaria :execrows
UPDATE campanha_diarias SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteDiaria :execrows
DELETE FROM campanha_diarias WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListDiariasExcluidas :many
SELECT d.id, d.data, d.deleted_at, COALESCE(u.nome, '')::text AS por
FROM campanha_diarias d LEFT JOIN usuarios u ON u.id = d.deleted_by
WHERE d.deleted_at IS NOT NULL ORDER BY d.deleted_at DESC LIMIT 500;

-- name: CountCampanhasDaDiariaQualquer :one
SELECT count(*)::int FROM campanhas WHERE diaria_id = $1;

-- name: CountCampanhasAtivasDaDiaria :one
SELECT count(*)::int FROM campanhas WHERE diaria_id = $1 AND deleted_at IS NULL;

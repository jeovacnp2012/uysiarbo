-- ---------------------------------------------------------------- Banners (12.4)

-- name: CreateBanner :one
INSERT INTO banners (nome, largura, altura, cor_fundo, definicao, ativo, criado_por)
VALUES (sqlc.arg(nome), sqlc.arg(largura), sqlc.arg(altura), sqlc.arg(cor_fundo), sqlc.arg(definicao), sqlc.arg(ativo), sqlc.narg(criado_por))
RETURNING id, nome, largura, altura, cor_fundo, fundo_tipo, definicao, ativo, criado_em, atualizado_em;

-- name: GetBanner :one
SELECT id, nome, largura, altura, cor_fundo, fundo_tipo, definicao, ativo, criado_em, atualizado_em
FROM banners WHERE id = $1 AND deleted_at IS NULL;

-- name: GetBannerQualquer :one
SELECT id, nome, deleted_at FROM banners WHERE id = $1;

-- name: GetBannerFundo :one
SELECT fundo_tipo, fundo FROM banners WHERE id = $1 AND deleted_at IS NULL AND fundo IS NOT NULL;

-- name: UpdateBanner :execrows
UPDATE banners SET nome = sqlc.arg(nome), largura = sqlc.arg(largura), altura = sqlc.arg(altura), cor_fundo = sqlc.arg(cor_fundo),
       definicao = sqlc.arg(definicao), ativo = sqlc.arg(ativo), atualizado_em = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SetBannerFundo :execrows
UPDATE banners SET fundo_tipo = sqlc.arg(fundo_tipo), fundo = sqlc.arg(fundo), atualizado_em = now() WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ClearBannerFundo :execrows
UPDATE banners SET fundo_tipo = '', fundo = NULL, atualizado_em = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: CountBanners :one
SELECT count(*)::int FROM banners b
WHERE b.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(b.nome)) > 0)
  AND (sqlc.arg(situacao)::text = '' OR (sqlc.arg(situacao)::text = 'ativo') = b.ativo);

-- name: ListBanners :many
SELECT b.id, b.nome, b.largura, b.altura, b.ativo, (b.fundo IS NOT NULL)::bool AS com_imagem, b.atualizado_em
FROM banners b
WHERE b.deleted_at IS NULL
  AND (sqlc.arg(busca)::text = '' OR position(lower(sqlc.arg(busca)::text) in lower(b.nome)) > 0)
  AND (sqlc.arg(situacao)::text = '' OR (sqlc.arg(situacao)::text = 'ativo') = b.ativo)
ORDER BY
  CASE WHEN sqlc.arg(ordem)::text = 'nome' AND sqlc.arg(dir)::text = 'asc' THEN lower(b.nome) END ASC,
  CASE WHEN sqlc.arg(ordem)::text = 'nome' AND sqlc.arg(dir)::text = 'desc' THEN lower(b.nome) END DESC,
  CASE WHEN sqlc.arg(ordem)::text = 'id' AND sqlc.arg(dir)::text = 'asc' THEN b.id END ASC,
  b.id DESC
LIMIT sqlc.arg(limite) OFFSET sqlc.arg(deslocamento);

-- ---- exclusão lógica

-- name: SoftDeleteBanner :execrows
UPDATE banners SET deleted_at = now(), deleted_by = sqlc.narg(por) WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreBanner :execrows
UPDATE banners SET deleted_at = NULL, deleted_by = NULL WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ForceDeleteBanner :execrows
DELETE FROM banners WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: ListBannersExcluidos :many
SELECT b.id, b.nome, b.deleted_at, COALESCE(u.nome, '')::text AS por
FROM banners b LEFT JOIN usuarios u ON u.id = b.deleted_by
WHERE b.deleted_at IS NOT NULL ORDER BY b.deleted_at DESC LIMIT 500;

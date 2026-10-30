-- +goose Up
-- Banners (12.4): editor visual de imagens (fundo + textos posicionados) para avisos e comunicações.
CREATE TABLE banners (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome          text NOT NULL CHECK (length(btrim(nome)) BETWEEN 1 AND 100),
    largura       int NOT NULL CHECK (largura BETWEEN 100 AND 4000),
    altura        int NOT NULL CHECK (altura BETWEEN 100 AND 4000),
    cor_fundo     text NOT NULL DEFAULT '#155a9e' CHECK (cor_fundo ~ '^#[0-9a-fA-F]{6}$'),
    fundo_tipo    text NOT NULL DEFAULT '' CHECK (fundo_tipo IN ('', 'image/jpeg', 'image/png', 'image/webp')),
    fundo         bytea,
    definicao     jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(definicao) = 'array'),
    ativo         boolean NOT NULL DEFAULT true,
    criado_por    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,
    deleted_by    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    CHECK ((fundo IS NULL) = (fundo_tipo = ''))
);
CREATE UNIQUE INDEX banners_nome_uq ON banners (lower(nome)) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE banners;

-- +goose Up
-- Leis municipais e seus trechos (artigos), usados em visitas, notificações e autos de infração.
-- Roda DENTRO do schema do cliente; não cite o schema aqui.
CREATE TABLE leis_municipais (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    numero        text NOT NULL CHECK (length(btrim(numero)) BETWEEN 1 AND 15),
    ano           smallint NOT NULL CHECK (ano BETWEEN 1900 AND 2100),
    nome          text NOT NULL CHECK (length(btrim(nome)) BETWEEN 2 AND 200),
    publicacao    date,
    ativo         boolean NOT NULL DEFAULT true,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,
    deleted_by    bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX leis_numero_ano_uq ON leis_municipais (lower(numero), ano) WHERE deleted_at IS NULL;
CREATE INDEX leis_excluidas_idx ON leis_municipais (deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TABLE leis_trechos (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lei_id        bigint NOT NULL REFERENCES leis_municipais (id) ON DELETE RESTRICT,
    artigo        text NOT NULL DEFAULT '' CHECK (length(artigo) <= 60),
    texto         text NOT NULL CHECK (length(btrim(texto)) BETWEEN 1 AND 4000),
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,
    deleted_by    bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE INDEX leis_trechos_lei_idx ON leis_trechos (lei_id, id);
CREATE INDEX leis_trechos_excluidos_idx ON leis_trechos (deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP TABLE leis_trechos;
DROP TABLE leis_municipais;

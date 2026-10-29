-- +goose Up
-- Locais: imóveis, comércios, terrenos e pontos estratégicos visitados. A cidade vem do bairro.
CREATE TABLE locais (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tipo          text NOT NULL CHECK (tipo IN ('residencial', 'comercio', 'terreno_baldio', 'ponto_estrategico', 'outro')),
    cep           text NOT NULL DEFAULT '' CHECK (cep ~ '^([0-9]{5}-[0-9]{3})?$'),
    endereco      text NOT NULL CHECK (length(btrim(endereco)) BETWEEN 2 AND 200),
    numero        text NOT NULL CHECK (length(btrim(numero)) BETWEEN 1 AND 15),
    referencia    text NOT NULL DEFAULT '' CHECK (length(referencia) <= 200),
    bairro_id     bigint NOT NULL REFERENCES bairros (id) ON DELETE RESTRICT,
    quarteirao_id bigint NOT NULL REFERENCES quarteiroes (id) ON DELETE RESTRICT,
    lado          text NOT NULL CHECK (length(btrim(lado)) BETWEEN 1 AND 5),
    sequencia     integer CHECK (sequencia >= 0),
    complemento   integer CHECK (complemento >= 0),
    latitude      double precision CHECK (latitude BETWEEN -90 AND 90),
    longitude     double precision CHECK (longitude BETWEEN -180 AND 180),
    -- moradores
    proprietario            text NOT NULL DEFAULT '' CHECK (length(proprietario) <= 150),
    proprietario_telefone   text NOT NULL DEFAULT '' CHECK (length(proprietario_telefone) <= 20),
    proprietario_cpf        text NOT NULL DEFAULT '' CHECK (proprietario_cpf ~ '^([0-9]{11})?$'), -- criptografar na etapa 13.3
    proprietario_whatsapp   text NOT NULL DEFAULT '' CHECK (length(proprietario_whatsapp) <= 20),
    proprietario_email      text NOT NULL DEFAULT '' CHECK (length(proprietario_email) <= 150),
    aceita_whatsapp         boolean NOT NULL DEFAULT false,
    aceita_email            boolean NOT NULL DEFAULT false,
    inquilino               text NOT NULL DEFAULT '' CHECK (length(inquilino) <= 150),
    inquilino_telefone      text NOT NULL DEFAULT '' CHECK (length(inquilino_telefone) <= 20),
    -- habitantes
    humanos       integer CHECK (humanos BETWEEN 0 AND 1000),
    cachorros     integer CHECK (cachorros BETWEEN 0 AND 1000),
    gatos         integer CHECK (gatos BETWEEN 0 AND 1000),
    roedores      integer CHECK (roedores BETWEEN 0 AND 1000),
    ativo         boolean NOT NULL DEFAULT true,
    criado_por    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    CHECK ((latitude IS NULL) = (longitude IS NULL))
);
CREATE UNIQUE INDEX locais_endereco_uq ON locais
    (quarteirao_id, lower(endereco), lower(numero), lower(lado), COALESCE(sequencia, 0), COALESCE(complemento, 0));
CREATE INDEX locais_bairro_idx ON locais (bairro_id, quarteirao_id);
CREATE INDEX locais_endereco_idx ON locais (lower(endereco), numero);
-- O ponto do local no mapa (PostGIS), sem coluna extra: o índice é sobre a expressão.
CREATE INDEX locais_ponto_idx ON locais USING gist ((ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)))
    WHERE latitude IS NOT NULL;

-- +goose Down
DROP TABLE locais;

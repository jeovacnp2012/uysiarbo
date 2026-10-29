-- +goose Up
-- Território do cliente: cidades, bairros (com responsáveis) e quarteirões.
-- Roda DENTRO do schema do cliente; não cite o schema aqui.
CREATE TABLE cidades (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome          text NOT NULL CHECK (length(btrim(nome)) BETWEEN 2 AND 100),
    uf            char(2) NOT NULL CHECK (uf ~ '^[A-Z]{2}$'),
    ibge          text NOT NULL DEFAULT '' CHECK (ibge ~ '^[0-9]{0,7}$'),
    ativo         boolean NOT NULL DEFAULT true,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX cidades_nome_uf_uq ON cidades (lower(nome), uf);

CREATE TABLE bairros (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cidade_id        bigint NOT NULL REFERENCES cidades (id) ON DELETE RESTRICT,
    nome             text NOT NULL CHECK (length(btrim(nome)) BETWEEN 2 AND 120),
    mutirao          boolean NOT NULL DEFAULT false,
    status           text NOT NULL DEFAULT 'ativo' CHECK (status IN ('ativo', 'extinto')),
    ibge             text NOT NULL DEFAULT '' CHECK (length(ibge) <= 20),
    classificacao    text NOT NULL DEFAULT 'urbano' CHECK (classificacao IN ('urbano', 'rural')),
    zona             text NOT NULL DEFAULT '' CHECK (length(zona) <= 30),
    categoria        text NOT NULL DEFAULT 'bairro' CHECK (categoria IN ('bairro', 'povoado')),
    latitude         double precision CHECK (latitude BETWEEN -90 AND 90),
    longitude        double precision CHECK (longitude BETWEEN -180 AND 180),
    altitude         double precision CHECK (altitude BETWEEN -500 AND 9000),
    distancia_centro double precision CHECK (distancia_centro >= 0),
    producao         boolean NOT NULL DEFAULT true, -- entra no relatório de produção?
    criado_em        timestamptz NOT NULL DEFAULT now(),
    atualizado_em    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX bairros_cidade_nome_uq ON bairros (cidade_id, lower(nome));
CREATE INDEX bairros_status_idx ON bairros (status);

-- Quem responde por cada bairro (também define o escopo de quem só vê "os bairros atribuídos").
CREATE TABLE bairro_responsaveis (
    bairro_id  bigint NOT NULL REFERENCES bairros (id) ON DELETE CASCADE,
    usuario_id bigint NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    PRIMARY KEY (bairro_id, usuario_id)
);
CREATE INDEX bairro_responsaveis_usuario_idx ON bairro_responsaveis (usuario_id);

CREATE TABLE quarteiroes (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    bairro_id bigint NOT NULL REFERENCES bairros (id) ON DELETE RESTRICT,
    nome      text NOT NULL CHECK (length(btrim(nome)) BETWEEN 1 AND 30),
    ativo     boolean NOT NULL DEFAULT true,
    criado_em timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX quarteiroes_bairro_nome_uq ON quarteiroes (bairro_id, lower(nome));

-- +goose Down
DROP TABLE quarteiroes;
DROP TABLE bairro_responsaveis;
DROP TABLE bairros;
DROP TABLE cidades;

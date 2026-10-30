-- +goose Up
-- Visitas (núcleo do negócio): o registro do agente num local, num ciclo, para uma atividade.
-- Roda DENTRO do schema do cliente; não cite o schema aqui.
CREATE TABLE visitas (
    id                    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- Identificador gerado no tablet; torna o envio repetido idempotente (6.4).
    id_externo            text CHECK (id_externo IS NULL OR length(id_externo) BETWEEN 1 AND 64),
    local_id              bigint NOT NULL REFERENCES locais (id) ON DELETE RESTRICT,
    ciclo_id              bigint NOT NULL REFERENCES ciclos (id) ON DELETE RESTRICT,
    atividade_id          smallint NOT NULL REFERENCES atividades (id) ON DELETE RESTRICT,
    agente_id             bigint NOT NULL REFERENCES usuarios (id) ON DELETE RESTRICT,
    data                  date NOT NULL,
    hora_entrada          time,
    tipo_visita           text NOT NULL DEFAULT 'normal' CHECK (tipo_visita IN ('normal', 'recuperacao')),
    pendencia             text NOT NULL DEFAULT 'nenhuma' CHECK (pendencia IN ('nenhuma', 'fechado', 'recusado', 'tentativa')),
    semana                smallint CHECK (semana BETWEEN 1 AND 53),
    semana_ano            smallint,
    anotacao              text NOT NULL DEFAULT '' CHECK (length(anotacao) <= 4000),
    assinatura            text NOT NULL DEFAULT '' CHECK (length(assinatura) <= 400000),

    achou_deposito        boolean NOT NULL DEFAULT false,
    dep_a1                integer CHECK (dep_a1 BETWEEN 0 AND 9999),
    dep_a2                integer CHECK (dep_a2 BETWEEN 0 AND 9999),
    dep_b                 integer CHECK (dep_b BETWEEN 0 AND 9999),
    dep_c                 integer CHECK (dep_c BETWEEN 0 AND 9999),
    dep_d1                integer CHECK (dep_d1 BETWEEN 0 AND 9999),
    dep_d2                integer CHECK (dep_d2 BETWEEN 0 AND 9999),
    dep_e                 integer CHECK (dep_e BETWEEN 0 AND 9999),
    total_eliminado       integer CHECK (total_eliminado BETWEEN 0 AND 9999),

    coletou_amostra       boolean NOT NULL DEFAULT false,

    fez_tratamento        boolean NOT NULL DEFAULT false,
    l1_gramas             numeric(8,2) CHECK (l1_gramas >= 0),
    l1_tratados           integer CHECK (l1_tratados BETWEEN 0 AND 9999),
    l2_gramas             numeric(8,2) CHECK (l2_gramas >= 0),
    l2_tratados           integer CHECK (l2_tratados BETWEEN 0 AND 9999),
    adulticida_tipo       text NOT NULL DEFAULT '' CHECK (length(adulticida_tipo) <= 40),
    adulticida_cargas     integer CHECK (adulticida_cargas BETWEEN 0 AND 9999),
    tratamento_eliminados integer CHECK (tratamento_eliminados BETWEEN 0 AND 9999),
    bri_volume            numeric(8,2) CHECK (bri_volume >= 0),

    latitude              double precision CHECK (latitude BETWEEN -90 AND 90),
    longitude             double precision CHECK (longitude BETWEEN -180 AND 180),
    precisao_gps          double precision CHECK (precisao_gps >= 0),

    ativo                 boolean NOT NULL DEFAULT true,
    data_tablet           timestamptz,
    data_sincronismo      timestamptz,
    criado_por            bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em             timestamptz NOT NULL DEFAULT now(),
    atualizado_em         timestamptz NOT NULL DEFAULT now(),
    deleted_at            timestamptz,
    deleted_by            bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX visitas_id_externo_uq ON visitas (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX visitas_local_idx ON visitas (local_id, data DESC);
CREATE INDEX visitas_ciclo_idx ON visitas (ciclo_id, atividade_id);
CREATE INDEX visitas_data_idx ON visitas (data);
CREATE INDEX visitas_agente_idx ON visitas (agente_id);
CREATE INDEX visitas_atualizado_idx ON visitas (atualizado_em);
CREATE INDEX visitas_excluidas_idx ON visitas (deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TABLE visita_fotos (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    visita_id    bigint NOT NULL REFERENCES visitas (id) ON DELETE RESTRICT,
    nome         text NOT NULL DEFAULT '' CHECK (length(nome) <= 200),
    tipo_midia   text NOT NULL CHECK (tipo_midia IN ('image/jpeg', 'image/png', 'image/webp')),
    conteudo     bytea NOT NULL,
    criado_por   bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em    timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz,
    deleted_by   bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE INDEX visita_fotos_visita_idx ON visita_fotos (visita_id, id);

-- Anotações internas: histórico de comentários da equipe sobre a visita (não editáveis).
CREATE TABLE visita_anotacoes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    visita_id  bigint NOT NULL REFERENCES visitas (id) ON DELETE RESTRICT,
    usuario_id bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    texto      text NOT NULL CHECK (length(btrim(texto)) BETWEEN 1 AND 2000),
    criado_em  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX visita_anotacoes_visita_idx ON visita_anotacoes (visita_id, id);

-- +goose Down
DROP TABLE visita_anotacoes;
DROP TABLE visita_fotos;
DROP TABLE visitas;

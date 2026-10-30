-- +goose Up
-- Reconhecimento Geográfico (um por bairro e ciclo) e Resumo Semanal (retrato calculado da produção).
CREATE TABLE reconhecimentos (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    bairro_id         bigint NOT NULL REFERENCES bairros (id) ON DELETE RESTRICT,
    ciclo_id          bigint NOT NULL REFERENCES ciclos (id) ON DELETE RESTRICT,
    data_inicio       date,
    data_fim          date,
    imoveis_residenciais integer CHECK (imoveis_residenciais BETWEEN 0 AND 999999),
    imoveis_comerciais   integer CHECK (imoveis_comerciais BETWEEN 0 AND 999999),
    imoveis_outros       integer CHECK (imoveis_outros BETWEEN 0 AND 999999),
    habitantes           integer CHECK (habitantes BETWEEN 0 AND 9999999),
    quarteiroes          integer CHECK (quarteiroes BETWEEN 0 AND 99999),
    pontos_estrategicos  integer CHECK (pontos_estrategicos BETWEEN 0 AND 99999),
    armadilhas           integer CHECK (armadilhas BETWEEN 0 AND 99999),
    terrenos_baldios     integer CHECK (terrenos_baldios BETWEEN 0 AND 99999),
    caes                 integer CHECK (caes BETWEEN 0 AND 999999),
    gatos                integer CHECK (gatos BETWEEN 0 AND 999999),
    roedores             integer CHECK (roedores BETWEEN 0 AND 999999),
    latitude          double precision CHECK (latitude BETWEEN -90 AND 90),
    longitude         double precision CHECK (longitude BETWEEN -180 AND 180),
    altitude          double precision,
    distancia_centro  double precision CHECK (distancia_centro >= 0),
    infraestrutura    jsonb NOT NULL DEFAULT '{}'::jsonb, -- chave -> sim | nao | ignorado
    riscos            jsonb NOT NULL DEFAULT '{}'::jsonb,
    observacao        text NOT NULL DEFAULT '' CHECK (length(observacao) <= 2000),
    criado_por        bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em         timestamptz NOT NULL DEFAULT now(),
    atualizado_em     timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz,
    deleted_by        bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    CHECK (data_fim IS NULL OR data_inicio IS NULL OR data_fim >= data_inicio)
);
CREATE UNIQUE INDEX reconhecimentos_bairro_ciclo_uq ON reconhecimentos (bairro_id, ciclo_id) WHERE deleted_at IS NULL;

CREATE TABLE resumos_semanais (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ciclo_id      bigint NOT NULL REFERENCES ciclos (id) ON DELETE RESTRICT,
    semana        smallint NOT NULL CHECK (semana BETWEEN 1 AND 53),
    bairro_id     bigint REFERENCES bairros (id) ON DELETE RESTRICT,   -- vazio = todos os bairros
    atividade_id  smallint REFERENCES atividades (id) ON DELETE RESTRICT, -- vazio = todas
    dados         jsonb NOT NULL, -- retrato dos números no momento em que foi gerado
    gerado_por    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    gerado_em     timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,
    deleted_by    bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE INDEX resumos_semanais_ciclo_idx ON resumos_semanais (ciclo_id, semana);

-- +goose Down
DROP TABLE resumos_semanais;
DROP TABLE reconhecimentos;

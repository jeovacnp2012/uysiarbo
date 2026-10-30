-- +goose Up
-- Armadilhas (10.1): ovitrampas (contagem de ovos) e EDLs (estações de monitoramento, com manutenção).
CREATE TABLE armadilhas (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tipo             text NOT NULL CHECK (tipo IN ('ovitrampa', 'edl')),
    id_externo       text CHECK (length(id_externo) <= 64),
    local_id         bigint NOT NULL REFERENCES locais (id) ON DELETE RESTRICT,
    instalada_em     timestamptz NOT NULL DEFAULT now(),
    agente_id        bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    latitude         double precision CHECK (latitude BETWEEN -90 AND 90),
    longitude        double precision CHECK (longitude BETWEEN -180 AND 180),
    anotacao         text NOT NULL DEFAULT '' CHECK (length(anotacao) <= 2000),
    status           text NOT NULL DEFAULT 'operando' CHECK (status IN ('operando', 'encerrado', 'extraviado', 'sem_acesso', 'cancelado', 'retirado')),
    retirada_em      timestamptz,
    retirado_por_id  bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_por       bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em        timestamptz NOT NULL DEFAULT now(),
    atualizado_em    timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz,
    deleted_by       bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX armadilhas_id_externo_uq ON armadilhas (tipo, id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX armadilhas_local_idx ON armadilhas (local_id);
CREATE INDEX armadilhas_tipo_status_idx ON armadilhas (tipo, status) WHERE deleted_at IS NULL;

-- Contagem de ovos (ovitrampa) ou manutenção (EDL): um registro por visita à armadilha.
CREATE TABLE armadilha_registros (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    armadilha_id  bigint NOT NULL REFERENCES armadilhas (id) ON DELETE RESTRICT,
    ovos          integer CHECK (ovos BETWEEN 0 AND 100000),
    nivel_agua    text NOT NULL DEFAULT '' CHECK (nivel_agua IN ('', 'Seco', '< 50%', '> 50%', 'Cheio')),
    registrado_em timestamptz NOT NULL DEFAULT now(),
    por_id        bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    anotacao      text NOT NULL DEFAULT '' CHECK (length(anotacao) <= 2000),
    deleted_at    timestamptz,
    deleted_by    bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE INDEX armadilha_registros_idx ON armadilha_registros (armadilha_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE armadilha_registros;
DROP TABLE armadilhas;

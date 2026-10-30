-- +goose Up
-- LIRAa (10.5): grupos (estratos de bairros) e agenda (ciclo, semana, quarteirão e agente).
CREATE TABLE liraa_grupos (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo text CHECK (length(id_externo) <= 64),
    nome       text NOT NULL CHECK (length(btrim(nome)) BETWEEN 1 AND 120),
    ativo      boolean NOT NULL DEFAULT true,
    criado_em  timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    deleted_by bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX liraa_grupos_nome_uq ON liraa_grupos (lower(nome)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX liraa_grupos_id_externo_uq ON liraa_grupos (id_externo) WHERE id_externo IS NOT NULL;

CREATE TABLE liraa_grupo_bairros (
    grupo_id  bigint NOT NULL REFERENCES liraa_grupos (id) ON DELETE CASCADE,
    bairro_id bigint NOT NULL REFERENCES bairros (id) ON DELETE RESTRICT,
    PRIMARY KEY (grupo_id, bairro_id)
);

CREATE TABLE liraa_agenda (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo    text CHECK (length(id_externo) <= 64),
    ciclo_id      bigint NOT NULL REFERENCES ciclos (id) ON DELETE RESTRICT,
    semana        integer NOT NULL CHECK (semana BETWEEN 1 AND 53),
    bairro_id     bigint NOT NULL REFERENCES bairros (id) ON DELETE RESTRICT,
    quarteirao_id bigint NOT NULL REFERENCES quarteiroes (id) ON DELETE RESTRICT,
    usuario_id    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    situacao      text NOT NULL DEFAULT 'aberto' CHECK (situacao IN ('aberto', 'encerrado', 'desativado')),
    criado_por    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,
    deleted_by    bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX liraa_agenda_id_externo_uq ON liraa_agenda (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX liraa_agenda_ciclo_idx ON liraa_agenda (ciclo_id, semana) WHERE deleted_at IS NULL;
CREATE INDEX liraa_agenda_usuario_idx ON liraa_agenda (usuario_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE liraa_agenda;
DROP TABLE liraa_grupo_bairros;
DROP TABLE liraa_grupos;

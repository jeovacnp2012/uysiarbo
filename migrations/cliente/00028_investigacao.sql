-- +goose Up
-- Investigação (9.2): doenças/agravos, entrevistas configuráveis e as investigações em si.

-- Cada entrevista é um conjunto de campos (JSON) que uma ou mais doenças usam.
-- Campo: {nome, label, tipo (texto|textarea|numero|select|multi_select), opcoes ("a, b, c"), obrigatorio, placeholder,
--         cond_campo, cond_operador (igual|diferente), cond_valor, exibir_se_agravo ("Dengue, Zika")}
CREATE TABLE entrevista_grupos (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo        text NOT NULL CHECK (codigo ~ '^[a-z][a-z0-9_]{1,40}$'),
    titulo        text NOT NULL CHECK (length(btrim(titulo)) BETWEEN 1 AND 120),
    ativo         boolean NOT NULL DEFAULT true,
    campos        jsonb NOT NULL DEFAULT '[]'::jsonb,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX entrevista_grupos_codigo_uq ON entrevista_grupos (codigo);

CREATE TABLE doencas (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome          text NOT NULL CHECK (length(btrim(nome)) BETWEEN 1 AND 100),
    grupo_id      bigint REFERENCES entrevista_grupos (id) ON DELETE RESTRICT,
    ativo         boolean NOT NULL DEFAULT true,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX doencas_nome_uq ON doencas (lower(nome));

CREATE TABLE investigacoes (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo     text CHECK (length(id_externo) <= 64),
    doenca_id      bigint NOT NULL REFERENCES doencas (id) ON DELETE RESTRICT,
    local_id       bigint REFERENCES locais (id) ON DELETE RESTRICT,
    endereco_texto text NOT NULL DEFAULT '' CHECK (length(endereco_texto) <= 300),
    paciente       text NOT NULL CHECK (length(btrim(paciente)) BETWEEN 1 AND 150),
    telefone       text NOT NULL DEFAULT '' CHECK (length(telefone) <= 40),
    data_inicio    date,
    situacao       text NOT NULL DEFAULT 'aguardando' CHECK (situacao IN ('aguardando', 'em_atendimento', 'encerrada')),
    resultado      text NOT NULL DEFAULT 'aguardando' CHECK (resultado IN ('aguardando', 'positivo', 'negativo', 'inconclusivo')),
    responsavel_id bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    observacao     text NOT NULL DEFAULT '' CHECK (length(observacao) <= 2000),
    grupo_id       bigint REFERENCES entrevista_grupos (id) ON DELETE RESTRICT,
    entrevista     jsonb NOT NULL DEFAULT '{}'::jsonb,
    criado_por     bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em      timestamptz NOT NULL DEFAULT now(),
    atualizado_em  timestamptz NOT NULL DEFAULT now(),
    encerrada_em   timestamptz,
    deleted_at     timestamptz,
    deleted_by     bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX investigacoes_id_externo_uq ON investigacoes (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX investigacoes_doenca_idx ON investigacoes (doenca_id);
CREATE INDEX investigacoes_local_idx ON investigacoes (local_id);
CREATE INDEX investigacoes_situacao_idx ON investigacoes (situacao, resultado) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE investigacoes;
DROP TABLE doencas;
DROP TABLE entrevista_grupos;

-- +goose Up
-- Grupos de Locais: listas nomeadas de locais (para trabalhar, exportar e mostrar no mapa).
CREATE TABLE local_grupos (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome          text NOT NULL CHECK (length(btrim(nome)) BETWEEN 2 AND 120),
    descricao     text NOT NULL DEFAULT '' CHECK (length(descricao) <= 300),
    criado_por    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX local_grupos_nome_uq ON local_grupos (lower(nome));

CREATE TABLE local_grupo_itens (
    grupo_id     bigint NOT NULL REFERENCES local_grupos (id) ON DELETE CASCADE,
    local_id     bigint NOT NULL REFERENCES locais (id) ON DELETE CASCADE,
    adicionado_em timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (grupo_id, local_id)
);
CREATE INDEX local_grupo_itens_local_idx ON local_grupo_itens (local_id);

-- +goose Down
DROP TABLE local_grupo_itens;
DROP TABLE local_grupos;

-- +goose Up
-- Estoque (11.2): retiradas de campo e permanentes, com itens por lote.
CREATE TABLE estoque_retiradas (
    id                    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tipo                  text NOT NULL CHECK (tipo IN ('campo', 'permanente')),
    usuario_id            bigint NOT NULL REFERENCES usuarios (id) ON DELETE RESTRICT,
    data_saida            timestamptz NOT NULL DEFAULT now(),
    data_retorno_prevista date,
    data_retorno_real     timestamptz,
    status                text NOT NULL DEFAULT 'aberta' CHECK (status IN ('aberta', 'finalizada', 'cancelada')),
    observacao            text NOT NULL DEFAULT '' CHECK (length(observacao) <= 1000),
    criado_por            bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em             timestamptz NOT NULL DEFAULT now(),
    deleted_at            timestamptz,
    deleted_by            bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE INDEX estoque_retiradas_status_idx ON estoque_retiradas (status) WHERE deleted_at IS NULL;

CREATE TABLE estoque_retirada_itens (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    retirada_id bigint NOT NULL REFERENCES estoque_retiradas (id) ON DELETE CASCADE,
    lote_id     bigint NOT NULL REFERENCES estoque_lotes (id) ON DELETE RESTRICT,
    retirado    numeric(12,3) NOT NULL CHECK (retirado > 0),
    usado       numeric(12,3) NOT NULL DEFAULT 0 CHECK (usado >= 0),
    devolvido   numeric(12,3) NOT NULL DEFAULT 0 CHECK (devolvido >= 0),
    perda       numeric(12,3) NOT NULL DEFAULT 0 CHECK (perda >= 0),
    UNIQUE (retirada_id, lote_id)
);
CREATE INDEX estoque_retirada_itens_lote_idx ON estoque_retirada_itens (lote_id);

ALTER TABLE estoque_movimentacoes ADD COLUMN retirada_id bigint REFERENCES estoque_retiradas (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE estoque_movimentacoes DROP COLUMN retirada_id;
DROP TABLE estoque_retirada_itens;
DROP TABLE estoque_retiradas;

-- +goose Up
-- Estoque (11.3): patrimônio (bens permanentes) com histórico de eventos.
CREATE TABLE patrimonios (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lote_id           bigint REFERENCES estoque_lotes (id) ON DELETE RESTRICT,
    numero_patrimonio text NOT NULL CHECK (length(btrim(numero_patrimonio)) BETWEEN 1 AND 40),
    numero_serie      text NOT NULL DEFAULT '' CHECK (length(numero_serie) <= 60),
    nome              text NOT NULL CHECK (length(btrim(nome)) BETWEEN 1 AND 150),
    responsavel_id    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    data_entrada      date,
    status            text NOT NULL DEFAULT 'ativo' CHECK (status IN ('ativo', 'manutencao', 'baixado')),
    observacao        text NOT NULL DEFAULT '' CHECK (length(observacao) <= 1000),
    criado_em         timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz,
    deleted_by        bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX patrimonios_numero_uq ON patrimonios (lower(numero_patrimonio)) WHERE deleted_at IS NULL;

CREATE TABLE patrimonio_eventos (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    patrimonio_id bigint NOT NULL REFERENCES patrimonios (id) ON DELETE CASCADE,
    tipo          text NOT NULL CHECK (tipo IN ('criacao', 'atribuicao', 'manutencao', 'retorno', 'anotacao', 'baixa', 'reativacao')),
    descricao     text NOT NULL DEFAULT '' CHECK (length(descricao) <= 500),
    usuario_id    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX patrimonio_eventos_idx ON patrimonio_eventos (patrimonio_id, id);

-- +goose Down
DROP TABLE patrimonio_eventos;
DROP TABLE patrimonios;

-- +goose Up
-- Solicitação de drone (9.5): pedido de sobrevoo de um local (casa fechada, calha, laje, terreno de difícil acesso).
CREATE TABLE drone_solicitacoes (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo     text CHECK (length(id_externo) <= 64),
    local_id       bigint NOT NULL REFERENCES locais (id) ON DELETE RESTRICT,
    visita_id      bigint REFERENCES visitas (id) ON DELETE RESTRICT,
    descricao      text NOT NULL CHECK (length(btrim(descricao)) BETWEEN 1 AND 2000),
    data_abertura  date NOT NULL DEFAULT CURRENT_DATE,
    situacao       text NOT NULL DEFAULT 'aguardando' CHECK (situacao IN ('aguardando', 'em_atendimento', 'finalizado', 'encerrada')),
    responsavel_id bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_por     bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em      timestamptz NOT NULL DEFAULT now(),
    atualizado_em  timestamptz NOT NULL DEFAULT now(),
    encerrada_em   timestamptz,
    deleted_at     timestamptz,
    deleted_by     bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX drone_solicitacoes_id_externo_uq ON drone_solicitacoes (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX drone_solicitacoes_situacao_idx ON drone_solicitacoes (situacao) WHERE deleted_at IS NULL;
CREATE INDEX drone_solicitacoes_local_idx ON drone_solicitacoes (local_id);

-- +goose Down
DROP TABLE drone_solicitacoes;

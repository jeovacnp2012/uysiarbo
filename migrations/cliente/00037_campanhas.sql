-- +goose Up
-- Campanha antirrábica (10.2): atendimento de um local com animais vacinados e não vacinados.
CREATE TABLE campanhas (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo    text CHECK (length(id_externo) <= 64),
    local_id      bigint NOT NULL REFERENCES locais (id) ON DELETE RESTRICT,
    data_hora     timestamptz NOT NULL DEFAULT now(),
    situacao      text NOT NULL DEFAULT 'atendido' CHECK (situacao IN ('atendido', 'recusa', 'vacinado', 'fechada', 'desabrigado', 'responsavel_ausente')),
    usar_estoque  boolean NOT NULL DEFAULT false,
    vac_cachorros integer NOT NULL DEFAULT 0 CHECK (vac_cachorros BETWEEN 0 AND 9999),
    vac_gatos     integer NOT NULL DEFAULT 0 CHECK (vac_gatos BETWEEN 0 AND 9999),
    nao_cachorros integer NOT NULL DEFAULT 0 CHECK (nao_cachorros BETWEEN 0 AND 9999),
    nao_gatos     integer NOT NULL DEFAULT 0 CHECK (nao_gatos BETWEEN 0 AND 9999),
    descricao     text NOT NULL DEFAULT '' CHECK (length(descricao) <= 2000),
    aplicador_id  bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_por    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,
    deleted_by    bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX campanhas_id_externo_uq ON campanhas (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX campanhas_local_idx ON campanhas (local_id);
CREATE INDEX campanhas_data_idx ON campanhas (data_hora) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE campanhas;

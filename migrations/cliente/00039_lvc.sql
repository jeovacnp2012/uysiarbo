-- +goose Up
-- Leishmaniose visceral canina (10.4): teste de um cachorro em um local, com DPP, reteste e desfecho do animal.
CREATE TABLE lvc_testes (
    id                    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo            text CHECK (length(id_externo) <= 64),
    local_id              bigint NOT NULL REFERENCES locais (id) ON DELETE RESTRICT,
    data                  date NOT NULL DEFAULT CURRENT_DATE,
    nome                  text NOT NULL DEFAULT '' CHECK (length(nome) <= 120),
    raca                  text NOT NULL DEFAULT '' CHECK (length(raca) <= 80),
    pelagem               text NOT NULL DEFAULT '' CHECK (length(pelagem) <= 80),
    idade                 text NOT NULL DEFAULT '' CHECK (length(idade) <= 40),
    sexo                  text NOT NULL DEFAULT '' CHECK (sexo IN ('', 'femea', 'macho')),
    proprietario          text NOT NULL DEFAULT '' CHECK (length(proprietario) <= 160),
    proprietario_telefone text NOT NULL DEFAULT '' CHECK (length(proprietario_telefone) <= 20),
    proprietario_cpf      text NOT NULL DEFAULT '' CHECK (proprietario_cpf ~ '^[0-9]{0,11}$'),
    observacao            text NOT NULL DEFAULT '' CHECK (length(observacao) <= 2000),
    dpp                   text NOT NULL DEFAULT 'aguardando' CHECK (dpp IN ('aguardando', 'positivo', 'negativo')),
    pedido_reteste        boolean NOT NULL DEFAULT false,
    data_pedido           date,
    data_pedido_recebido  date,
    resultado_reteste     text NOT NULL DEFAULT 'aguardando' CHECK (resultado_reteste IN ('aguardando', 'positivo', 'negativo')),
    situacao_cachorro     text NOT NULL DEFAULT 'acompanhamento' CHECK (situacao_cachorro IN ('acompanhamento', 'eutanasia', 'obito')),
    data_morte            date,
    visita_id             bigint REFERENCES visitas (id) ON DELETE SET NULL,
    criado_por            bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em             timestamptz NOT NULL DEFAULT now(),
    atualizado_em         timestamptz NOT NULL DEFAULT now(),
    deleted_at            timestamptz,
    deleted_by            bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX lvc_testes_id_externo_uq ON lvc_testes (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX lvc_testes_local_idx ON lvc_testes (local_id) WHERE deleted_at IS NULL;
CREATE INDEX lvc_testes_data_idx ON lvc_testes (data) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE lvc_testes;

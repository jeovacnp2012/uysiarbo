-- +goose Up
-- Denúncia (9.3): pedido de um cidadão (ou anônimo) para a vigilância verificar um problema num endereço.
CREATE TABLE denuncias (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo       text CHECK (length(id_externo) <= 64),
    anonima          boolean NOT NULL DEFAULT false,
    cidadao_nome     text NOT NULL DEFAULT '' CHECK (length(cidadao_nome) <= 150),
    cidadao_telefone text NOT NULL DEFAULT '' CHECK (length(cidadao_telefone) <= 40),
    cidadao_cpf      text NOT NULL DEFAULT '' CHECK (cidadao_cpf ~ '^[0-9]{0,11}$'),
    tipo             text NOT NULL CHECK (length(btrim(tipo)) BETWEEN 1 AND 60),
    titulo           text NOT NULL CHECK (length(btrim(titulo)) BETWEEN 1 AND 150),
    descricao        text NOT NULL DEFAULT '' CHECK (length(descricao) <= 4000),
    data_denuncia    date NOT NULL DEFAULT CURRENT_DATE,
    origem           text NOT NULL DEFAULT 'balcao' CHECK (origem IN ('balcao', 'telefone', 'aplicativo', 'site', 'outro')),
    endereco_texto   text NOT NULL DEFAULT '' CHECK (length(endereco_texto) <= 300),
    bairro_id        bigint REFERENCES bairros (id) ON DELETE RESTRICT,
    quarteirao_id    bigint REFERENCES quarteiroes (id) ON DELETE RESTRICT,
    local_id         bigint REFERENCES locais (id) ON DELETE RESTRICT,
    visita_id        bigint REFERENCES visitas (id) ON DELETE RESTRICT,
    situacao         text NOT NULL DEFAULT 'aguardando' CHECK (situacao IN ('aguardando', 'em_atendimento', 'encerrada', 'cancelada')),
    responsavel_id   bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_por       bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em        timestamptz NOT NULL DEFAULT now(),
    atualizado_em    timestamptz NOT NULL DEFAULT now(),
    encerrada_em     timestamptz,
    deleted_at       timestamptz,
    deleted_by       bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    CHECK (NOT anonima OR (cidadao_nome = '' AND cidadao_telefone = '' AND cidadao_cpf = ''))
);
CREATE UNIQUE INDEX denuncias_id_externo_uq ON denuncias (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX denuncias_situacao_idx ON denuncias (situacao) WHERE deleted_at IS NULL;
CREATE INDEX denuncias_local_idx ON denuncias (local_id);
CREATE INDEX denuncias_telefone_idx ON denuncias (cidadao_telefone) WHERE cidadao_telefone <> '';

-- +goose Down
DROP TABLE denuncias;

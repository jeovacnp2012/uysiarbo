-- +goose Up
-- BAAP (9.5): busca ativa de acidentes com animais peçonhentos. Cada registro é a notificação de um acidente.
CREATE TABLE baap_notificacoes (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo        text CHECK (length(id_externo) <= 64),
    sinam             text NOT NULL DEFAULT '' CHECK (length(sinam) <= 30),
    data_notificado   date NOT NULL DEFAULT CURRENT_DATE,
    paciente          text NOT NULL CHECK (length(btrim(paciente)) BETWEEN 1 AND 150),
    nome_mae          text NOT NULL DEFAULT '' CHECK (length(nome_mae) <= 150),
    data_nascimento   date,
    telefone          text NOT NULL DEFAULT '' CHECK (length(telefone) <= 40),
    telefone2         text NOT NULL DEFAULT '' CHECK (length(telefone2) <= 40),
    animal            text NOT NULL CHECK (animal IN ('Serpente', 'Aranha', 'Escorpião', 'Lagarta', 'Abelha', 'Lacraia', 'Outro', 'Ignorado')),
    animal_outro      text NOT NULL DEFAULT '' CHECK (length(animal_outro) <= 100),
    especie           text NOT NULL DEFAULT '' CHECK (length(especie) <= 100),
    local_picada      text NOT NULL DEFAULT 'Ignorado' CHECK (local_picada IN ('Cabeça', 'Braço', 'Ante-Braço', 'Mão', 'Dedo da Mão', 'Tronco', 'Coxa', 'Perna', 'Pé', 'Dedo do Pé', 'Ignorado')),
    situacao_paciente text NOT NULL DEFAULT '' CHECK (length(situacao_paciente) <= 2000),
    atuacao_vga       text NOT NULL DEFAULT '' CHECK (length(atuacao_vga) <= 200),
    observacao        text NOT NULL DEFAULT '' CHECK (length(observacao) <= 4000),
    endereco_texto    text NOT NULL DEFAULT '' CHECK (length(endereco_texto) <= 200),
    numero            text NOT NULL DEFAULT '' CHECK (length(numero) <= 20),
    referencia        text NOT NULL DEFAULT '' CHECK (length(referencia) <= 200),
    bairro_id         bigint REFERENCES bairros (id) ON DELETE RESTRICT,
    local_id          bigint REFERENCES locais (id) ON DELETE RESTRICT,
    visita_id         bigint REFERENCES visitas (id) ON DELETE RESTRICT,
    situacao          text NOT NULL DEFAULT 'aguardando' CHECK (situacao IN ('aguardando', 'em_atendimento', 'encerrada')),
    responsavel_id    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_por        bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em         timestamptz NOT NULL DEFAULT now(),
    atualizado_em     timestamptz NOT NULL DEFAULT now(),
    encerrada_em      timestamptz,
    deleted_at        timestamptz,
    deleted_by        bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX baap_id_externo_uq ON baap_notificacoes (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX baap_situacao_idx ON baap_notificacoes (situacao) WHERE deleted_at IS NULL;
CREATE INDEX baap_local_idx ON baap_notificacoes (local_id);

-- +goose Down
DROP TABLE baap_notificacoes;

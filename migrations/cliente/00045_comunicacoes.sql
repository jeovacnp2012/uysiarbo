-- +goose Up
-- Comunicações (12.1): fila de mensagens de WhatsApp e e-mail, com tentativas, reenvio e linha do tempo.
CREATE TABLE comunicacoes (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    canal             text NOT NULL CHECK (canal IN ('whatsapp', 'email')),
    destinatario      text NOT NULL CHECK (length(btrim(destinatario)) BETWEEN 1 AND 160),
    destinatario_nome text NOT NULL DEFAULT '' CHECK (length(destinatario_nome) <= 150),
    assunto           text NOT NULL DEFAULT '' CHECK (length(assunto) <= 150),
    corpo             text NOT NULL CHECK (length(btrim(corpo)) BETWEEN 1 AND 2000),
    origem            text NOT NULL DEFAULT 'manual' CHECK (length(origem) <= 40),
    origem_id         bigint,
    status            text NOT NULL DEFAULT 'fila' CHECK (status IN ('fila', 'enviada', 'entregue', 'visualizada', 'falhou', 'cancelada')),
    tentativas        int NOT NULL DEFAULT 0 CHECK (tentativas >= 0),
    max_tentativas    int NOT NULL DEFAULT 3 CHECK (max_tentativas BETWEEN 1 AND 10),
    proxima_tentativa timestamptz NOT NULL DEFAULT now(),
    ultimo_erro       text NOT NULL DEFAULT '' CHECK (length(ultimo_erro) <= 500),
    provedor          text NOT NULL DEFAULT '' CHECK (length(provedor) <= 40),
    id_provedor       text NOT NULL DEFAULT '' CHECK (length(id_provedor) <= 120),
    enviada_em        timestamptz,
    entregue_em       timestamptz,
    visualizada_em    timestamptz,
    criado_por        bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em         timestamptz NOT NULL DEFAULT now(),
    atualizado_em     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX comunicacoes_status_idx ON comunicacoes (status, proxima_tentativa);
CREATE INDEX comunicacoes_origem_idx ON comunicacoes (origem, origem_id);

CREATE TABLE comunicacao_eventos (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    comunicacao_id bigint NOT NULL REFERENCES comunicacoes (id) ON DELETE CASCADE,
    tipo           text NOT NULL CHECK (tipo IN ('criada', 'tentativa', 'enviada', 'falha', 'entregue', 'visualizada', 'reenvio', 'cancelada')),
    descricao      text NOT NULL DEFAULT '' CHECK (length(descricao) <= 500),
    usuario_id     bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX comunicacao_eventos_idx ON comunicacao_eventos (comunicacao_id, id);

-- +goose Down
DROP TABLE comunicacao_eventos;
DROP TABLE comunicacoes;

-- +goose Up
-- API para sistemas externos: credenciais (client credentials), registro de chamadas e idempotência.
CREATE TABLE api_credenciais (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome            text NOT NULL CHECK (length(btrim(nome)) BETWEEN 2 AND 100),
    descricao       text NOT NULL DEFAULT '' CHECK (length(descricao) <= 300),
    client_id       text NOT NULL UNIQUE,
    segredo_hash    bytea NOT NULL,                  -- SHA-256 do segredo; o segredo em si nunca é guardado
    versao_segredo  integer NOT NULL DEFAULT 1,      -- sobe a cada rotação: tokens antigos deixam de valer
    escopos         text[] NOT NULL DEFAULT '{}',
    ativo           boolean NOT NULL DEFAULT true,
    valida_ate      timestamptz,                     -- NULL = sem validade
    limite_por_minuto integer NOT NULL DEFAULT 120 CHECK (limite_por_minuto BETWEEN 1 AND 10000),
    criado_por      bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em       timestamptz NOT NULL DEFAULT now(),
    atualizado_em   timestamptz NOT NULL DEFAULT now(),
    rotacionado_em  timestamptz,
    ultimo_uso_em   timestamptz
);
CREATE UNIQUE INDEX api_credenciais_nome_uq ON api_credenciais (lower(nome));

CREATE TABLE api_chamadas (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    credencial_id bigint NOT NULL REFERENCES api_credenciais (id) ON DELETE CASCADE,
    quando        timestamptz NOT NULL DEFAULT now(),
    metodo        text NOT NULL,
    caminho       text NOT NULL,
    status        integer NOT NULL,
    duracao_ms    integer NOT NULL,
    ip            text NOT NULL DEFAULT '',
    request_id    text NOT NULL DEFAULT ''
);
CREATE INDEX api_chamadas_cred_idx ON api_chamadas (credencial_id, quando DESC);

-- Resposta guardada de cada Idempotency-Key (válida por 24 h): repetir o pedido devolve o mesmo resultado.
CREATE TABLE api_idempotencia (
    credencial_id bigint NOT NULL REFERENCES api_credenciais (id) ON DELETE CASCADE,
    chave         text NOT NULL,
    hash_pedido   bytea NOT NULL,
    status        integer NOT NULL DEFAULT 0,      -- 0 = ainda em andamento
    resposta      bytea,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (credencial_id, chave)
);
CREATE INDEX api_idempotencia_criado_idx ON api_idempotencia (criado_em);

-- A auditoria passa a aceitar a própria API como autora de uma ação.
ALTER TABLE auditoria DROP CONSTRAINT auditoria_ator_tipo_check;
ALTER TABLE auditoria ADD CONSTRAINT auditoria_ator_tipo_check CHECK (ator_tipo IN ('usuario', 'plataforma', 'sistema', 'api'));

-- +goose Down
ALTER TABLE auditoria DROP CONSTRAINT auditoria_ator_tipo_check;
ALTER TABLE auditoria ADD CONSTRAINT auditoria_ator_tipo_check CHECK (ator_tipo IN ('usuario', 'plataforma', 'sistema'));
DROP TABLE api_idempotencia;
DROP TABLE api_chamadas;
DROP TABLE api_credenciais;

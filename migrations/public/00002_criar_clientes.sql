-- +goose Up
-- Catálogo de clientes (prefeituras). Cada cliente tem um schema próprio no banco (schema_nome).
CREATE TABLE clientes (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nome          text NOT NULL,
    -- identificador curto e estável; vira o nome do schema ("c_" + slug) e o subdomínio padrão
    slug          text NOT NULL UNIQUE CHECK (slug ~ '^[a-z][a-z0-9]{2,29}$'),
    schema_nome   text NOT NULL UNIQUE CHECK (schema_nome ~ '^c_[a-z][a-z0-9]{2,29}$'),
    subdominio    text NOT NULL UNIQUE CHECK (subdominio ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    dominio       text UNIQUE CHECK (dominio ~ '^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$'),
    plano         text NOT NULL DEFAULT 'padrao',
    status        text NOT NULL DEFAULT 'provisionando'
                  CHECK (status IN ('provisionando', 'ativo', 'suspenso', 'cancelado')),
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    CHECK (schema_nome = 'c_' || slug)
);

-- +goose Down
DROP TABLE clientes;

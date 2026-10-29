-- +goose Up
ALTER TABLE plataforma_usuarios
    ADD COLUMN tentativas_falhas integer NOT NULL DEFAULT 0,
    ADD COLUMN bloqueado_ate     timestamptz,
    ADD COLUMN ultimo_acesso     timestamptz;

-- Sessões do console da plataforma. O cookie leva o token; aqui só fica o hash dele (SHA-256),
-- então um vazamento do banco não entrega sessões válidas.
CREATE TABLE sessoes_plataforma (
    token_hash  bytea PRIMARY KEY,
    usuario_id  uuid NOT NULL REFERENCES plataforma_usuarios (id) ON DELETE CASCADE,
    expira_em   timestamptz NOT NULL,
    criado_em   timestamptz NOT NULL DEFAULT now(),
    ultimo_uso  timestamptz NOT NULL DEFAULT now(),
    ip          text NOT NULL DEFAULT '',
    user_agent  text NOT NULL DEFAULT ''
);
CREATE INDEX sessoes_plataforma_usuario_idx ON sessoes_plataforma (usuario_id);
CREATE INDEX sessoes_plataforma_expira_idx ON sessoes_plataforma (expira_em);

-- +goose Down
DROP TABLE sessoes_plataforma;
ALTER TABLE plataforma_usuarios
    DROP COLUMN ultimo_acesso,
    DROP COLUMN bloqueado_ate,
    DROP COLUMN tentativas_falhas;

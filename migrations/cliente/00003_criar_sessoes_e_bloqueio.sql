-- +goose Up
ALTER TABLE usuarios
    ADD COLUMN tentativas_falhas integer NOT NULL DEFAULT 0,
    ADD COLUMN bloqueado_ate     timestamptz;

-- Sessões neste cliente: de um usuário do cliente OU do master da plataforma (que entra em qualquer
-- cliente). Só o hash do token fica guardado.
CREATE TABLE sessoes (
    token_hash  bytea PRIMARY KEY,
    usuario_id  bigint REFERENCES usuarios (id) ON DELETE CASCADE,
    master_id   uuid,
    expira_em   timestamptz NOT NULL,
    criado_em   timestamptz NOT NULL DEFAULT now(),
    ultimo_uso  timestamptz NOT NULL DEFAULT now(),
    ip          text NOT NULL DEFAULT '',
    user_agent  text NOT NULL DEFAULT '',
    CHECK ((usuario_id IS NULL) <> (master_id IS NULL))
);
CREATE INDEX sessoes_usuario_idx ON sessoes (usuario_id);
CREATE INDEX sessoes_master_idx ON sessoes (master_id);
CREATE INDEX sessoes_expira_idx ON sessoes (expira_em);

-- +goose Down
DROP TABLE sessoes;
ALTER TABLE usuarios DROP COLUMN bloqueado_ate, DROP COLUMN tentativas_falhas;

-- +goose Up
-- Pedidos de "esqueci a senha": só o hash do código fica guardado; vale uma vez e por pouco tempo.
CREATE TABLE recuperacoes_senha (
    token_hash bytea PRIMARY KEY,
    usuario_id bigint NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    criado_em  timestamptz NOT NULL DEFAULT now(),
    expira_em  timestamptz NOT NULL,
    usado_em   timestamptz,
    ip         text NOT NULL DEFAULT ''
);
CREATE INDEX recuperacoes_senha_usuario_idx ON recuperacoes_senha (usuario_id, criado_em DESC);

-- +goose Down
DROP TABLE recuperacoes_senha;

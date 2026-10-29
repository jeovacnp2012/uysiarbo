-- +goose Up
-- Migração de teste da fundação: tabela chave/valor com informações da aplicação.
CREATE TABLE app_info (
    chave      text PRIMARY KEY,
    valor      text NOT NULL,
    criado_em  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO app_info (chave, valor) VALUES ('sistema', 'UYSI ARBO');

-- +goose Down
DROP TABLE app_info;

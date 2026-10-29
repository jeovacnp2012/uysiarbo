-- +goose Up
-- Roda DENTRO do schema de cada cliente (search_path = c_<slug>, public). Não cite o schema aqui.
CREATE TABLE cliente_info (
    chave      text PRIMARY KEY,
    valor      text NOT NULL,
    criado_em  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE cliente_info;

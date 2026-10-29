-- +goose Up
-- Cidade, estado e fuso horário de cada cliente (a empresa que usa o sistema). O sistema inteiro
-- mostra horários e calcula "dias" no fuso do cliente, não no do servidor.
ALTER TABLE clientes
    ADD COLUMN cidade text NOT NULL DEFAULT '',
    ADD COLUMN uf     text NOT NULL DEFAULT '' CHECK (uf = '' OR uf ~ '^[A-Z]{2}$'),
    ADD COLUMN fuso   text NOT NULL DEFAULT 'America/Sao_Paulo' CHECK (fuso ~ '^[A-Za-z]+/[A-Za-z_]+$');

-- +goose Down
ALTER TABLE clientes DROP COLUMN fuso, DROP COLUMN uf, DROP COLUMN cidade;

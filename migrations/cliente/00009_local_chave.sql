-- +goose Up
-- Chave de cada local (vai no QR Code colado no imóvel e lida pelo tablet). Gerada no banco, única.
ALTER TABLE locais ADD COLUMN qr_chave text NOT NULL
    DEFAULT substr(replace(gen_random_uuid()::text, '-', ''), 1, 20);
CREATE UNIQUE INDEX locais_qr_chave_uq ON locais (qr_chave);

-- +goose Down
ALTER TABLE locais DROP COLUMN qr_chave;

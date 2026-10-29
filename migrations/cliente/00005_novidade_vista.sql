-- +goose Up
-- Até qual versão (das Novidades) cada usuário já viu, para o aviso de "novidade".
ALTER TABLE usuarios ADD COLUMN novidade_vista_id bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE usuarios DROP COLUMN novidade_vista_id;

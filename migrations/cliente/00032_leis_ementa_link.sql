-- +goose Up
-- Leis municipais: ementa, preâmbulo e link para o PDF (como no Geo Controle).
ALTER TABLE leis_municipais
    ADD COLUMN ementa        text NOT NULL DEFAULT '' CHECK (length(ementa) <= 2000),
    ADD COLUMN preambulo     text NOT NULL DEFAULT '' CHECK (length(preambulo) <= 4000),
    ADD COLUMN link_download text NOT NULL DEFAULT '' CHECK (length(link_download) <= 500);

-- +goose Down
ALTER TABLE leis_municipais DROP COLUMN ementa, DROP COLUMN preambulo, DROP COLUMN link_download;

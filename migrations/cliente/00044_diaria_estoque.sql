-- +goose Up
-- Estoque (11.4): a diária da campanha escolhe os lotes de vacina e seringa; ao finalizar, baixa o consumo.
ALTER TABLE campanha_diarias
    ADD COLUMN vacina_lote_id  bigint REFERENCES estoque_lotes (id) ON DELETE RESTRICT,
    ADD COLUMN seringa_lote_id bigint REFERENCES estoque_lotes (id) ON DELETE RESTRICT;

-- O que a diária baixou do estoque ao ser finalizada (desfeito quando ela é reaberta).
CREATE TABLE campanha_diaria_baixas (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    diaria_id  bigint NOT NULL REFERENCES campanha_diarias (id) ON DELETE CASCADE,
    lote_id    bigint NOT NULL REFERENCES estoque_lotes (id) ON DELETE RESTRICT,
    categoria  text NOT NULL CHECK (categoria IN ('vacina', 'seringa')),
    quantidade numeric(12,3) NOT NULL CHECK (quantidade > 0)
);
CREATE INDEX campanha_diaria_baixas_diaria_idx ON campanha_diaria_baixas (diaria_id);
CREATE INDEX campanha_diaria_baixas_lote_idx ON campanha_diaria_baixas (lote_id);

-- +goose Down
DROP TABLE campanha_diaria_baixas;
ALTER TABLE campanha_diarias DROP COLUMN seringa_lote_id, DROP COLUMN vacina_lote_id;

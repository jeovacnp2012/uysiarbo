-- +goose Up
-- Identificador do sistema de origem (ex.: o tablet, que cria registros offline com ID próprio).
-- É único por tipo de registro quando informado, e permite repetir a criação sem duplicar.
ALTER TABLE bairros ADD COLUMN id_externo text CHECK (id_externo IS NULL OR length(id_externo) BETWEEN 1 AND 64);
ALTER TABLE quarteiroes ADD COLUMN id_externo text CHECK (id_externo IS NULL OR length(id_externo) BETWEEN 1 AND 64);
ALTER TABLE locais ADD COLUMN id_externo text CHECK (id_externo IS NULL OR length(id_externo) BETWEEN 1 AND 64);
CREATE UNIQUE INDEX bairros_id_externo_uq ON bairros (id_externo) WHERE id_externo IS NOT NULL;
CREATE UNIQUE INDEX quarteiroes_id_externo_uq ON quarteiroes (id_externo) WHERE id_externo IS NOT NULL;
CREATE UNIQUE INDEX locais_id_externo_uq ON locais (id_externo) WHERE id_externo IS NOT NULL;

-- Quarteirões passam a registrar quando mudaram (a API usa isso para sincronizar só o que mudou).
ALTER TABLE quarteiroes ADD COLUMN atualizado_em timestamptz NOT NULL DEFAULT now();
CREATE INDEX bairros_atualizado_idx ON bairros (atualizado_em);
CREATE INDEX quarteiroes_atualizado_idx ON quarteiroes (atualizado_em);
CREATE INDEX locais_atualizado_idx ON locais (atualizado_em);

-- +goose Down
DROP INDEX locais_atualizado_idx;
DROP INDEX quarteiroes_atualizado_idx;
DROP INDEX bairros_atualizado_idx;
ALTER TABLE quarteiroes DROP COLUMN atualizado_em;
DROP INDEX locais_id_externo_uq;
DROP INDEX quarteiroes_id_externo_uq;
DROP INDEX bairros_id_externo_uq;
ALTER TABLE locais DROP COLUMN id_externo;
ALTER TABLE quarteiroes DROP COLUMN id_externo;
ALTER TABLE bairros DROP COLUMN id_externo;

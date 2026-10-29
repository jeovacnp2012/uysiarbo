-- +goose Up
-- Limites no mapa (PostGIS, SRID 4326). Ficam em tabelas à parte para que as listas e fichas
-- (SELECT * em bairros e quarteirões) não carreguem polígonos grandes sem necessidade.
CREATE TABLE bairro_geometrias (
    bairro_id     bigint PRIMARY KEY REFERENCES bairros (id) ON DELETE CASCADE,
    geom          geometry(MultiPolygon, 4326) NOT NULL CHECK (ST_IsValid(geom) AND NOT ST_IsEmpty(geom)),
    atualizado_em timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX bairro_geometrias_geom_idx ON bairro_geometrias USING gist (geom);

CREATE TABLE quarteirao_geometrias (
    quarteirao_id bigint PRIMARY KEY REFERENCES quarteiroes (id) ON DELETE CASCADE,
    geom          geometry(MultiPolygon, 4326) NOT NULL CHECK (ST_IsValid(geom) AND NOT ST_IsEmpty(geom)),
    atualizado_em timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX quarteirao_geometrias_geom_idx ON quarteirao_geometrias USING gist (geom);

-- +goose Down
DROP TABLE quarteirao_geometrias;
DROP TABLE bairro_geometrias;

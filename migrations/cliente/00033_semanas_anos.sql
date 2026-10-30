-- +goose Up
-- Calendário de semanas epidemiológicas cadastrado por ano (como em Parâmetros > Semanas SINAN do Geo Controle).
CREATE TABLE semanas_anos (
    ano           smallint PRIMARY KEY CHECK (ano BETWEEN 1990 AND 2100),
    primeiro_dia  date NOT NULL,
    ultimo_dia    date NOT NULL,
    semanas       smallint NOT NULL CHECK (semanas IN (52, 53)),
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now(),
    CHECK (ultimo_dia - primeiro_dia + 1 = semanas * 7)
);

-- Anos já conferidos com o calendário SINAN do Ministério da Saúde.
INSERT INTO semanas_anos (ano, primeiro_dia, ultimo_dia, semanas) VALUES
    (2025, '2024-12-29', '2026-01-03', 53),
    (2026, '2026-01-04', '2027-01-02', 52),
    (2027, '2027-01-03', '2028-01-01', 52);

-- +goose Down
DROP TABLE semanas_anos;

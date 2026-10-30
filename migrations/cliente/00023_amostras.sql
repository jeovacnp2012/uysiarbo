-- +goose Up
-- Laboratório (6.3): amostras (tubitos) coletadas numa visita e o resultado da análise.
-- Roda DENTRO do schema do cliente; não cite o schema aqui.
CREATE TABLE amostras (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    visita_id        bigint NOT NULL REFERENCES visitas (id) ON DELETE RESTRICT,
    numero_tubito    text NOT NULL CHECK (length(numero_tubito) BETWEEN 1 AND 30),
    deposito         text NOT NULL DEFAULT '' CHECK (deposito IN ('', 'A1', 'A2', 'B', 'C', 'D1', 'D2', 'E')),
    larvas           integer NOT NULL DEFAULT 0 CHECK (larvas BETWEEN 0 AND 9999),
    pupas            integer NOT NULL DEFAULT 0 CHECK (pupas BETWEEN 0 AND 9999),
    status           text NOT NULL DEFAULT 'em_analise' CHECK (status IN ('em_analise', 'concluido')),
    total_aegypti    integer NOT NULL DEFAULT 0 CHECK (total_aegypti BETWEEN 0 AND 99999),
    total_albopictus integer NOT NULL DEFAULT 0 CHECK (total_albopictus BETWEEN 0 AND 99999),
    total_negativo   integer NOT NULL DEFAULT 0 CHECK (total_negativo BETWEEN 0 AND 99999),
    resultado_por    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    resultado_em     timestamptz,
    observacao       text NOT NULL DEFAULT '' CHECK (length(observacao) <= 1000),
    criado_por       bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em        timestamptz NOT NULL DEFAULT now(),
    atualizado_em    timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz,
    deleted_by       bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
-- O número do tubito identifica o frasco no laboratório: não se repete entre amostras não excluídas.
CREATE UNIQUE INDEX amostras_tubito_uq ON amostras (numero_tubito) WHERE deleted_at IS NULL;
CREATE INDEX amostras_visita_idx ON amostras (visita_id);
CREATE INDEX amostras_status_idx ON amostras (status) WHERE deleted_at IS NULL;
CREATE INDEX amostras_excluidas_idx ON amostras (deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP TABLE amostras;

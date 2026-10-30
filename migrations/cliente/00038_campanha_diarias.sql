-- +goose Up
-- Diária de trabalho da campanha antirrábica (10.3): equipe, início e fim do trabalho, temperaturas e perdas.
CREATE TABLE campanha_diarias (
    id                    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo            text CHECK (length(id_externo) <= 64),
    data                  date NOT NULL DEFAULT CURRENT_DATE,
    anotacao              text NOT NULL DEFAULT '' CHECK (length(anotacao) <= 2000),
    status                text NOT NULL DEFAULT 'aguardando' CHECK (status IN ('aguardando', 'executando', 'finalizado', 'cancelado')),
    -- início do trabalho
    ini_seringas          integer NOT NULL DEFAULT 0 CHECK (ini_seringas BETWEEN 0 AND 99999),
    ini_frascos_fechados  integer NOT NULL DEFAULT 0 CHECK (ini_frascos_fechados BETWEEN 0 AND 99999),
    ini_frascos_abertos   integer NOT NULL DEFAULT 0 CHECK (ini_frascos_abertos BETWEEN 0 AND 99999),
    ini_doses             integer NOT NULL DEFAULT 0 CHECK (ini_doses BETWEEN 0 AND 99999),
    -- final do trabalho
    fim_seringas_usadas   integer NOT NULL DEFAULT 0 CHECK (fim_seringas_usadas BETWEEN 0 AND 99999),
    fim_seringas_perdidas integer NOT NULL DEFAULT 0 CHECK (fim_seringas_perdidas BETWEEN 0 AND 99999),
    fim_seringas_defeito  integer NOT NULL DEFAULT 0 CHECK (fim_seringas_defeito BETWEEN 0 AND 99999),
    fim_frascos_vazios    integer NOT NULL DEFAULT 0 CHECK (fim_frascos_vazios BETWEEN 0 AND 99999),
    fim_frascos_cheios    integer NOT NULL DEFAULT 0 CHECK (fim_frascos_cheios BETWEEN 0 AND 99999),
    fim_frascos_abertos   integer NOT NULL DEFAULT 0 CHECK (fim_frascos_abertos BETWEEN 0 AND 99999),
    fim_vacinas_perdidas  integer NOT NULL DEFAULT 0 CHECK (fim_vacinas_perdidas BETWEEN 0 AND 99999),
    -- defeitos, descartes e perdas
    def_seringas_defeito  integer NOT NULL DEFAULT 0 CHECK (def_seringas_defeito BETWEEN 0 AND 99999),
    def_seringas_descartadas integer NOT NULL DEFAULT 0 CHECK (def_seringas_descartadas BETWEEN 0 AND 99999),
    def_vacinas_descartadas integer NOT NULL DEFAULT 0 CHECK (def_vacinas_descartadas BETWEEN 0 AND 99999),
    def_vacinas_perdidas  integer NOT NULL DEFAULT 0 CHECK (def_vacinas_perdidas BETWEEN 0 AND 99999),
    -- temperaturas: {"recebimento": "07:49", "pontos": {"08:00": {"estoque": "4.5", "uso": "5.0"}}}
    temperaturas          jsonb NOT NULL DEFAULT '{}'::jsonb,
    criado_por            bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em             timestamptz NOT NULL DEFAULT now(),
    atualizado_em         timestamptz NOT NULL DEFAULT now(),
    deleted_at            timestamptz,
    deleted_by            bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX campanha_diarias_id_externo_uq ON campanha_diarias (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX campanha_diarias_data_idx ON campanha_diarias (data) WHERE deleted_at IS NULL;

CREATE TABLE campanha_diaria_equipe (
    diaria_id  bigint NOT NULL REFERENCES campanha_diarias (id) ON DELETE CASCADE,
    usuario_id bigint NOT NULL REFERENCES usuarios (id) ON DELETE RESTRICT,
    lider      boolean NOT NULL DEFAULT false,
    PRIMARY KEY (diaria_id, usuario_id)
);

-- Os atendimentos podem ser ligados à diária em que foram feitos.
ALTER TABLE campanhas ADD COLUMN diaria_id bigint REFERENCES campanha_diarias (id) ON DELETE SET NULL;
CREATE INDEX campanhas_diaria_idx ON campanhas (diaria_id) WHERE diaria_id IS NOT NULL;

-- +goose Down
DROP INDEX campanhas_diaria_idx;
ALTER TABLE campanhas DROP COLUMN diaria_id;
DROP TABLE campanha_diaria_equipe;
DROP TABLE campanha_diarias;

-- +goose Up
-- Sincronização em lote do tablet (6.4): id do tubito gerado no aparelho e histórico dos envios.
-- Roda DENTRO do schema do cliente; não cite o schema aqui.
ALTER TABLE amostras ADD COLUMN id_externo text CHECK (id_externo IS NULL OR length(id_externo) BETWEEN 1 AND 64);
CREATE UNIQUE INDEX amostras_id_externo_uq ON amostras (id_externo) WHERE id_externo IS NOT NULL;

CREATE TABLE sincronismos (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    credencial_id bigint REFERENCES api_credenciais (id) ON DELETE SET NULL,
    dispositivo   text NOT NULL DEFAULT '' CHECK (length(dispositivo) <= 80),
    enviado_em    timestamptz,                         -- relógio do aparelho
    recebido_em   timestamptz NOT NULL DEFAULT now(),
    visitas       integer NOT NULL DEFAULT 0,
    criadas       integer NOT NULL DEFAULT 0,
    existentes    integer NOT NULL DEFAULT 0,
    conflitos     integer NOT NULL DEFAULT 0,
    rejeitadas    integer NOT NULL DEFAULT 0,
    amostras      integer NOT NULL DEFAULT 0
);
CREATE INDEX sincronismos_recebido_idx ON sincronismos (recebido_em DESC);

-- +goose Down
DROP TABLE sincronismos;
DROP INDEX amostras_id_externo_uq;
ALTER TABLE amostras DROP COLUMN id_externo;

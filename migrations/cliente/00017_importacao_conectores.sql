-- +goose Up
-- Conectores, mapeamento de colunas salvo e a etapa "mapeando" (o arquivo espera a pessoa ligar as colunas).
ALTER TABLE importacoes ADD COLUMN conector text NOT NULL DEFAULT 'planilha';
ALTER TABLE importacoes ADD COLUMN arquivo bytea;                       -- só enquanto está em "mapeando"; apagado depois da análise
ALTER TABLE importacoes ADD COLUMN origem_info text NOT NULL DEFAULT '';
ALTER TABLE importacoes DROP CONSTRAINT importacoes_status_check;
ALTER TABLE importacoes ADD CONSTRAINT importacoes_status_check
    CHECK (status IN ('mapeando', 'analisada', 'executando', 'concluida', 'interrompida', 'cancelada', 'desfeita', 'erro'));

CREATE TABLE importacao_mapeamentos (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome        text NOT NULL CHECK (length(btrim(nome)) BETWEEN 2 AND 100),
    conector    text NOT NULL,
    tipo        text NOT NULL CHECK (tipo IN ('bairros', 'quarteiroes', 'locais')),
    mapa        jsonb NOT NULL,               -- coluna do sistema -> cabeçalho da origem
    criado_por  bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em   timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz,
    deleted_by  bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX importacao_mapeamentos_nome_uq ON importacao_mapeamentos (conector, tipo, lower(nome)) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE importacao_mapeamentos;
ALTER TABLE importacoes DROP CONSTRAINT importacoes_status_check;
ALTER TABLE importacoes ADD CONSTRAINT importacoes_status_check
    CHECK (status IN ('analisada', 'executando', 'concluida', 'interrompida', 'cancelada', 'desfeita', 'erro'));
ALTER TABLE importacoes DROP COLUMN origem_info, DROP COLUMN arquivo, DROP COLUMN conector;

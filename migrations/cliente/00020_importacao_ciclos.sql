-- +goose Up
-- A importação passa a aceitar o tipo "ciclos".
ALTER TABLE importacoes DROP CONSTRAINT importacoes_tipo_check;
ALTER TABLE importacoes ADD CONSTRAINT importacoes_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais', 'ciclos'));

ALTER TABLE importacao_mapeamentos DROP CONSTRAINT importacao_mapeamentos_tipo_check;
ALTER TABLE importacao_mapeamentos ADD CONSTRAINT importacao_mapeamentos_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais', 'ciclos'));

-- +goose Down
ALTER TABLE importacao_mapeamentos DROP CONSTRAINT importacao_mapeamentos_tipo_check;
ALTER TABLE importacao_mapeamentos ADD CONSTRAINT importacao_mapeamentos_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais'));
ALTER TABLE importacoes DROP CONSTRAINT importacoes_tipo_check;
ALTER TABLE importacoes ADD CONSTRAINT importacoes_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais'));

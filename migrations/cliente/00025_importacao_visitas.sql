-- +goose Up
-- A importação passa a aceitar os tipos "visitas" e "amostras" (laboratório).
ALTER TABLE importacoes DROP CONSTRAINT importacoes_tipo_check;
ALTER TABLE importacoes ADD CONSTRAINT importacoes_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais', 'ciclos', 'visitas', 'amostras'));

ALTER TABLE importacao_mapeamentos DROP CONSTRAINT importacao_mapeamentos_tipo_check;
ALTER TABLE importacao_mapeamentos ADD CONSTRAINT importacao_mapeamentos_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais', 'ciclos', 'visitas', 'amostras'));

-- +goose Down
ALTER TABLE importacao_mapeamentos DROP CONSTRAINT importacao_mapeamentos_tipo_check;
ALTER TABLE importacao_mapeamentos ADD CONSTRAINT importacao_mapeamentos_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais', 'ciclos'));
ALTER TABLE importacoes DROP CONSTRAINT importacoes_tipo_check;
ALTER TABLE importacoes ADD CONSTRAINT importacoes_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais', 'ciclos'));

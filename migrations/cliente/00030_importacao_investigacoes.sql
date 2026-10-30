-- +goose Up
-- A importação passa a aceitar o tipo "investigacoes".
ALTER TABLE importacoes DROP CONSTRAINT importacoes_tipo_check;
ALTER TABLE importacoes ADD CONSTRAINT importacoes_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais', 'ciclos', 'visitas', 'amostras', 'investigacoes'));

ALTER TABLE importacao_mapeamentos DROP CONSTRAINT importacao_mapeamentos_tipo_check;
ALTER TABLE importacao_mapeamentos ADD CONSTRAINT importacao_mapeamentos_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais', 'ciclos', 'visitas', 'amostras', 'investigacoes'));

-- +goose Down
ALTER TABLE importacao_mapeamentos DROP CONSTRAINT importacao_mapeamentos_tipo_check;
ALTER TABLE importacao_mapeamentos ADD CONSTRAINT importacao_mapeamentos_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais', 'ciclos', 'visitas', 'amostras'));
ALTER TABLE importacoes DROP CONSTRAINT importacoes_tipo_check;
ALTER TABLE importacoes ADD CONSTRAINT importacoes_tipo_check CHECK (tipo IN ('bairros', 'quarteiroes', 'locais', 'ciclos', 'visitas', 'amostras'));

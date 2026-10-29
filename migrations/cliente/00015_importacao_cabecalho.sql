-- +goose Up
-- O cabeçalho original da planilha, para devolver as linhas com erro no mesmo formato.
ALTER TABLE importacoes ADD COLUMN cabecalho jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE importacoes DROP COLUMN cabecalho;

-- +goose Up
-- Exclusão lógica: nada some de verdade. "Excluir" preenche deleted_at (o registro vai para a Lixeira e pode
-- ser restaurado); a exclusão definitiva é um passo à parte, com autorização e sem vínculos.
-- Os índices únicos passam a valer só para os registros não excluídos, para o nome poder ser reutilizado.
ALTER TABLE cidades     ADD COLUMN deleted_at timestamptz, ADD COLUMN deleted_by bigint REFERENCES usuarios (id) ON DELETE SET NULL;
ALTER TABLE bairros     ADD COLUMN deleted_at timestamptz, ADD COLUMN deleted_by bigint REFERENCES usuarios (id) ON DELETE SET NULL;
ALTER TABLE quarteiroes ADD COLUMN deleted_at timestamptz, ADD COLUMN deleted_by bigint REFERENCES usuarios (id) ON DELETE SET NULL;
ALTER TABLE locais      ADD COLUMN deleted_at timestamptz, ADD COLUMN deleted_by bigint REFERENCES usuarios (id) ON DELETE SET NULL;
ALTER TABLE local_grupos ADD COLUMN deleted_at timestamptz, ADD COLUMN deleted_by bigint REFERENCES usuarios (id) ON DELETE SET NULL;

DROP INDEX cidades_nome_uf_uq;
CREATE UNIQUE INDEX cidades_nome_uf_uq ON cidades (lower(nome), uf) WHERE deleted_at IS NULL;

DROP INDEX bairros_cidade_nome_uq;
CREATE UNIQUE INDEX bairros_cidade_nome_uq ON bairros (cidade_id, lower(nome)) WHERE deleted_at IS NULL;
DROP INDEX bairros_id_externo_uq;
CREATE UNIQUE INDEX bairros_id_externo_uq ON bairros (id_externo) WHERE id_externo IS NOT NULL AND deleted_at IS NULL;

DROP INDEX quarteiroes_bairro_nome_uq;
CREATE UNIQUE INDEX quarteiroes_bairro_nome_uq ON quarteiroes (bairro_id, lower(nome)) WHERE deleted_at IS NULL;
DROP INDEX quarteiroes_id_externo_uq;
CREATE UNIQUE INDEX quarteiroes_id_externo_uq ON quarteiroes (id_externo) WHERE id_externo IS NOT NULL AND deleted_at IS NULL;

DROP INDEX locais_endereco_uq;
CREATE UNIQUE INDEX locais_endereco_uq ON locais
    (quarteirao_id, lower(endereco), lower(numero), lower(lado), COALESCE(sequencia, 0), COALESCE(complemento, 0)) WHERE deleted_at IS NULL;
DROP INDEX locais_id_externo_uq;
CREATE UNIQUE INDEX locais_id_externo_uq ON locais (id_externo) WHERE id_externo IS NOT NULL AND deleted_at IS NULL;

DROP INDEX local_grupos_nome_uq;
CREATE UNIQUE INDEX local_grupos_nome_uq ON local_grupos (lower(nome)) WHERE deleted_at IS NULL;

-- A Lixeira lista os excluídos de cada tipo.
CREATE INDEX cidades_excluidas_idx ON cidades (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX bairros_excluidos_idx ON bairros (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX quarteiroes_excluidos_idx ON quarteiroes (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX locais_excluidos_idx ON locais (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX local_grupos_excluidos_idx ON local_grupos (deleted_at) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP INDEX local_grupos_excluidos_idx;
DROP INDEX locais_excluidos_idx;
DROP INDEX quarteiroes_excluidos_idx;
DROP INDEX bairros_excluidos_idx;
DROP INDEX cidades_excluidas_idx;
DROP INDEX local_grupos_nome_uq;
CREATE UNIQUE INDEX local_grupos_nome_uq ON local_grupos (lower(nome));
DROP INDEX locais_id_externo_uq;
CREATE UNIQUE INDEX locais_id_externo_uq ON locais (id_externo) WHERE id_externo IS NOT NULL;
DROP INDEX locais_endereco_uq;
CREATE UNIQUE INDEX locais_endereco_uq ON locais
    (quarteirao_id, lower(endereco), lower(numero), lower(lado), COALESCE(sequencia, 0), COALESCE(complemento, 0));
DROP INDEX quarteiroes_id_externo_uq;
CREATE UNIQUE INDEX quarteiroes_id_externo_uq ON quarteiroes (id_externo) WHERE id_externo IS NOT NULL;
DROP INDEX quarteiroes_bairro_nome_uq;
CREATE UNIQUE INDEX quarteiroes_bairro_nome_uq ON quarteiroes (bairro_id, lower(nome));
DROP INDEX bairros_id_externo_uq;
CREATE UNIQUE INDEX bairros_id_externo_uq ON bairros (id_externo) WHERE id_externo IS NOT NULL;
DROP INDEX bairros_cidade_nome_uq;
CREATE UNIQUE INDEX bairros_cidade_nome_uq ON bairros (cidade_id, lower(nome));
DROP INDEX cidades_nome_uf_uq;
CREATE UNIQUE INDEX cidades_nome_uf_uq ON cidades (lower(nome), uf);
ALTER TABLE local_grupos DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE locais DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE quarteiroes DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE bairros DROP COLUMN deleted_by, DROP COLUMN deleted_at;
ALTER TABLE cidades DROP COLUMN deleted_by, DROP COLUMN deleted_at;

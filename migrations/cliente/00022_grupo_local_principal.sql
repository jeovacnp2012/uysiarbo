-- +goose Up
-- Todo grupo de locais nasce de um local principal (como no Geo Controle). Grupos já importados guardam o ID de
-- origem na descrição ("Importado do Geo Controle (ID n)"): o principal é o local com esse id_externo.
ALTER TABLE local_grupos ADD COLUMN principal_id bigint REFERENCES locais (id) ON DELETE SET NULL;
CREATE INDEX local_grupos_principal_idx ON local_grupos (principal_id) WHERE principal_id IS NOT NULL;

UPDATE local_grupos g SET principal_id = l.id
FROM locais l
WHERE g.principal_id IS NULL
  AND g.descricao ~ '^Importado do Geo Controle \(ID [0-9]+\)$'
  AND l.id_externo = substring(g.descricao FROM '[0-9]+') AND l.deleted_at IS NULL;

-- +goose Down
ALTER TABLE local_grupos DROP COLUMN principal_id;

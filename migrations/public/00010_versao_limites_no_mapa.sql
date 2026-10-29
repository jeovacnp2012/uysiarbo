-- +goose Up
-- Versão 1.0.007: limites de bairros e quarteirões no mapa (PostGIS).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.007', 'Limites de bairros e quarteirões no mapa', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Na ficha do bairro, a seção Limite no mapa guarda o contorno do bairro a partir de um arquivo GeoJSON e mostra a área em hectares.', 1),
    ('novo', 'Importação em lote dos contornos dos quarteirões: cada polígono é ligado ao quarteirão pelo nome (os que não existem são criados).', 2),
    ('melhorado', 'Desenhos com pequenos erros são consertados automaticamente, e arquivos fora do padrão de coordenadas são recusados com explicação.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.007';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.007';

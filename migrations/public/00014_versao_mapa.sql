-- +goose Up
-- Versão 1.0.011: mapa.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.011', 'Mapa de bairros, quarteirões e locais', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Mapa: veja os contornos dos bairros, os quarteirões e os locais com coordenadas, com cores por tipo, e clique em um item para abri-lo.', 1),
    ('novo', 'Filtros do mapa por bairro, grupo de locais e tipo, e o botão Ver no mapa nas fichas de bairro, grupo e local.', 2),
    ('melhorado', 'O mapa só carrega o que está na tela e respeita os limites de cada papel (bairros atribuídos, só pontos estratégicos, desativados).', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.011';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.011';

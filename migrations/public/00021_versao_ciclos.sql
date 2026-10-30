-- +goose Up
-- Versão 1.0.018: ciclos e catálogo de atividades.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.018', 'Ciclos e atividades', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Ciclos: cadastre os ciclos de trabalho com datas, situação (operando ou fechado) e as atividades de cada um (LIT, LI, PE, T, DF, PVE, LIRAa, LIA e BRI).', 1),
    ('novo', 'Cada ciclo pode aparecer no painel público, com as atividades escolhidas, e pode ser sincronizado com outros ciclos.', 2),
    ('novo', 'O sistema avisa quando o período de um ciclo cruza o de outro. Ciclos excluídos vão para a Lixeira.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.018';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.018';

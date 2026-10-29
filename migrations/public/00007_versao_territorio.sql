-- +goose Up
-- Versão 1.0.005: cadastro do território (Etapa 3.1).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.005', 'Cadastro de cidades, bairros e quarteirões', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Cidades: cadastro dos municípios atendidos.', 1),
    ('novo', 'Bairros e povoados com todos os dados do sistema anterior (situação, categoria, zona, coordenadas, mutirão e outros), filtros e lista com páginas.', 2),
    ('novo', 'Responsáveis por bairro. Quem tem a permissão "apenas atribuídos" passa a ver somente os seus bairros.', 3),
    ('novo', 'Quarteirões em lote: um por linha ou uma faixa, como 1-20. Dá para desativar e reativar sem perder o histórico.', 4),
    ('novo', 'O módulo Bairros aparece como "disponível" no painel inicial.', 5)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.005';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.005';

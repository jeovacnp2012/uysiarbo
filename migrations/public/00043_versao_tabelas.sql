-- +goose Up
-- Versão 1.0.040: tabelas com colunas à escolha, ordenação, exportação e base da ficha de caso.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.040', 'Tabelas mais completas: colunas, ordenação e exportação', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nas listas, o botão Colunas deixa escolher quais colunas aparecem; o sistema lembra a sua escolha.', 1),
    ('novo', 'Clique no título de uma coluna para ordenar a lista (clique de novo para inverter).', 2),
    ('novo', 'Os botões PDF e Planilha exportam tudo o que está filtrado, só com as colunas visíveis; o PDF sai com cabeçalho e logo.', 3),
    ('novo', 'Filtros agora ficam numa gaveta que abre pela direita, em todas as telas.', 4),
    ('melhorado', 'Laboratório: lista por visita, com os mesmos títulos e a mesma ordem de colunas do Geo Controle.', 5),
    ('melhorado', 'Rastro GPS usa a posição do local quando a visita não tem GPS.', 6),
    ('novo', 'Base da ficha de caso (comentários, encaminhamento, anexos e linha do tempo), que será usada nas telas de investigação, denúncia e notificação.', 7)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.040';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.040';

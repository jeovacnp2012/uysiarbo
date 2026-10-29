-- +goose Up
-- Versão 1.0.013: controle de acesso aos locais.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.013', 'Controle de acesso aos locais', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Controle de acesso (Administração): escolha se a pessoa enxerga os bairros em que é responsável ou o que a lista de acessos disser.', 1),
    ('novo', 'Na lista de acessos, dê a uma pessoa o acesso a um bairro inteiro, a um quarteirão ou a um local específico, e retire quando quiser.', 2),
    ('melhorado', 'Listas, fichas, grupos, mapa, cadastro em lote e seleção de quarteirões respeitam o acesso por quarteirão e por local, não só o do bairro.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.013';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.013';

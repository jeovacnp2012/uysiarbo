-- +goose Up
-- Versão 1.0.060: Estoque (retiradas e balanço físico).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.060', 'Estoque: retiradas e balanço físico', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Retiradas: registre saídas de campo ou permanentes por responsável, com itens por lote.', 1),
    ('novo', 'Ao finalizar, informe o usado, o devolvido e a perda; o devolvido volta ao estoque.', 2),
    ('novo', 'Balanço Físico: veja por produto o que há em estoque e em retiradas abertas, e os vencimentos.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.060';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.060';

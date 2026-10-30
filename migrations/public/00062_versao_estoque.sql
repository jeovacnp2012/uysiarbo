-- +goose Up
-- Versão 1.0.059: Estoque (unidades, produtos e lotes).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.059', 'Estoque: unidades, produtos e lotes', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Unidades de Estoque: cadastre os setores que guardam produtos.', 1),
    ('novo', 'Produtos: vacinas, equipamentos e insumos, com opção de produto fracionável (doses por frasco).', 2),
    ('novo', 'Entradas / Lotes: registre lotes com validade e saldo, e lance perda, baixa ou ajuste com histórico.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.059';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.059';

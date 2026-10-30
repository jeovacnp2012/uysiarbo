-- +goose Up
-- Versão 1.0.023: paginação em todas as listagens.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.023', 'Paginação nas listagens', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'Todas as listagens agora mostram 10 registros por página e têm, em cima e embaixo da tabela, a escolha de quantos registros ver (10, 20, 30, 50 ou 100) e os botões Primeira, Anterior, Próxima e Última.', 1),
    ('novo', 'Cadastradas as 142 cidades do Mato Grosso, com o código IBGE, no cadastro de cidades de Nova Mutum.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.023';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.023';

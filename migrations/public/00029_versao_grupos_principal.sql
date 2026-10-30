-- +goose Up
-- Versão 1.0.026: grupos com local principal e filtros ao vivo em Locais.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.026', 'Grupos com local principal e pesquisa ao vivo em Locais', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Locais: todos os filtros (ID, endereço, número, bairro, quarteirão, lado, sequência, complemento, referência, proprietário, tipo, situação e ordenação) pesquisam na hora, sem apertar botão.', 1),
    ('novo', 'Os grupos de locais agora ficam dentro de Locais, no botão verde "Visualizar Grupos", e aparecem em cartões com o local principal e a lista de locais.', 2),
    ('novo', 'A ficha do grupo mostra os dados do local principal (moradores e habitantes), com os botões "Acessar Local Principal" e "Excluir Grupo", e a lista "Locais do Grupo" com busca e o botão "Adicionar Local".', 3),
    ('melhorado', 'O botão "Cadastro em lote" saiu da tela de Locais.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.026';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.026';

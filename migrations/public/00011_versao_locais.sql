-- +goose Up
-- Versão 1.0.008: cadastro de locais.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.008', 'Cadastro de locais', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Locais (Território): cadastro de imóveis, comércios, terrenos e pontos estratégicos, com endereço, bairro, quarteirão, lado, sequência e complemento.', 1),
    ('novo', 'Moradores (proprietário com CPF conferido, contatos, inquilino) e habitantes (pessoas, cachorros, gatos e roedores) em cada local.', 2),
    ('novo', 'Lista de locais com busca por endereço ou proprietário e filtros por bairro, tipo e situação. Os papéis podem limitar a visão aos bairros atribuídos ou só aos pontos estratégicos.', 3),
    ('melhorado', 'O CPF do proprietário aparece mascarado na Auditoria, e a ficha avisa quando outra pessoa está com o mesmo local aberto.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.008';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.008';

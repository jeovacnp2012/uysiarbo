-- +goose Up
-- Versão 1.0.017: conectores de importação, mapeamento de colunas e importação pela API.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.017', 'Importação: origens, mapeamento de colunas e API', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Importação com várias origens: planilha, planilha exportada do Geo Controle (com os nomes de coluna dele já reconhecidos) e busca em uma API de outro sistema, com token que não fica guardado.', 1),
    ('novo', 'Tela Ligar as colunas: quando os nomes da planilha não batem com os do sistema, você escolhe qual coluna vai para cada campo e pode salvar o mapeamento para as próximas importações.', 2),
    ('novo', 'Outros sistemas podem enviar bairros, quarteirões e locais pela API de importação, com prévia, confirmação e acompanhamento do andamento.', 3),
    ('melhorado', 'A coluna "situação" com os textos Ativo e Desativado passou a ser entendida na importação.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.017';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.017';

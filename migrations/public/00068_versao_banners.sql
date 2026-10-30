-- +goose Up
-- Versão 1.0.065: Banners (editor visual).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.065', 'Banners: editor visual', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Menu Banners: monte imagens com texto sobre cor ou foto de fundo, arrastando os textos na tela.', 1),
    ('novo', 'Fonte, tamanho, cor, alinhamento, negrito e sombra para cada texto.', 2),
    ('novo', 'Botão Baixar PNG gera a imagem no tamanho real do banner.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.065';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.065';

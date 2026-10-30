-- +goose Up
-- Versão 1.0.045: menu reorganizado como no Geo Controle e paginação só com ícones.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.045', 'Menu reorganizado e paginação com ícones', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'O menu lateral segue a ordem do Geo Controle: telas de trabalho primeiro e os grupos Zoonose, LIRAa, Relatórios, Estoque, Parâmetros e Configuração depois.', 1),
    ('melhorado', 'Cada módulo do menu ganhou um ícone próprio (microscópio, sino, megafone, lupa, engrenagem e outros).', 2),
    ('melhorado', 'Os botões de página das tabelas mostram só as setas; o texto aparece ao passar o mouse.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.045';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.045';

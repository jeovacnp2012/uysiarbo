-- +goose Up
-- Versão 1.0.025: ícones e botões mais elegantes.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.025', 'Ícones e botões mais elegantes', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'Os botões de todas as telas ganharam ícones de traço fino que combinam com a ação (salvar, novo, editar, excluir, voltar, filtrar...).', 1),
    ('melhorado', 'Novas cores, sombras e efeitos nos botões: o principal tem degradê, todos respondem ao passar o mouse, ao clicar e ao receber o foco do teclado; o botão de perigo ganha destaque ao passar o mouse.', 2),
    ('melhorado', 'O campo "Mostrar por página" das listagens agora usa o mesmo seletor elegante dos formulários.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.025';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.025';

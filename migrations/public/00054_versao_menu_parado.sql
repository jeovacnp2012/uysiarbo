-- +goose Up
-- Versão 1.0.051: menu lateral parado.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.051', 'Menu lateral parado', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'Ao clicar num item do menu lateral, só o conteúdo da página é carregado: o menu não rola nem volta ao topo e o item clicado fica marcado.', 1),
    ('corrigido', 'O item do menu da página aberta passa a aparecer realçado em todas as telas.', 2),
    ('melhorado', 'O menu Parâmetros ficou com Ciclos, Geral e Controle de acesso; as demais telas estão nas abas de Geral.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.051';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.051';

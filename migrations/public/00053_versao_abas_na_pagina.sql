-- +goose Up
-- Versão 1.0.050: abas de Parâmetros dentro da página e menu pela URL.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.050', 'Abas dentro da página e menu marcado', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'As abas de Parâmetros agora trocam o conteúdo dentro da própria página, sem recarregar a tela, e o endereço continua atualizado.', 1),
    ('melhorado', 'As abas ficaram em uma linha, com letra menor e a aba ativa em destaque escuro, como no Geo Controle.', 2),
    ('melhorado', 'O menu lateral marca o item da página aberta, inclusive nas telas de ficha, nova e edição.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.050';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.050';

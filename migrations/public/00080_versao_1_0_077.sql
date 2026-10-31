-- +goose Up
-- Versão 1.0.077: B.I. Epidemiológico em painel.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.077', 'B.I. Epidemiológico em painel', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'B.I. Epidemiológico refeito em sete seções: resumo, epidemiologia, histórico de casos, entomologia, histórico de campo, cruzamento por bairro e fontes.', 1),
    ('novo', 'Escolha do ano e da doença; histórico dos quatro últimos anos e curva mensal sobreposta.', 2),
    ('novo', 'Indicadores de campo por ciclo: imóveis visitados, fechados, recuperados e fechados sem recuperação.', 3),
    ('novo', 'Cruzamento de casos com o campo por bairro, com gráfico de dispersão e tabela de bairros para atenção.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.077';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.077';

-- +goose Up
-- Versão 1.0.076: B.I. com novos gráficos.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.076', 'B.I. com novos gráficos', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'B.I. refeito: cartões de quarteirões, locais, imóveis fechados, IIP e Breteau com a faixa de risco.', 1),
    ('melhorado', 'Novos gráficos: visitas por bairro, depósitos A1 a E, positivos por bairro, cobertura, amostras, tipo de local e cobertura do ciclo.', 2),
    ('novo', 'Filtro por bairro no B.I.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.076';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.076';

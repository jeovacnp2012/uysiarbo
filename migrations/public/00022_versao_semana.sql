-- +goose Up
-- Versão 1.0.019: semana epidemiológica e regras de ciclo.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.019', 'Semana epidemiológica', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Na tela Ciclos você escolhe como as semanas epidemiológicas são numeradas: padrão SINAN (domingo a sábado) ou ISO 8601 (segunda a domingo).', 1),
    ('novo', 'A lista e a ficha de cada ciclo mostram as semanas que ele cobre, como "SE 1/2026 a SE 9/2026".', 2),
    ('melhorado', 'Regras prontas para as visitas: ciclo fechado só aceita alteração de quem tem permissão própria, e datas fora do período do ciclo são recusadas quando o ciclo não permite.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.019';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.019';

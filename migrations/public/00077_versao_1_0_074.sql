-- +goose Up
-- Versão 1.0.074: B.I. Epidemiológico.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.074', 'B.I. Epidemiológico', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela B.I. Epidemiológico: casos investigados por doença, semana epidemiológica e bairro, com a taxa de confirmação.', 1),
    ('novo', 'Filtros por período e por doença; o padrão é o ano corrente.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.074';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.074';

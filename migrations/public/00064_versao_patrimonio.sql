-- +goose Up
-- Versão 1.0.061: Patrimônio.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.061', 'Estoque: patrimônio', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Patrimônio: cadastre bens permanentes com número de patrimônio, série e responsável.', 1),
    ('novo', 'Atribuição, manutenção, anotação, baixa e reativação, tudo no histórico do bem.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.061';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.061';

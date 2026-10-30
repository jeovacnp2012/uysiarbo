-- +goose Up
-- Versão 1.0.072: backups por schema com conferência e restauração.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.072', 'Backups conferidos e restauração testada', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Novo comando de backup: copia o catálogo e cada cliente separadamente, confere a cópia e guarda só as mais recentes.', 1),
    ('novo', 'A restauração foi testada com os dados reais do ambiente local.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.072';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.072';

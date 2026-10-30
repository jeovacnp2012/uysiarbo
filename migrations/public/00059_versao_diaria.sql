-- +goose Up
-- Versão 1.0.056: Diária de trabalho da campanha antirrábica.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.056', 'Diária de trabalho da campanha', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Diária de trabalho: monte a equipe do dia, registre o início e o fim do trabalho, as temperaturas das vacinas e as perdas.', 1),
    ('melhorado', 'O atendimento da campanha antirrábica pode ser ligado à diária em que foi feito.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.056';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.056';

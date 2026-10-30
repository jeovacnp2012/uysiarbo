-- +goose Up
-- Versão 1.0.055: Campanha antirrábica.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.055', 'Campanha antirrábica', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Campanha antirrábica: registre o atendimento de um local com os cães e gatos vacinados e não vacinados, e acompanhe tudo numa lista com filtros.', 1),
    ('melhorado', 'A ficha do local mostra os atendimentos da campanha e o botão Nova Campanha.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.055';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.055';

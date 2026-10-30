-- +goose Up
-- Versão 1.0.058: LIRAa (grupos e agenda).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.058', 'LIRAa: grupos e agenda', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'LIRAa — Grupo: cadastre os estratos e os bairros de cada um.', 1),
    ('novo', 'LIRAa — Agenda: agende quarteirões por ciclo, semana e agente, com filtros e situação.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.058';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.058';

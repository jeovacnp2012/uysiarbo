-- +goose Up
-- Versão 1.0.053: Ovitrampas e EDLs.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.053', 'Ovitrampas e EDLs', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Ovitrampas: instale a armadilha num local, registre as contagens de ovos e acompanhe o status até a retirada.', 1),
    ('novo', 'EDLs: instale a estação, registre as manutenções com o nível da água e altere o status.', 2),
    ('melhorado', 'A ficha do local lista as armadilhas instaladas e permite instalar uma nova ali mesmo.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.053';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.053';

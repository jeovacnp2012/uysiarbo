-- +goose Up
-- Versão 1.0.036: Rastro GPS no menu Mapa.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.036', 'Rastro GPS no mapa', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Novo item Rastro GPS no menu Mapa: mostra os pontos de GPS das visitas feitas pelo aplicativo e liga, por agente e dia, o caminho percorrido.', 1),
    ('novo', 'Filtros por ciclo, agente e período, aplicados na hora. Clique num ponto para abrir a visita.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.036';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.036';

-- +goose Up
-- Versão 1.0.030: reincidência e histórico de visitas na ficha do local; ajustes no mapa.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.030', 'Reincidência e histórico de visitas na ficha do local', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'A ficha do local mostra as 10 últimas visitas e o resumo dos últimos 12 meses, com a marca "Reincidente" quando 2 ou mais visitas encontraram depósito.', 1),
    ('novo', 'No Mapa, os filtros de bairro, grupo e tipo valem na hora, sem o botão Aplicar.', 2),
    ('novo', 'O menu Mapa virou um grupo (Visitas, Rastro GPS, Doenças, Campanha Antirrábica, Ovitrampas e EDLs); os que ainda não existem aparecem apagados.', 3),
    ('corrigido', 'Ao escolher um bairro sem contorno desenhado, o mapa agora se enquadra pelos locais dele, em vez de ficar no mapa do Brasil.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.030';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.030';

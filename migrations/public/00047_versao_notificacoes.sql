-- +goose Up
-- Versão 1.0.044: notificações e autos de infração.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.044', 'Notificações e autos de infração', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Notificações: avise o responsável de um imóvel, com motivo, prazo e enquadramento na lei municipal.', 1),
    ('novo', 'As notificações têm número automático do ano (NT-AAAA0000001) e mostram o selo Vencida quando o prazo passa.', 2),
    ('novo', 'Da notificação aberta dá para gerar o Auto de infração, com valor, vencimento e número próprio (INF-AAAA00001).', 3),
    ('novo', 'Nova tela Autos de infração: marque como pago ou cancele com motivo; cancelar o auto reabre a notificação.', 4),
    ('novo', 'A ficha do local mostra as notificações e os autos do imóvel e permite notificar o local.', 5),
    ('novo', 'Notificações e autos usam a ficha de caso: linha do tempo, comentários, encaminhamento e anexos.', 6)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.044';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.044';

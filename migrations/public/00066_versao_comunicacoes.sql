-- +goose Up
-- Versão 1.0.063: Comunicações (fila de WhatsApp e e-mail).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.063', 'Comunicações: fila de WhatsApp e e-mail', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Menu Comunicações: fila de mensagens de WhatsApp e e-mail, com situação, tentativas e linha do tempo.', 1),
    ('novo', 'Reenvio de mensagens que falharam ou foram canceladas, e acompanhamento de entrega e leitura.', 2),
    ('novo', 'Sem provedor de envio configurado, as mensagens esperam na fila (a ligação com os provedores vem a seguir).', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.063';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.063';

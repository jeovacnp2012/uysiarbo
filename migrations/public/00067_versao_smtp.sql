-- +goose Up
-- Versão 1.0.064: envio de e-mail por SMTP.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.064', 'Comunicações: envio de e-mail por SMTP', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Os e-mails da fila agora saem pelo servidor SMTP configurado, com conexão criptografada.', 1),
    ('novo', 'O sistema envia sozinho, a cada minuto, as mensagens vencidas da fila e tenta de novo quando há erro.', 2),
    ('novo', 'Mensagens de WhatsApp continuam na fila até haver um provedor.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.064';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.064';

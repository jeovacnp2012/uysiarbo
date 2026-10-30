-- +goose Up
-- Versão 1.0.029: sincronização em lote de visitas pela API.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.029', 'API de sincronização de visitas e amostras', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova API para o aplicativo do tablet enviar visitas e amostras em lote (POST /api/v1/sync): reenviar é seguro, e se o servidor já tem a visita com dados diferentes o servidor prevalece e o conflito é informado.', 1),
    ('novo', 'Nova consulta GET /api/v1/visitas com as amostras, por ciclo, local, agente e data de alteração, para o aplicativo baixar o que mudou.', 2),
    ('novo', 'Cada envio do aplicativo fica registrado no histórico de sincronismos.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.029';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.029';

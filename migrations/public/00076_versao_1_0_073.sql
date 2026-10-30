-- +goose Up
-- Versão 1.0.073: métricas, aviso de requisição lenta e teste de carga.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.073', 'Monitoramento e teste de carga', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'O sistema passa a expor métricas de uso (requisições, tempos e conexões do banco) para o monitoramento da equipe técnica.', 1),
    ('novo', 'Requisições lentas passam a ser registradas em aviso no log do servidor.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.073';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.073';

-- +goose Up
-- Versão 1.0.027: listagem de visitas com filtros ao vivo, desativar/reativar, ação em lote e impressão.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.027', 'Visitas: filtros ao vivo, desativar e ação em lote', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Visitas: 17 filtros que pesquisam na hora (ID, endereço, número, bairro, quarteirão, lado, ciclo, agente, atividade, semana, período, pendência, depósito, amostra, tratamento, foto e situação).', 1),
    ('novo', 'Dá para desativar e reativar uma visita (sem apagar) na ficha, e marcar várias na lista para desativar ou reativar de uma vez.', 2),
    ('novo', 'Botão Imprimir na ficha da visita, com a página limpa de menus e botões.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.027';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.027';

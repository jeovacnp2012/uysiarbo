-- +goose Up
-- Versão 1.0.028: laboratório (tubitos e resultados).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.028', 'Laboratório: tubitos e resultados da análise', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Na ficha da visita, a seção "Amostras (tubitos)" registra cada frasco com depósito, larvas e pupas; o número do tubito é automático (ou digitado, conforme o parâmetro).', 1),
    ('novo', 'O laboratório registra o resultado de cada tubito (status e totais de Aedes aegypti, albopictus e negativo), com permissão própria e regra para ciclo fechado.', 2),
    ('novo', 'Novo menu Laboratório: lista de todos os tubitos com filtros ao vivo (tubito, endereço, status, ciclo, agente e período).', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.028';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.028';

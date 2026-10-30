-- +goose Up
-- Versão 1.0.043: importação de investigações e relatório Investigação: totais.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.043', 'Importação de investigações e relatório de totais', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Importação de dados passa a aceitar Investigações (planilha com paciente, doença, situação e resultado); repetir a importação não duplica.', 1),
    ('novo', 'Novo relatório Investigação: totais, por doença, com situação e resultado.', 2),
    ('novo', 'O histórico de investigações e as entrevistas por doença do Geo Controle foram carregados no sistema.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.043';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.043';

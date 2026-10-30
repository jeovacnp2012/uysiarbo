-- +goose Up
-- Versão 1.0.066: planilha de investigações no layout SINAN / e-SUS.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.066', 'Investigações: planilha para SINAN / e-SUS', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Novo relatório "Investigações: layout SINAN / e-SUS", em tela, PDF, planilha e CSV.', 1),
    ('novo', 'A classificação final sai com os termos da vigilância: Confirmado, Descartado, Inconclusivo e Em investigação.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.066';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.066';

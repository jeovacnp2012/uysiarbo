-- +goose Up
-- Versão 1.0.021: importação de ciclos.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.021', 'Importação de ciclos', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'A Importação de dados agora traz ciclos de uma planilha: nome, datas, situação, atividades e painel público, com prévia, relatório e desfazer.', 1),
    ('novo', 'Outros sistemas também podem enviar ciclos pela API de importação (escopos importacoes:write e ciclos:write).', 2),
    ('melhorado', 'As datas da planilha são entendidas em dd/mm/aaaa, aaaa-mm-dd e também como data do Excel.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.021';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.021';

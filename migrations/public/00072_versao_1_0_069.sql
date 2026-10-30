-- +goose Up
-- Versão 1.0.069: Resumo Semanal como no Geo Controle (semana vem do ciclo, PDF do PNCD).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.069', 'Resumo Semanal refeito', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Resumo Semanal: a semana é escolhida entre as semanas do ciclo, com filtros na lista.', 1),
    ('novo', 'O resumo sai no formulário oficial do PNCD em PDF, com Visualizar antes de Salvar.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.069';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.069';

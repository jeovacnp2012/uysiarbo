-- +goose Up
-- Versão 1.0.039: Resumo semanal, Reconhecimento geográfico e ordem do menu.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.039', 'Resumo semanal e Reconhecimento geográfico', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Resumo semanal: escolha ciclo, semana, bairro e atividade, e o sistema grava o retrato da produção da semana para imprimir e arquivar.', 1),
    ('novo', 'Nova tela Reconhecimento geográfico: a ficha de cada bairro em cada ciclo, com imóveis, habitantes, infraestrutura, riscos, animais e dados geográficos.', 2),
    ('melhorado', 'No menu lateral, Dashboard, B.I. e B.I. Epidemiológico passaram a ficar logo depois de Início.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.039';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.039';

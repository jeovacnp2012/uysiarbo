-- +goose Up
-- Versão 1.0.035: B.I. (IIP, Breteau e cobertura por bairro).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.035', 'B.I.: IIP, Breteau e cobertura', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela B.I. no menu: IIP, índice de Breteau e cobertura por bairro, com filtros de ciclo, atividade e agente.', 1),
    ('novo', 'As fórmulas seguem o padrão do programa de controle da dengue e ainda vão ser validadas com a área técnica.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.035';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.035';

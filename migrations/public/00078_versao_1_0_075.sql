-- +goose Up
-- Versão 1.0.075: relatórios de campanha, LIRAa, criticidade e folha de campo.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.075', 'Relatórios de campanha, LIRAa e criticidade', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Relatório Campanha de vacinação: por bairro, atendimentos, cães e gatos vacinados e não vacinados, recusas e casas fechadas.', 1),
    ('novo', 'Relatório LIRAa: por bairro, quarteirões programados e encerrados, imóveis pesquisados, positivos, IIP e Breteau.', 2),
    ('novo', 'Relatório Locais por criticidade: locais com depósito em 2 ou mais visitas nos últimos 12 meses, os com tubitos positivos primeiro.', 3),
    ('novo', 'Folha de campo (Pesquisa entomológica / tratamento em papel): lista de locais por quarteirão com colunas em branco para preencher a mão.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.075';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.075';

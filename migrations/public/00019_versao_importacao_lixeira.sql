-- +goose Up
-- Versão 1.0.016: importação de dados por planilha, exclusão lógica e Lixeira.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.016', 'Importação por planilha e Lixeira', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Importação de dados (Administração): traga bairros, quarteirões e locais de planilhas Excel ou CSV, com modelos para baixar, prévia do que será criado, atualizado ou ignorado, e relatório.', 1),
    ('novo', 'A importação roda em segundo plano com barra de progresso, pode ser interrompida e retomada, e pode ser desfeita depois de concluída.', 2),
    ('novo', 'Lixeira (Administração): cidades, bairros, quarteirões, locais e grupos excluídos ficam guardados e podem ser restaurados.', 3),
    ('novo', 'Excluir definitivamente exige permissão própria e só é permitido quando nada mais depende do registro; senão o sistema avisa o que está ligado.', 4),
    ('melhorado', 'Excluir um bairro, quarteirão ou cidade com dados ativos dentro é recusado com aviso. A API de sincronização mostra os registros excluídos (excluido_em) para o aplicativo remover do tablet.', 5)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.016';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.016';

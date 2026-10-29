-- +goose Up
-- Versão 1.0.010: grupos de locais e ficha do local.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.010', 'Grupos de locais e ficha do local', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Grupos de locais: crie listas nomeadas, traga locais por bairro, quarteirão e tipo, clone um grupo e exporte uma planilha para o Excel.', 1),
    ('novo', 'A ficha do local ganhou os grupos a que ele pertence, o histórico de alterações e espaços reservados para visitas, laboratório, casos, infrações, armadilhas, animais e comunicação.', 2),
    ('melhorado', 'Exportações ficam registradas na Auditoria, e a planilha traz só endereços e chaves, sem dados pessoais dos moradores.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.010';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.010';

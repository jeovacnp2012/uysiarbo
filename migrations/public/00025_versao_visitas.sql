-- +goose Up
-- Versão 1.0.022: visita completa (6.1).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.022', 'Registro de visitas', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova área Visitas: registre a visita de um agente num local, com ciclo, atividade, tipo, pendência, depósitos (A1 a E), amostra, tratamento (L1, L2, adulticida), BRI, posição, assinatura e anotações.', 1),
    ('novo', 'Cada visita aceita até 20 fotos e anotações internas da equipe, e mostra a semana epidemiológica calculada pela data.', 2),
    ('novo', 'A ficha do local ganhou atalhos para ver e registrar visitas. Visitas excluídas vão para a Lixeira.', 3),
    ('melhorado', 'Ciclo fechado só aceita registrar ou alterar visitas com a permissão própria, e a data precisa estar dentro do período do ciclo.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.022';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.022';

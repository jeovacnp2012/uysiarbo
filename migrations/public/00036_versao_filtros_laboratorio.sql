-- +goose Up
-- Versão 1.0.033: todos os filtros do Laboratório.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.033', 'Mais filtros no Laboratório', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'A tela do Laboratório agora filtra também por número, bairro, quarteirão, lado, ID da visita, atividade e semana, como no Geo Controle.', 1)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.033';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.033';

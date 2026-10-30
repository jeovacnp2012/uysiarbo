-- +goose Up
-- Versão 1.0.046: Parâmetros em abas.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.046', 'Parâmetros em abas', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'As telas de Parâmetros ganharam uma barra de abas, como no Geo Controle: Geral, Semanas, Numeração, Mobilidade, Acesso, Leis e Investigação.', 1),
    ('novo', 'Nova aba Semanas epidemiológicas: calendário de cada ano e lista das semanas, no padrão SINAN ou ISO do cliente.', 2),
    ('novo', 'Nova aba Numeração: formato e contador atual das notificações e dos autos de infração.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.046';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.046';

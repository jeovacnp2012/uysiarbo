-- +goose Up
-- Versão 1.0.034: Dashboard.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.034', 'Dashboard das visitas', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Dashboard no menu: cartões com visitas, depósitos, pendências, amostras e tratamentos do ciclo, e gráficos por semana, por agente e por situação das amostras.', 1),
    ('novo', 'Dá para escolher o ciclo (o ciclo em andamento vem marcado) ou somar todos. Quem só vê as próprias visitas enxerga só os números dela.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.034';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.034';

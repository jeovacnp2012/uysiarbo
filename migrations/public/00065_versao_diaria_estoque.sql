-- +goose Up
-- Versão 1.0.062: Diária da campanha consome o estoque.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.062', 'Campanha: diária baixa vacina e seringa do estoque', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Na diária de trabalho, escolha o lote da vacina e o da seringa.', 1),
    ('novo', 'Ao finalizar a diária, as seringas e os frascos vazios saem do estoque; ao reabrir, voltam.', 2),
    ('novo', 'Com o parâmetro de uso obrigatório do estoque ligado, a diária só finaliza com os lotes escolhidos.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.062';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.062';

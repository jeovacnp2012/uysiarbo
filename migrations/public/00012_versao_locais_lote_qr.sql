-- +goose Up
-- Versão 1.0.009: desativar locais, cadastro em lote e QR Code.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.009', 'Locais: desativar, cadastro em lote e QR Code', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Cadastro em lote de locais: informe a rua, o trecho de números (de 1 em 1 ou só pares/ímpares) e a sequência inicial, e o sistema cria até 500 locais de uma vez, pulando os que já existem.', 1),
    ('novo', 'Cada local ganhou uma chave e um QR Code para imprimir e colar no imóvel; dá para gerar uma chave nova quando o adesivo se perde.', 2),
    ('novo', 'Locais podem ser desativados e reativados sem perder nada. Quem não tem a permissão "Ver desativados" não os enxerga.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.009';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.009';

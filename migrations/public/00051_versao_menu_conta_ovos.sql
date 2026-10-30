-- +goose Up
-- Versão 1.0.048: menu Conta Ovos.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.048', 'Menu Conta Ovos', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Novo menu Conta Ovos, com a configuração da API; as demais telas da integração virão em seguida.', 1)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.048';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.048';

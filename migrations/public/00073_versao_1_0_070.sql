-- +goose Up
-- Versão 1.0.070: anonimização de dados pessoais a pedido do titular (LGPD).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.070', 'LGPD: anonimização a pedido do titular', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Anonimização (LGPD): busca a pessoa por CPF, telefone ou e-mail e apaga os dados pessoais dela nos registros.', 1),
    ('novo', 'A Auditoria registra quem anonimizou, o motivo e quantos registros, sem guardar o dado da pessoa.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.070';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.070';

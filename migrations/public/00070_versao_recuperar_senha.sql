-- +goose Up
-- Versão 1.0.067: recuperação de senha por e-mail.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.067', 'Esqueci a senha: recuperação por e-mail', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Na tela de entrar, o link "Esqueci a senha" envia por e-mail um link para escolher uma nova senha.', 1),
    ('novo', 'O link vale por 1 hora e só pode ser usado uma vez; ao trocar a senha, as outras sessões são encerradas.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.067';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.067';

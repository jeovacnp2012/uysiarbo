-- +goose Up
-- Versão 1.0.068: LGPD - CPF cifrado e registro de acesso a fichas com dados pessoais.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.068', 'LGPD: CPF cifrado e registro de acesso', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'O CPF de proprietários e denunciantes passa a ser guardado cifrado no banco.', 1),
    ('novo', 'A Auditoria registra quem abriu a ficha de locais, LVC, denúncias, investigações e BAAP.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.068';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.068';

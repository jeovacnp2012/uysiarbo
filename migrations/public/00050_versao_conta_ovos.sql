-- +goose Up
-- Versão 1.0.047: aba API Conta Ovos.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.047', 'Parâmetros: API Conta Ovos', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova aba API Conta Ovos em Parâmetros: ativar a integração, escolher pública ou privada, informar estado, município e a chave da API.', 1),
    ('novo', 'A chave da API fica guardada sem aparecer na tela nem na auditoria.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.047';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.047';

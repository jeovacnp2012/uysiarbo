-- +goose Up
-- Versão 1.0.015: API de bairros, quarteirões e locais.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.015', 'API de bairros, quarteirões e locais', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'A API para sistemas externos agora lê e grava bairros, quarteirões e locais, com as mesmas regras das telas e tudo registrado na Auditoria.', 1),
    ('novo', 'Aplicativos que criam registros sem internet enviam o próprio identificador (id_externo): se o envio for repetido, nada é duplicado.', 2),
    ('novo', 'A sincronização pega só o que mudou desde a última vez (updated_since), incluindo os registros desativados.', 3),
    ('melhorado', 'O CPF do proprietário não sai nem entra pela API.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.015';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.015';

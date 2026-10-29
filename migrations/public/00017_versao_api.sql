-- +goose Up
-- Versão 1.0.014: API para sistemas externos.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.014', 'API para sistemas externos', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela API e integrações (Administração): crie credenciais para outros sistemas lerem e gravarem dados, com escopos de leitura e gravação, validade e limite de uso.', 1),
    ('novo', 'O segredo da credencial aparece uma única vez; gerar um novo ou desativar derruba na hora os acessos já emitidos.', 2),
    ('novo', 'Cada credencial mostra o consumo (chamadas, erros e tempo médio) e as últimas chamadas; o que a API altera fica na Auditoria com o nome da credencial.', 3),
    ('novo', 'Contrato OpenAPI público, guia de uso e exemplos em curl e Python. Nesta versão a API expõe o recurso Cidades.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.014';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.014';

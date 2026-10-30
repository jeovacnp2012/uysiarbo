-- +goose Up
-- Versão 1.0.071: retenção de dados pessoais por prazo (LGPD).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.071', 'LGPD: retenção de dados por prazo', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Retenção de dados (LGPD): defina por quantos meses guardar os dados pessoais de denúncias, investigações e BAAP encerrados.', 1),
    ('novo', 'Vencido o prazo, o sistema anonimiza o caso sozinho e registra na Auditoria.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.071';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.071';

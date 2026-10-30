-- +goose Up
-- Versão 1.0.031: histórico de mobilidade e histórico de alterações da visita.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.031', 'Histórico de mobilidade e histórico da visita', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Histórico de mobilidade: os envios dos aplicativos dos tablets, com aparelho, credencial, quantas visitas foram criadas, já existiam, deram conflito ou foram rejeitadas, e filtros ao vivo.', 1),
    ('novo', 'A ficha da visita mostra o histórico de alterações dela e dos tubitos (para quem tem acesso à Auditoria).', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.031';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.031';

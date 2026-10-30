-- +goose Up
-- Versão 1.0.020: parâmetros gerais, mobilidade e leis municipais.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.020', 'Parâmetros e leis municipais', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Parâmetros gerais: numeração automática de tubitos, estoque na campanha antirrábica e CEP padrão dos novos locais.', 1),
    ('novo', 'Nova tela Mobilidade (aplicativo): aviso de precisão do GPS, intervalo de coleta da posição, dias de dados que o aplicativo baixa e senha para limpar os dados do aparelho.', 2),
    ('novo', 'Nova tela Leis municipais: cadastre leis e seus trechos (artigos) para citar em visitas, notificações e autos de infração. Excluídas vão para a Lixeira.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.020';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.020';

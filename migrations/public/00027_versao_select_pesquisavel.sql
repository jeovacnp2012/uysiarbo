-- +goose Up
-- Versão 1.0.024: listas de escolha pesquisáveis.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.024', 'Listas de escolha com pesquisa', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'Toda lista de escolha com mais de 5 opções (cidade, bairro, agente, ciclo...) agora tem uma caixa de pesquisa dentro dela: digite parte do nome e escolha, com setas, Enter e Esc também funcionando.', 1)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.024';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.024';

-- +goose Up
-- Versão 1.0.032: importação de visitas e tubitos, selo do Laboratório e menu recolhível.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.032', 'Importação de visitas e laboratório, selo e menu recolhível', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Importação de visitas por planilha, com as mesmas regras da tela de visita; aceita ciclos fechados e agentes desativados e não duplica ao repetir.', 1),
    ('novo', 'Importação de tubitos do laboratório (com o resultado da análise) por planilha, ligados às visitas já importadas.', 2),
    ('novo', 'O menu Laboratório mostra, ao lado do nome, quantos tubitos estão em análise.', 3),
    ('novo', 'O menu lateral pode ser escondido para a esquerda no computador (botão ☰) e o sistema lembra a sua escolha.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.032';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.032';

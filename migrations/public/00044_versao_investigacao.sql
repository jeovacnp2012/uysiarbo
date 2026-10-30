-- +goose Up
-- Versão 1.0.041: investigações de casos suspeitos e confirmados, com entrevista por doença.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.041', 'Investigação de casos e entrevistas por doença', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Investigações: registre o paciente, a doença, o endereço e acompanhe o atendimento até o resultado.', 1),
    ('novo', 'Cada doença tem a sua entrevista, com perguntas que aparecem só quando a resposta anterior pede.', 2),
    ('novo', 'É possível vincular a investigação a um local cadastrado, para ver o caso no endereço.', 3),
    ('novo', 'Doenças e agravos e Entrevistas ficam no menu Administração, para montar as perguntas de cada doença.', 4),
    ('novo', 'A investigação usa a ficha de caso: linha do tempo, comentários, encaminhamento e anexos.', 5)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.041';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.041';

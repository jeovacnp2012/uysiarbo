-- +goose Up
-- Versão 1.0.049: Parâmetros como no Geo Controle.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.049', 'Parâmetros como no Geo Controle', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'A barra de abas de Parâmetros aparece em todas as telas da área (Geral, Semanas SINAN, Numeração, Mobilidade, Leis, Investigação e Conta Ovos).', 1),
    ('melhorado', 'A aba Geral reúne o método de acesso aos locais, o padrão das semanas, a numeração dos tubitos e o uso do estoque na campanha antirrábica.', 2),
    ('melhorado', 'A aba Mobilidade ganhou campos Sim/Não com explicação, o CEP padrão e o cartão de dias máximos para download do aplicativo.', 3),
    ('novo', 'Semanas SINAN agora é um cadastro de anos: adicione, edite e exclua o calendário de cada ano.', 4),
    ('melhorado', 'Numeração mostra prefixo, formato, reset anual, contador e o próximo número.', 5),
    ('melhorado', 'Leis municipais ganharam ementa, preâmbulo e link para baixar o PDF.', 6),
    ('melhorado', 'A aba API Conta Ovos só mostra os campos quando a integração está ativa, e a chave só na integração privada.', 7)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.049';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.049';

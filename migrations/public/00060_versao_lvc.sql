-- +goose Up
-- Versão 1.0.057: Leishmaniose visceral canina.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.057', 'Leishmaniose visceral canina (LVC)', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'LVC: registre o teste de cada cachorro com DPP, reteste e situação do animal, e acompanhe tudo numa lista com filtros.', 1),
    ('melhorado', 'A ficha do local mostra os testes de LVC e o botão Novo teste LVC.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.057';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.057';

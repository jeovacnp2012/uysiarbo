-- +goose Up
-- Versão 1.0.038: mais oito relatórios.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.038', 'Mais relatórios', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Oito relatórios novos em Relatórios: Local x visita, Acompanhamento de produção, Serviço antivetorial, Dia trabalhado, Laboratório (totais), Animais (totais), BRI e Locais com cadastro incompleto. Todos saem em tela, PDF, planilha e CSV.', 1),
    ('novo', 'Cada relatório aparece só para quem tem a permissão dele no papel.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.038';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.038';

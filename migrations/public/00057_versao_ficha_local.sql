-- +goose Up
-- Versão 1.0.054: nova ficha do local.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.054', 'Nova ficha do local', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('melhorado', 'A ficha do local ficou como a do Geo Controle: dados, moradores, habitantes e QR Code no topo, totais de visitas, reincidência dos últimos 12 meses e uma seção para cada módulo.', 1),
    ('melhorado', 'Dá para registrar visita, notificação, ovitrampa e EDL direto da ficha do local; o botão Editar abre o formulário de cadastro.', 2)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.054';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.054';

-- +goose Up
-- Versão 1.0.042: denúncias de cidadãos (identificadas ou anônimas).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.042', 'Denúncias de cidadãos', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Nova tela Denúncias: registre pedidos de cidadãos, identificados ou anônimos, com tipo, origem, endereço e bairro.', 1),
    ('novo', 'Ao digitar o telefone, o sistema mostra as denúncias anteriores da mesma pessoa.', 2),
    ('novo', 'A denúncia segue o fluxo aguardando, em atendimento, encerrada ou cancelada (o cancelamento exige motivo).', 3),
    ('novo', 'É possível vincular a denúncia a um local cadastrado e à visita feita por causa dela.', 4),
    ('novo', 'A denúncia usa a ficha de caso: linha do tempo, comentários, encaminhamento e anexos.', 5)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.042';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.042';

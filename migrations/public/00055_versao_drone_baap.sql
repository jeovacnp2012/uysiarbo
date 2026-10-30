-- +goose Up
-- Versão 1.0.052: Solicitação de drone e animais peçonhentos (BAAP).
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.052', 'Solicitação de drone e animais peçonhentos', '2026-10-30', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Solicitações de drone: peça o sobrevoo de um local (casa fechada, calha, laje), acompanhe o atendimento e encerre, com linha do tempo, anotações, encaminhamento e anexos.', 1),
    ('novo', 'Animais peçonhentos (BAAP), em Zoonose: registre os acidentes com serpentes, aranhas, escorpiões e outros, ligue ao local e à visita e acompanhe até o encerramento.', 2),
    ('melhorado', 'Um local com solicitação de drone ou notificação de animal peçonhento ativa não vai para a Lixeira; as exclusões das duas telas também passam pela Lixeira.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.052';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.052';

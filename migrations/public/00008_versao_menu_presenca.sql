-- +goose Up
-- Versão 1.0.006: menu por categoria e aviso de quem mais está no registro.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.006', 'Menu por categoria, aviso de quem está no registro e fuso por cliente', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'Menu lateral organizado por categoria (Território, Administração e outras), com os grupos recolhidos. A categoria da tela em que você está abre sozinha.', 1),
    ('novo', 'Nas fichas de bairro, cidade, usuário e papel, um aviso mostra quem mais está com o mesmo registro aberto, vendo ou editando, e desde que horas.', 2),
    ('novo', 'Cada cliente tem cidade, estado e fuso horário: todos os horários e datas do sistema passam a seguir o fuso da empresa que usa, e não o do servidor. O master ajusta isso ao criar ou editar o cliente.', 3),
    ('melhorado', 'O nome do ambiente (a prefeitura) aparece em destaque no topo do menu, e cada item tem um ícone.', 4)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.006';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.006';

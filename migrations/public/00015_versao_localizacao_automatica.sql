-- +goose Up
-- Versão 1.0.012: localização automática de endereços.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por)
VALUES ('1.0.012', 'Buscar endereço pelo CEP e localizar no mapa', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('novo', 'No cadastro do local, o botão Buscar pelo CEP preenche a rua e escolhe o bairro cadastrado que combina.', 1),
    ('novo', 'O botão Localizar no mapa descobre a latitude e a longitude pelo endereço e avisa se a posição é só aproximada ou se cai fora do limite do bairro.', 2),
    ('melhorado', 'Só o endereço é consultado nos serviços gratuitos (ViaCEP e OpenStreetMap), nunca dados dos moradores.', 3)
) AS i(tipo, texto, ordem) ON true
WHERE v.versao = '1.0.012';

-- +goose Down
DELETE FROM versoes WHERE versao = '1.0.012';

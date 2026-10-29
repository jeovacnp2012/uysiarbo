-- +goose Up
-- Novidades do sistema: o que mudou em cada versão, em linguagem simples. Vale para todos os clientes.
CREATE TABLE versoes (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    versao       text NOT NULL UNIQUE CHECK (versao ~ '^[0-9]+\.[0-9]+\.[0-9]{3}$'),
    titulo       text NOT NULL,
    publicada_em date NOT NULL DEFAULT current_date,
    criado_em    timestamptz NOT NULL DEFAULT now(),
    criado_por   text NOT NULL DEFAULT ''
);

CREATE TABLE versao_itens (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    versao_id bigint NOT NULL REFERENCES versoes (id) ON DELETE CASCADE,
    tipo      text NOT NULL CHECK (tipo IN ('novo', 'melhorado', 'corrigido')),
    texto     text NOT NULL CHECK (length(texto) BETWEEN 1 AND 500),
    ordem     integer NOT NULL DEFAULT 0
);
CREATE INDEX versao_itens_versao_idx ON versao_itens (versao_id, ordem);

-- Até qual versão cada conta da plataforma já viu (para o aviso de "novidade").
ALTER TABLE plataforma_usuarios ADD COLUMN novidade_vista_id bigint NOT NULL DEFAULT 0;

-- Histórico do que já foi entregue.
INSERT INTO versoes (versao, titulo, publicada_em, criado_por) VALUES
    ('1.0.001', 'Fundação do sistema', '2026-10-29', 'Sistema'),
    ('1.0.002', 'Clientes, acesso e administração', '2026-10-29', 'Sistema'),
    ('1.0.003', 'Auditoria e console do master', '2026-10-29', 'Sistema'),
    ('1.0.004', 'Guia do sistema e Novidades', '2026-10-29', 'Sistema');

INSERT INTO versao_itens (versao_id, tipo, texto, ordem)
SELECT v.id, i.tipo, i.texto, i.ordem FROM versoes v JOIN (VALUES
    ('1.0.001', 'novo', 'Sistema no ar, com banco de dados preparado para mapas.', 1),
    ('1.0.001', 'novo', 'Tela com menu lateral, tema claro e escuro, tabelas com filtros e páginas, janelas e avisos.', 2),
    ('1.0.001', 'novo', 'Instalação em um único arquivo, o que facilita atualizar o servidor.', 3),
    ('1.0.002', 'novo', 'Cada prefeitura tem o seu ambiente, com endereço e dados separados das demais.', 1),
    ('1.0.002', 'novo', 'Entrada com e-mail e senha, com o desafio "não sou um robô" e bloqueio depois de 5 tentativas erradas.', 2),
    ('1.0.002', 'novo', 'Papéis com 170 permissões organizadas por bloco; cadastro de usuários e de papéis.', 3),
    ('1.0.002', 'novo', 'Senha temporária forte, mostrada uma única vez, com troca obrigatória no primeiro acesso. Todo campo de senha tem a opção de revelar.', 4),
    ('1.0.002', 'novo', 'O usuário master enxerga e faz tudo, em qualquer prefeitura.', 5),
    ('1.0.003', 'novo', 'Histórico de tudo o que é feito no sistema: quem, quando, de onde e o que mudou (antes e depois). Ninguém altera nem apaga.', 1),
    ('1.0.003', 'novo', 'Console do master: criar prefeituras escolhendo o subdomínio, suspender e reativar, e gerenciar os usuários da plataforma.', 2),
    ('1.0.003', 'novo', 'Painel inicial com atalhos e todos os módulos do sistema.', 3),
    ('1.0.003', 'corrigido', 'Resetar a senha de um master agora encerra as sessões dele em todos os ambientes.', 4),
    ('1.0.004', 'novo', 'Guia do sistema, com busca, explicando cada tela.', 1),
    ('1.0.004', 'novo', 'Página de Novidades, com o que mudou em cada versão, e aviso quando há versão nova.', 2)
) AS i(versao, tipo, texto, ordem) ON i.versao = v.versao;

-- +goose Down
ALTER TABLE plataforma_usuarios DROP COLUMN novidade_vista_id;
DROP TABLE versao_itens;
DROP TABLE versoes;

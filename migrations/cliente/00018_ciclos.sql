-- +goose Up
-- Ciclos de trabalho e catálogo de atividades.
-- Roda DENTRO do schema do cliente; não cite o schema aqui.

-- Catálogo fixo (igual em todos os clientes): as atividades que um ciclo pode ter.
CREATE TABLE atividades (
    id        smallint PRIMARY KEY,
    sigla     text NOT NULL UNIQUE,
    nome      text NOT NULL,
    descricao text NOT NULL DEFAULT ''
);
INSERT INTO atividades (id, sigla, nome, descricao) VALUES
    (1, 'LIT',   'Levantamento de índice com tratamento', 'Levantamento de índice entomológico seguido de tratamento.'),
    (2, 'LI',    'Levantamento de índice', 'Levantamento de índice entomológico, sem tratamento.'),
    (3, 'PE',    'Ponto estratégico', 'Visita a pontos estratégicos (borracharias, ferros-velhos e semelhantes).'),
    (4, 'T',     'Tratamento', 'Tratamento focal e/ou adulticida.'),
    (5, 'DF',    'Delimitação de foco', 'Atividade realizada ao redor de um foco encontrado.'),
    (6, 'PVE',   'Pesquisa vetorial especial', 'Pesquisa vetorial especial.'),
    (7, 'LIRAa', 'Levantamento rápido de índices para Aedes aegypti', 'Levantamento por amostragem de quarteirões.'),
    (8, 'LIA',   'Levantamento de índice amostral', 'Levantamento de índice por amostragem.'),
    (9, 'BRI',   'Bloqueio com borrifação residual', 'Aplicação de inseticida de ação residual.');

CREATE TABLE ciclos (
    id                   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome                 text NOT NULL CHECK (length(btrim(nome)) BETWEEN 2 AND 60),
    data_inicio          date NOT NULL,
    data_final           date NOT NULL,
    situacao             text NOT NULL DEFAULT 'operando' CHECK (situacao IN ('operando', 'fechado')),
    permite_fora_periodo boolean NOT NULL DEFAULT false,
    painel_publico       boolean NOT NULL DEFAULT false,
    painel_publico_texto text NOT NULL DEFAULT '' CHECK (length(painel_publico_texto) <= 100),
    criado_em            timestamptz NOT NULL DEFAULT now(),
    atualizado_em        timestamptz NOT NULL DEFAULT now(),
    deleted_at           timestamptz,
    deleted_by           bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    CHECK (data_final >= data_inicio)
);
CREATE UNIQUE INDEX ciclos_nome_uq ON ciclos (lower(nome)) WHERE deleted_at IS NULL;
CREATE INDEX ciclos_periodo_idx ON ciclos (data_inicio, data_final);
CREATE INDEX ciclos_excluidos_idx ON ciclos (deleted_at) WHERE deleted_at IS NOT NULL;

-- Atividades habilitadas no ciclo.
CREATE TABLE ciclo_atividades (
    ciclo_id     bigint NOT NULL REFERENCES ciclos (id) ON DELETE CASCADE,
    atividade_id smallint NOT NULL REFERENCES atividades (id),
    PRIMARY KEY (ciclo_id, atividade_id)
);

-- Atividades mostradas no painel público do ciclo.
CREATE TABLE ciclo_painel_atividades (
    ciclo_id     bigint NOT NULL REFERENCES ciclos (id) ON DELETE CASCADE,
    atividade_id smallint NOT NULL REFERENCES atividades (id),
    PRIMARY KEY (ciclo_id, atividade_id)
);

-- Ciclos sincronizados: o ciclo "destino" recebe também o que se registra no ciclo de origem.
CREATE TABLE ciclo_sincronizar (
    ciclo_id         bigint NOT NULL REFERENCES ciclos (id) ON DELETE CASCADE,
    ciclo_destino_id bigint NOT NULL REFERENCES ciclos (id) ON DELETE RESTRICT,
    PRIMARY KEY (ciclo_id, ciclo_destino_id),
    CHECK (ciclo_id <> ciclo_destino_id)
);

-- +goose Down
DROP TABLE ciclo_sincronizar;
DROP TABLE ciclo_painel_atividades;
DROP TABLE ciclo_atividades;
DROP TABLE ciclos;
DROP TABLE atividades;

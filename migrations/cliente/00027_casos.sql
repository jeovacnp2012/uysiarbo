-- +goose Up
-- Ficha de caso (9.1): linha do tempo, encaminhamentos e anexos, compartilhados por todo tipo de caso
-- (investigação, denúncia, notificação, drone, animal peçonhento...). `tipo` é o código do tipo de caso
-- e `caso_id` o registro dele; não há chave estrangeira porque cada tipo tem a sua tabela.

-- Histórico imutável: o que aconteceu, quem fez e quando.
CREATE TABLE caso_eventos (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tipo          text NOT NULL CHECK (length(tipo) BETWEEN 1 AND 40),
    caso_id       bigint NOT NULL,
    evento        text NOT NULL CHECK (evento IN ('criacao', 'comentario', 'acao', 'encaminhamento', 'anexo')),
    texto         text NOT NULL DEFAULT '' CHECK (length(texto) <= 2000),
    situacao_de   text,
    situacao_para text,
    destino_id    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    usuario_id    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX caso_eventos_caso_idx ON caso_eventos (tipo, caso_id, id);

CREATE TABLE caso_anexos (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tipo        text NOT NULL CHECK (length(tipo) BETWEEN 1 AND 40),
    caso_id     bigint NOT NULL,
    nome        text NOT NULL DEFAULT '' CHECK (length(nome) <= 200),
    tipo_midia  text NOT NULL CHECK (tipo_midia IN ('image/jpeg', 'image/png', 'image/webp', 'application/pdf')),
    conteudo    bytea NOT NULL,
    criado_por  bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em   timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz,
    deleted_by  bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE INDEX caso_anexos_caso_idx ON caso_anexos (tipo, caso_id, id);

-- +goose Down
DROP TABLE caso_anexos;
DROP TABLE caso_eventos;

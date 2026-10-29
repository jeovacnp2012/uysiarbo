-- +goose Up
-- Roda DENTRO do schema de cada cliente. As permissões válidas vêm do catálogo em Go
-- (internal/authz/catalogo.txt); o banco só guarda os códigos concedidos a cada papel.
CREATE TABLE papeis (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome          text NOT NULL,
    descricao     text NOT NULL DEFAULT '',
    ativo         boolean NOT NULL DEFAULT true,
    criado_em     timestamptz NOT NULL DEFAULT now(),
    atualizado_em timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX papeis_nome_uq ON papeis (lower(nome));

CREATE TABLE papel_permissoes (
    papel_id  bigint NOT NULL REFERENCES papeis (id) ON DELETE CASCADE,
    permissao text   NOT NULL CHECK (permissao ~ '^[a-z0-9_]+$'),
    PRIMARY KEY (papel_id, permissao)
);

CREATE TABLE usuarios (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome              text NOT NULL,
    email             text NOT NULL,
    telefone          text NOT NULL DEFAULT '',
    senha_hash        text NOT NULL,
    papel_id          bigint NOT NULL REFERENCES papeis (id) ON DELETE RESTRICT,
    login_web         boolean NOT NULL DEFAULT true,
    login_tablet      boolean NOT NULL DEFAULT false,
    ativo             boolean NOT NULL DEFAULT true,
    deve_trocar_senha boolean NOT NULL DEFAULT false,
    ultimo_acesso     timestamptz,
    criado_em         timestamptz NOT NULL DEFAULT now(),
    atualizado_em     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX usuarios_email_uq ON usuarios (lower(email));
CREATE INDEX usuarios_papel_idx ON usuarios (papel_id);

-- +goose Down
DROP TABLE usuarios;
DROP TABLE papel_permissoes;
DROP TABLE papeis;

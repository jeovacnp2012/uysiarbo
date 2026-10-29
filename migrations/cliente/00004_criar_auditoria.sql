-- +goose Up
-- Histórico de tudo que acontece neste cliente. Só se acrescenta: o banco recusa alterar ou apagar.
-- Roda DENTRO do schema do cliente; não cite o schema aqui.
CREATE TABLE auditoria (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    quando      timestamptz NOT NULL DEFAULT now(),
    ator_tipo   text NOT NULL CHECK (ator_tipo IN ('usuario', 'plataforma', 'sistema')),
    ator_id     text NOT NULL DEFAULT '',
    ator_nome   text NOT NULL DEFAULT '',
    ator_email  text NOT NULL DEFAULT '',
    ip          text NOT NULL DEFAULT '',
    user_agent  text NOT NULL DEFAULT '',
    request_id  text NOT NULL DEFAULT '',
    acao        text NOT NULL,
    entidade    text NOT NULL DEFAULT '',
    entidade_id text NOT NULL DEFAULT '',
    rotulo      text NOT NULL DEFAULT '',
    antes       jsonb,
    depois      jsonb,
    detalhes    jsonb
);
CREATE INDEX auditoria_quando_idx ON auditoria (quando DESC, id DESC);
CREATE INDEX auditoria_acao_idx ON auditoria (acao);
CREATE INDEX auditoria_ator_idx ON auditoria (ator_tipo, ator_id);

-- +goose StatementBegin
CREATE FUNCTION auditoria_imutavel() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'a auditoria não pode ser alterada nem apagada';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER auditoria_imutavel
    BEFORE UPDATE OR DELETE ON auditoria
    FOR EACH ROW EXECUTE FUNCTION auditoria_imutavel();
CREATE TRIGGER auditoria_sem_truncate
    BEFORE TRUNCATE ON auditoria
    FOR EACH STATEMENT EXECUTE FUNCTION auditoria_imutavel();

-- +goose Down
DROP TRIGGER auditoria_sem_truncate ON auditoria;
DROP TRIGGER auditoria_imutavel ON auditoria;
DROP FUNCTION auditoria_imutavel();
DROP TABLE auditoria;

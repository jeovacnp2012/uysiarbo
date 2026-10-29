-- +goose Up
-- Histórico da plataforma: console, linha de comando e o que se faz com os clientes (criar, sincronizar).
-- Só se acrescenta: o banco recusa alterar ou apagar linhas.
CREATE TABLE auditoria_plataforma (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    quando      timestamptz NOT NULL DEFAULT now(),
    ator_tipo   text NOT NULL CHECK (ator_tipo IN ('plataforma', 'sistema')),
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
CREATE INDEX auditoria_plataforma_quando_idx ON auditoria_plataforma (quando DESC, id DESC);
CREATE INDEX auditoria_plataforma_acao_idx ON auditoria_plataforma (acao);

-- +goose StatementBegin
CREATE FUNCTION auditoria_plataforma_imutavel() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'a auditoria não pode ser alterada nem apagada';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER auditoria_plataforma_imutavel
    BEFORE UPDATE OR DELETE ON auditoria_plataforma
    FOR EACH ROW EXECUTE FUNCTION auditoria_plataforma_imutavel();
CREATE TRIGGER auditoria_plataforma_sem_truncate
    BEFORE TRUNCATE ON auditoria_plataforma
    FOR EACH STATEMENT EXECUTE FUNCTION auditoria_plataforma_imutavel();

-- +goose Down
DROP TRIGGER auditoria_plataforma_sem_truncate ON auditoria_plataforma;
DROP TRIGGER auditoria_plataforma_imutavel ON auditoria_plataforma;
DROP FUNCTION auditoria_plataforma_imutavel();
DROP TABLE auditoria_plataforma;

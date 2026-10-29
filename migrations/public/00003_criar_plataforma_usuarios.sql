-- +goose Up
-- Usuários da plataforma (não pertencem a nenhum cliente). O master faz tudo em qualquer cliente.
CREATE TABLE plataforma_usuarios (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nome              text NOT NULL,
    email             text NOT NULL,
    senha_hash        text NOT NULL,
    master            boolean NOT NULL DEFAULT false,
    ativo             boolean NOT NULL DEFAULT true,
    deve_trocar_senha boolean NOT NULL DEFAULT false,
    criado_em         timestamptz NOT NULL DEFAULT now(),
    atualizado_em     timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX plataforma_usuarios_email_uq ON plataforma_usuarios (lower(email));

-- O master é protegido no banco: não pode ser excluído, rebaixado nem desativado pelo sistema.
-- (Uma mudança deliberada exige administrar o banco diretamente.)
-- +goose StatementBegin
CREATE FUNCTION proteger_master() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.master THEN
            RAISE EXCEPTION 'o usuário master não pode ser excluído';
        END IF;
        RETURN OLD;
    END IF;
    IF OLD.master AND (NOT NEW.master OR NOT NEW.ativo) THEN
        RAISE EXCEPTION 'o usuário master não pode ser rebaixado nem desativado';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER plataforma_usuarios_proteger_master
    BEFORE UPDATE OR DELETE ON plataforma_usuarios
    FOR EACH ROW EXECUTE FUNCTION proteger_master();

-- +goose Down
DROP TRIGGER plataforma_usuarios_proteger_master ON plataforma_usuarios;
DROP FUNCTION proteger_master();
DROP TABLE plataforma_usuarios;

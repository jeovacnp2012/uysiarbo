-- +goose Up
-- Controle de acesso aos locais. Quem tem uma permissão "apenas atribuídos" só enxerga o que foi
-- atribuído a si, e a atribuição pode seguir dois métodos (cliente_info, chave 'acesso_local'):
--   responsavel_bairro (padrão): os bairros em que a pessoa é responsável;
--   parametro: a lista abaixo, com atribuições por bairro, por quarteirão ou por local.
CREATE TABLE acessos_local (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    usuario_id    bigint NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
    tipo          text NOT NULL CHECK (tipo IN ('bairro', 'quarteirao', 'local')),
    referencia_id bigint NOT NULL,
    descricao     text NOT NULL DEFAULT '' CHECK (length(descricao) <= 200),
    criado_por    bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX acessos_local_uq ON acessos_local (usuario_id, tipo, referencia_id);
CREATE INDEX acessos_local_ref_idx ON acessos_local (tipo, referencia_id);

-- +goose StatementBegin
CREATE FUNCTION acesso_modo() RETURNS text LANGUAGE sql STABLE AS $$
    SELECT COALESCE((SELECT valor FROM cliente_info WHERE chave = 'acesso_local'), 'responsavel_bairro')
$$;
-- +goose StatementEnd

-- A pessoa enxerga o bairro se é responsável por ele ou, no método "parametro", se tem acesso ao bairro
-- ou a algum quarteirão ou local dele.
-- +goose StatementBegin
CREATE FUNCTION usuario_ve_bairro(u bigint, b bigint) RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT CASE WHEN acesso_modo() = 'parametro' THEN
        EXISTS (SELECT 1 FROM acessos_local a WHERE a.usuario_id = u AND (
            (a.tipo = 'bairro' AND a.referencia_id = b)
            OR (a.tipo = 'quarteirao' AND EXISTS (SELECT 1 FROM quarteiroes q WHERE q.id = a.referencia_id AND q.bairro_id = b))
            OR (a.tipo = 'local' AND EXISTS (SELECT 1 FROM locais l WHERE l.id = a.referencia_id AND l.bairro_id = b))))
    ELSE
        EXISTS (SELECT 1 FROM bairro_responsaveis r WHERE r.bairro_id = b AND r.usuario_id = u)
    END
$$;
-- +goose StatementEnd

-- O quarteirão: responsável pelo bairro, ou acesso ao bairro, ao próprio quarteirão ou a um local dele.
-- +goose StatementBegin
CREATE FUNCTION usuario_ve_quarteirao(u bigint, q bigint, b bigint) RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT CASE WHEN acesso_modo() = 'parametro' THEN
        EXISTS (SELECT 1 FROM acessos_local a WHERE a.usuario_id = u AND (
            (a.tipo = 'bairro' AND a.referencia_id = b)
            OR (a.tipo = 'quarteirao' AND a.referencia_id = q)
            OR (a.tipo = 'local' AND EXISTS (SELECT 1 FROM locais l WHERE l.id = a.referencia_id AND l.quarteirao_id = q))))
    ELSE
        EXISTS (SELECT 1 FROM bairro_responsaveis r WHERE r.bairro_id = b AND r.usuario_id = u)
    END
$$;
-- +goose StatementEnd

-- O local: responsável pelo bairro, ou acesso ao bairro, ao quarteirão ou ao próprio local.
-- +goose StatementBegin
CREATE FUNCTION usuario_ve_local(u bigint, l bigint, b bigint, q bigint) RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT CASE WHEN acesso_modo() = 'parametro' THEN
        EXISTS (SELECT 1 FROM acessos_local a WHERE a.usuario_id = u AND (
            (a.tipo = 'bairro' AND a.referencia_id = b)
            OR (a.tipo = 'quarteirao' AND a.referencia_id = q)
            OR (a.tipo = 'local' AND a.referencia_id = l)))
    ELSE
        EXISTS (SELECT 1 FROM bairro_responsaveis r WHERE r.bairro_id = b AND r.usuario_id = u)
    END
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION usuario_ve_local(bigint, bigint, bigint, bigint);
DROP FUNCTION usuario_ve_quarteirao(bigint, bigint, bigint);
DROP FUNCTION usuario_ve_bairro(bigint, bigint);
DROP FUNCTION acesso_modo();
DROP TABLE acessos_local;

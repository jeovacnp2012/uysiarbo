-- +goose Up
-- Notificação e auto de infração (9.4): a vigilância notifica o responsável do imóvel; se o prazo passa sem solução,
-- a notificação gera um auto de infração, que depois é pago ou cancelado.

-- Contadores da numeração (NT-AAAA0000001 e INF-AAAA00001), reiniciados a cada ano.
CREATE TABLE numeracoes (
    tipo   text NOT NULL CHECK (tipo IN ('notificacao', 'infracao')),
    ano    int  NOT NULL,
    ultimo int  NOT NULL DEFAULT 0,
    PRIMARY KEY (tipo, ano)
);

CREATE TABLE notificacoes (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo      text CHECK (length(id_externo) <= 64),
    numero          text NOT NULL CHECK (length(numero) <= 30),
    local_id        bigint NOT NULL REFERENCES locais (id) ON DELETE RESTRICT,
    visita_id       bigint REFERENCES visitas (id) ON DELETE RESTRICT,
    lei_trecho_id   bigint REFERENCES leis_trechos (id) ON DELETE RESTRICT,
    motivo          text NOT NULL CHECK (length(btrim(motivo)) BETWEEN 1 AND 150),
    descricao       text NOT NULL DEFAULT '' CHECK (length(descricao) <= 4000),
    data_notificacao date NOT NULL DEFAULT CURRENT_DATE,
    prazo           date,
    situacao        text NOT NULL DEFAULT 'aberta' CHECK (situacao IN ('aberta', 'resolvida', 'autuada', 'cancelada')),
    resolvida_em    timestamptz,
    criado_por      bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em       timestamptz NOT NULL DEFAULT now(),
    atualizado_em   timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,
    deleted_by      bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    CHECK (prazo IS NULL OR prazo >= data_notificacao)
);
CREATE UNIQUE INDEX notificacoes_numero_uq ON notificacoes (numero);
CREATE UNIQUE INDEX notificacoes_id_externo_uq ON notificacoes (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX notificacoes_local_idx ON notificacoes (local_id);
CREATE INDEX notificacoes_situacao_idx ON notificacoes (situacao) WHERE deleted_at IS NULL;

CREATE TABLE autos_infracao (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo      text CHECK (length(id_externo) <= 64),
    numero          text NOT NULL CHECK (length(numero) <= 30),
    notificacao_id  bigint NOT NULL REFERENCES notificacoes (id) ON DELETE RESTRICT,
    local_id        bigint NOT NULL REFERENCES locais (id) ON DELETE RESTRICT,
    lei_trecho_id   bigint REFERENCES leis_trechos (id) ON DELETE RESTRICT,
    descricao       text NOT NULL DEFAULT '' CHECK (length(descricao) <= 4000),
    valor_centavos  bigint NOT NULL DEFAULT 0 CHECK (valor_centavos BETWEEN 0 AND 100000000000),
    data_auto       date NOT NULL DEFAULT CURRENT_DATE,
    vencimento      date,
    situacao        text NOT NULL DEFAULT 'aberto' CHECK (situacao IN ('aberto', 'pago', 'cancelado')),
    pago_em         date,
    criado_por      bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    criado_em       timestamptz NOT NULL DEFAULT now(),
    atualizado_em   timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,
    deleted_by      bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    CHECK (vencimento IS NULL OR vencimento >= data_auto),
    CHECK ((situacao = 'pago') = (pago_em IS NOT NULL))
);
CREATE UNIQUE INDEX autos_numero_uq ON autos_infracao (numero);
CREATE UNIQUE INDEX autos_id_externo_uq ON autos_infracao (id_externo) WHERE id_externo IS NOT NULL;
-- só um auto vivo (aberto ou pago) por notificação
CREATE UNIQUE INDEX autos_notificacao_uq ON autos_infracao (notificacao_id) WHERE situacao <> 'cancelado' AND deleted_at IS NULL;
CREATE INDEX autos_local_idx ON autos_infracao (local_id);
CREATE INDEX autos_situacao_idx ON autos_infracao (situacao) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE autos_infracao;
DROP TABLE notificacoes;
DROP TABLE numeracoes;

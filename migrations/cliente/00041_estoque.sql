-- +goose Up
-- Estoque (11.1): unidades, produtos (inclusive fracionáveis), lotes e movimentações.
CREATE TABLE estoque_unidades (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo text CHECK (length(id_externo) <= 64),
    nome       text NOT NULL CHECK (length(btrim(nome)) BETWEEN 1 AND 100),
    descricao  text NOT NULL DEFAULT '' CHECK (length(descricao) <= 500),
    ativo      boolean NOT NULL DEFAULT true,
    criado_em  timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    deleted_by bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX estoque_unidades_nome_uq ON estoque_unidades (lower(nome)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX estoque_unidades_id_externo_uq ON estoque_unidades (id_externo) WHERE id_externo IS NOT NULL;

CREATE TABLE estoque_produtos (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo          text CHECK (length(id_externo) <= 64),
    unidade_id          bigint NOT NULL REFERENCES estoque_unidades (id) ON DELETE RESTRICT,
    nome                text NOT NULL CHECK (length(btrim(nome)) BETWEEN 1 AND 150),
    tipo                text NOT NULL CHECK (tipo IN ('vacina', 'equipamento', 'insumo', 'outros')),
    unidade_medida      text NOT NULL CHECK (length(btrim(unidade_medida)) BETWEEN 1 AND 20),
    saida_permitida     text NOT NULL DEFAULT 'campo_permanente' CHECK (saida_permitida IN ('campo_permanente', 'campo', 'permanente')),
    fracionavel         boolean NOT NULL DEFAULT false,
    doses_por_unidade   numeric(10,3) CHECK (doses_por_unidade IS NULL OR doses_por_unidade > 0),
    unidade_fracao      text CHECK (unidade_fracao IS NULL OR unidade_fracao IN ('dose', 'ml', 'mg', 'ui')),
    descricao           text NOT NULL DEFAULT '' CHECK (length(descricao) <= 1000),
    selecionavel_diaria boolean NOT NULL DEFAULT false,
    categoria_diaria    text CHECK (categoria_diaria IS NULL OR categoria_diaria IN ('vacina', 'seringa')),
    ativo               boolean NOT NULL DEFAULT true,
    criado_em           timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz,
    deleted_by          bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    CHECK (NOT fracionavel OR (doses_por_unidade IS NOT NULL AND unidade_fracao IS NOT NULL)),
    CHECK (NOT selecionavel_diaria OR categoria_diaria IS NOT NULL)
);
CREATE UNIQUE INDEX estoque_produtos_nome_uq ON estoque_produtos (unidade_id, lower(nome)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX estoque_produtos_id_externo_uq ON estoque_produtos (id_externo) WHERE id_externo IS NOT NULL;

CREATE TABLE estoque_lotes (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id_externo      text CHECK (length(id_externo) <= 64),
    produto_id      bigint NOT NULL REFERENCES estoque_produtos (id) ON DELETE RESTRICT,
    numero_lote     text NOT NULL CHECK (length(btrim(numero_lote)) BETWEEN 1 AND 40),
    fabricante      text NOT NULL DEFAULT '' CHECK (length(fabricante) <= 100),
    data_fabricacao date,
    data_validade   date,
    quantidade      numeric(12,3) NOT NULL CHECK (quantidade >= 0),
    saldo           numeric(12,3) NOT NULL CHECK (saldo >= 0),
    custo_unitario  numeric(12,2) CHECK (custo_unitario IS NULL OR custo_unitario >= 0),
    nota_fiscal     text NOT NULL DEFAULT '' CHECK (length(nota_fiscal) <= 30),
    fornecedor      text NOT NULL DEFAULT '' CHECK (length(fornecedor) <= 120),
    observacao      text NOT NULL DEFAULT '' CHECK (length(observacao) <= 1000),
    criado_em       timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,
    deleted_by      bigint REFERENCES usuarios (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX estoque_lotes_numero_uq ON estoque_lotes (produto_id, lower(numero_lote)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX estoque_lotes_id_externo_uq ON estoque_lotes (id_externo) WHERE id_externo IS NOT NULL;
CREATE INDEX estoque_lotes_validade_idx ON estoque_lotes (data_validade) WHERE deleted_at IS NULL;

-- Histórico do lote: nunca se altera nem se apaga (salvo com o lote, na exclusão definitiva).
CREATE TABLE estoque_movimentacoes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    lote_id    bigint NOT NULL REFERENCES estoque_lotes (id) ON DELETE CASCADE,
    tipo       text NOT NULL CHECK (tipo IN ('entrada', 'retirada', 'devolucao', 'uso', 'perda', 'baixa', 'ajuste')),
    quantidade numeric(12,3) NOT NULL,
    saldo_apos numeric(12,3) NOT NULL,
    usuario_id bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    descricao  text NOT NULL DEFAULT '' CHECK (length(descricao) <= 255),
    origem     text NOT NULL DEFAULT '' CHECK (length(origem) <= 40),
    criado_em  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX estoque_movimentacoes_lote_idx ON estoque_movimentacoes (lote_id, id);

-- +goose Down
DROP TABLE estoque_movimentacoes;
DROP TABLE estoque_lotes;
DROP TABLE estoque_produtos;
DROP TABLE estoque_unidades;

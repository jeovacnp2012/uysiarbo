-- +goose Up
-- Importação de dados por planilha: cada importação guarda a análise (prévia) linha a linha, o
-- andamento da execução e o que foi feito, para o relatório e para desfazer.
CREATE TABLE importacoes (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tipo                text NOT NULL CHECK (tipo IN ('bairros', 'quarteiroes', 'locais')),
    origem              text NOT NULL DEFAULT 'planilha',
    arquivo_nome        text NOT NULL,
    arquivo_hash        bytea NOT NULL,
    atualizar_existentes boolean NOT NULL DEFAULT false,
    status              text NOT NULL CHECK (status IN ('analisada', 'executando', 'concluida', 'interrompida', 'cancelada', 'desfeita', 'erro')),
    total_linhas        integer NOT NULL DEFAULT 0,
    prev_criar          integer NOT NULL DEFAULT 0,
    prev_atualizar      integer NOT NULL DEFAULT 0,
    prev_ignorar        integer NOT NULL DEFAULT 0,
    prev_erro           integer NOT NULL DEFAULT 0,
    processadas         integer NOT NULL DEFAULT 0,
    criados             integer NOT NULL DEFAULT 0,
    atualizados         integer NOT NULL DEFAULT 0,
    ignorados           integer NOT NULL DEFAULT 0,
    erros               integer NOT NULL DEFAULT 0,
    desfeitos           integer NOT NULL DEFAULT 0,
    colunas_ignoradas   text NOT NULL DEFAULT '',
    responsavel_id      bigint REFERENCES usuarios (id) ON DELETE SET NULL,
    responsavel_nome    text NOT NULL DEFAULT '',
    mensagem            text NOT NULL DEFAULT '',
    criado_em           timestamptz NOT NULL DEFAULT now(),
    iniciado_em         timestamptz,
    concluido_em        timestamptz,
    desfeito_em         timestamptz
);
CREATE INDEX importacoes_criado_idx ON importacoes (criado_em DESC);

CREATE TABLE importacao_linhas (
    importacao_id bigint NOT NULL REFERENCES importacoes (id) ON DELETE CASCADE,
    linha         integer NOT NULL,            -- número da linha na planilha (o cabeçalho é a 1)
    acao          text NOT NULL CHECK (acao IN ('criar', 'atualizar', 'ignorar', 'erro')),
    mensagem      text NOT NULL DEFAULT '',    -- explicação da análise (por que ignorar, qual o erro)
    alvo_id       bigint,                      -- registro existente, quando a ação é atualizar
    dados         jsonb,                       -- o que será gravado
    antes         jsonb,                       -- como o registro estava (para desfazer uma atualização)
    original      jsonb NOT NULL,              -- as células da planilha, para devolver as linhas com erro
    resultado     text CHECK (resultado IN ('criado', 'atualizado', 'ignorado', 'erro', 'desfeito')),
    resultado_msg text NOT NULL DEFAULT '',
    registro_id   bigint,                      -- o registro criado ou atualizado
    PRIMARY KEY (importacao_id, linha)
);
CREATE INDEX importacao_linhas_acao_idx ON importacao_linhas (importacao_id, acao, linha);

-- +goose Down
DROP TABLE importacao_linhas;
DROP TABLE importacoes;

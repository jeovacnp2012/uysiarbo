-- Geo Controle — modelo de dados INFERIDO por engenharia reversa (2026-10-29)
-- Dialeto: MySQL 8 / utf8mb4. Nada aqui foi lido do banco real: tipos, tamanhos e
-- constraints são sugestões deduzidas de formulários, listagens e filtros.
-- Convenção: ids de negócio gerados no tablet são BIGINT (prefixo + timestamp Unix, ver descritivo).
-- Colunas de auditoria comuns (created_at, updated_at, created_by, cad_situacao) omitidas onde óbvias,
-- exceto nas tabelas centrais.

SET NAMES utf8mb4;

-- =========================================================================
-- 1. ACESSO
-- =========================================================================
CREATE TABLE papeis (
  id            INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  nome          VARCHAR(80) NOT NULL UNIQUE,
  cad_situacao  ENUM('Ativo','Desativado') NOT NULL DEFAULT 'Ativo',
  created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL
);

-- ~190 permissões em 24 categorias (ex.: visita_adicionar, local_meubairro, relatorio_producao ...)
CREATE TABLE permissoes (
  chave      VARCHAR(80) PRIMARY KEY,          -- ex.: visita_visualizar_minha
  categoria  VARCHAR(60) NOT NULL,             -- Visitas, Locais, Estoque, Configuração ...
  subgrupo   VARCHAR(60) NULL,                 -- Notificações, Infrações, Camadas ...
  rotulo     VARCHAR(120) NOT NULL
);
CREATE TABLE papel_permissao (
  id_papel  INT UNSIGNED NOT NULL,
  chave     VARCHAR(80) NOT NULL,
  PRIMARY KEY (id_papel, chave),
  FOREIGN KEY (id_papel) REFERENCES papeis(id) ON DELETE CASCADE,
  FOREIGN KEY (chave) REFERENCES permissoes(chave) ON DELETE CASCADE
);

CREATE TABLE usuarios (
  id            INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name          VARCHAR(150) NOT NULL,
  email         VARCHAR(150) NOT NULL UNIQUE,
  telefone      VARCHAR(20) NULL,
  password      VARCHAR(255) NOT NULL,
  id_papel      INT UNSIGNED NULL,
  login_web     TINYINT(1) NOT NULL DEFAULT 1,
  login_tablet  TINYINT(1) NOT NULL DEFAULT 0,
  cad_situacao  ENUM('Ativo','Desativado') NOT NULL DEFAULT 'Ativo',
  created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  FOREIGN KEY (id_papel) REFERENCES papeis(id)
);
-- Atividades que o usuário pode lançar em visita (LI-T, LI, PE, T, DF, PVE, LIRAa)
CREATE TABLE usuario_atividade (
  id_usuario INT UNSIGNED NOT NULL,
  id_atividade TINYINT UNSIGNED NOT NULL,
  PRIMARY KEY (id_usuario, id_atividade),
  FOREIGN KEY (id_usuario) REFERENCES usuarios(id) ON DELETE CASCADE
);

-- =========================================================================
-- 2. TERRITÓRIO
-- =========================================================================
CREATE TABLE cidades (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  nome VARCHAR(100) NOT NULL, uf CHAR(2) NOT NULL, ibge VARCHAR(10) NULL
);

CREATE TABLE bairros (
  id              INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_cidade       INT UNSIGNED NOT NULL,
  nome            VARCHAR(120) NOT NULL,
  mutirao         TINYINT(1) NOT NULL DEFAULT 0,
  status          ENUM('Ativo','Extinto') NOT NULL DEFAULT 'Ativo',
  ibge            VARCHAR(20) NULL,
  classificacao   ENUM('Urbano','Rural') NOT NULL DEFAULT 'Urbano',
  zona            VARCHAR(30) NULL,
  categoria       ENUM('Bairro','Povoado') NOT NULL DEFAULT 'Bairro',
  latitude DECIMAL(10,7) NULL, longitude DECIMAL(10,7) NULL, altitude DECIMAL(8,2) NULL,
  distancia_centro DECIMAL(8,2) NULL,
  producao        TINYINT(1) NOT NULL DEFAULT 1,   -- "Entra no relatório de produção?"
  FOREIGN KEY (id_cidade) REFERENCES cidades(id)
);
CREATE TABLE bairro_responsavel (           -- N:N bairro x usuário
  id_bairro INT UNSIGNED NOT NULL, id_usuario INT UNSIGNED NOT NULL,
  PRIMARY KEY (id_bairro, id_usuario),
  FOREIGN KEY (id_bairro) REFERENCES bairros(id), FOREIGN KEY (id_usuario) REFERENCES usuarios(id)
);
CREATE TABLE quarteiroes (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_bairro INT UNSIGNED NOT NULL, nome VARCHAR(30) NOT NULL,
  UNIQUE (id_bairro, nome), FOREIGN KEY (id_bairro) REFERENCES bairros(id)
);

CREATE TABLE locais (
  id               BIGINT UNSIGNED PRIMARY KEY,     -- gerado no cliente (prefixo + timestamp)
  tipo             ENUM('Residencial','Comércio','Terreno Baldio','Ponto Estratégico','Outro') NOT NULL,
  cep              VARCHAR(9) NULL,
  endereco         VARCHAR(200) NOT NULL,
  numero           VARCHAR(15) NOT NULL,
  referencia       VARCHAR(200) NULL,
  id_bairro        INT UNSIGNED NOT NULL,
  id_quarteirao    INT UNSIGNED NOT NULL,
  lado             VARCHAR(5) NOT NULL,
  sequencia        INT NULL,
  complemento      INT NULL,
  id_cidade        INT UNSIGNED NULL,
  latitude DECIMAL(10,7) NULL, longitude DECIMAL(10,7) NULL,
  criticidade      VARCHAR(20) NULL,                -- (inferido) relatório "Locais por Criticidade"
  sobrevoo         TINYINT(1) NULL,                 -- (inferido)
  -- moradores
  proprietario VARCHAR(150) NULL, proprietario_telefone VARCHAR(20) NULL, proprietario_cpf VARCHAR(14) NULL,
  proprietario_whatsapp VARCHAR(20) NULL, proprietario_email VARCHAR(150) NULL,
  proprietario_whatsapp_aceita TINYINT(1) NOT NULL DEFAULT 0, proprietario_email_aceita TINYINT(1) NOT NULL DEFAULT 0,
  inquilino VARCHAR(150) NULL, inquilino_telefone VARCHAR(20) NULL,
  -- habitantes
  humanos INT NULL, cachorros INT NULL, gatos INT NULL, roedores INT NULL,
  qr_chave VARCHAR(64) NULL,                        -- "QR Code / Chave"
  cad_situacao ENUM('Ativo','Desativado') NOT NULL DEFAULT 'Ativo',
  created_by INT UNSIGNED NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  KEY ix_end (endereco, numero), KEY ix_bq (id_bairro, id_quarteirao),
  FOREIGN KEY (id_bairro) REFERENCES bairros(id), FOREIGN KEY (id_quarteirao) REFERENCES quarteiroes(id)
);

CREATE TABLE local_grupos (                         -- "Grupo de Locais" (gerenciar, clonar, exportar, mapa)
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, nome VARCHAR(120) NOT NULL
);
CREATE TABLE local_grupo_item (
  id_grupo INT UNSIGNED NOT NULL, id_local BIGINT UNSIGNED NOT NULL, PRIMARY KEY (id_grupo, id_local)
);

-- =========================================================================
-- 3. CICLO E ATIVIDADES
-- =========================================================================
CREATE TABLE atividades (
  id TINYINT UNSIGNED PRIMARY KEY,
  sigla VARCHAR(8) NOT NULL, nome VARCHAR(80) NOT NULL
);
-- LIT, LI, PE, T, DF, PVE, LIRAA, LIA, BRI

CREATE TABLE ciclos (
  id                    INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  nome                  VARCHAR(60) NOT NULL,         -- "05 Ciclo - 2026"
  data_inicio           DATE NOT NULL, data_final DATE NOT NULL,
  situacao              ENUM('Operando','Fechado') NOT NULL DEFAULT 'Operando',
  permite_fora_periodo  TINYINT(1) NOT NULL DEFAULT 0,
  painel_publico        TINYINT(1) NOT NULL DEFAULT 0,
  painel_publico_texto  VARCHAR(100) NULL
);
CREATE TABLE ciclo_atividade (id_ciclo INT UNSIGNED, id_atividade TINYINT UNSIGNED, PRIMARY KEY (id_ciclo, id_atividade));
CREATE TABLE ciclo_painel_atividade (id_ciclo INT UNSIGNED, id_atividade TINYINT UNSIGNED, PRIMARY KEY (id_ciclo, id_atividade));
CREATE TABLE ciclo_sincronizar (id_ciclo INT UNSIGNED, id_ciclo_destino INT UNSIGNED, PRIMARY KEY (id_ciclo, id_ciclo_destino));

-- =========================================================================
-- 4. VISITA, LABORATÓRIO
-- =========================================================================
CREATE TABLE visitas (
  id               BIGINT UNSIGNED PRIMARY KEY,
  id_local         BIGINT UNSIGNED NOT NULL,
  id_ciclo         INT UNSIGNED NOT NULL,
  id_atividade     TINYINT UNSIGNED NOT NULL,
  id_usuario       INT UNSIGNED NOT NULL,             -- agente ("por")
  data             DATE NOT NULL, hora_entrada TIME NULL,
  tipo_visita      ENUM('Normal','Recuperação') NOT NULL DEFAULT 'Normal',
  pendencia        ENUM('Nenhuma','Fechado','Recusado','Tentativa') NOT NULL DEFAULT 'Nenhuma',
  semana           TINYINT UNSIGNED NULL,              -- semana epidemiológica (ISO ou SINAN)
  assinatura       MEDIUMTEXT NULL,                    -- imagem/base64 ou caminho
  anotacao         TEXT NULL,
  -- depósitos
  achou_deposito   TINYINT(1) NOT NULL DEFAULT 0,
  a1 INT NULL, a2 INT NULL, b INT NULL, c INT NULL, d1 INT NULL, d2 INT NULL, e INT NULL,
  total_eliminado  INT NULL,
  -- amostra
  coletou_amostra  TINYINT(1) NOT NULL DEFAULT 0,
  -- tratamento
  fez_tratamento   TINYINT(1) NOT NULL DEFAULT 0,
  l1_gramas DECIMAL(8,2) NULL, l1_tratados INT NULL, l2_gramas DECIMAL(8,2) NULL, l2_tratados INT NULL,
  perifocal_tipo VARCHAR(40) NULL, perifocal_qtd INT NULL, tratamento_eliminados INT NULL,
  bri_volume       DECIMAL(8,2) NULL,
  -- snapshot dos moradores/habitantes na hora da visita (inferido: tela mostra "Dados do Local" da visita)
  latitude DECIMAL(10,7) NULL, longitude DECIMAL(10,7) NULL, precisao_gps DECIMAL(8,2) NULL,
  cad_situacao     ENUM('Ativo','Desativado') NOT NULL DEFAULT 'Ativo',
  data_tablet DATETIME NULL, data_sincronismo DATETIME NULL,
  created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  KEY ix_v_ciclo (id_ciclo, id_atividade), KEY ix_v_data (data), KEY ix_v_local (id_local),
  FOREIGN KEY (id_local) REFERENCES locais(id), FOREIGN KEY (id_ciclo) REFERENCES ciclos(id),
  FOREIGN KEY (id_atividade) REFERENCES atividades(id), FOREIGN KEY (id_usuario) REFERENCES usuarios(id)
);
CREATE TABLE visita_foto (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_visita BIGINT UNSIGNED NOT NULL,
  caminho VARCHAR(255) NOT NULL, FOREIGN KEY (id_visita) REFERENCES visitas(id) ON DELETE CASCADE
);
CREATE TABLE visita_anotacao (                       -- "Anotações Internas"
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_visita BIGINT UNSIGNED NOT NULL,
  id_usuario INT UNSIGNED NOT NULL, texto TEXT NOT NULL, created_at TIMESTAMP NULL,
  FOREIGN KEY (id_visita) REFERENCES visitas(id) ON DELETE CASCADE
);

CREATE TABLE leis_municipais (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, numero VARCHAR(15) NOT NULL, ano SMALLINT NOT NULL,
  nome VARCHAR(200) NOT NULL, publicacao DATE NULL, ativo TINYINT(1) NOT NULL DEFAULT 1
);
CREATE TABLE leis_trechos (                          -- "Selecionar Trechos da Lei Municipal"
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_lei INT UNSIGNED NOT NULL,
  artigo VARCHAR(60) NULL, texto TEXT NOT NULL, FOREIGN KEY (id_lei) REFERENCES leis_municipais(id)
);
CREATE TABLE visita_lei_trecho (id_visita BIGINT UNSIGNED, id_trecho INT UNSIGNED, PRIMARY KEY (id_visita, id_trecho));

CREATE TABLE amostras (                              -- tubitos / tubetes
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_visita BIGINT UNSIGNED NOT NULL,
  numero_tubito VARCHAR(30) NULL,                    -- automático ou digitado (parâmetro)
  status ENUM('Em Análise','Concluído') NOT NULL DEFAULT 'Em Análise',
  total_aegypti INT NOT NULL DEFAULT 0, total_albopictus INT NOT NULL DEFAULT 0, total_negativo INT NOT NULL DEFAULT 0,
  id_usuario_resultado INT UNSIGNED NULL, data_resultado DATETIME NULL,
  FOREIGN KEY (id_visita) REFERENCES visitas(id) ON DELETE CASCADE
);

-- =========================================================================
-- 5. VIGILÂNCIA DE CASOS
-- =========================================================================
CREATE TABLE doencas (id SMALLINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, nome VARCHAR(80) NOT NULL, tem_entrevista TINYINT(1) NOT NULL DEFAULT 0);

CREATE TABLE investigacoes (
  id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  sinan          BIGINT UNSIGNED NOT NULL,
  id_doenca      SMALLINT UNSIGNED NOT NULL,
  psf            VARCHAR(120) NOT NULL,
  data_notificado DATE NOT NULL,
  paciente       VARCHAR(150) NOT NULL, nome_mae VARCHAR(150) NOT NULL, data_nascimento DATE NULL,
  telefone VARCHAR(20) NULL, telefone2 VARCHAR(20) NULL, observacao TEXT NULL,
  endereco VARCHAR(200) NOT NULL, numero VARCHAR(15) NOT NULL, referencia VARCHAR(200) NULL,
  id_bairro INT UNSIGNED NULL, quarteirao VARCHAR(30) NULL, lado VARCHAR(5) NULL,
  status         VARCHAR(30) NOT NULL DEFAULT 'Aguardando',   -- Aguardando, Em atendimento, Encerrado... (inferido)
  resultado      ENUM('Positivo','Negativo','Inconclusivo') NULL,
  id_local       BIGINT UNSIGNED NULL,                        -- "Vincular a local"
  id_usuario     INT UNSIGNED NULL,                           -- responsável atual
  envio_tablet   TINYINT(1) NOT NULL DEFAULT 0,
  excluida       TINYINT(1) NOT NULL DEFAULT 0,
  created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  KEY ix_inv (id_doenca, status), FOREIGN KEY (id_doenca) REFERENCES doencas(id)
);
-- Linha do tempo genérica ("Atividades"): atender, encaminhar, anotação, resultado, vincular local, encerrar, reabrir
CREATE TABLE investigacao_atividade (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_investigacao BIGINT UNSIGNED NOT NULL,
  tipo VARCHAR(30) NOT NULL, descricao TEXT NULL, id_usuario INT UNSIGNED NULL, id_usuario_destino INT UNSIGNED NULL,
  created_at TIMESTAMP NULL, FOREIGN KEY (id_investigacao) REFERENCES investigacoes(id) ON DELETE CASCADE
);
CREATE TABLE investigacao_anexo (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_atividade BIGINT UNSIGNED NOT NULL, caminho VARCHAR(255) NOT NULL
);
-- Entrevista Leishmaniose (campos vistos); outras doenças têm entrevistas próprias (param "Doenças/Entrevistas")
CREATE TABLE investigacao_entrevista_leish (
  id_investigacao BIGINT UNSIGNED PRIMARY KEY,
  possui_caes ENUM('Sim','Não') NULL, quantidade_caes INT NULL, cao_sinais_sugestivos ENUM('Sim','Não') NULL,
  presenca_mosquito_palha ENUM('Sim','Não') NULL, area_mata_rural ENUM('Sim','Não') NULL,
  abrigo_animais_proximo ENUM('Sim','Não') NULL, materia_organica_peridomicilio ENUM('Sim','Não') NULL,
  outros_casos_vizinhanca ENUM('Sim','Não') NULL, resumo_peridomicilio TEXT NULL
);

CREATE TABLE denuncias (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, anonima TINYINT(1) NOT NULL DEFAULT 0,
  nome_cidadao VARCHAR(150) NULL, cpf_cidadao VARCHAR(14) NULL, telefone_cidadao VARCHAR(20) NULL,
  data_denuncia DATETIME NOT NULL, tipo VARCHAR(60) NULL, titulo VARCHAR(150) NULL, descricao TEXT NULL,
  endereco VARCHAR(200) NULL, numero VARCHAR(15) NULL, referencia VARCHAR(200) NULL,
  id_bairro INT UNSIGNED NULL, id_quarteirao INT UNSIGNED NULL, lado VARCHAR(5) NULL,
  status VARCHAR(30) NOT NULL DEFAULT 'Aguardando', origem VARCHAR(30) NULL,   -- origem: web, WhatsApp... (inferido)
  id_local BIGINT UNSIGNED NULL, id_visita BIGINT UNSIGNED NULL,
  created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL
);
CREATE TABLE denuncia_atividade (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_denuncia BIGINT UNSIGNED NOT NULL,
  tipo VARCHAR(30) NOT NULL, descricao TEXT NULL, id_usuario INT UNSIGNED NULL, created_at TIMESTAMP NULL
);

-- Notificação -> Auto de infração (originados de uma visita)
CREATE TABLE notificacoes (
  id BIGINT UNSIGNED PRIMARY KEY, id_visita BIGINT UNSIGNED NOT NULL,
  data_notificacao DATETIME NOT NULL, prazo DATETIME NOT NULL, motivo TEXT NOT NULL, morador_ausente TINYINT(1) NOT NULL DEFAULT 0,
  assinatura MEDIUMTEXT NULL, id_notificador INT UNSIGNED NOT NULL,
  status ENUM('Notificado','Em análise','Resolvido','Cancelado','Infração') NOT NULL DEFAULT 'Notificado',
  data_resolucao DATETIME NULL, id_resolvido_por INT UNSIGNED NULL,
  data_cancelamento DATETIME NULL, id_cancelado_por INT UNSIGNED NULL,
  FOREIGN KEY (id_visita) REFERENCES visitas(id)
);
CREATE TABLE auto_infracao (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_notificacao BIGINT UNSIGNED NOT NULL UNIQUE,
  data_auto DATETIME NOT NULL, prazo_pagamento DATETIME NOT NULL, dias_prazo SMALLINT NULL,
  valor DECIMAL(12,2) NOT NULL, classificacao ENUM('Leve','Média','Grave','Gravíssima') NOT NULL,
  upfm ENUM('100','150','200','250') NOT NULL, motivo TEXT NOT NULL,
  status ENUM('Aberto','Pago','Cancelado') NOT NULL DEFAULT 'Aberto',
  data_pagamento DATETIME NULL, id_pago_por INT UNSIGNED NULL,
  FOREIGN KEY (id_notificacao) REFERENCES notificacoes(id)
);
CREATE TABLE notificacao_lei_trecho (id_notificacao BIGINT UNSIGNED, id_trecho INT UNSIGNED, PRIMARY KEY (id_notificacao, id_trecho));

CREATE TABLE drone_solicitacoes (
  id BIGINT UNSIGNED PRIMARY KEY, id_local BIGINT UNSIGNED NOT NULL, id_usuario INT UNSIGNED NOT NULL,
  data DATETIME NOT NULL,
  status ENUM('Aguardando','Em atendimento','Encerrado','Cancelada','Atendimento Finalizado') NOT NULL DEFAULT 'Aguardando'
);
CREATE TABLE drone_atividade (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_solicitacao BIGINT UNSIGNED NOT NULL,
  tipo VARCHAR(30) NOT NULL, descricao TEXT NULL, id_usuario INT UNSIGNED NULL, id_usuario_destino INT UNSIGNED NULL,
  created_at TIMESTAMP NULL
);
CREATE TABLE drone_anexo (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_atividade BIGINT UNSIGNED NOT NULL, caminho VARCHAR(255) NOT NULL);

-- =========================================================================
-- 6. ARMADILHAS
-- =========================================================================
CREATE TABLE ovitrampas (
  id BIGINT UNSIGNED PRIMARY KEY, numero VARCHAR(20) NULL, id_local BIGINT UNSIGNED NOT NULL,
  data DATE NOT NULL, id_usuario INT UNSIGNED NOT NULL,
  status ENUM('Ativo','Encerrado','Extraviado','Sem Acesso','Cancelado') NOT NULL DEFAULT 'Ativo',
  data_retirada DATE NULL, id_por_retirada INT UNSIGNED NULL, anotacao TEXT NULL,
  cad_situacao ENUM('Ativo','Desativado') NOT NULL DEFAULT 'Ativo'
);
CREATE TABLE ovitrampa_contagem (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_ovitrampa BIGINT UNSIGNED NOT NULL,
  quantidade INT NOT NULL, data DATE NOT NULL, id_usuario INT UNSIGNED NOT NULL,
  FOREIGN KEY (id_ovitrampa) REFERENCES ovitrampas(id) ON DELETE CASCADE
);

CREATE TABLE edls (
  id BIGINT UNSIGNED PRIMARY KEY, numero VARCHAR(20) NULL, id_local BIGINT UNSIGNED NOT NULL,
  data DATE NOT NULL, id_usuario INT UNSIGNED NOT NULL,
  status ENUM('Operando','Encerrado','Extraviado','Sem Acesso','Cancelado','Retirado') NOT NULL DEFAULT 'Operando',
  data_retirada DATE NULL, id_por_retirada INT UNSIGNED NULL, anotacao TEXT NULL,
  cad_situacao ENUM('Ativo','Desativado') NOT NULL DEFAULT 'Ativo'
);
CREATE TABLE edl_manutencao (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_edl BIGINT UNSIGNED NOT NULL,
  data DATE NOT NULL, id_usuario INT UNSIGNED NOT NULL,
  nivel ENUM('Seco','< 50%','> 50%') NULL,
  larvas TINYINT(1) DEFAULT 0, pulpas TINYINT(1) DEFAULT 0, tecido TINYINT(1) DEFAULT 0, fechado TINYINT(1) DEFAULT 0,
  nao_instalada TINYINT(1) DEFAULT 0, desaparecida TINYINT(1) DEFAULT 0, quebrada TINYINT(1) DEFAULT 0, retirada TINYINT(1) DEFAULT 0,
  anotacao TEXT NULL, FOREIGN KEY (id_edl) REFERENCES edls(id) ON DELETE CASCADE
);

-- =========================================================================
-- 7. RECONHECIMENTO, RESUMO, LIRAa
-- =========================================================================
CREATE TABLE reconhecimentos (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_bairro INT UNSIGNED NOT NULL, id_ciclo INT UNSIGNED NOT NULL,
  data_reconhecimento DATE NOT NULL, data_atualizacao DATE NOT NULL,
  qtd_imoveis_residenciais INT, qtd_imoveis_comerciais INT, qtd_outros_residenciais INT, qtd_habitantes INT,
  qtd_quarteiroes INT, qtd_pontos INT, qtd_armadilha INT, qtd_terrenos INT,
  animais_cachorro INT, animais_gato INT, animais_roedores INT,
  longitude DECIMAL(10,7), latitude DECIMAL(10,7), altitude DECIMAL(8,2), distancia_centro DECIMAL(8,2),
  cad_situacao ENUM('Ativo','Desativado') NOT NULL DEFAULT 'Ativo'
);
-- infra_* e risco_* = Sim/Não/Ignorado; tabela chave-valor evita 22 colunas
CREATE TABLE reconhecimento_item (
  id_reconhecimento INT UNSIGNED NOT NULL,
  grupo ENUM('infra','risco') NOT NULL,
  item VARCHAR(30) NOT NULL,   -- infra: energia, agua, esgoto, lavanderia, privada, lixo, telefonica, transporte, rua, escola, postosaude, acesso, pacs | risco: malaria, dengue, esquisto, leish, maculosa, peste, chagas, amarela
  valor ENUM('Sim','Não','Ignorado') NOT NULL DEFAULT 'Ignorado',
  PRIMARY KEY (id_reconhecimento, grupo, item)
);

CREATE TABLE resumos_semanais (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_ciclo INT UNSIGNED NOT NULL, semana TINYINT UNSIGNED NOT NULL,
  id_bairro INT UNSIGNED NOT NULL, id_atividade TINYINT UNSIGNED NOT NULL, gerado_em DATETIME NOT NULL,
  dados JSON NULL           -- (inferido) consolidado da produção impresso na tela "Imprimir"
);

CREATE TABLE liraa_grupos (id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, nome VARCHAR(80) NOT NULL, status ENUM('Ativo','Desativado') NOT NULL DEFAULT 'Ativo');
CREATE TABLE liraa_grupo_bairro (id_grupo INT UNSIGNED, id_bairro INT UNSIGNED, status ENUM('Manter','Excluir') DEFAULT 'Manter', PRIMARY KEY (id_grupo, id_bairro));
CREATE TABLE liraa_agenda (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_ciclo INT UNSIGNED NOT NULL, semana TINYINT UNSIGNED NOT NULL,
  id_bairro INT UNSIGNED NOT NULL, id_usuario INT UNSIGNED NOT NULL, status ENUM('Ativo','Desativado') NOT NULL DEFAULT 'Ativo'
);
CREATE TABLE liraa_agenda_quarteirao (id_agenda INT UNSIGNED, id_quarteirao INT UNSIGNED, PRIMARY KEY (id_agenda, id_quarteirao));

-- =========================================================================
-- 8. ZOONOSE
-- =========================================================================
CREATE TABLE campanhas_registro (                   -- "Registros": visita de vacinação em um local
  id BIGINT UNSIGNED PRIMARY KEY, id_local BIGINT UNSIGNED NOT NULL, data DATETIME NOT NULL,
  id_usuario INT UNSIGNED NOT NULL, id_diaria INT UNSIGNED NULL,
  uso_estoque TINYINT(1) NOT NULL DEFAULT 0
);
CREATE TABLE campanha_animal (                      -- vacinados / não vacinados
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_registro BIGINT UNSIGNED NOT NULL,
  tipo ENUM('Cão','Gato','Outro') NOT NULL, nome VARCHAR(80) NULL,
  vacinado TINYINT(1) NOT NULL, motivo_nao_vacinado VARCHAR(120) NULL,
  id_produto INT UNSIGNED NULL, id_lote INT UNSIGNED NULL     -- "Registro por Animal — Estoque"
);
CREATE TABLE campanha_diarias (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, data DATE NOT NULL, anotacao TEXT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'Aberta', ativo ENUM('Ativo','Inativo') NOT NULL DEFAULT 'Ativo',
  id_criado_por INT UNSIGNED NOT NULL,
  inicio TIME NULL, ini_seringa INT NULL, ini_vacina_fechado INT NULL, ini_vacina_aberto INT NULL, ini_vacina_doses DECIMAL(10,3) NULL,
  temp_inicio TIME NULL, temp_estoque_ini DECIMAL(4,1) NULL, temp_uso_ini DECIMAL(4,1) NULL,
  fim TIME NULL, fim_seringa INT NULL, fim_seringa_perda INT NULL, fim_seringa_defeito INT NULL,
  fim_vacina_usada INT NULL, fim_vacina_vazio INT NULL, fim_vacina_cheio INT NULL, fim_vacina_aberto INT NULL, fim_vacina_perda INT NULL,
  seringa_defeito INT NULL, seringa_descarte INT NULL, vacina_descarte INT NULL, vacina_perda INT NULL
);
CREATE TABLE campanha_diaria_equipe (id_diaria INT UNSIGNED, id_usuario INT UNSIGNED, lider TINYINT(1) DEFAULT 0, PRIMARY KEY (id_diaria, id_usuario));
CREATE TABLE campanha_diaria_temperatura (
  id_diaria INT UNSIGNED, horario ENUM('08','10','12','14','17') NOT NULL, temp_estoque DECIMAL(4,1) NULL, temp_uso DECIMAL(4,1) NULL,
  PRIMARY KEY (id_diaria, horario)
);

CREATE TABLE lvc (                                  -- Leishmaniose Visceral Canina
  id BIGINT UNSIGNED PRIMARY KEY, id_local BIGINT UNSIGNED NULL, data DATETIME NOT NULL,
  numero_controle VARCHAR(30) NULL, proprietario VARCHAR(150) NULL, cachorro VARCHAR(80) NULL, idade VARCHAR(20) NULL,
  sexo ENUM('M','F') NULL, dpp VARCHAR(30) NULL, reteste VARCHAR(30) NULL, acompanhamento VARCHAR(120) NULL,
  id_usuario INT UNSIGNED NOT NULL
);

CREATE TABLE baap (                                 -- Busca Ativa Animais Peçonhentos
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, sinan BIGINT UNSIGNED NOT NULL,
  local_picada ENUM('Cabeça','Braço','Ante-Braço','Mão','Dedo da Mão','Tronco','Coxa','Perna','Pé','Dedo do Pé','Outro') NULL,
  animal ENUM('Serpente','Aranha','Escorpião','Lagarta','Abelha','Lacraia','Outro') NULL,
  animal_outro VARCHAR(80) NULL, especie VARCHAR(80) NULL, situacao_paciente TEXT NULL, atuacao_vga VARCHAR(120) NULL,
  data_notificado DATE NOT NULL, paciente VARCHAR(150) NOT NULL, nome_mae VARCHAR(150) NULL, data_nascimento DATE NULL,
  telefone VARCHAR(20) NULL, telefone2 VARCHAR(20) NULL, observacao TEXT NULL,
  endereco VARCHAR(200) NOT NULL, numero VARCHAR(15) NOT NULL, referencia VARCHAR(200) NULL,
  id_bairro INT UNSIGNED NOT NULL, id_quarteirao INT UNSIGNED NULL, lado VARCHAR(5) NULL,
  status VARCHAR(30) NOT NULL DEFAULT 'Aguardando', id_local BIGINT UNSIGNED NULL
);
CREATE TABLE baap_atividade (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_baap BIGINT UNSIGNED NOT NULL,
  tipo VARCHAR(30) NOT NULL, descricao TEXT NULL, id_usuario INT UNSIGNED NULL, created_at TIMESTAMP NULL
);

-- =========================================================================
-- 9. ESTOQUE E PATRIMÔNIO
-- =========================================================================
CREATE TABLE estoque_unidades (id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, nome VARCHAR(100) NOT NULL, descricao TEXT NULL, ativo TINYINT(1) NOT NULL DEFAULT 1);
CREATE TABLE estoque_produtos (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_unidade INT UNSIGNED NOT NULL, nome VARCHAR(150) NOT NULL,
  tipo ENUM('Vacina','Equipamento','Insumo','Outros') NOT NULL, unidade_medida VARCHAR(20) NOT NULL,
  tipo_saida_permitido ENUM('Campo e Permanente','Apenas Campo','Apenas Permanente') NOT NULL,
  fracionavel TINYINT(1) NOT NULL DEFAULT 0, doses_por_unidade DECIMAL(10,3) NULL, unidade_fracao ENUM('Dose','Mililitro (ml)','Miligrama (mg)','Unidade Internacional (UI)') NULL,
  descricao TEXT NULL, selecionavel_diaria TINYINT(1) NOT NULL DEFAULT 0, categoria_diaria ENUM('Vacina','Seringa') NULL,
  ativo TINYINT(1) NOT NULL DEFAULT 1, FOREIGN KEY (id_unidade) REFERENCES estoque_unidades(id)
);
CREATE TABLE estoque_lotes (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_produto INT UNSIGNED NOT NULL, numero_lote VARCHAR(40) NOT NULL,
  fabricante VARCHAR(100) NULL, data_fabricacao DATE NULL, data_validade DATE NULL,
  quantidade DECIMAL(12,3) NOT NULL, saldo DECIMAL(12,3) NOT NULL, custo_unitario DECIMAL(12,2) NULL,
  nota_fiscal VARCHAR(30) NULL, fornecedor VARCHAR(120) NULL, observacao TEXT NULL,
  FOREIGN KEY (id_produto) REFERENCES estoque_produtos(id)
);
CREATE TABLE estoque_movimentacao (                 -- "Histórico de Movimentações" do lote
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_lote INT UNSIGNED NOT NULL,
  tipo ENUM('Entrada','Retirada','Devolução','Uso','Perda','Baixa','Ajuste') NOT NULL,
  quantidade DECIMAL(12,3) NOT NULL, id_usuario INT UNSIGNED NOT NULL, descricao VARCHAR(255) NULL,
  id_retirada INT UNSIGNED NULL, origem VARCHAR(40) NULL, created_at TIMESTAMP NULL
);
CREATE TABLE estoque_retiradas (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, tipo ENUM('Campo','Permanente') NOT NULL,
  id_usuario INT UNSIGNED NOT NULL, data_saida DATETIME NOT NULL, data_retorno_prevista DATETIME NULL, data_retorno_real DATETIME NULL,
  status ENUM('Aberta','Finalizada','Cancelada') NOT NULL DEFAULT 'Aberta', observacao TEXT NULL
);
CREATE TABLE estoque_retirada_itens (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_retirada INT UNSIGNED NOT NULL, id_lote INT UNSIGNED NOT NULL,
  retirado DECIMAL(12,3) NOT NULL, usado DECIMAL(12,3) NOT NULL DEFAULT 0, devolvido DECIMAL(12,3) NOT NULL DEFAULT 0, perda DECIMAL(12,3) NOT NULL DEFAULT 0
);
CREATE TABLE patrimonios (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_lote INT UNSIGNED NOT NULL, numero_patrimonio VARCHAR(30) NOT NULL UNIQUE,
  numero_serie VARCHAR(60) NULL, nome VARCHAR(150) NOT NULL, id_responsavel INT UNSIGNED NULL, data_entrada DATE NOT NULL,
  status ENUM('Ativo','Em manutenção','Baixado') NOT NULL DEFAULT 'Ativo', observacao TEXT NULL
);
CREATE TABLE patrimonio_evento (                    -- atribuir/transferir, manutenção, anotação, baixa, reativar
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_patrimonio INT UNSIGNED NOT NULL,
  tipo VARCHAR(30) NOT NULL, descricao TEXT NULL, id_usuario INT UNSIGNED NOT NULL, created_at TIMESTAMP NULL
);

-- =========================================================================
-- 10. COMUNICAÇÃO, BANNER, PARÂMETROS, AUDITORIA, MOBILIDADE
-- =========================================================================
CREATE TABLE comunicacoes (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, tipo ENUM('WhatsApp','E-mail') NOT NULL,
  status VARCHAR(20) NOT NULL, destinatario VARCHAR(150) NOT NULL, assunto VARCHAR(200) NULL, corpo TEXT NULL,
  tentativas SMALLINT NOT NULL DEFAULT 0, enviado_em DATETIME NULL, visualizado_em DATETIME NULL,
  contexto JSON NULL,           -- "Dados de Contexto" (ex.: id_notificacao)
  created_at TIMESTAMP NULL
);
CREATE TABLE comunicacao_timeline (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_comunicacao BIGINT UNSIGNED NOT NULL, evento VARCHAR(40), detalhe VARCHAR(255), created_at TIMESTAMP NULL);

CREATE TABLE banners (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, nome VARCHAR(100) NOT NULL, largura INT NOT NULL, altura INT NOT NULL,
  imagem_fundo VARCHAR(255) NULL, definicao JSON NOT NULL   -- elementos (texto, fonte, cor, negrito, sombra) do editor visual
);

CREATE TABLE parametros (                           -- chave/valor para telas Geral e Mobilidade
  chave VARCHAR(60) PRIMARY KEY, valor VARCHAR(255) NULL
);
-- Chaves vistas: acesso_local (responsavel_bairro|parametro), semanas_sinan (iso|sinan), visita_tubito_automatico,
-- campanha_estoque, metros (aviso de precisão GPS), visita_numero_tubito, coleta_gps (a cada 5 s), permite_fechado_df,
-- cep_padrao, senha_limpardados, dashboard_grafico, dias_download_visitas_fotos, dias_download_campanhas,
-- dias_download_investigacoes, dias_download_investigacao_anexos

CREATE TABLE acesso_local (                         -- "Controle de Acesso aos Locais"
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_usuario INT UNSIGNED NOT NULL,
  tipo ENUM('Bairro','Quarteirão','Local') NOT NULL, id_referencia BIGINT UNSIGNED NOT NULL,
  descricao VARCHAR(200) NULL, data DATE NULL
);

CREATE TABLE auditoria (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, tabela VARCHAR(40) NOT NULL, id_registro BIGINT UNSIGNED NOT NULL,
  tipo ENUM('INSERT','UPDATE','DELETE') NOT NULL, id_usuario INT UNSIGNED NULL, data DATETIME NOT NULL,
  KEY ix_aud (tabela, id_registro)
);
CREATE TABLE auditoria_coluna (id_auditoria BIGINT UNSIGNED, coluna VARCHAR(60), valor_antigo TEXT NULL, valor_novo TEXT NULL);

CREATE TABLE mobilidade_historico (                 -- sincronismo do tablet
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, descricao VARCHAR(255) NOT NULL, id_relacao BIGINT UNSIGNED NULL,
  data_sincronismo DATETIME NOT NULL, data_tablet DATETIME NULL, id_usuario INT UNSIGNED NOT NULL
);
CREATE TABLE coleta_gps (                           -- "Rastro GPS": ponto a cada 5 s
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, id_usuario INT UNSIGNED NOT NULL,
  latitude DECIMAL(10,7) NOT NULL, longitude DECIMAL(10,7) NOT NULL, data DATETIME NOT NULL, KEY ix_gps (id_usuario, data)
);
CREATE TABLE changelog (
  id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY, versao VARCHAR(15) NOT NULL, data DATE NOT NULL,
  tipo VARCHAR(30) NULL, titulo VARCHAR(150) NOT NULL, descricao TEXT NULL
);

# Geo Controle (geocontrole.novamutum.mt.gov.br) — Descritivo de engenharia reversa

Levantamento feito em 2026-10-29 com sessão logada (perfil com acesso total), usando só requisições GET e
leitura do DOM. Nenhum registro foi criado, alterado ou excluído, e nenhum dado pessoal foi copiado para este
documento. Versão do sistema no momento: **v1.87.01** (tela "Atualizações").

Itens marcados **(inferido)** não foram vistos diretamente: vêm de nomes de campos, rotas e respostas.

> **Segunda varredura:** o aprofundamento por módulo está em
> [engenharia-reversa-geocontrole-apendice.md](engenharia-reversa-geocontrole-apendice.md) e o modelo de dados
> sugerido (SQL, inferido) em [geocontrole-modelo-dados.sql](geocontrole-modelo-dados.sql).

## 1. Visão geral

Sistema de **vigilância ambiental e controle de endemias** (dengue, chikungunya etc.) e **zoonoses**:
agentes visitam imóveis ("Locais") em ciclos bimestrais, registram depósitos, amostras e tratamento, e o
laboratório lança resultados. O sistema ainda cobre investigação de casos notificados, denúncias, campanha
antirrábica, LVC, animais peçonhentos, ovitrampas, EDLs (estações disseminadoras de larvicida), LIRAa,
estoque/patrimônio e mapas/BI. Há **aplicativo de tablet** (sincronismo offline) e **painel público**.

## 2. Stack técnica observada

| Camada | Observado |
|---|---|
| Backend | Laravel (HTML server-rendered, erros 500 com formato do Laravel) + **Livewire** (`livewire.js`) |
| Front | AdminLTE 3 + Bootstrap 4 + jQuery; select2, daterangepicker, inputmask, jquery-validation (pt_BR), toastr, SweetAlert2, bootstrap-table, summernote, duallistbox, knob |
| Gráficos | Chart.js (dashboard) e ApexCharts (BI) |
| Mapas | Mapbox GL JS 1.7 |
| Auth | Sessão web; usuário com flags `login_web` e `login_tablet` |
| Padrão de listagem | Página com filtros + parcial HTML em `GET /{recurso}/results?page=N` carregado por AJAX; paginação de 50 linhas |
| Selects dependentes | `GET /select/quarteirao` (select2 AJAX, devolve `{total_count, incomplete_results, items}`) |
| BI | `GET /bi/dados?` e `GET /bi/bi-epi/dados?` devolvem JSON agregado |
| Mapas | `/mapa/{visita/pontos, doenca/pontos, campanha/pontos, ovitrampa/pontos, edl/pontos, coleta/rota, coleta/users, coleta/visitas, visitas/ciclos}` |

## 3. Navegação (menu lateral) e rotas

**Início:** Dashboard `/dashboard` · B.I. `/bi` · B.I. Epidemiológico `/bi/bi-epi`

**Mapa:** Visitas `/mapa/visitas` · Rastro GPS `/mapa/coleta` · Doenças `/mapa/doenca` · Campanha Antirrábica
`/mapa/campanha` · Ovitrampas `/mapa/ovitrampa` · EDLs `/mapa/edl`

**Operação:** Locais `/local` · Visitas `/visita` · Laboratório `/laboratorio` (com contador de pendentes) ·
Investigação `/investigacao` · Denúncias `/denuncia` · Histórico Mobilidade `/mobilidade/historico` · Bairros
`/bairro` · Ovitrampas `/ovitrampa` · EDLs `/edl` · Notificados `/notificado` · Solicitações de Drone
`/dronesolicitacao` · Comunicações `/comunicacao` · Banners `/banner`

**Zoonose:** Campanha Antirrábica (Registros `/zoonose/campanha`, Diária de Trabalho
`/zoonose/campanha/diaria`) · LVC `/zoonose/lvc` · BAAP `/zoonose/baap`

**LIRAa:** Grupo `/liraa/grupo` · Agenda `/liraa/agenda`

**Relatórios:** Reconhecimento Geográfico `/reconhecimento` · Resumo Semanal `/resumosemanal` · e 13
relatórios em `/relatorio/*` (seção 6)

**Estoque:** Balanço Físico `/estoque/balanco` · Produtos `/estoque/produto` · Entradas/Lotes `/estoque/lote` ·
Retiradas `/estoque/retirada` · Patrimônio `/estoque/patrimonio` · Unidades `/estoque/unidade`

**Parâmetros:** Ciclo `/parametros/ciclo` · Geral `/parametros/geral` · Controle de Acesso
`/parametros/acessolocal` (existem também Mobilidade, Leis Municipais e Doenças/Entrevistas, só vistos como
permissões)

**Auditoria:** Visita `/auditoria/visita` · Local `/auditoria/local`

**Configuração:** Usuários `/configuracao/usuario` · Papel `/configuracao/papel` · Atualizações `/changelog`

Padrão de rotas CRUD: `/{r}` lista, `/{r}/novo` formulário, `/{r}/{id}` detalhe, `/{r}/{id}/editar` ou
`/{r}/editar/{id}` edição (inconsistente entre módulos), `POST /{r}` grava.

## 4. Modelo de dados inferido (entidades e campos)

### Estrutura territorial
- **Cidade** (Nova Mutum/MT; select pronto para multi-cidade).
- **Bairro**: nome, mutirão (S/N), status (Ativo/Extinto), responsáveis (N:N com usuários), código IBGE,
  classificação (Urbano/Rural), zona, categoria (Bairro/Povoado), lat/long/altitude, distância do centro (km),
  "entra no relatório de produção" (S/N). Listagem: ID, Nome, Responsável, Multirão, Locais, Quarteirões, Status.
- **Quarteirão**: pertence a bairro; usa-se também "Lado".
- **Local** (imóvel): proprietário (nome, telefone, CPF, WhatsApp, e-mail, aceita WhatsApp, aceita e-mail),
  inquilino (nome, telefone), habitantes (humanos, cachorros, gatos, roedores), tipo (Residencial, Comércio,
  Terreno Baldio, Ponto Estratégico, Outro), CEP, endereço, número, referência, bairro, quarteirão, lado,
  sequência, complemento, cidade, latitude/longitude. Cadastro em lote "em sequência" ou "em complemento".
  Filtros: endereço, número, bairro, quarteirão, lado, ID, referência, sequência, complemento, tipo, ordenação.

### Ciclo e atividades
- **Ciclo**: nome, data início/fim, situação (Operando/Fechado), permite visita fora do período, ciclos
  sincronizados (N:N), atividades habilitadas (N:N), visibilidade no painel público (+ nome público e
  atividades exibidas). Convenção: 6 ciclos por ano.
- **Atividades**: LIT, LI, PE, T, DF, PVE, LIRAa, LIA, BRI.

### Visita (entidade central)
- Dados: ID numérico longo, data, hora de entrada, tipo (Normal/Recuperação), pendência
  (Nenhuma/Fechado/Recusado/Tentativa), atividade, ciclo, semana epidemiológica (ISO ou SINAN, parâmetro),
  assinatura, agente, anotação, fotos (upload múltiplo), situação (Ativo/Desativado).
- **Depósitos encontrados**: flag "achou depósito"; contadores A1, A2, B, C, D1, D2, E e total eliminados.
- **Amostras**: flag "coletou amostra"; tubitos com numeração automática (parâmetro); resultado no laboratório.
- **Tratamento**: L1 (gramas, depósitos tratados), L2 (gramas, depósitos tratados), adulticida (tipo, cargas),
  depósitos eliminados no tratamento.
- **BRI**: volume aplicado.
- Vínculo com **trechos de Lei Municipal** (select de lei + trechos), usado em notificação/infração.
- Telas: lista (ID, Endereço, Data/Agente, Ciclo, Sem., Atividade, Pendência, Depósito, Amostra, Tratamento,
  Ações), detalhe `/local/visita/{id}` (Dados do Local, Moradores, Habitantes, Dados da Visita, Anotações
  internas, Alterar resultados, imprimir, editar) e edição `/visita/editar/{id}`. **Visitas não são criadas
  pelo web**: vêm do tablet.
- Filtros: endereço, número, bairro, quarteirão, lado, ID, ciclo, agente, atividade, semana, período,
  pendência, depósito, amostra, tratamento, foto.

### Laboratório
Mesma visita com resultado: status (Em Análise/Concluído), nº de tubetes, total *A. aegypti*, total
*A. albopictus*, total negativo. Permissões separadas para alterar resultados e para ciclo fechado.

### Casos e vigilância
- **Investigação** (caso notificado): SINAN, doença (Dengue, Chikungunya, Chagas, Esquistossomose, Atendimento
  Antirrábico etc.), PSF, data de notificação, paciente, nome da mãe, nascimento, telefones, observação,
  endereço/bairro antigo/quarteirão/lado, status, resultado, usuário/tipo de usuário. Ações: atender,
  anotação, encaminhar, resultado, vincular a local, entrevista, encerrar, reabrir, importar, excluir.
- **Denúncia**: anônima (S/N), cidadão (busca por telefone, nome, CPF), data, tipo, título, descrição,
  endereço, bairro, quarteirão, status e origem. Ações: atender, encaminhar, anotação, visita, local,
  encerrar, cancelar.
- **Notificado / Auto de infração**: número, status (Notificado, Em análise, Resolvido, Cancelado, Infração),
  data, local, por, autuado. Ações: resolver, criar auto de infração, cancelar, marcar infração como paga.
- **Solicitação de Drone**: status (Aguardando, Em atendimento, Encerrado, Cancelada, Atendimento
  Finalizado), local, por; ações atender, anotação, encaminhar, encerrar.
- **Ovitrampa**: status (Ativo, Encerrado, Extraviado, Sem Acesso, Cancelado), local, por, última contagem,
  retirada, contagem de ovos (incluir/excluir).
- **EDL**: status (Operando, Encerrado, Extraviado, Sem Acesso, Cancelado, Retirado), última manutenção,
  retirada, manutenção da EDL (incluir/editar).
- **Reconhecimento Geográfico** (por bairro/ciclo): datas, qtd. imóveis residenciais/comerciais/outros,
  habitantes, quarteirões, pontos estratégicos, armadilhas, terrenos baldios; infraestrutura (energia, água,
  esgoto, lavanderia, privada, lixo, telefonia, transporte, rua, escola, posto de saúde, acesso, PACS) e
  riscos (malária, dengue, esquistossomose, leishmaniose, febre maculosa, peste, Chagas, febre amarela), cada
  um Sim/Não/Ignorado; animais (cães, gatos, roedores); lat/long/altitude/distância do centro.
- **Resumo Semanal**: ciclo, semana, bairro, atividade (consolida a produção).
- **LIRAa**: Grupos (nome + itens) e Agenda (ciclo, semana, bairro, quarteirões, agente, situação).

### Zoonose
- **Campanha antirrábica**: registros (data, local, registro/aplicação, total vacinados e não vacinados, uso
  do estoque), **Diária de Trabalho** (data, anotação, equipe com líder, início/fim, temperaturas,
  finalizar; calendário de eventos), consumo de vacina/seringa via estoque.
- **LVC**: endereço, data/hora, nº de controle, proprietário, cachorro, idade, sexo, DPP, reteste,
  acompanhamento, visita por.
- **BAAP** (animais peçonhentos): SINAN, local da picada, animal (serpente, aranha, escorpião, lagarta,
  abelha, lacraia, outro), espécie, situação do paciente, atuação VGA, paciente, endereço; ações incluem
  captura e encaminhar ficha.

### Estoque e patrimônio
- **Unidade de estoque** (nome, descrição, situação). **Produto** (unidade, nome, tipo Vacina/Equipamento/
  Insumo/Outros, unidade de medida, tipo de saída permitido, fracionável com doses por unidade e unidade da
  fração, descrição, "selecionável na diária" com categoria Vacina/Seringa). **Lote/Entrada** (produto, nº
  lote, fabricante, fabricação, validade, quantidade, custo unitário, NF, fornecedor, observação).
  **Retirada** (tipo campo ou permanente, responsável, data/hora de saída, retorno previsto/real, itens com
  retirado/usado/devolvido/perda, status, cancelar/finalizar). **Patrimônio** (lote de origem, nº
  patrimônio, série, nome, responsável inicial, data de entrada; atribuir/transferir, manutenção, anotação,
  baixa, reativar, excluir). **Balanço físico** (produto, tipo, lotes, saldo total).

### Acesso e configuração
- **Usuário**: nome, telefone, e-mail, senha, papel, login web (S/N), login tablet (S/N), atividades
  permitidas de visita.
- **Papel** com ~190 permissões granulares em 24 categorias (seção 7). Papéis atuais: Administrador,
  Coordenador, ACE, AGENTE, Epidemiológica, Sala de Gestão.
- **Controle de acesso a locais** (parâmetro): vínculo usuário → bairro/quarteirão/local, tipo, descrição, data.
- **Parâmetros gerais**: método de controle de acesso (por responsável de bairro ou por parâmetro), padrão de
  semanas (ISO 8601 ou SINAN), numeração automática de tubitos, uso obrigatório de estoque na campanha.
- **Auditoria** de visita e local: ID auditoria, ID do registro, tipo, tabela, data, usuário, e diff
  coluna/antigo/novo.
- **Histórico de mobilidade**: descrição, ID, data de sincronismo, data do tablet, usuário.
- **Comunicação** (fila de mensagens): tipo, status, destinatário, assunto, tentativas, enviado em,
  visualizado em, criado em; reenviar. **Banner**: editor visual (nome, largura/altura, imagem de fundo,
  texto, fonte, cor, negrito, sombra).

## 5. Dashboard, B.I. e mapas

- **Dashboard** (por nível de acesso, filtro de Ciclo): cartões Bairros, Locais, Visitas, Com coleta, Locais
  visitados, Locais fechados, Tentativas, Recuperados; gráficos Laboratório-Tubitos (positivas/negativas),
  Visitas por mês, Visitas por dia do ciclo, Locais por tipo, Depósitos por tipo, Tubitos positivos por mês.
- **B.I.** (filtros ciclo, bairro, agente, atividade; `/bi/dados`): IIP (Índice de Infestação Predial),
  Índice de Breteau, visitas por bairro, casos positivos por bairro e tipo de local, positivos x negativos,
  depósitos por tipo, cobertura LIT por bairro e por ciclo, quarteirões.
- **B.I. Epidemiológico** (filtros doença, bairro, período rápido, de/até): curva de incidência temporal,
  bairros monitorados, casos positivos por faixa etária.
- **Mapas**: camadas Visitas, Rastro GPS do app, Doenças, Campanha, Ovitrampa, EDL; filtros por ciclo ou
  data, agente, bairro, atividade, tipo de visita, pendência e coleta de amostra.

## 6. Relatórios (formulários de parâmetros)

Todos geram relatório em tela e/ou PDF (`Gerar Relatório`, `Baixar PDF`), com "Mais opções".

| Relatório | Filtros |
|---|---|
| Visitas | filtrar por (data/ciclo), período, ciclo, atividade, visita, bairro, quarteirão, pendência, agente, achou depósito |
| Local x Visita | bairro, quarteirão, ciclo, semana, atividade, visita, pendência |
| Pesquisa Entomológica / Tratamento (papel, imprimível) | data, ciclo, bairro, atividade, agente, visita, pendência, campos PE |
| Campanha de Vacinação | período, bairro, quarteirão, situação, agentes |
| Acompanhamento de Produção | ciclo, agente, colunas LI+T, pendências, coluna Fechado |
| Serviço Antivetorial | período, atividade |
| Locais por Criticidade | sobrevoo, criticidade |
| LIRAa — Resumo Boletim de Campo/Lab | ciclo, grupo, semana (0 = todas), imóveis programados |
| Visitas por Dia Trabalhado | período, agente |
| Investigação — Totais por Bairro | período, doença, bairro |
| Laboratório — Totais por Bairro | período |
| Total de Animais | bairro |
| BRI | filtrar por, período, ciclo, bairro, quarteirão |
| Locais com Cadastro Incompleto | bairro, quarteirão, tipo |

## 7. Matriz de permissões (papel)

Categorias e exemplos de permissões, úteis para recriar o RBAC:
Visitas (adicionar, visualizar, editar, editar em ciclo fechado, desativar, só as minhas, em grupo, anotação,
editar pendência, ver desativados) · Notificações/Infrações · Locais (adicionar/editar, visualizar, só
atribuídos, só PE, desativar) · Grupo de Locais · B.I. / B.I. Epi · Bairros · Laboratório (visualizar,
alterar resultados, ciclo fechado) · Mapa (camadas) · Drone · Ovitrampa · EDL · Investigação · Denúncia ·
Comunicação · Banner · Zoonose (campanha, diária, LVC, BAAP) · Histórico/Auditoria · Reconhecimento ·
LIRAa · Resumo Semanal · Relatórios (um por relatório) · Estoque (balanço, produto, entrada, retirada,
patrimônio, unidade) · Parâmetros (ciclo, mobilidade, leis municipais, geral, acesso local, doenças) ·
Auditoria · Configuração (usuário, papel). Há padrão recorrente "visualizar apenas as minhas / meu bairro /
atribuídos" (escopo de dados por usuário).

## 8. Fluxos principais

1. Parametrizar ciclo (datas, atividades) → agenda/atribuição de bairros e quarteirões aos agentes.
2. Agente no tablet visita o Local, registra pendência, depósitos, amostras, tratamento e fotos → sincroniza
   (aparece em Histórico Mobilidade e Rastro GPS).
3. Laboratório recebe tubitos, lança resultados (Em Análise → Concluído) → alimenta B.I. e Dashboard.
4. Casos notificados viram Investigação (atender → entrevista → resultado → encerrar), vinculados a local; o
   resultado dispara visitas de bloqueio (Delimitação de foco).
5. Denúncia → atender → visita → notificação → auto de infração → pago/cancelado.
6. Campanha antirrábica: diária com equipe e líder → registros por local → baixa automática no estoque.
7. Fechamento do ciclo (Operando → Fechado) bloqueia edição, salvo permissão específica.

## 9. Observações para a nova versão

- Quase todas as telas são variações do mesmo padrão (filtros + tabela paginada + detalhe + formulário +
  status + anotações). Vale criar componentes genéricos.
- O ID de visita (12 dígitos) parece composto, não sequencial **(inferido)**; defina a regra de geração.
- Visitas nascem no tablet: a nova versão precisa de **API de sincronismo offline** com conflito e histórico.
- Há dados pessoais (CPF, telefone, WhatsApp, paciente, mãe): trate com LGPD, perfis de acesso e auditoria.
- Rotas e nomes têm inconsistências (`editar/{id}` x `{id}/editar`, "Relátorio" com erro de grafia).
  Padronize.
- Dois endpoints testados por tentativa (`/bairro/results`, `/parametros/ciclo/results`) devolveram erro 500:
  não são os endpoints reais; ignorar. Telas de edição de bairro, ciclo, usuário e papel não foram abertas
  individualmente (os formulários "novo" foram lidos).
- Não foi possível ver: regras de negócio do servidor, cálculo exato de IIP/Breteau, formato dos PDFs,
  funcionamento do aplicativo de tablet e do painel público.

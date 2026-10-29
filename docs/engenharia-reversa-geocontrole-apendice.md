# Geo Controle — Apêndice A: aprofundamento por módulo (segunda varredura)

Complementa [engenharia-reversa-geocontrole.md](engenharia-reversa-geocontrole.md). O modelo de dados sugerido
em SQL está em [geocontrole-modelo-dados.sql](geocontrole-modelo-dados.sql) (MySQL 8, tudo **inferido**, sem
acesso ao banco real). Só leituras GET e leitura do DOM; nenhum dado pessoal foi copiado.

## A.1 Convenções que se repetem

- **IDs de negócio compostos:** visitas, locais, ovitrampas, EDLs, notificações e solicitações de drone têm IDs
  de 11 a 13 dígitos no formato `{prefixo curto}{timestamp Unix de 10 dígitos}`. Parece gerado no tablet
  **(inferido)**. Investigação, BAAP, comunicação, retirada, diária, reconhecimento, LIRAa e estoque usam ID
  sequencial simples.
- **Ações em modal na própria ficha**: "Atividades" + "Ação" (atender, encaminhar, anotar, encerrar) em
  Investigação, BAAP, Drone e Denúncia, com anexos múltiplos. Vale um componente único de linha do tempo.
- **Desativação lógica:** `cad_situacao` Ativo/Desativado em local, visita, usuário e papel; "Ver desativados"
  é permissão própria.
- **Edição por POST com ids aninhados**, ex.: `POST /visita/{idLocal}/{idVisita}`.
- **Escopo de dados por usuário:** muitas permissões "apenas as minhas / meu bairro / atribuídos".

## A.2 Operação

**Local `/local/{id}`.** Seções: Dados do Local, Moradores, Habitantes, QR Code/Chave, Reincidência (últimos
12 meses), Visitas, Zoonose (Campanha e LVC), Ovitrampa, Notificações e EDL. É a ficha completa do imóvel.
Edição `/local/{id}/editar`: mesmos campos do cadastro, mais situação, botão "Buscar pelo endereço"
(geocodificação) e modal "Adicionar Bairro". **Grupo de Locais** (`/local/grupo/todos`): filtros por endereço,
número, bairro e quarteirão; ações Detalhes, Gerenciar, Clonar (`/local/grupo/{id}/create?clone=`), Exportar
e Ver no mapa.

**Bairro.** Edição: nome, mutirão, status, IBGE, classificação, zona, categoria, lat/long/altitude, distância
do centro, entra na produção; exibe aviso "Atenção". Os responsáveis (N:N) só aparecem no cadastro novo.

**Ovitrampa `/ovitrampa/{id}`.** Dados, anotação e **Contagem de Ovos** (Contagem/Data/Por; adicionar, editar,
excluir). Edição: data, status, número opcional, data de retirada, retirado por, anotação. Não existe cadastro
pela web (`/ovitrampa/novo` = 404): nasce no tablet.

**EDL `/edl/{id}`.** Status colorido, dados, retirada/anotação, alterar status e tabela de **Manutenção**: nível
da água (Seco, <50%, >50%) e marcadores larvas, pulpas, tecido, fechado, não instalada, desaparecida, quebrada,
retirada; data, por, anotação. Também sem cadastro pela web.

**Notificado `/notificado/visualizar/{id}`.** Local e visita de origem; dados da notificação (número, data,
notificado por, prazo com atalhos 7d/10d/15d/30d/60d, status, morador ausente, assinatura, motivo); histórico
de comunicações (tipo, destinatário, assunto, enviado, visualizado, tentativas). Fluxo: **Notificação →
Resolver | Cancelar | Criar Auto de Infração → Pago/Cancelar**. O auto tem data, prazo de pagamento calculado,
valor da multa, classificação (Leve, Média, Grave, Gravíssima), UPFM (100/150/200/250), motivo e trechos da lei
municipal. "Criar Comunicação e Enviar" dispara WhatsApp/e-mail.

**Solicitação de Drone `/dronesolicitacao/{id}`.** Local vinculado (moradores, habitantes), atividades e modal
"Ação": descrição, encaminhar para (usuário), anexos, marcar "Atender". Vem do tablet.

**Visita `/local/visita/{id}`.** Cabeçalho do local, moradores, habitantes, dados da visita (data, hora, tipo,
pendência, atividade, semana, ciclo, assinatura), anotações internas e modal "Alterar resultados" de
laboratório (`POST /local/visita/laboratorio/coleta/{id}`); imprimir e editar.

## A.3 Casos e zoonose

**Investigação `/investigacao/{id}`.** Atividades (linha do tempo); modal Ação (encaminhar para, descrição,
**resultado** Positivo/Negativo/Inconclusivo, anexos, "Enviar para o aplicativo móvel", "Atender");
**Vincular Local** (busca por endereço, número, bairro, quarteirão, lado); **Entrevista** por doença. A de
Leishmaniose tem 8 perguntas Sim/Não (cães no domicílio e quantidade, sinais sugestivos, mosquito-palha, área
de mata/rural, abrigo de animais, matéria orgânica, outros casos na vizinhança) mais resumo do peridomicílio
(`POST /investigacao/atividade/entrevista/{id}`). Outras doenças têm modelos próprios (parâmetro
"Doenças/Entrevistas", tela não vista). Não há tela `/editar` (404).

**Denúncia.** Cadastro (`/denuncia/novo` → `POST /denuncia/store`): anônima, busca de cidadão por telefone,
nome, CPF, tipo, título, descrição, endereço, bairro, quarteirão. A lista tinha poucos registros e o detalhe
não pôde ser aberto (erro 500 com o id que tentei).

**BAAP `/zoonose/baap/{id}`.** Atividades, Ação (encaminhar, atender), Vincular Local. Permissões extras:
Captura e Encaminhar Ficha. Cadastro com 17 campos (ver documento principal, seção 4).

**LVC `/zoonose/lvc/{id}`.** Só leitura na web: endereço, data/hora, nº de controle, proprietário, cachorro,
idade, sexo, DPP, reteste, acompanhamento, visita por.

**Campanha `/zoonose/campanha/{id}`.** Local, moradores, habitantes, **Vacinados** e **Não vacinados**, com
"Registro por Animal — Estoque" (tipo, nome, vacina, lote, fabricante, validade). Uso do estoque obrigatório ou
opcional conforme parâmetro geral.

**Diária de Trabalho `/zoonose/campanha/diaria/{id}`.** Equipe com líder e quatro ações:
- **Iniciar:** hora, seringas, vacinas fechadas/abertas, doses totais, horário de recebimento, temperaturas
  (caixa de estoque e em uso, °C);
- **Registrar temperaturas:** 08h, 10h, 12h, 14h, 17h (estoque e em uso);
- **Defeitos/descartes/perdas:** seringa defeito, seringa descarte, vacina descarte, vacina perda;
- **Finalizar:** hora fim, seringas (sobra, perda, defeito), vacina usada, vazia, cheia, aberta, perda.
Mostra histórico de uso de vacina (horário, tipo, animal, vacina, endereço). Edição: data, anotação, situação,
gerenciar equipe.

**LIRAa.** Grupo (nome, situação; **Itens** = bairros com Manter/Excluir) e Agenda (ciclo, semana, bairro,
quarteirões, agente, situação). O relatório "LIRAa — Resumo" pede "imóveis programados" manualmente.

**Reconhecimento Geográfico.** Quatro blocos: Infraestrutura, Ocorrência de animais, Ocorrência ou risco de
ocorrência e Dados geográficos (campos Sim/Não/Ignorado e numéricos já listados no documento principal).

**Resumo Semanal.** Lista: bairro, ciclo, semana, atividade, gerado em; ações Imprimir
(`/resumosemanal/{id}`) e Excluir. Novo: só ciclo, semana, bairro, atividade; o conteúdo é calculado.

## A.4 Estoque

- **Produto** (edição): unidade/sede, nome, tipo, unidade de medida, tipo de saída permitido, fracionável
  (doses por unidade e unidade da fração), descrição, selecionável na diária (categoria Vacina/Seringa), ativo.
- **Lote `/estoque/lote/{id}`:** "Lote nº — produto" e histórico de movimentações (data, tipo, quantidade,
  usuário, descrição, retirada).
- **Retirada `/estoque/retirada/{id}`:** status (aberta, finalizada, cancelada), itens (produto, lote,
  validade, retirado, usado, devolvido, perda) e histórico de uso. Tipo **Campo** (sai e volta) ou
  **Permanente** (vira patrimônio).
- **Patrimônio:** lote de origem (só produtos de saída permanente com saldo), nº de patrimônio (etiqueta),
  série, nome, responsável inicial, data de entrada; eventos atribuir/transferir, manutenção, anotação, baixa,
  reativar, excluir.
- **Unidade:** nome, descrição, ativo.

## A.5 Parâmetros e administração (telas antes desconhecidas)

- **Leis Municipais** (`/parametros/leis`): número, ano, nome, publicação, ativo. Alimenta o seletor de lei em
  visita, notificação e infração.
- **Mobilidade** (`/parametros/mobilidade`, parâmetros do app): aviso de precisão do GPS (metros), permitir
  digitar nº do tubito, coletar GPS (a cada 5 s), permitir DF/PVE/LIRAa com pendência "fechado", CEP padrão,
  senha para "Limpar dados", gráficos do dashboard do app, e dias de download de visitas/fotos, campanhas,
  investigações e anexos de investigação.
- **Geral:** acesso a locais, semanas ISO/SINAN, tubito automático, estoque na campanha.
- **Ciclo** (edição): igual ao novo, com aviso ao mexer em ciclo com visitas.
- **Usuário** (edição): inclui situação; papéis também incluem "PE (Temporário)" e "SUPERVISOR Nova Mutum".
- **Papel** (edição): 22 categorias de permissão, busca, "Selecionar/Desmarcar todas" e situação do papel.
- **Comunicação `/comunicacao/{id}`:** dados gerais, timeline, dados de contexto; ação Reenviar.
- **Changelog:** versão, data, tipo (Novidade etc.) e texto por release.

## A.6 Conclusões para reescrever

1. O núcleo é **Local → Visita → Amostra/Laboratório**; o resto são módulos satélites ligados ao Local.
2. Quatro "motores" reaproveitáveis: filtros + tabela paginada; ficha com linha do tempo, ações e anexos;
   editor de papéis/permissões por categoria; relatórios parametrizados.
3. O tablet é cliente offline-first com IDs gerados localmente e GPS contínuo: a nova API precisa aceitar IDs
   do cliente, lotes de sincronismo e resolução de conflito.
4. Regras de calendário (ciclo bimestral, semana ISO ou SINAN, ciclo fechado bloqueando edição) concentram boa
   parte das regras de negócio.
5. Estoque ligado à campanha antirrábica (a diária consome vacina e seringa) é a principal integração entre
   módulos fora do "Local".

## A.7 Lacunas

- Fluxos de **criação** que só existem no tablet (visita, ovitrampa, EDL, drone, LVC, campanha) não foram
  vistos; só leitura e edição web.
- Detalhe da denúncia e entrevistas de outras doenças (Dengue, Chagas etc.) não abertos.
- Valores exatos dos status de Investigação, Denúncia e BAAP: só "Aguardando" foi observado.
- Cálculo dos índices (IIP, Breteau, cobertura LIT) e layout dos PDFs.
- Painel público de ciclos e aplicativo de tablet.

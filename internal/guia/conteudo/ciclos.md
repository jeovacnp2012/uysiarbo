titulo: Ciclos e atividades
resumo: Como cadastrar os ciclos de trabalho, as atividades de cada um, o painel público e a sincronização entre ciclos.
ordem: 13
permissao: param_ciclo
palavras: semana epidemiológica, SE, SINAN, ISO, ciclo, ciclos, atividade, LIT, LI, PE, DF, PVE, LIRAa, LIA, BRI, período, fechado, operando, painel público, sincronizar
---
# Ciclos e atividades

Em **Parâmetros → Ciclos** você cadastra os períodos de trabalho (em geral **6 por ano**, como "05 Ciclo - 2026"). Cada visita e cada atividade registrada pertence a um ciclo.

## Cadastrar um ciclo
- **Nome** (único; pode ser reaproveitado se o ciclo antigo estiver na Lixeira), **data de início** e **data final**. A data final não pode ser anterior à de início.
- **Ciclo fechado**: marque quando o ciclo terminar. Um ciclo fechado bloqueia a edição dos registros dele, salvo para quem tem a permissão própria para isso (a regra é aplicada nas telas de visitas, nas próximas etapas). Desmarque para reabrir.
- **Permitir registros fora do período**: libera registros com data fora das datas do ciclo.
- Se o período cruza o de outro ciclo, o sistema **avisa** na ficha, mas não impede.

## Semana epidemiológica
Na própria tela **Ciclos** você escolhe como as semanas são numeradas neste ambiente:
- **SINAN / Ministério da Saúde** (padrão): semanas de **domingo a sábado**; a semana 1 é a que tem ao menos 4 dias em janeiro.
- **ISO 8601**: semanas de **segunda a domingo**; a semana 1 é a que tem a primeira quinta-feira do ano.

A lista e a ficha de cada ciclo mostram as semanas que ele cobre (por exemplo, "SE 1/2026 a SE 9/2026"). As visitas e os relatórios usam a mesma regra. Mudar o tipo não altera nenhum registro: só muda a numeração mostrada.

## Atividades
O catálogo é fixo: **LIT** (levantamento de índice com tratamento), **LI** (levantamento de índice), **PE** (ponto estratégico), **T** (tratamento), **DF** (delimitação de foco), **PVE** (pesquisa vetorial especial), **LIRAa**, **LIA** (levantamento de índice amostral) e **BRI** (bloqueio com borrifação residual). Marque as que valem em cada ciclo.

## Painel público
Marque **Mostrar este ciclo no painel público**, escolha as atividades que aparecem (só entre as do ciclo) e, se quiser, um nome para o painel. O painel em si chega junto com os relatórios.

## Ciclos sincronizados
Indica que o que se registra neste ciclo também vale para outros (por exemplo, um ciclo de campanha que acompanha o ciclo regular). Escolha os ciclos na lista da ficha.

## O que o ciclo fechado e o período controlam
- **Ciclo fechado**: quem não tem a permissão **Editar (ciclo fechado)** não consegue alterar os registros do ciclo; o sistema explica o motivo.
- **Data fora do período**: um registro com data fora das datas do ciclo é recusado, a não ser que o ciclo permita registros fora do período.

Essas regras passam a valer nas telas de visitas e de laboratório, nas próximas etapas.

## Excluir
**Excluir** envia o ciclo para a **Lixeira** (dá para restaurar). A exclusão definitiva só é possível com autorização e se nenhum outro ciclo estiver sincronizado com ele.

A auditoria registra quem criou, editou ou excluiu cada ciclo, com o antes e o depois.

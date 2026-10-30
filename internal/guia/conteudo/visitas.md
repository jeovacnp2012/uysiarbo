titulo: Visitas
resumo: Como registrar e consultar a visita de um agente a um local: depósitos, amostra, tratamento, fotos, assinatura e anotações.
ordem: 14
permissao: visita_visualizar
palavras: visita, pendência, fechado, recusado, tentativa, depósito, A1, A2, B, C, D1, D2, E, amostra, tratamento, larvicida, L1, L2, adulticida, BRI, foto, assinatura, anotação, semana epidemiológica, recuperação
---
# Visitas

A visita é o registro do que o agente fez num **local**, dentro de um **ciclo**, para uma **atividade**. Em **Visitas** você vê as últimas visitas; na ficha de um local, os botões **Ver visitas** e **Registrar visita** levam direto às dele.

> As visitas de campo chegam do aplicativo do tablet. O registro pela tela serve para lançar visitas feitas no papel, corrigir e conferir.

## Registrar uma visita
Abra o local e clique em **Registrar visita**. Preencha:
- **Data** (não pode ser futura) e **hora de entrada**. A data precisa estar dentro do período do ciclo, a não ser que o ciclo permita registros fora do período.
- **Ciclo** e **atividade** (só as atividades que fazem parte do ciclo), **agente** e **tipo** (Normal ou Recuperação).
- **Pendência**: *Nenhuma*, *Fechado* (imóvel fechado), *Recusado* ou *Tentativa*. Com pendência, a visita **não tem** depósitos, amostra nem tratamento.
- **Depósitos**: marque "Encontrou depósitos" e informe a quantidade de cada tipo (A1, A2, B, C, D1, D2, E) e o total eliminado, que não pode passar da soma.
- **Amostra**: marque se coletou. Os tubitos e o resultado ficam no Laboratório.
- **Tratamento**: gramas e depósitos tratados de larvicida **L1** e **L2**, adulticida (tipo e cargas), depósitos eliminados no tratamento e o volume do **BRI**.
- **Posição** (latitude, longitude e precisão do GPS) e a **anotação** do agente.

A **semana epidemiológica** é calculada pela data, segundo o tipo escolhido em Ciclos (SINAN ou ISO).

## Fotos, anotações e assinatura
Na ficha da visita:
- **Fotos**: JPEG, PNG ou WebP, até 5 MB cada e 20 por visita.
- **Anotações internas**: comentários da equipe; ficam em ordem e não se alteram depois de enviados (precisa da permissão de anotar).
- **Assinatura** do morador, enviada pelo aplicativo, aparece na ficha.

## Regras que valem a pena saber
- **Ciclo fechado**: só quem tem a permissão de editar em ciclo fechado registra ou altera visitas dele.
- **Pendência**: só quem tem a permissão de editar pendência muda esse campo numa visita já registrada.
- **Apenas as minhas**: quem tem essa restrição só vê as próprias visitas.
- **Excluir** envia a visita para a **Lixeira** (dá para restaurar). Um local ou ciclo com visitas ativas não pode ser excluído.
- Toda mudança fica na **Auditoria**, com o antes e o depois.

## Listagem e filtros
Em **Visitas**, os filtros pesquisam enquanto você digita ou escolhe (sem botão): ID, endereço, número, bairro, quarteirão, lado, ciclo, agente, atividade, semana, período (de/até), pendência, depósito, amostra, tratamento, foto e situação. "Limpar filtros" volta ao começo. A lista tem 10 por página (dá para mudar) e o endereço da página guarda os filtros.

## Desativar, reativar e lote
- Na ficha, **Desativar visita** esconde a visita das listagens sem apagar nada; **Reativar visita** a traz de volta (precisa da permissão de desativar). Quem não tem "ver desativados" não enxerga as desativadas.
- Na lista, marque várias visitas e use **Desativar marcadas** ou **Reativar marcadas**. Visitas de ciclo fechado (sem a permissão) ou fora do seu acesso são puladas, e o aviso diz quantas.
- **Imprimir** na ficha abre a impressão do navegador só com o conteúdo.

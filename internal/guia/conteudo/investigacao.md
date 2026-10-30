titulo: Investigações e entrevistas
resumo: Como registrar um caso, vincular o local, fazer a entrevista e dar o resultado.
ordem: 12
permissao: investigacao_visualizar
palavras: investigação, caso, doença, agravo, entrevista, paciente, resultado, positivo, negativo, encerrar
---
# Investigações e entrevistas

Uma **investigação** acompanha um caso suspeito ou confirmado de uma doença ou agravo, do primeiro registro até o resultado.

## Registrar um caso
1. Em **Investigações**, clique em **Nova investigação**.
2. Escolha a doença, informe o paciente, o telefone, a data de início e, se não houver local cadastrado, o endereço.
3. Salve. A ficha abre com o andamento do caso.

## Situação e resultado
- **Aguardando atendimento**: ainda ninguém assumiu o caso.
- **Em atendimento**: alguém assumiu (botão **Assumir atendimento**) ou já deu um resultado.
- **Encerrada**: só encerra com resultado **positivo**, **negativo** ou **inconclusivo**. Uma investigação encerrada não aceita alterações; use **Reabrir** (se tiver permissão).

Cada mudança aparece na linha do tempo, junto com comentários, encaminhamentos e anexos.

## Local do caso
Na seção **Local do caso**, busque pelo endereço, número ou bairro e clique em **Vincular**. Dá para desvincular a qualquer momento.

## Entrevista
Cada doença usa uma entrevista. As perguntas marcadas como obrigatórias precisam de resposta, e algumas só aparecem depois de outra resposta (por exemplo, "Para onde viajou?" só se respondeu "Sim" a "Viajou?").

## Montar as entrevistas (administração)
- **Doenças e agravos**: cadastre as doenças e escolha a entrevista de cada uma.
- **Entrevistas**: crie as perguntas (texto, número, escolha uma ou várias), defina as obrigatórias, as condições e as doenças a que cada pergunta se aplica.

## Excluir
Excluir envia a investigação para a **Lixeira**, de onde pode ser restaurada.

## Importar investigações
Em **Importação de dados**, escolha **Investigações** e envie uma planilha (CSV ou Excel). As colunas são: ID de origem, doença (já cadastrada), paciente, telefone, data de início, endereço, local (ID de origem, opcional), situação, resultado, responsável e observação. Só cria: se o ID de origem já existe, a linha é ignorada, então dá para repetir a importação sem duplicar. Antes de importar, o sistema mostra uma prévia com os erros de cada linha.

## Relatório de totais
Em **Relatórios**, o item **Investigação totais** mostra, por doença, quantas investigações existem, a situação de cada uma e o resultado (positivas, negativas e inconclusivas). O período vale para a data de início.


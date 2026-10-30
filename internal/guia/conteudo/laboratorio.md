titulo: Laboratório
resumo: Tubitos coletados nas visitas, numeração, resultado da análise e listagem do laboratório.
ordem: 15
permissao: visita_laboratorio_visualizar
palavras: laboratório, amostra, tubito, tubete, resultado, aegypti, albopictus, negativo, em análise, concluído, larvas, pupas
---
# Laboratório

Quando o agente coleta material numa visita, cada frasco vira um **tubito** (amostra). O laboratório recebe os tubitos e registra o resultado da análise.

## Registrar os tubitos
1. Na ficha da visita, marque **Coletou amostra** e salve.
2. Na seção **Amostras (tubitos)**, preencha o depósito (A1 a E), as larvas e as pupas e use **Adicionar tubito**.
3. O número do tubito é **gerado automaticamente** (o próximo da sequência). Se o seu município preferir digitar o número, desmarque "Numerar os tubitos automaticamente" em **Parâmetros gerais**; aí o número é obrigatório e **não pode se repetir**.

Dá para corrigir depósito, larvas, pupas e observação com **Salvar coleta** (precisa da permissão de editar visita).

## Resultado da análise
Cada tubito tem o **status** (Em análise ou Concluído) e os totais de ***Aedes aegypti***, ***Aedes albopictus*** e **negativo**. Salve com **Salvar resultado**. Quem faz isso precisa da permissão **Alterar resultados** do Laboratório; em **ciclo fechado**, da permissão **Alterar resultados (ciclo fechado)**. O nome de quem registrou e a hora ficam guardados, e tudo vai para a **Auditoria**.

## Contagem no menu
Ao lado de **Laboratório**, no menu, um selo mostra quantos tubitos ainda estão **em análise**. Quem vê só as próprias visitas conta só os próprios tubitos.

## Listagem do laboratório
O menu **Laboratório** lista os tubitos de todas as visitas, com filtros que pesquisam na hora: tubito, endereço, status, ciclo, agente e período. **Abrir visita** leva à ficha, onde se altera o resultado. Quem vê só as próprias visitas vê só os próprios tubitos.

## Excluir
**Excluir tubito** envia para a **Lixeira** (dá para restaurar). Uma visita só vai para a Lixeira depois que os tubitos ativos dela forem excluídos.

titulo: Diária de trabalho da campanha
resumo: Como montar a equipe do dia, registrar início e fim do trabalho, temperaturas das vacinas e perdas.
ordem: 18
permissao: campanha_diaria_visualizar
palavras: diária, campanha, antirrábica, equipe, temperatura, vacina, seringa, perda, zoonose
---
# Diária de trabalho da campanha

A diária é o **dia de trabalho** da equipe na campanha antirrábica: quem trabalhou, o material com que começou e terminou, as temperaturas das vacinas e as perdas.

## Criar e acompanhar
1. Em **Zoonose > Diária de Trabalho**, clique em **Nova diária** e escolha a data.
2. Na ficha, em **Equipe**, adicione os membros um a um (marque **Líder** quando for o caso).
3. Clique em **Iniciar trabalho** e preencha o registro do dia: seringas e vacinas no início, no final, defeitos, descartes e perdas, e as temperaturas (no estoque e em uso) às 08:00, 10:00, 12:00, 14:00 e 17:00.
4. Ao terminar, **Finalizar**. Se precisar corrigir, **Reabrir**.

## Situações
Aguardando, Executando, Finalizado e Cancelado. Diária finalizada ou cancelada não aceita alterações até ser reaberta ou reativada.

## Atendimentos
Ao registrar um atendimento da campanha, escolha a **Diária de trabalho**. A ficha da diária mostra a lista dos atendimentos e os totais de vacinados e não vacinados.

## Quem vê
Quem tem a permissão restrita "apenas o meu" vê só as diárias em que está na equipe.

## Excluir
A diária vai para a **Lixeira**. Não é possível excluí-la de vez se houver atendimentos ligados a ela.

## Estoque da diária
Na ficha, em **Estoque da diária**, escolha o **lote da vacina** e o **lote da seringa** (só aparecem produtos marcados como selecionáveis na diária, com saldo).

Ao **finalizar**, o sistema baixa do estoque:
- do lote da seringa: seringas usadas + perdidas + com defeito (final de trabalho);
- do lote da vacina: frascos vazios.

Se o saldo não cobrir, a diária não finaliza e a mensagem diz o que falta. Ao **reabrir**, as baixas são devolvidas ao estoque. Se o parâmetro "Uso obrigatório do Estoque" (Parâmetros > Geral) estiver em Sim, não é possível finalizar sem escolher os dois lotes. Lote usado por diária não pode ser excluído de vez.

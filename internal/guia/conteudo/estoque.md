titulo: Estoque — unidades, produtos e lotes
resumo: Como cadastrar unidades, produtos (inclusive fracionáveis) e registrar a entrada e o saldo dos lotes.
ordem: 21
permissao: estoque_produto_visualizar
palavras: estoque, unidade, produto, lote, entrada, saldo, validade, vacina, seringa, fracionável, perda, baixa, ajuste
---
# Estoque — unidades, produtos e lotes

## Unidades de Estoque
Em **Estoque > Unidades de Estoque**, cadastre os setores que guardam produtos (por exemplo, Vigilância Ambiental). O nome é único. Uma unidade com produtos não vai para a Lixeira.

## Produtos
Em **Estoque > Produtos**, cada produto tem:
- **Tipo**: vacina, equipamento, insumo ou outros.
- **Unidade de medida**: frasco, caixa, unidade...
- **Saída permitida**: campo e permanente, só campo ou só permanente.
- **Fracionável**: marque quando o produto é usado em partes. Informe quantas frações há em cada unidade (ex.: 25 doses por frasco) e a unidade da fração (dose, ml, mg ou UI).
- **Aparece na diária da campanha**: marque para vacinas e seringas que a diária da campanha antirrábica vai consumir, e escolha a categoria (vacina ou seringa).

A lista tem filtros por nome, tipo, unidade e situação, e mostra o saldo somado dos lotes.

## Entradas / Lotes
Em **Estoque > Entradas / Lotes**, clique em **Nova entrada** para registrar um lote recebido: produto, número do lote (único por produto), quantidade, fabricação, validade, custo, nota fiscal e fornecedor. A entrada já cria o primeiro item do **histórico do lote**.

Na ficha do lote, **Movimentar o saldo** permite:
- **Entrada adicional**: soma ao saldo.
- **Perda** e **Baixa**: subtraem do saldo (não pode ficar negativo).
- **Ajuste**: informa o novo saldo contado; o sistema registra a diferença.

Toda movimentação exige um motivo e fica no histórico, com quem fez e quando. O histórico não pode ser alterado.

A lista de lotes mostra **(vencido)** e **(vence em breve)**, e tem filtro por situação: com saldo, sem saldo, vence em 30 dias e vencido.

## Cuidados
Excluir envia para a **Lixeira**. Um produto só sai enquanto não tiver lotes ativos; a exclusão definitiva é recusada se houver ligação com outro registro. As retiradas (campo e permanente), o balanço físico e o patrimônio chegam nas próximas etapas.

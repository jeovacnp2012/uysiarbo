titulo: Importação de dados
resumo: Como trazer bairros, quarteirões e locais de uma planilha, com prévia, relatório e desfazer.
ordem: 11
permissao: conf_importacao
palavras: importar, importação, planilha, excel, csv, xlsx, modelo, prévia, desfazer, bairros, quarteirões, locais, migração
---
# Importação de dados

Em **Administração → Importação de dados** você traz bairros, quarteirões e locais de uma **planilha** (Excel `.xlsx` ou `.csv`). A importação é feita em etapas seguras: o arquivo é **analisado primeiro, sem gravar nada**; você confere a **prévia** e só então confirma.

## Passo a passo
1. **Nova importação** → escolha o que importar e **baixe o modelo** (Excel ou CSV). O modelo traz os nomes certos das colunas e um exemplo; no Excel há também uma aba **Ajuda** explicando cada coluna.
2. Preencha a planilha. A primeira linha tem os nomes das colunas; use os nomes do modelo (acentos e maiúsculas não importam). Colunas que o sistema não conhece são ignoradas, e a prévia avisa quais.
3. Envie o arquivo. Se faltar uma coluna obrigatória, o sistema diz qual.
4. Na **prévia**, cada linha aparece como **criar**, **atualizar**, **ignorar** ou **erro**, com a explicação (por exemplo, "a cidade X não está cadastrada" ou "repetida na planilha"). Dá para filtrar por ação e baixar **só as linhas com erro**, no mesmo formato da sua planilha e com o motivo na última coluna: corrija e envie de novo.
5. **Confirmar e importar**: a gravação roda em segundo plano, com **barra de progresso**. Você pode sair da tela e voltar; também pode **Interromper** e **Retomar** depois, que continua de onde parou.
6. Ao terminar, baixe o **relatório** (CSV) com o resultado de cada linha.

## O que cada tipo precisa
- **Bairros**: a **cidade** já cadastrada (e a UF, se houver cidades com o mesmo nome) e o nome do bairro.
- **Quarteirões**: o **bairro** já cadastrado e o nome do quarteirão. Se houver bairros com o mesmo nome em cidades diferentes, informe a cidade.
- **Locais**: o **bairro** e o **quarteirão** já cadastrados, tipo, rua, número e lado. Os quarteirões precisam ser importados antes. O **CPF não é importado**.

Importe nessa ordem: **bairros → quarteirões → locais**.

## Registros que já existem
Por padrão, o que já existe é **ignorado**. Marcando **Atualizar os registros que já existem**, a importação altera esses registros, **só com o que vier preenchido** na planilha (célula vazia mantém o valor atual). Um registro é reconhecido pelo nome (bairro: cidade e nome; quarteirão: bairro e nome; local: quarteirão, rua, número, lado, sequência e complemento) ou pelo **ID de origem** (`id_externo`), se informado.

## Desfazer
Depois de concluída, **Desfazer esta importação**:
- o que foi **criado** vai para a **Lixeira** (nada é apagado de verdade; dá para restaurar);
- o que foi **atualizado** volta ao estado de antes.

O que já estiver em uso por outros dados (por exemplo, um bairro que ganhou quarteirões depois) é **mantido** e a tela avisa; desfaça os dados ligados primeiro e tente de novo.

## Limites e cuidados
- Até **10 MB** e **20 mil linhas** por importação; planilhas maiores devem ser divididas.
- Só uma importação roda por vez em cada ambiente.
- Quem só enxerga parte dos dados (papel "apenas atribuídos") não pode importar, porque a importação mexe no ambiente inteiro.
- A Auditoria registra o envio, o início, a conclusão (com os totais) e o desfazer.

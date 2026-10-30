titulo: Importação de dados
resumo: Como trazer bairros, quarteirões e locais de uma planilha, com prévia, relatório e desfazer.
ordem: 11
permissao: conf_importacao
palavras: visitas, tubitos, laboratório, ciclos, importar, importação, planilha, excel, csv, xlsx, modelo, prévia, desfazer, bairros, quarteirões, locais, migração
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

## De onde vêm os dados (origens)
Na primeira etapa da importação você escolhe a **origem**:
- **Planilha (Excel ou CSV)**: uma planilha no modelo do sistema, ou qualquer planilha (a próxima tela deixa você ligar as colunas).
- **Geo Controle (planilha exportada)**: uma planilha exportada do Geo Controle. Os nomes de coluna dele (por exemplo, `Logradouro`, `Nº`, `Quadra`, `Cad_Situacao`, `ID`) já são reconhecidos, e o `ID` dele vira o **ID de origem** do registro.
- **Geo Controle (API)** e **Outra API (JSON)**: o sistema busca os dados num **endereço HTTPS** (e, se for o caso, o **caminho** onde está a lista no JSON, como `data`), com um **token** (Bearer) ou usuário e senha. As credenciais valem só para essa importação: **não ficam guardadas**. Por segurança, o servidor só acessa endereços da **internet pública** (nunca a rede interna).

Observação: a conexão direta com o Geo Controle depende de ele oferecer uma API; ainda não foi testada contra o sistema real. Enquanto isso, a exportação em planilha funciona.

## Ligar as colunas e salvar o mapeamento
Quando os nomes das colunas da origem não batem com os do sistema (ou quando você marca **Revisar o mapeamento**), aparece a tela **Ligar as colunas**: ela mostra as primeiras linhas da origem e, para cada campo do sistema, uma lista com as colunas da origem (já sugerida pelo nome). Escolha **(não importar)** para ignorar um campo. Dando um **nome** ao mapeamento, ele fica salvo para essa origem e esse tipo de dado e pode ser escolhido nas próximas importações, sem refazer a ligação. Os mapeamentos salvos ficam em **Mapeamentos salvos** e, se excluídos, vão para a Lixeira.

## O que cada tipo precisa
- **Bairros**: a **cidade** já cadastrada (e a UF, se houver cidades com o mesmo nome) e o nome do bairro.
- **Quarteirões**: o **bairro** já cadastrado e o nome do quarteirão. Se houver bairros com o mesmo nome em cidades diferentes, informe a cidade.
- **Locais**: o **bairro** e o **quarteirão** já cadastrados, tipo, rua, número e lado. Os quarteirões precisam ser importados antes. O **CPF não é importado**.
- **Ciclos** (exige a permissão de Ciclos): **nome**, **data de início** e **data final** (dd/mm/aaaa ou aaaa-mm-dd); opcionais: situação (operando ou fechado), atividades (siglas separadas por vírgula, como `LIT, LI, PE`), permite fora do período, painel público e as atividades do painel. Um ciclo que já existe é reconhecido pelo nome; célula vazia mantém o valor atual (inclusive a lista de atividades). Desfazer manda os ciclos criados para a Lixeira e restaura os atualizados.

Importe nessa ordem: **bairros → quarteirões → locais**.

- **Visitas** (exige a permissão de adicionar visitas; importe **depois** dos locais, ciclos e usuários): **ID de origem** da visita, **local** (o ID de origem do local), **ciclo** (nome), **atividade** (sigla), **agente** (nome ou e-mail) e **data**; opcionais: hora, tipo, pendência, depósitos A1 a E, total eliminado, coletou amostra, tratamento (L1, L2, adulticida), BRI, latitude e longitude e anotação. A visita passa pelas **mesmas regras** da tela de visita (período do ciclo, atividade do ciclo, pendência, depósitos); a diferença é que o importador **aceita ciclo fechado e agente desativado**, porque o histórico antigo costuma ter os dois. Só cria: uma visita com o mesmo ID de origem é **ignorada**, então dá para repetir a importação sem duplicar. Desfazer manda as visitas criadas para a Lixeira.
- **Laboratório (tubitos)** (exige a permissão de registrar resultados do laboratório; importe **depois** das visitas): **visita** (o ID de origem), e opcionais número do tubito (vazio usa a numeração automática, se o município a adota), depósito, larvas, pupas, observação, status, totais de *aegypti*, *albopictus* e negativo. A visita precisa ter **coletado amostra**. Se vierem totais ou status "concluído", o resultado é gravado junto. Um tubito com o mesmo número (ou o mesmo ID de origem) é ignorado.

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

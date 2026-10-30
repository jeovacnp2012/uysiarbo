titulo: Parâmetros e leis municipais
resumo: Parâmetros gerais, ajustes do aplicativo do tablet e o cadastro de leis municipais com seus trechos.
ordem: 14
permissao: param_geral
palavras: parâmetros, geral, mobilidade, aplicativo, tablet, GPS, tubito, CEP padrão, senha, limpar dados, lei, leis municipais, trecho, artigo
---
# Parâmetros e leis municipais

## Parâmetros gerais
Em **Parâmetros gerais** ficam as regras que valem para todo o ambiente:
- **Numerar os tubitos automaticamente**: desmarcado, o número do tubito é digitado em cada amostra.
- **A campanha antirrábica consome o estoque**: vacinas e seringas da campanha saem do estoque.
- **CEP padrão dos novos locais**: preenche o CEP ao cadastrar um local.

O **método de controle de acesso** fica em *Controle de acesso*, e o **tipo de semana epidemiológica**, em *Ciclos*.

## Mobilidade (aplicativo)
Ajustes de como o aplicativo do tablet se comporta:
- **Precisão do GPS**: o aplicativo avisa o agente quando a precisão é pior que o limite (em metros).
- **Coleta da posição**: a cada quantos segundos o aplicativo registra o rastro do agente (0 desliga).
- **Digitar o número do tubito**, **permitir DF, PVE e LIRAa com pendência "fechado"** e **gráficos no painel do aplicativo**.
- **Dias que o aplicativo baixa**: de visitas e fotos, campanhas, investigações e anexos de investigação.
- **Senha para "Limpar dados"**: só quem souber a senha limpa os dados do aparelho. A senha **não é mostrada** depois de salva; para trocar, digite outra, e para tirar, marque "Remover a senha".

Toda alteração é registrada na Auditoria (a senha aparece só como "definida" ou "sem senha").

## Leis municipais
Em **Leis municipais** você cadastra as leis (número, ano, nome e data de publicação) e, dentro de cada uma, os **trechos**: o rótulo (como "Art. 12, §2º") e o texto. É dessa lista que visitas, notificações e autos de infração vão escolher o que citar.
- O número e o ano formam uma identificação única; a lei pode ser **desativada** sem apagar.
- **Excluir** envia para a **Lixeira** (dá para restaurar). Uma lei só pode ser excluída depois de excluir seus trechos ativos.

## Abas de Parâmetros
Todas as telas de Parâmetros têm uma barra de abas no topo, como no Geo Controle: **Geral**, **Semanas epidemiológicas**, **Numeração**, **Mobilidade**, **Controle de acesso**, **Leis municipais** e **Investigação** (grupos e doenças). Só aparecem as abas que a sua permissão libera.

- **Semanas epidemiológicas**: o calendário de cada ano (primeiro dia, último dia e quantas semanas), calculado pelo padrão escolhido em **Ciclos**. Clique em **Ver semanas** para listar todas as semanas do ano.
- **Numeração**: mostra o formato dos números de notificação (`NT-AAAA0000000`) e de auto de infração (`INF-AAAA00000`), o último número usado no ano e o próximo. A contagem recomeça todo ano.
- **API Conta Ovos**: liga a integração com a Conta Ovos (contagem de ovos das ovitrampas). Informe o tipo (pública ou privada), o estado, o município e, na integração privada, a chave da API. A chave não é mostrada depois de salva; para trocar, digite outra, e para apagar, marque **Remover**. Nas alterações, a auditoria só registra se a chave está definida, nunca o valor.

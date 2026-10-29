titulo: Controle de acesso aos locais
resumo: Definir o que cada pessoa com "apenas atribuídos" enxerga: bairros, quarteirões ou locais.
ordem: 9
permissao: acesso_local
palavras: acesso, atribuídos, responsável, bairro, quarteirão, local, apenas atribuídos, escopo, restrição
---
# Controle de acesso aos locais

Alguns papéis têm permissões como **"Visualizar apenas atribuídos"** (de bairros e de locais). Quem tem uma delas só enxerga o que foi **atribuído** a si: nas listas, nas fichas, nos grupos de locais, no mapa, no cadastro de locais e no cadastro em lote. Esta tela, em **Administração → Controle de acesso**, define **como** essa atribuição é feita.

## Os dois métodos
- **Pelo responsável do bairro** (padrão): a pessoa enxerga os bairros em que está marcada como **responsável**, na ficha do bairro.
- **Pela lista de acessos**: vale só a lista desta tela. Nela você concede acesso a:
  - um **bairro** inteiro (a pessoa vê todos os quarteirões e locais dele);
  - um **quarteirão** (vê só os locais dele e o bairro aparece com esse quarteirão);
  - um **local** específico (vê só aquele local). Esse acesso é dado na **ficha do local**, em "Quem tem acesso a este local".

Ao mudar o método, a mudança vale **na hora** e fica na Auditoria. No método da lista de acessos, ser responsável por um bairro **não** dá acesso a ele.

## Conceder e retirar
1. Em **Conceder acesso**, escolha a pessoa e o bairro. Se quiser limitar a um quarteirão, escolha-o também. A descrição é opcional.
2. A lista mostra cada acesso, com a data; **Retirar** desfaz. Dá para filtrar pela pessoa.
3. Nada é apagado dos locais: só muda o que cada pessoa enxerga.

## Quem não é afetado
Quem **não** tem a permissão "apenas atribuídos" enxerga tudo, como antes, e o **master** vê sempre tudo.

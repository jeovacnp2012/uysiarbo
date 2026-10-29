titulo: Cidades, bairros e quarteirões
resumo: Como cadastrar o território: cidades, bairros, responsáveis e quarteirões.
ordem: 5
permissao: bairro_visualizar
palavras: bairro, quarteirão, cidade, povoado, responsável, mutirão, território, ibge, lote, atribuídos
---
# Cidades, bairros e quarteirões

O território é organizado em três níveis: **cidade → bairro → quarteirão**. Mais adiante, cada **local** (imóvel) pertencerá a um quarteirão.

## Cidades
Em **Cidades**, cadastre cada município atendido: nome, **UF** (sigla do estado, como MT) e o código IBGE (opcional, 7 dígitos). Uma cidade **inativa** não recebe novos bairros.

## Bairros
Em **Bairros** você vê a lista com filtros (nome, cidade, situação, categoria). Para cadastrar, clique em **Novo bairro**:

- **Nome** e **cidade**.
- **Situação**: *ativo* ou *extinto* (quando o bairro deixou de existir, sem apagar o histórico).
- **Categoria**: *bairro* ou *povoado*. **Classificação**: *urbano* ou *rural*. **Zona** e **código IBGE**, se houver.
- **Latitude e longitude** (informe as duas ou nenhuma), **altitude** e **distância do centro** em km.
- **Bairro de mutirão** e **entra no relatório de produção**.

### Responsáveis
Marque quem responde pelo bairro. Isso também controla o acesso: quem tem a permissão **"Visualizar apenas atribuídos"** só enxerga os bairros em que está marcado como responsável.

## Quarteirões
Depois de salvar o bairro, a seção **Quarteirões** aparece na mesma tela. Para cadastrar em lote, escreva **um por linha** ou uma **faixa**, como `1-20` (cria do 1 ao 20). Os que já existem são ignorados. Um quarteirão que não é mais usado pode ser **desativado** e **reativado** depois: nada se perde.

## Permissões
- **Visualizar**: ver as listas e as fichas.
- **Adicionar/Editar**: criar e alterar cidades, bairros, responsáveis e quarteirões.
- **Visualizar apenas atribuídos**: limita a lista aos bairros em que a pessoa é responsável.

Tudo o que é criado ou alterado fica registrado na **Auditoria**.

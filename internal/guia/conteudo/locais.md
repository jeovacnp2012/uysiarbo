titulo: Locais
resumo: Como cadastrar imóveis, comércios, terrenos e pontos estratégicos, com moradores e habitantes.
ordem: 6
permissao: local_visualizar
palavras: local, imóvel, endereço, quarteirão, lado, sequência, complemento, proprietário, inquilino, habitantes, cpf, ponto estratégico, terreno baldio
---
# Locais

Um **local** é cada imóvel ou ponto que a equipe visita: uma casa, um comércio, um terreno baldio, um ponto estratégico. Fica em **Território → Locais**.

## Cadastrar
Clique em **Novo local** e preencha:

- **Tipo**: residencial, comércio, terreno baldio, ponto estratégico ou outro.
- **Endereço**: rua ou avenida, **número** (use `S/N` quando não houver), **CEP** (opcional) e **ponto de referência**.
- **Bairro e quarteirão**: ao escolher o bairro, a lista de quarteirões mostra só os quarteirões ativos dele.
- **Lado**: o lado da quadra (por exemplo, Par, Ímpar ou A).
- **Sequência e complemento**: números que ordenam os locais no quarteirão e separam, por exemplo, as casas dos fundos.
- **Latitude e longitude**: informe as duas ou nenhuma. Elas serão usadas no mapa.

Dois locais não podem ter o mesmo endereço, número, lado, sequência e complemento no mesmo quarteirão: o sistema avisa quando isso acontece.

### Moradores
Proprietário (nome, **CPF**, telefone, WhatsApp, e-mail), inquilino e telefone dele. As caixas **aceita WhatsApp** e **aceita e-mail** registram se o proprietário concordou em receber mensagens. O CPF é conferido pelos dígitos verificadores e, na Auditoria, aparece **mascarado**.

### Habitantes
Quantidade de pessoas, cachorros, gatos e roedores. Pode ficar em branco quando não se sabe.

## Cadastro em lote
Para uma rua inteira, use **Cadastro em lote** (na lista de locais). Informe o tipo, a rua, o bairro e o quarteirão, o **lado**, o **primeiro e o último número** e o intervalo (**de 1 em 1**, ou **de 2 em 2** para só pares ou só ímpares). Se preencher a **sequência inicial**, ela sobe de um em um a cada local. Cada lote cria no máximo 500 locais, e os que já existem são pulados (a tela avisa quantos foram criados e quantos já existiam). Depois, complete moradores e habitantes abrindo cada local.

## Chave e QR Code
A ficha de um local já salvo mostra o **QR Code** e a **chave** dele: imprima e cole no imóvel, e o aplicativo do tablet encontra o local ao ler o código. Se o adesivo se perder ou vazar, **Gerar nova chave** invalida o QR Code antigo.

## Desativar e reativar
Quem tem a permissão **Desativar** vê, na ficha, o botão **Desativar local**. Nada é apagado: o local sai das listas de quem não tem **Ver desativados** e pode ser **reativado** a qualquer momento.

## Lista e filtros
A lista busca por **endereço, número ou proprietário** e filtra por bairro, tipo e situação.

## Permissões
- **Visualizar**: ver a lista e as fichas.
- **Adicionar/Editar**: cadastrar e alterar locais.
- **Desativar** e **Ver desativados**: desativar/reativar locais e enxergar os desativados.
- **Visualizar apenas atribuídos**: só os locais dos bairros em que a pessoa é responsável (e só pode cadastrar nesses bairros).
- **Visualizar apenas ponto estratégico**: só enxerga e cadastra pontos estratégicos.

Tudo o que é criado ou alterado fica registrado na **Auditoria**, com o antes e o depois. Se outra pessoa estiver com o mesmo local aberto, a ficha avisa quem é.

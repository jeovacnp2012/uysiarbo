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

## Lista e filtros
A lista busca por **endereço, número ou proprietário** e filtra por bairro, tipo e situação.

## Permissões
- **Visualizar**: ver a lista e as fichas.
- **Adicionar/Editar**: cadastrar e alterar locais.
- **Visualizar apenas atribuídos**: só os locais dos bairros em que a pessoa é responsável (e só pode cadastrar nesses bairros).
- **Visualizar apenas ponto estratégico**: só enxerga e cadastra pontos estratégicos.

Tudo o que é criado ou alterado fica registrado na **Auditoria**, com o antes e o depois. Se outra pessoa estiver com o mesmo local aberto, a ficha avisa quem é.

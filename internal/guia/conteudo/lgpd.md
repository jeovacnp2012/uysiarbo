titulo: Dados pessoais e LGPD
resumo: O que o sistema faz para proteger CPF e outros dados pessoais, e o registro de quem abre as fichas.
ordem: 24
permissao: master
palavras: lgpd, cpf, dados pessoais, criptografia, cifra, chave, privacidade, auditoria, acesso
---
# Dados pessoais e LGPD

## CPF cifrado
O CPF de proprietários (Locais e LVC) e de denunciantes é guardado **cifrado** no banco (AES-256). Quem tem acesso ao banco, a cópias de segurança ou a planilhas exportadas do banco não lê o número. Na tela, quem tem permissão de ver a ficha continua vendo o CPF normalmente. Na auditoria, o CPF nunca aparece inteiro (só mascarado).

## A chave (DADOS_CHAVE)
- É uma chave de 32 bytes no arquivo `.env` do servidor. **Obrigatória em produção.**
- Para gerar: `uysiarbo dados chave`. Guarde uma cópia em lugar seguro **fora do servidor**: sem a chave, os CPFs cifrados não abrem (a tela mostra o campo vazio).
- **Nunca troque a chave** de uma instalação com dados já cifrados.
- Ao ligar a chave pela primeira vez, rode `uysiarbo dados cifrar` uma vez: ele cifra o que já estava gravado em texto puro (pode rodar de novo sem problema).

## Quem abriu a ficha
Toda vez que alguém abre a ficha de um **local, LVC, denúncia, investigação ou BAAP** (registros com dados pessoais), a Auditoria registra a ação **"Abriu ficha com dados pessoais"**, com quem foi e quando. Filtre por essa ação para responder "quem viu os dados desta pessoa?".

## Anonimização a pedido do titular
Em **Configuração > Anonimização (LGPD)** (exige a permissão de exclusão definitiva) escolha o identificador da pessoa (CPF, telefone ou e-mail) e clique em **Buscar registros**. O sistema mostra quantos registros há em Locais, Denúncias, LVC, Investigações e BAAP. Informe o **motivo** (por exemplo, o número do pedido), marque a confirmação e clique em **Anonimizar**: nome, CPF, telefone, e-mail e mãe/inquilino são apagados (o paciente vira "ANONIMIZADO"); endereço, datas e resultados ficam. **Não tem volta.** A Auditoria guarda quem fez, o motivo e as quantidades, mas não o dado da pessoa.

## O que ainda não está coberto
Telefones, nomes e e-mails ficam em texto (precisam ser pesquisados e usados nas mensagens); são protegidos pelas permissões e pelo registro de acesso. A anonimização não alcança o texto livre (observações, entrevistas) nem as mensagens já enviadas em Comunicações. Prazos de retenção ainda serão feitos.

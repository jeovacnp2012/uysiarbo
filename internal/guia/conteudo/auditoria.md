titulo: Auditoria
resumo: O histórico de tudo o que é feito no sistema: quem, quando, de onde e o que mudou.
ordem: 12
permissao: auditoria_sistema
palavras: auditoria, histórico, log, quem fez, antes e depois, rastro, filtros
---
# Auditoria

A **Auditoria** guarda o histórico de tudo o que acontece no ambiente. Ninguém consegue **alterar nem apagar** esses registros, nem mesmo o master: o próprio banco de dados recusa.

## O que fica registrado
Entradas e saídas, tentativas de senha errada e bloqueios, trocas e resets de senha, criação e edição de usuários e papéis. Qualquer outra gravação feita no sistema também deixa um registro automático.

Cada registro mostra **quem** fez, **quando**, de qual **endereço de rede (IP)**, o **navegador** e o **registro afetado**. Quando algo foi editado, o detalhe mostra o **antes e o depois**, campo a campo.

## Senhas nunca aparecem
O histórico não guarda senhas, nem sequer as temporárias, nem códigos de acesso.

## Filtrar
Use **De/Até** (período), **Quem** (nome ou e-mail), **Ação** e **Registro afetado**. Clique na ação para abrir os detalhes. A lista tem 30 registros por página.

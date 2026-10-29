titulo: Clientes (console)
resumo: Criar prefeituras, escolher o subdomínio, suspender e abrir cada ambiente.
ordem: 20
permissao: master
palavras: cliente, prefeitura, subdomínio, console, suspender, reativar, ambiente, schema
---
# Clientes (console da plataforma)

Só o **master** vê esta tela, no **console** (o endereço sem cliente).

## Criar um cliente
1. **Clientes → Novo cliente**.
2. **Nome**: o nome da prefeitura.
3. **Identificador**: de 3 a 30 letras minúsculas e números, começando por letra. Vira o nome do banco do cliente e **não muda depois**.
4. **Subdomínio de acesso**: você escolhe. A tela mostra o endereço que o cliente terá (por exemplo, `uysiarbonm.inovarsistemas.biz`). Alguns nomes são reservados por já terem outro uso no servidor.
5. **Domínio próprio** (opcional) e **plano**.

O sistema cria o ambiente, o banco separado do cliente e o papel **Administrador**, tudo na hora.

## Suspender e reativar
**Suspender** bloqueia o acesso do cliente na hora: quem tentar entrar vê um aviso. **Reativar** libera de novo. Nada é apagado.

## Abrir o ambiente
**Abrir ambiente** leva ao endereço do cliente. Lá, o master entra com o mesmo e-mail e senha e vê todas as telas.

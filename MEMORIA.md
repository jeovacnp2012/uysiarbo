# MEMÓRIA DO PROJETO — UYSI ARBO

Arquivo vivo, versionado no Git, para retomar o trabalho em qualquer PC. **O Claude deve atualizá-lo ao fim de cada sub-etapa ou decisão.** Plano completo e checklist: `docs/plano-de-execucao.md`.

## Regras permanentes de trabalho
- Toda comunicação, narração de ferramentas, commits, PRs e textos de interface em **pt_BR**. Código (variáveis, classes) em inglês técnico.
- Uma sub-etapa por vez: propor, executar, testar, mostrar, esperar aprovação.
- Nada de commit ou produção sem o usuário pedir. Quando ele pedir para commitar, o Claude faz commit **e push** para `origin/main` (autorização permanente de 2026-10-29); se o push for bloqueado pelo sistema de permissões, só commit e avisar.
- Mudança de escopo/decisão: atualizar `docs/plano-de-execucao.md` antes do código.
- Regra de "pronto": testes passando, telas conferidas, Guia atualizado, versão em Novidades, migrações em todos os schemas, aprovação do usuário.

## O projeto
Nova versão do Geo Controle (controle de endemias/dengue para prefeituras) em Go. Nome: **UYSI ARBO**, módulo Go `uysiarbo`.
- Stack: Go + chi + templ + HTMX + Alpine; PostgreSQL + PostGIS (goose, sqlc, pgx); um schema por cliente; app Flutter offline-first no tablet (mesma API); VPS Linux (Nginx + binário + Postgres).
- Login com Google reCAPTCHA v2 (secret key só no `.env`).
- Identidade visual: padrão UYSI, igual a `C:\laragon\www\uysilicitaproc` (paleta slate/brand `#155a9e`, tema escuro `data-tema="escuro"`, logo/brasão em `public/brand/`).
- Referências: `docs/engenharia-reversa-geocontrole.md`, `docs/engenharia-reversa-geocontrole-apendice.md`, `docs/geocontrole-modelo-dados.sql`.

## Estado atual
- Etapa 0: **0.1 concluída** (repo, `go.mod`, pastas, `cmd/server/main.go` "olá mundo").
- **0.2 concluída** (Postgres 18 + PostGIS do Laragon, `.env.example`, `scripts/criar-banco.ps1`; `.env` local criado, o `postgres` local não exige senha).
- **0.3 concluída**: `internal/config` (.env + ambiente), `internal/httpx` (JSON e erro padrão `{error:{code,message,request_id}}`), `internal/server` (chi, request-id, log de acesso, recoverer, `/healthz`), `cmd/server` com shutdown gracioso. Testes: `go test ./...`. Rodar: `go run ./cmd/server` (porta `APP_ADDR`, padrão `:8080`). `/readyz` com ping no banco entra na 0.4.
- **0.4 concluída**: `migrations/` (goose, embutido via `embed`), `internal/db` (pool pgx, `Migrate`), `internal/db/queries` + `sqlc.yaml` → `internal/db/sqlc` (gerado; regenerar com `go tool sqlc generate`), `/readyz`. O servidor não sobe sem banco. Testes de integração usam o banco `uysiarbo_test` (`TEST_DATABASE_URL` para mudar). Rodar scripts .ps1 com `powershell -ExecutionPolicy Bypass -File ...` (a política padrão bloqueia).
- **Decisão (2026-10-29): o app Flutter do tablet é a ÚLTIMA etapa do projeto (agora Etapa 14, depois de Produção/Etapa 13).** A API de sincronização em lote (6.4) continua no backend, antes.
- **0.5 concluída**: `web/` (embed, CSS UYSI claro/escuro, `app.js` com tema, toasts e campo de senha), `web/static/vendor` (htmx 2.0.4, alpine 3.14.9, baixados do jsDelivr), `web/views` (templ: layout com menu lateral, PageHeader, Table, Pager por HTMX, Modal, Toasts, PasswordField), rotas `/`, `/componentes`, `/componentes/tabela`, `/static/*`. Gerar telas: `go tool templ generate` (os `*_templ.go` vão para o Git). Conferido no navegador (desktop e celular). Ainda sem CSRF (entra com o login, etapa 1.5). Páginas `/componentes` são só demonstração e saem quando houver telas reais.
- **0.6 concluída — Etapa 0 (Fundação) completa.** `cmd/server` virou o binário único (`uysiarbo migrate [up|down|status|reset]`, `uysiarbo version`; o `cmd/migrate` foi removido). `scripts/verificar.ps1` roda as mesmas checagens do CI antes de commitar; `scripts/build.ps1 [-Linux] [-Versao x]` gera `bin/uysiarbo.exe` ou `bin/uysiarbo-linux-amd64` (~15 MB, estático, testado isolado). CI em `.github/workflows/ci.yml` (imagem `postgis/postgis:18-3.6`, **ainda não executado no GitHub**: conferir a primeira execução). Aviso: no Windows o `go test` às vezes sai com erro "unlinkat" por causa do antivírus, mesmo com testes verdes.
- **Próxima: Etapa 1** (clientes/schemas, acesso, usuário master, reset de senha). Antes de começar, rever com o usuário as decisões pendentes da Etapa 1 (chaves do reCAPTCHA, domínio, modelo comercial).

## Ambiente (PC principal do dono)
- Windows 11, Laragon em `C:\laragon\www\uysiarbo`, Go 1.27.3.
- **Decisão (2026-10-29): usar o Postgres do Laragon, sem Docker.** É o PostgreSQL **18.6**, porta 5432, binários em `C:\laragon\bin\postgresql\postgresql\bin`, dados em `C:\laragon\data\postgresql`.
- **PostGIS 3.6.2 INSTALADO (2026-10-29)** e banco `uysiarbo` criado com a extensão. Em outro PC, repetir o procedimento abaixo (o ZIP `postgis-bundle-pg18-3.6.2x64.zip` é o oficial do osgeo; copiar com robocopy `/XC /XN /XO` para não sobrescrever nada, com o Postgres parado).
- (Histórico) Como instalar: Instalar a versão compatível com PG 18 (Windows, 64 bits): baixar o ZIP/instalador em https://download.osgeo.org/postgis/windows/ (pasta pg18), com o Postgres parado, copiar `bin`, `lib` e `share` para dentro de `C:\laragon\bin\postgresql\postgresql\` (ou usar o Stack Builder da EDB), e reiniciar o Postgres pelo Laragon.
- Depois: copiar `.env.example` para `.env` e rodar `powershell -File scripts\criar-banco.ps1` (cria o banco `uysiarbo` e habilita o PostGIS).
- Em outro PC: instalar Go, clonar o repositório, ler este arquivo e o plano.

## Requisitos de acesso definidos pelo usuário (etapa 1, ver plano 1.4, 1.5.1 e 1.5.2)
- Usuário **master**: JEOVA OLIVEIRA (`jeovacnp2012@gmail.com`). Nada de permissão se aplica a ele; vê e faz tudo, em todos os clientes. Protegido contra exclusão/rebaixamento. Senha dele nunca em arquivo/código.
- Permissões granulares por **blocos**, com **perfis** (papéis) diversos.
- **Reset de senha**: gera senha forte, mostra uma vez a quem resetou; no próximo login a pessoa é obrigada a definir a própria senha.
- **Todo campo de senha** tem opção de revelar/ocultar.

## VPS de produção (referência para a etapa 13 (produção))
Detalhes e credenciais ficam em `C:\laragon\www\VPS_STATUS.md` (fora do Git; **nunca copiar senhas para este arquivo**). Resumo útil:
- Rapid Cloud, Ubuntu 22.04. Acesso por SSH com chave, usuário `jeova` (não é root; `sudo` pede senha, então o Claude nunca roda sudo: imprime o comando para o usuário rodar).
- Stack já instalada: PostgreSQL 18, Nginx (roda como usuário `nginx`), Supervisor, Certbot, Valkey, UFW e Fail2ban. **Não tem Go instalado e não sabemos se o PostGIS está instalado**; conferir antes do deploy.
- Padrão dos outros projetos: código em `/var/www/<projeto>`, deploy key só-leitura por projeto (alias em `~/.ssh/config`), vhost em `/etc/nginx/conf.d/<projeto>.conf`, `.env` com grupo `www-data` e modo 640, DNS no Cloudflare (registro A, proxy desligado), domínios `*.inovarsistemas.biz`.
- Diferença para o UYSI ARBO: os outros projetos usam um schema por projeto no banco compartilhado `inovarsistemas`. O UYSI ARBO tem um schema por **cliente**, então deve ter banco próprio (`uysiarbo`) e role própria com permissão de criar schemas, além da extensão PostGIS (instalação exige sudo).
- Cuidados: Certbot pode aplicar o certificado no vhost errado quando existe vhost curinga (conferir `ssl_certificate` depois de emitir); o curinga `*.inovarsistemas.biz` já existe e o LabPront o usa; binário Go roda como serviço systemd (ou Supervisor) atrás do Nginx.

## Repositório
- Remoto: https://github.com/jeovacnp2012/uysiarbo.git (branch `main`). Em outro PC: `git clone` dessa URL, instalar Go e Postgres+PostGIS, copiar `.env.example` para `.env`.

## Pendências de decisão do usuário
Chaves do reCAPTCHA; domínio; provedor de geocodificação (etapa 3); WhatsApp/e-mail (12); formato SINAN/e-SUS (12); conectores de outros sistemas (4); fórmulas IIP/Breteau (7–8); modelo comercial (1 ou 14).

## Diário
- 2026-10-29: arquivo de memória criado; retomada do projeto em 0.2.

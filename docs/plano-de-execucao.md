# Plano de execução — nova versão do Geo Controle em Go

Documento vivo: cada etapa só começa quando a anterior estiver **pronta e aprovada**. Marque as caixas `[x]` conforme
for concluindo. Base de referência: `engenharia-reversa-geocontrole.md`, `engenharia-reversa-geocontrole-apendice.md` e
`geocontrole-modelo-dados.sql` (todos nesta pasta).

## 1. Decisões já tomadas

| Tema | Decisão |
|---|---|
| Linguagem | Go no backend, na API e no front web |
| Front web | Go + `templ` + HTMX (+ Alpine.js); mapas e gráficos como "ilhas" de JavaScript |
| Banco | PostgreSQL + PostGIS |
| Multi-cliente | **Um schema por cliente** (prefeitura) no mesmo banco |
| Tablet | App Flutter (offline-first), falando com a mesma API |
| Hospedagem | VPS Linux própria (Nginx + executável Go + Postgres) |
| Equipe | Só o dono do projeto, com o Claude: etapas pequenas, testadas e aprovadas uma a uma |
| Dados | Começar do zero; **gerenciador de importação** para migrar de outras fontes |
| Importação | Planilhas CSV/Excel com modelo, conector do Geo Controle (via API) e conectores de outros sistemas, por tipo de origem |
| API | Leitura e gravação, com credenciais por cliente (OAuth2 *client credentials*, escopos, limite de uso) |
| Acesso web | Login e senha com **Google reCAPTCHA** (versão v2 "não sou um robô", validada no servidor Go; chaves por ambiente no `.env`) |
| Integrações | WhatsApp e e-mail, geocodificação por endereço/CEP, SINAN/e-SUS/SUS |
| Fixos desde a etapa 1 | **Guia do sistema** e **Novidades/versões** (tela com o histórico de mudanças) |
| Nome do sistema e da pasta | **UYSI ARBO** (`uysiarbo`), módulo Go `uysiarbo` |
| Identidade visual | Padrão **UYSI**, igual ao `C:\laragon\www\uysilicitaproc`: paleta (`resources/css/app.css`: escalas slate/brand `#155a9e`/azul, tema escuro por `data-tema="escuro"`), logo e brasão em `public/brand/`, barra superior, componentes (títulos, listas em cartões, abas, selos, formulários) e `config/uysi.php`. Será reproduzida em Go + templ na sub-etapa 0.5 |
| Idioma | pt_BR em toda a interface e documentação; código em inglês técnico |

### Regra de "pronto" (vale para toda etapa)
1. Testes automatizados passando (unidade + integração com Postgres real).
2. Telas conferidas no navegador (desktop e celular).
3. **Guia do sistema** atualizado com o que a etapa entregou.
4. **Nova versão cadastrada em Novidades** (`1.0.001`, `1.0.002`...), com o que mudou em linguagem simples.
5. Migrações aplicáveis em todos os schemas de clientes existentes.
6. Você aprova antes de passarmos para a próxima etapa.

---

## Etapa 0 — Fundação do projeto
Objetivo: base técnica pronta para receber qualquer módulo.
- [x] 0.1 Repositório Git, módulo Go (`go.mod`), estrutura de pastas (`cmd/`, `internal/`, `web/`, `migrations/`, `docs/`)
- [x] 0.2 Ambiente local: Postgres 18 + PostGIS 3.6.2 do Laragon (sem Docker); `.env.example`; `scripts/criar-banco.ps1` (comando único)
- [x] 0.3 Servidor HTTP (`chi`), configuração (`.env`), logs estruturados (`slog`), erros padronizados em JSON, health check `/healthz`, encerramento gracioso
- [x] 0.4 Migrações (`goose`, embutidas no binário; ``uysiarbo migrate up|down|status|reset` (ou `go run ./cmd/server migrate ...`)`) e acesso a dados (`sqlc` + `pgx`, `go tool sqlc generate`); primeira migração `00001_criar_app_info`; `/readyz` com ping no banco
- [x] 0.5 Front base: layout com menu lateral, tema claro/escuro, `templ` + HTMX + Alpine (arquivos em `web/static/vendor`, embutidos no binário), componentes de tabela paginada, filtros, modal, toasts e campo de senha com revelar
- [x] 0.6 CI (GitHub Actions: gofmt, templ/sqlc em dia, go vet, testes com Postgres+PostGIS, build Linux) e scripts: `scripts/verificar.ps1` (mesmas checagens, local) e `scripts/build.ps1` (binário único, `-Linux` para a VPS)
**Entrega:** "olá mundo" com layout, banco e testes rodando.

## Etapa 1 — Clientes (schemas), acesso e administração
Objetivo: múltiplas prefeituras isoladas, com usuários e permissões.
- [x] 1.1 Schema `public` (catálogo): tabela `clientes` (nome, slug, schema `c_<slug>`, subdomínio, domínio próprio opcional, plano, status), restrições no banco e queries; migrações separadas em `migrations/public` (catálogo) e, na 1.2, `migrations/cliente`
- [x] 1.2 Provisionamento: `uysiarbo cliente criar` (registra, cria o schema `c_<slug>`, roda `migrations/cliente`, ativa; repetível se parar no meio), `uysiarbo cliente listar` e `uysiarbo migrate-all` (catálogo + todos os clientes)
- [x] 1.3 Resolução do cliente por requisição: `<subdominio>.<APP_BASE_DOMAIN>` (subdomínio escolhido por quem cria o cliente) ou domínio próprio; console da plataforma em `APP_ADMIN_HOST`; cliente suspenso = 403, inexistente = 404; conexões por schema com `search_path` fixado na conexão (`tenant.Pools`); subdomínios reservados recusados
- [x] 1.4 Usuários, papéis e permissões (modelo inspirado nas ~190 permissões e 24 categorias do sistema atual): permissões **granulares organizadas por blocos** e **perfis diversos** (papéis). **Usuário master: JEOVA OLIVEIRA, e-mail de acesso `jeovacnp2012@gmail.com`**: nenhuma regra de permissão, restrição de bloco, "apenas atribuídos" ou escopo de cliente se aplica a ele; vê e faz tudo, em todos os clientes. O master é protegido (não pode ser excluído, rebaixado ou ter o acesso reduzido por outro usuário); a senha dele nunca é registrada em arquivo, plano ou código.
  - [x] 1.4a Catálogo de permissões (`internal/authz/catalogo.txt`, 169 permissões em 23 blocos, 11 restritivas) e motor `authz.Principal` (master pode tudo e nunca é restringido)
  - [x] 1.4b Tabelas `papeis`, `papel_permissoes` e `usuarios` (migração de cliente 00002) e `plataforma_usuarios` (migração pública 00003, com gatilho que impede excluir, rebaixar ou desativar o master); `Provision` cria o papel Administrador (158 permissões, sem as restritivas); `cliente sincronizar` repõe permissões novas do catálogo
  - [x] 1.4c `uysiarbo master criar/resetar/listar`: senha forte de 16 caracteres gerada, mostrada uma vez, guardada só como hash argon2id, troca obrigatória no primeiro login (`internal/auth`)
- [x] 1.5 Login com Google reCAPTCHA v2 (verificação do token no servidor), sessão em cookie seguro, bloqueio por tentativas, recuperação de senha
  - [x] 1.5a Núcleo (`internal/auth`): sessões no banco (só o hash SHA-256 do token), login com bloqueio (5 falhas = 15 min), master entra em qualquer cliente, troca de senha (política: 10+ caracteres, letras e números; derruba as outras sessões), reset por administrador (senha forte mostrada uma vez, derruba sessões, remove bloqueio, troca obrigatória)
  - [x] 1.5b Web: CSRF (cookie + token no formulário/cabeçalho + conferência de Origin), reCAPTCHA v2 validado no servidor (chaves de teste do Google no `.env.example`; em produção as chaves são obrigatórias), telas `/entrar`, `/sair` e `/trocar-senha` com campo de senha com revelar, todas as páginas protegidas por login (HTMX recebe `HX-Redirect`), troca obrigatória depois de reset, destino pós-login só no mesmo site, cookie `HttpOnly` + `SameSite=Lax` + `Secure` atrás de HTTPS, cabeçalhos de segurança
  - [ ] 1.5c Recuperação de senha por e-mail ("esqueci a senha"): depende do envio de e-mail (SMTP, etapa 12.2); até lá, quem esquece pede o reset a um administrador
- [x] 1.5.1 **Reset de senha por administrador**: ao resetar a senha de alguém, o sistema gera uma senha **forte** e a **mostra uma única vez** a quem resetou (com botão copiar). No próximo login do usuário com essa senha temporária, o sistema o obriga a **escolher a própria senha** antes de continuar (a temporária deixa de valer). Registrar o reset na auditoria (quem resetou, para quem, quando), sem guardar a senha
- [x] 1.5.2 **Revelar senha**: todo campo de senha da interface (login, troca, criação, reset) tem a opção de mostrar/ocultar o que foi digitado
- [x] 1.6 Telas de **Papéis** (lista com contagens; editor de permissões em 23 blocos e grupos, com busca, marcar tudo/bloco/todas e selo "restringe" nas 11 restritivas) e **Usuários** (lista, novo, editar, ativar/desativar, escolher papel e acessos web/tablet, **reset de senha** com senha forte mostrada uma vez). Regras: exigem `conf_papel`/`conf_usuario` (master pode tudo); só existem no endereço de um cliente; quem edita só concede e retira o que ele mesmo tem; não desativa papel com usuários; ninguém desativa a si mesmo, troca o próprio papel ou reseta a própria senha; trocar o papel, desativar ou resetar encerra as sessões; o menu esconde o que a pessoa não pode abrir
- [x] 1.7 **Auditoria de tudo o que se faz no sistema** (`internal/audit`): histórico por cliente (`auditoria`, schema do cliente) e da plataforma (`auditoria_plataforma`), só acrescenta (gatilhos recusam UPDATE, DELETE e TRUNCATE); registra ator, IP, navegador, requisição, ação, registro afetado e antes/depois; senhas, hashes e tokens nunca entram; eventos de login (sucesso, falha de conta existente, bloqueio), saída, troca de senha, papéis, usuários, reset, clientes, masters; registro automático `requisicao.escrita` para qualquer gravação que não tenha evento próprio; tela `/auditoria` com filtros (período, quem, ação, registro), detalhe do antes/depois e paginação (permissão `auditoria_sistema`; no console, só o master)
- [x] 1.7b **Console do master**: Clientes (criar com o subdomínio que o master escolhe, suspender/reativar, abrir o ambiente), Usuários da plataforma (criar master e resetar senha, derrubando as sessões em todos os ambientes), Auditoria, painel inicial com atalhos; o master vê todas as telas em cada ambiente (Usuários, Papéis, Auditoria e todos os módulos do catálogo)
- [x] 1.8 **Guia do sistema** (`internal/guia`: artigos em Markdown embutidos no binário, permissão por artigo, busca que ignora acento e maiúsculas, com trecho) e **Novidades/Versões** (tabela `versoes` com o histórico 1.0.001 a 1.0.004 em linguagem simples; tela `/novidades`; o master registra versões no console; aviso "Novidades · nova" no topo até a pessoa abrir a página; versão atual no rodapé do menu)
**Entrega:** entrar como administrador de uma prefeitura de teste, criar usuários e papéis, ver Guia e Novidades. **✅ Etapa 1 concluída em 2026-10-29.**

## Etapa 2 — API com credenciais por cliente
Objetivo: leitura e gravação por sistemas externos, de forma segura.
- [x] 2.1 Cadastro de credenciais de API por cliente (`client_id`, segredo guardado só como hash, status, validade, rotação)
- [x] 2.2 Emissão de token (OAuth2 *client credentials*, JWT de vida curta) e escopos `recurso:read` / `recurso:write`
- [x] 2.3 Middleware: identifica o cliente pelo token, aplica o schema, confere escopo, limita taxa de uso
- [x] 2.4 Log de chamadas (quem, o quê, resultado) e tela de consumo por credencial
- [x] 2.5 Contrato OpenAPI, documentação publicada e SDK de exemplo
- [x] 2.6 Padrões de gravação: `Idempotency-Key`, IDs gerados no cliente, erros padronizados, paginação por cursor
- [x] 2.7 Tela de gestão de credenciais e escopos
**Decisões da Etapa 2 (2026-10-29):** a API fica em `https://<subdominio-do-cliente>/api/v1/...` (o cliente vem do endereço, como no resto do sistema); token JWT HS256 de 15 min assinado com `API_JWT_SECRET` (obrigatória em produção), com checagem da credencial no banco a cada chamada (desativar ou rotacionar vale na hora); escopos `recurso:read|write`; o recurso simples da entrega é **cidades**; as demais rotas (bairros, quarteirões, locais) entram na 3.9; nova permissão `conf_api` ("API e integrações") no bloco Configuração; limite padrão de 120 chamadas por minuto por credencial; documentação = contrato OpenAPI em `/api/v1/openapi.yaml` + artigo do Guia + exemplos em `docs/api/`.
**Entrega:** um cliente de teste lê e grava um recurso simples via API, com credencial própria. **✅ Etapa 2 concluída em 2026-10-29 (versão 1.0.014).**

## Etapa 3 — Território: cidades, bairros, quarteirões e locais
- [x] 3.1 Cidades (nome, UF, IBGE, ativa), Bairros (todos os campos do sistema atual: situação, categoria, classificação, zona, IBGE, coordenadas, altitude, distância do centro, mutirão, entra na produção) com **responsáveis** (e escopo "apenas atribuídos" funcionando), e Quarteirões em lote (um por linha ou faixa `1-20`, desativar/reativar); telas, filtros, paginação, auditoria com antes/depois; módulo Bairros "disponível" no painel (versão 1.0.005)
- [x] 3.2 Geometrias PostGIS: polígonos de bairro e quarteirão (GeoJSON colado/importado, validado e consertado pelo PostGIS; versão 1.0.007). **O ponto do local entra na 3.3**, junto com a tabela de locais.
- [x] 3.3 Local (cadastro completo: moradores, habitantes, tipo, endereço, referência, lado, sequência, complemento; escopo "apenas atribuídos" e "apenas ponto estratégico"; ponto no PostGIS por índice sobre latitude/longitude; versão 1.0.008). Ficaram para a 3.4: desativar/reativar (permissões `local_desativar` e `local_ver_desativados`), lote e QR/chave. CPF ainda em texto: criptografia na 13.3.
- [x] 3.4 Cadastro em lote por trecho de números e sequência (até 500, pula existentes), desativação lógica (`local_desativar`, `local_ver_desativados`), chave e QR Code do local, nova chave (versão 1.0.009)
- [x] 3.5 Ficha do local (seções reservadas por etapa, grupos do local e histórico de alterações) e Grupos de Locais (criar, acrescentar por bairro/quarteirão/tipo, retirar, clonar, excluir, exportar CSV sem dados pessoais; versão 1.0.010). **Mostrar o grupo no mapa entra na 3.6.**
- [x] 3.6 Mapa base (MapLibre GL 4.7.1 em `web/static/vendor`; fundo OpenStreetMap configurável por `MAP_TILE_URL`): bairros, quarteirões, locais e grupos, com filtros, escopo do papel e "Ver no mapa" nas fichas (versão 1.0.011)
- [x] 3.7 Geocodificação por endereço/CEP: **gratuita** (ViaCEP + Nominatim/OpenStreetMap, decidido em 2026-10-29), 1 consulta/s, cache de 1 dia, provedor configurável (`GEOCODE_PROVIDER`); botões no cadastro do local (versão 1.0.012). Em lote para locais sem coordenadas: fica para a fila de tarefas (4.4)
- [x] 3.8 Controle de acesso por bairro/quarteirão/local ("apenas atribuídos"): método "pelo responsável do bairro" ou "pela lista de acessos" (tabela `acessos_local`, funções `usuario_ve_*` no banco, tela Controle de acesso, acesso por local na ficha); aplicado em listas, fichas, grupos, mapa, lote e quarteirões (versão 1.0.013)
- [x] 3.9 API: leitura e gravação de bairros, quarteirões e locais (escopos `bairros|quarteiroes|locais:read|write`, `id_externo` para IDs gerados no cliente com criação idempotente, `updated_since` para sincronizar, sem CPF; versão 1.0.015). **✅ Etapa 3 concluída em 2026-10-29.**
**Entrega:** cadastrar e consultar a base territorial inteira de uma prefeitura.

## Etapa 4 — Gerenciador de importação
Objetivo: migrar dados de outros sistemas e de planilhas, com segurança.
- [x] 4.1 Modelo de "importação": origem, tipo de dado, arquivo, status, responsável, log de erros
- [x] 4.2 Importador de planilhas CSV/Excel com **modelos para baixar** (usuários, bairros, quarteirões, locais)
- [x] 4.3 Etapas de segurança: validação linha a linha, **prévia** (o que será criado/atualizado/ignorado), confirmar, desfazer
- [x] 4.4 Execução em segundo plano (fila `river`) com barra de progresso e relatório final baixável
- [x] 4.5 Mapeamento de colunas (origem → destino) salvo por conector
- [x] 4.6 Conector "Geo Controle (API)": credenciais da origem, descoberta do que dá para importar, mapeamento automático
- [x] 4.7 Estrutura para conectores de outros sistemas (interface única, cada conector em um pacote)
- [x] 4.8 Importação via API (outro sistema empurra dados)
**Regra transversal nova (2026-10-29): exclusão lógica.** Nenhum registro é apagado de verdade: `deleted_at`/`deleted_by`, Lixeira para restaurar, e "force delete" só com a permissão `conf_exclusao_definitiva`, só do que já está na lixeira e só sem vínculo com outro registro (aviso do que está ligado). Aplicada a cidades, bairros, quarteirões, locais e grupos (migração `cliente/00016`); toda entidade futura segue a mesma regra (MEMORIA.md, regra transversal 7). A "Importação desfazer" manda para a lixeira.
**Decisões da Etapa 4, bloco 2 (4.5 a 4.8, 2026-10-29):** o Geo Controle não tem API documentada (só o portal com sessão web), então: (a) **4.7** define a interface de conectores em `internal/importacao` e três conectores: `planilha`, `geocontrole` (planilha exportada do Geo Controle, com os nomes de coluna dele reconhecidos) e `api_rest` (busca um JSON em um endereço HTTPS com token, para o Geo Controle quando existir API e para outros sistemas); (b) **4.6** entrega o Geo Controle como o preset de colunas acima + o conector `api_rest` já com os nomes de campo do Geo Controle, **sem estar testado contra o sistema real** (as rotas e credenciais precisam ser confirmadas pelo usuário); credenciais da origem **não são guardadas**, são digitadas a cada importação; o servidor só acessa endereços HTTPS públicos (bloqueia rede privada, loopback e redirecionamentos para eles, configurável só para testes); (c) **4.5** guarda mapeamentos de colunas por conector e tipo (com exclusão lógica) e mostra a tela "Mapear colunas" quando o cabeçalho não bate ou a pessoa pede; o arquivo fica guardado temporariamente (bytea) até a análise; (d) **4.8**: `POST /api/v1/importacoes` (escopo `importacoes:write`) recebe as linhas em JSON, faz a mesma análise com prévia, `POST .../{id}/executar` inicia, `GET .../{id}` e `.../linhas` acompanham.
**Decisões da Etapa 4 (2026-10-29):** primeiro bloco = 4.1 a 4.4 (planilhas). Tipos: bairros, quarteirões e locais (usuários e demais entram quando as etapas deles existirem). Formatos: CSV (UTF-8 ou Latin-1; `;`, `,` ou tabulação) e XLSX, com modelos para baixar nos dois. Fluxo: enviar → **análise sem gravar nada** (prévia com criar/atualizar/ignorar/erro por linha) → confirmar → execução em segundo plano dentro do servidor (sem a biblioteca `river`: o estado fica no banco, com progresso, interrupção e retomada; a fila externa só se justificar depois) → relatório CSV → **desfazer** (apaga o que foi criado e restaura o "antes" do que foi atualizado). Por padrão registros já existentes são ignorados; "atualizar existentes" é uma opção. Auditoria: um evento-resumo por importação (os registros ficam ligados à importação em `importacao_linhas`). Nova permissão `conf_importacao`; quem tem papel restrito ("apenas atribuídos") não importa. Sem CPF na importação (LGPD, 13.3). Fica para depois: 4.5 (mapeamento de colunas salvo), 4.6 (conector Geo Controle, depende do acesso que o usuário vai definir), 4.7 e 4.8.
**Entrega:** importar bairros e locais de uma planilha e do Geo Controle, com prévia e relatório. **✅ Etapa 4 concluída em 2026-10-29 (versões 1.0.016 e 1.0.017); o conector "Geo Controle (API)" fica pronto mas só vale quando o Geo Controle oferecer API (a exportação em planilha já funciona).** (Os demais tipos de dado ganham importadores nas etapas em que forem criados.)

## Etapa 5 — Ciclos, atividades e parâmetros
- [ ] 5.1 Ciclos (datas, situação, atividades, sincronização, painel público) e catálogo de atividades (LIT, LI, PE, T, DF, PVE, LIRAa, LIA, BRI)
- [ ] 5.2 Semana epidemiológica (ISO ou SINAN) e regra de ciclo fechado bloqueando edição
- [ ] 5.3 Parâmetros gerais, de mobilidade (app) e Leis Municipais com trechos
- [ ] 5.4 Importador de ciclos; guia e Novidades atualizados

## Etapa 6 — Visitas e laboratório (núcleo do negócio)
- [ ] 6.1 Visita completa: pendência, tipo, depósitos A1–E, amostra, tratamento L1/L2, adulticida, BRI, fotos, assinatura, anotações
- [ ] 6.2 Listagem com os 17 filtros, ficha da visita, edição e desativação lógica
- [ ] 6.3 Laboratório: amostras/tubitos (numeração automática ou digitada), resultados, status, permissões por ciclo fechado
- [ ] 6.4 **API de sincronização em lote** (`POST /sync`) para o tablet: idempotente, com resolução de conflito
- [ ] 6.5 Reincidência (12 meses) e histórico na ficha do local
- [ ] 6.6 Auditoria de visita/local e histórico de mobilidade (sincronismos)
- [ ] 6.7 Importador de visitas e laboratório

## Etapa 7 — Dashboard, B.I. e mapas
- [ ] 7.1 Dashboard por nível de acesso (cartões e gráficos do sistema atual)
- [ ] 7.2 B.I.: IIP, Breteau, cobertura LIT por bairro e ciclo (**validar as fórmulas com a área técnica**)
- [ ] 7.3 B.I. Epidemiológico (curva de incidência, faixa etária, bairros monitorados)
- [ ] 7.4 Mapas: visitas, doenças, rastro GPS, campanha, ovitrampa, EDL, com filtros

## Etapa 8 — Relatórios e PDFs
- [ ] 8.1 Motor de relatórios parametrizados (filtros, tela, PDF, planilha)
- [ ] 8.2 Os 14 relatórios do sistema atual, um a um (cada um validado contra um exemplo real)
- [ ] 8.3 Resumo semanal e Reconhecimento Geográfico

## Etapa 9 — Casos e fiscalização
- [ ] 9.1 Componente único de **ficha com linha do tempo, ações, encaminhamento e anexos**
- [ ] 9.2 Investigação (doenças, resultado, vincular local, entrevistas por doença configuráveis)
- [ ] 9.3 Denúncia (inclui cidadão e denúncia anônima)
- [ ] 9.4 Notificação → Auto de infração → pagamento/cancelamento (com leis municipais)
- [ ] 9.5 Solicitação de drone e BAAP (animais peçonhentos)

## Etapa 10 — Armadilhas, zoonose e LIRAa
- [ ] 10.1 Ovitrampa (contagem de ovos) e EDL (manutenção)
- [ ] 10.2 Campanha antirrábica: registros, vacinados e não vacinados
- [ ] 10.3 Diária de trabalho (equipe, temperaturas, defeitos, início e fim)
- [ ] 10.4 LVC
- [ ] 10.5 LIRAa: grupos e agenda

## Etapa 11 — Estoque e patrimônio
- [ ] 11.1 Unidades, produtos (fracionáveis), lotes e movimentação
- [ ] 11.2 Retiradas (campo e permanente), balanço físico
- [ ] 11.3 Patrimônio (atribuir, manutenção, baixa)
- [ ] 11.4 Integração com a campanha antirrábica (a diária consome vacina e seringa)

## Etapa 12 — Comunicação e integrações externas
- [ ] 12.1 Comunicações (WhatsApp e e-mail): fila, tentativas, reenvio, timeline, status de visualização
- [ ] 12.2 Provedor de WhatsApp e SMTP (a decidir; ver pendências)
- [ ] 12.3 SINAN / e-SUS / SUS: levantar formato oficial, exportar e/ou importar notificações
- [ ] 12.4 Banners (editor visual)

## Etapa 13 — Produção, segurança e LGPD
- [ ] 13.1 Deploy na VPS (Nginx, serviço systemd, Postgres, TLS) e script de atualização sem queda
- [ ] 13.2 Backups e restauração testada, por schema de cliente
- [ ] 13.3 LGPD: criptografia de CPF/telefone, registro de acesso a dados pessoais, retenção e exclusão
- [ ] 13.4 Observabilidade (logs, métricas, alertas) e testes de carga

## Etapa 14 — Aplicativo do tablet (Flutter) — **última etapa do projeto**
- [ ] 14.1 Autenticação, download de dados por ciclo (respeitando os parâmetros de "dias de download")
- [ ] 14.2 Visita offline completa, fotos e assinatura
- [ ] 14.3 GPS contínuo (rastro a cada 5 s) e sincronização em lote
- [ ] 14.4 Campanha, diária, ovitrampa, EDL, investigação e drone no app

---

## 2. Decisões ainda pendentes (precisam de você antes da etapa indicada)

| # | Pergunta | Quando decidir |
|---|---|---|
| 1 | ~~Captcha~~ **Decidido: Google reCAPTCHA v2.** Falta só criar as chaves (site key e secret key) no painel do reCAPTCHA, cadastrando os domínios de desenvolvimento e produção. A secret key fica só no servidor (`.env`), nunca no front. | Etapa 1 |
| 2 | ~~Nome, logotipo e cores~~ **Decidido: UYSI ARBO, identidade do uysilicitaproc.** ~~Domínio~~ **Decidido (2026-10-29): produção em `https://uysiarbo.inovarsistemas.biz`.** Falta decidir como o cliente (prefeitura) é identificado: subdomínio por cliente (ex.: `novamutum.uysiarbo.inovarsistemas.biz`, exige certificado curinga de 2º nível, que o `*.inovarsistemas.biz` atual NÃO cobre) ou escolha do cliente no login/caminho (ex.: `/c/novamutum`) | Etapa 1.3 |
| 3 | Provedor de geocodificação (ViaCEP + Nominatim/OpenStreetMap gratuitos, ou Google/Mapbox pagos, com melhor precisão) | Etapa 3 |
| 4 | Provedor de WhatsApp (API oficial da Meta, ou serviços como Z-API/Evolution) e de e-mail (SMTP) | Etapa 12 |
| 5 | Formato oficial de integração com SINAN/e-SUS (varia por município/estado) | Etapa 12 |
| 6 | Quais conectores de "outros sistemas" interessam primeiro, e se haverá acesso/exportação de exemplo | Etapa 4 |
| 7 | Fórmulas do IIP, Breteau e cobertura, e exemplos reais de PDFs a reproduzir | Etapas 7 e 8 |
| 8 | Modelo comercial: haverá plano/licença por cliente (limite de usuários, módulos por contrato)? | Etapa 1 ou 13 |

## 3. Como vamos trabalhar
- Uma **sub-etapa por vez**: eu proponho o que será feito, executo, testo e mostro; você aprova e seguimos.
- Mudança de escopo ou de decisão vira atualização deste documento antes de código.
- Nada de produção, commits ou push sem você pedir.

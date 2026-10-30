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
7. **Toda lista nova segue o padrão de tabelas** (MEMORIA.md, regra transversal 10): ícones, gaveta de filtros, colunas visíveis, ordenação pelo título, exportar PDF/planilha, reticências e paginação.

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
- [x] 5.1 Ciclos (datas, situação, atividades, sincronização, painel público) e catálogo de atividades (LIT, LI, PE, T, DF, PVE, LIRAa, LIA, BRI)
  - Feito em 2026-10-29 (versão 1.0.018): migração `cliente/00018` (`atividades` fixa com as 9 siglas, `ciclos` com exclusão lógica, `ciclo_atividades`, `ciclo_painel_atividades`, `ciclo_sincronizar`), tela `/ciclos` (permissão `param_ciclo`), aviso (não bloqueio) de períodos que se cruzam. O bloqueio por ciclo fechado e a semana epidemiológica ficam na 5.2.
- [x] 5.2 Semana epidemiológica (ISO ou SINAN) e regra de ciclo fechado bloqueando edição
  - Feito em 2026-10-29 (versão 1.0.019): pacote `internal/epi` (SINAN e ISO, testado em viradas de ano), parâmetro `semana_epidemiologica` em `cliente_info` (padrão SINAN, escolhido na tela Ciclos), semanas mostradas na lista e na ficha do ciclo, e as regras reutilizáveis `cicloAceitaEdicao` (ciclo fechado exige `visita_editar_ciclo_fechado`) e `cicloAceitaData` (período, ou `permite_fora_periodo`) em `internal/server/semana.go`, que as telas de visita da Etapa 6 vão chamar.
- [x] 5.3 Parâmetros gerais, de mobilidade (app) e Leis Municipais com trechos
  - Feito em 2026-10-29 (versão 1.0.020): registro tipado de parâmetros em `internal/server/parametros.go` (bool, int, CEP, senha; guardados em `cliente_info`), telas `/parametros/geral` e `/parametros/mobilidade`, migração `cliente/00019` (`leis_municipais` e `leis_trechos`, ambas com exclusão lógica e Lixeira), telas `/leis` com trechos, permissões `param_geral`, `param_mobilidade`, `param_leis`.
- [x] 5.4 Importador de ciclos; guia e Novidades atualizados
  - Feito em 2026-10-29 (versão 1.0.021): tipo `ciclos` no gerenciador de importação (planilha, conectores e API, com prévia, atualização e desfazer), migração `cliente/00020` (a restrição de tipo aceita `ciclos`). **Etapa 5 concluída.**

## Etapa 6 — Visitas e laboratório (núcleo do negócio)
- [x] 6.1 Visita completa: pendência, tipo, depósitos A1–E, amostra, tratamento L1/L2, adulticida, BRI, fotos, assinatura, anotações
  - Feito em 2026-10-29 (versão 1.0.022): migração `cliente/00021` (`visitas`, `visita_fotos`, `visita_anotacoes`, com `id_externo` para o tablet e exclusão lógica), telas `/visitas` (últimas visitas), `/visitas/nova?local=`, `/visitas/{id}` (ficha e edição), fotos (JPEG/PNG/WebP, 5 MB, 20 por visita), anotações internas, Lixeira, regras de ciclo fechado/período/pendência, semana calculada pela data, atalhos na ficha do local. A listagem com os 17 filtros, a desativação e a edição completa ficam na 6.2.
- [x] 6.2 Listagem com os 17 filtros, ficha da visita, edição e desativação lógica
  - Feito em 2026-10-30 (versão 1.0.027): `internal/server/visitas_lista.go` (`CountVisitas`/`ListVisitas` com os filtros, ao vivo por HTMX como em Locais), `POST /visitas/{id}/ativo` e `POST /visitas/lote` (desativar/reativar, permissão `visita_desativar`, auditoria `visita.desativar|reativar`), botão Imprimir (CSS `@media print`). Excluir (Lixeira) continua em `visita_desativar`.
- [x] 6.3 Laboratório: amostras/tubitos (numeração automática ou digitada), resultados, status, permissões por ciclo fechado
  - Feito em 2026-10-30 (versão 1.0.028): migração `cliente/00023` (`amostras`: tubito único entre as não excluídas, depósito, larvas, pupas, status em_analise|concluido, totais aegypti/albopictus/negativo, quem e quando registrou, exclusão lógica), `internal/server/laboratorio.go` (criar com numeração automática `ProximoNumeroTubito` ou digitada conforme `visita_tubito_automatico`, corrigir coleta, resultado com `visita_laboratorio_adicionar` e `visita_laboratorio_ciclo_fechado`, excluir para a Lixeira), seção "Amostras (tubitos)" na ficha da visita, tela `/laboratorio` com filtros ao vivo e menu Laboratório. Visita com tubito ativo não vai para a Lixeira.
- [x] 6.4 **API de sincronização em lote** (`POST /sync`) para o tablet: idempotente, com resolução de conflito (servidor vence; `GET /visitas` para baixar; histórico em `sincronismos`)
- [x] 6.5 Reincidência (12 meses) e histórico na ficha do local
  - Feito em 2026-10-30 (versão 1.0.030): `ListVisitasDoLocal` e `ResumoDoLocal12Meses` (`queries_cliente/visitas.sql`), `visitasDoLocal` em `internal/server/ficha_local.go`, seção "Visitas e reincidência" na ficha. Critério confirmado pelo usuário (2026-10-30): reincidente = 2 ou mais visitas com depósito em 12 meses (`reincidenciaMinima`). Também: filtros do Mapa ao vivo e menu Mapa como grupo.
- [x] 6.6 Auditoria de visita/local e histórico de mobilidade (sincronismos)
  - Feito em 2026-10-30 (versão 1.0.031): tela `/mobilidade/historico` (permissão `historico_mobilidade`, filtros ao vivo por aparelho, credencial, período no fuso do cliente e só problemas; `internal/server/mobilidade.go`, `ListSincronismos`), e seção "Histórico de alterações" na ficha da visita (`ListAuditoriaDaVisita`: visita + tubitos). A ficha do local já tinha o histórico (3.5). O histórico do GC (login, local novo etc.) não é reproduzido: só os envios de sincronização.
- [x] 6.3b (feito em 2026-10-30, versão 1.0.033: `CountAmostras`/`ListAmostras` com número, bairro, quarteirão, lado, ID da visita, atividade e semana; tela como a de Visitas) **Filtros do Laboratório como no Geo Controle** (pedido do usuário, 2026-10-30; referência: `/laboratorio` do GC): Endereço, Número, Bairro, Quarteirão (pesquisável, depende do bairro), Lado, ID da visita, Status, Ciclo, Agente, Atividade e Semana, mais o botão "Limpar filtros". Hoje a tela tem tubito, endereço, status, ciclo, agente e período. Fazer logo depois da 6.7.
- [x] 6.7 Importador de visitas e laboratório
  - Feito em 2026-10-30 (versão 1.0.032): tipos `visitas` e `amostras` no gerenciador de importação (`internal/server/importacao_visitas.go`, migração `cliente/00025`; reaproveita `validarVisita` com permissão interna `visita_agente_inativo`, só cria, ignora o que já existe, desfaz pela Lixeira; pela API, `tipo` visitas/amostras com o escopo `visitas:write`). Também: selo de tubitos em análise no menu Laboratório e menu lateral recolhível. **Etapa 6 concluída**, salvo os filtros do Laboratório (6.3b).

## Etapa 7 — Dashboard, B.I. e mapas
- [x] 7.1 Dashboard por nível de acesso (cartões e gráficos do sistema atual) — rota `/dashboard`, ciclo escolhível, escopo `visita_visualizar_minha` (versão 1.0.034)
- [x] 7.2 B.I.: IIP, Breteau, cobertura LIT por bairro e ciclo — rota `/bi` (`bi_visualizar`), fórmulas padrão PNCD (**ainda validar com a área técnica**) (versão 1.0.035)
- [ ] 7.3 B.I. Epidemiológico (curva de incidência, faixa etária, bairros monitorados)
- [ ] 7.4 Mapas: visitas, doenças, rastro GPS, campanha, ovitrampa, EDL, com filtros — **feitos: visitas (`/mapa`) e rastro GPS (`/mapa/rastro`, `mapa_coleta`, versão 1.0.036)**; doenças, campanha, ovitrampa e EDL entram com as etapas dos seus dados

## Etapa 8 — Relatórios e PDFs
- [x] 8.1 Motor de relatórios parametrizados (filtros, tela, PDF, planilha) — `internal/relatorio` (Doc: tela, CSV, XLSX, PDF com fpdf), `internal/server/relatorios.go` (`relatoriosDoSistema`), `/relatorios`; primeiro relatório: Visitas (versão 1.0.037)
- [x] 8.2 (parcial) Relatórios do sistema atual: **feitos 9 de 14** (Visitas, Local x visita, Produção, Antivetorial, Dia trabalhado, Laboratório totais, Animais, BRI, Locais incompletos; versão 1.0.038). **Faltam por depender de módulos futuros:** Campanha de vacinação, LIRAa, Investigação totais, Locais por criticidade, Pesquisa entomológica/Tratamento em papel. Cada um ainda precisa ser validado contra um exemplo real do GC
- [x] 8.3 Resumo semanal e Reconhecimento Geográfico — `/resumosemanal` (retrato gravado, imprimir) e `/reconhecimentos` (ficha por bairro e ciclo), ambos com Lixeira, auditoria e escopo "meu bairro" (versão 1.0.039). Menu: Dashboard, B.I. e B.I. Epidemiológico agora logo após Início

## Etapa 9 — Casos e fiscalização
- [x] 9.1 Componente único de **ficha com linha do tempo, ações, encaminhamento e anexos**
- [x] 9.2 Investigação (doenças, resultado, vincular local, entrevistas por doença configuráveis)
- [x] 9.3 Denúncia (inclui cidadão e denúncia anônima)
- [x] 9.4 Notificação → Auto de infração → pagamento/cancelamento (com leis municipais)
- [x] 9.5 Solicitação de drone e BAAP (animais peçonhentos) — concluída em 2026-10-30 (versão 1.0.052)

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

## 2.9 Parâmetros em abas e Conta Ovos (pedido do usuário, 2026-10-30)
Ver `MEMORIA.md`, seção "Pedidos anotados": conferir cada aba de `/parametros` do Geo Controle (Novo, Ver, Editar, Excluir de cada uma), ajustar as nossas e criar as telas do menu **Conta Ovos** com a API de https://contaovos.com/pt-br/api/.
- [ ] Comparar aba por aba (Leis, Semanas SINAN, Numeração, Mobilidade, Grupos, Doenças, Conta Ovos) e ajustar
- [ ] Ler a documentação da API Conta Ovos e definir as telas do menu Conta Ovos

## 3. Como vamos trabalhar
- Uma **sub-etapa por vez**: eu proponho o que será feito, executo, testo e mostro; você aprova e seguimos.
- Mudança de escopo ou de decisão vira atualização deste documento antes de código.
- Nada de produção, commits ou push sem você pedir.

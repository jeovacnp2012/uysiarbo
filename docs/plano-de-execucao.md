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
- [ ] 1.3 Resolução do cliente por requisição (subdomínio ou domínio) e `search_path` seguro por conexão
- [ ] 1.4 Usuários, papéis e permissões (modelo inspirado nas ~190 permissões e 24 categorias do sistema atual): permissões **granulares organizadas por blocos** e **perfis diversos** (papéis). **Usuário master: JEOVA OLIVEIRA, e-mail de acesso `jeovacnp2012@gmail.com`**: nenhuma regra de permissão, restrição de bloco, "apenas atribuídos" ou escopo de cliente se aplica a ele; vê e faz tudo, em todos os clientes. O master é protegido (não pode ser excluído, rebaixado ou ter o acesso reduzido por outro usuário); a senha dele nunca é registrada em arquivo, plano ou código.
- [ ] 1.5 Login com Google reCAPTCHA v2 (verificação do token no servidor), sessão em cookie seguro, bloqueio por tentativas, recuperação de senha
- [ ] 1.5.1 **Reset de senha por administrador**: ao resetar a senha de alguém, o sistema gera uma senha **forte** e a **mostra uma única vez** a quem resetou (com botão copiar). No próximo login do usuário com essa senha temporária, o sistema o obriga a **escolher a própria senha** antes de continuar (a temporária deixa de valer). Registrar o reset na auditoria (quem resetou, para quem, quando), sem guardar a senha
- [ ] 1.5.2 **Revelar senha**: todo campo de senha da interface (login, troca, criação, reset) tem a opção de mostrar/ocultar o que foi digitado
- [ ] 1.6 Telas: Usuários, Papéis (editor de permissões por bloco/categoria, com busca e "marcar todas")
- [ ] 1.7 Auditoria básica (quem fez o quê, antes/depois)
- [ ] 1.8 **Guia do sistema** (esqueleto, com busca) e **Novidades/Versões** (cadastro + tela + aviso de versão nova)
**Entrega:** entrar como administrador de uma prefeitura de teste, criar usuários e papéis, ver Guia e Novidades.

## Etapa 2 — API com credenciais por cliente
Objetivo: leitura e gravação por sistemas externos, de forma segura.
- [ ] 2.1 Cadastro de credenciais de API por cliente (`client_id`, segredo guardado só como hash, status, validade, rotação)
- [ ] 2.2 Emissão de token (OAuth2 *client credentials*, JWT de vida curta) e escopos `recurso:read` / `recurso:write`
- [ ] 2.3 Middleware: identifica o cliente pelo token, aplica o schema, confere escopo, limita taxa de uso
- [ ] 2.4 Log de chamadas (quem, o quê, resultado) e tela de consumo por credencial
- [ ] 2.5 Contrato OpenAPI, documentação publicada e SDK de exemplo
- [ ] 2.6 Padrões de gravação: `Idempotency-Key`, IDs gerados no cliente, erros padronizados, paginação por cursor
- [ ] 2.7 Tela de gestão de credenciais e escopos
**Entrega:** um cliente de teste lê e grava um recurso simples via API, com credencial própria.

## Etapa 3 — Território: cidades, bairros, quarteirões e locais
- [ ] 3.1 Cidade, Bairro (todos os campos do sistema atual), Quarteirão, responsáveis por bairro
- [ ] 3.2 Geometrias PostGIS: polígonos de bairro/quarteirão e pontos de local
- [ ] 3.3 Local (cadastro completo: moradores, habitantes, tipo, endereço, referência, lado, sequência, complemento)
- [ ] 3.4 Cadastro em lote por sequência/complemento, desativação lógica, QR/chave do local
- [ ] 3.5 Ficha do local (seções vazias que os módulos seguintes vão preencher) e Grupos de Locais (clonar, exportar)
- [ ] 3.6 Mapa base (MapLibre): ver locais, bairros, quarteirões
- [ ] 3.7 Geocodificação por endereço/CEP (provedor a decidir)
- [ ] 3.8 Controle de acesso por bairro/quarteirão/local ("apenas atribuídos")
- [ ] 3.9 API: leitura e gravação de bairros, quarteirões e locais
**Entrega:** cadastrar e consultar a base territorial inteira de uma prefeitura.

## Etapa 4 — Gerenciador de importação
Objetivo: migrar dados de outros sistemas e de planilhas, com segurança.
- [ ] 4.1 Modelo de "importação": origem, tipo de dado, arquivo, status, responsável, log de erros
- [ ] 4.2 Importador de planilhas CSV/Excel com **modelos para baixar** (usuários, bairros, quarteirões, locais)
- [ ] 4.3 Etapas de segurança: validação linha a linha, **prévia** (o que será criado/atualizado/ignorado), confirmar, desfazer
- [ ] 4.4 Execução em segundo plano (fila `river`) com barra de progresso e relatório final baixável
- [ ] 4.5 Mapeamento de colunas (origem → destino) salvo por conector
- [ ] 4.6 Conector "Geo Controle (API)": credenciais da origem, descoberta do que dá para importar, mapeamento automático
- [ ] 4.7 Estrutura para conectores de outros sistemas (interface única, cada conector em um pacote)
- [ ] 4.8 Importação via API (outro sistema empurra dados)
**Entrega:** importar bairros e locais de uma planilha e do Geo Controle, com prévia e relatório. (Os demais tipos de dado ganham importadores nas etapas em que forem criados.)

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

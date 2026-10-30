titulo: API para sistemas externos
resumo: Como criar credenciais e usar a API (token, escopos, limites, erros e exemplos).
ordem: 10
permissao: conf_api
palavras: api, integração, token, credencial, client_id, segredo, oauth, bearer, escopo, openapi, idempotência, limite
---
# API e integrações

A API permite que **outros sistemas** leiam e gravem dados deste ambiente, com segurança. Cada cliente (prefeitura) tem a sua, no próprio endereço: `https://<endereço-do-ambiente>/api/v1/...`. A gestão fica em **Administração → API e integrações**.

## 1. Criar uma credencial
1. **Nova credencial**: dê um nome (quem vai usar), marque os **escopos** (ler e/ou gravar cada recurso) e, se quiser, a **validade** e o **limite de chamadas por minuto** (padrão 120).
2. Ao salvar, a tela mostra o **client_id** e o **segredo** **uma única vez**. Copie o segredo na hora: ele não fica guardado em lugar nenhum. Se for perdido, use **Gerar novo segredo**.

## 2. Pegar um token e chamar
```bash
curl -s -X POST https://<endereço>/api/v1/token \
  -d grant_type=client_credentials -d client_id=uac_... -d client_secret=uas_...

curl -s https://<endereço>/api/v1/cidades -H "Authorization: Bearer <token>"
```
O token vale **15 minutos**; peça outro quando vencer. Também dá para enviar `client_id` e `client_secret` por HTTP Basic, e pedir só alguns escopos com `scope=cidades:read`.

## Escopos
Têm a forma `recurso:read` e `recurso:write`; **gravar não dá direito de ler**. A API expõe **Cidades**, **Bairros**, **Quarteirões** e **Locais**, cada um com os escopos `...:read` e `...:write` (por exemplo, `locais:read`).

## Território: bairros, quarteirões e locais
Cada recurso tem listar (`GET /api/v1/locais`), buscar (`GET /api/v1/locais/{id}`), criar (`POST`) e alterar (`PATCH`, só os campos enviados; `null` limpa um campo opcional). Os campos estão no contrato OpenAPI. Algumas regras:

- **A API não apaga nada.** Para tirar um local ou quarteirão de uso, grave `"ativo": false`; ele continua nas listas, marcado como inativo. Registros **excluídos** no sistema (que vão para a Lixeira) somem das listas comuns e da busca por id; na **sincronização** (`updated_since`) eles aparecem com o campo **`excluido_em`** preenchido, para o aplicativo removê-los do tablet.
- **As mesmas regras das telas valem na API**: endereço sem repetição no quarteirão, quarteirão do bairro escolhido, latitude e longitude juntas, etc. Quem erra recebe `422 validation_error` com a explicação, ou `409 conflict` para repetição.
- **O CPF do proprietário não passa pela API**: nem sai nas leituras, nem entra nas gravações; o que já está cadastrado é preservado quando o local é alterado.
- **O que a API altera aparece na Auditoria**, com o nome da credencial.

### Identificador do aplicativo (`id_externo`) e sincronização
Aplicativos que criam registros **sem internet** (como o do tablet) geram o próprio identificador e o enviam em **`id_externo`** (até 64 caracteres, único por tipo de registro):

- Se a criação for **reenviada** (a resposta se perdeu, o sinal caiu), a API responde `200` com o registro que **já existe**, em vez de duplicar.
- O `id_externo` pode ser definido uma vez e não muda depois. Dá para buscar por ele: `GET /api/v1/locais?id_externo=...`.
- Para **sincronizar**, peça só o que mudou: `GET /api/v1/locais?updated_since=2026-10-29T12:00:00Z` (data e hora em RFC 3339), percorrendo as páginas pelo `next_cursor`. Guarde o horário do início da última sincronização e use-o na próxima.

## Importar dados por API
Um sistema externo pode **empurrar** bairros, quarteirões e locais pelo mesmo caminho da tela de Importação de dados, com prévia e confirmação:
1. `POST /api/v1/importacoes` com `{"tipo": "locais", "atualizar_existentes": false, "linhas": [ {...}, {...} ]}`: cada linha é um objeto com as colunas do modelo de importação (`bairro`, `quarteirao`, `tipo`, `endereco`, `numero`, `lado`, `id_externo`...). **Nada é gravado**: a resposta traz a **prévia** (quantas linhas seriam criadas, atualizadas, ignoradas, com erro).
2. `GET /api/v1/importacoes/{id}/linhas?acao=erro` mostra o motivo de cada linha com erro; corrija e envie de novo, ou siga com as boas (as linhas com erro não entram).
3. `POST /api/v1/importacoes/{id}/executar` inicia a gravação em segundo plano (resposta 202); acompanhe em `GET /api/v1/importacoes/{id}` (andamento e resultado). Só uma importação roda por vez; se houver outra, a resposta é 409 com `Retry-After`.
4. `POST /api/v1/importacoes/{id}/descartar` cancela uma prévia que ainda não foi executada.

A credencial precisa do escopo **`importacoes:write`** (e **`importacoes:read`** para acompanhar) **e** do escopo de gravação do tipo (por exemplo, `locais:write`). Importe na ordem **bairros → quarteirões → locais**. O que a API importa aparece na tela Importação de dados, com o relatório e a opção de desfazer, e na Auditoria com o nome da credencial.

## Sincronizar visitas (tablet)
- **Enviar**: `POST /api/v1/sync` com `{"dispositivo": "...", "visitas": [ {...} ]}` (até 100 visitas por lote, escopo `visitas:write`). Cada visita leva o seu **`id_externo`** e, se houver coleta, as `amostras` (tubitos), cada uma com `id_externo`. A atividade vai pela sigla (`LIT`, `LI`, `PE`...), o local por `local_id` ou `local_id_externo`.
- **Resposta**: um resultado por visita: `criada`, `existente` (já tinha, reenvio seguro), `conflito` (o servidor já tem a visita com dados diferentes: **o servidor vence**, nada é alterado, e `campos_divergentes` mostra quais) ou `rejeitada` (com os motivos em `erros`: ciclo fechado, pendência, depósito fora da faixa etc.). Uma visita com erro não impede as outras do lote.
- **Tubitos**: o número é gerado pelo servidor (ou digitado, conforme o parâmetro); a semana epidemiológica também é calculada pelo servidor.
- **Baixar**: `GET /api/v1/visitas` (escopo `visitas:read`) com `ciclo_id`, `local_id`, `agente_id`, `id_externo` e `updated_since`, por cursor; traz as amostras junto.
- Cada envio fica registrado no histórico de sincronismos (quando, de qual aparelho, quantas visitas).

## Regras que valem para todas as chamadas
- **Erros** sempre no mesmo formato: `{"error": {"code": "...", "message": "...", "request_id": "..."}}`. Códigos comuns: `invalid_token` (401), `insufficient_scope` (403), `not_found` (404), `conflict` (409), `validation_error` (422), `rate_limited` (429).
- **Limite de uso**: ao passar do limite por minuto, a API responde `429` com `Retry-After` (segundos); os cabeçalhos `X-RateLimit-Limit` e `X-RateLimit-Remaining` mostram o saldo.
- **Listas**: `limit` (até 200) e `cursor` (use o `next_cursor` da resposta anterior; vazio na última página).
- **Gravações repetíveis**: envie `Idempotency-Key` (até 64 caracteres). Repetir o mesmo pedido com a mesma chave devolve a resposta anterior (`Idempotent-Replayed: true`) por 24 horas, sem duplicar; a mesma chave com outro conteúdo é recusada (422).

## Segurança
- **Gerar novo segredo** ou **Desativar** derruba **na hora** os tokens já emitidos.
- Cada chamada fica registrada: na ficha da credencial há o **consumo** (chamadas, erros, tempo médio) e as últimas chamadas. As alterações feitas pela API aparecem na **Auditoria** com o nome da credencial como autora.
- Guarde o segredo como uma senha: nunca no código-fonte nem em e-mail.

## Documentação técnica
O contrato completo (OpenAPI 3) está em **/api/v1/openapi.yaml** (público) e pode ser aberto em ferramentas como Swagger Editor ou Postman. Exemplos prontos em `docs/api/exemplo.sh` e `docs/api/exemplo.py`, no repositório do sistema.

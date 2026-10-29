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
Têm a forma `recurso:read` e `recurso:write`; **gravar não dá direito de ler**. Hoje a API expõe o recurso **Cidades** (`cidades:read`, `cidades:write`); bairros, quarteirões e locais chegam nas próximas versões.

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

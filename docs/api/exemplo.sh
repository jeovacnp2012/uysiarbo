#!/usr/bin/env bash
# Exemplo de uso da API do UYSI ARBO (precisa de curl e jq).
# Crie a credencial em Administração -> API e integrações e rode:
#   BASE=https://uysiarbonm.inovarsistemas.biz CLIENT_ID=uac_... CLIENT_SECRET=uas_... ./exemplo.sh
set -euo pipefail
: "${BASE:?defina BASE, por exemplo https://uysiarbonm.inovarsistemas.biz}" "${CLIENT_ID:?}" "${CLIENT_SECRET:?}"

# 1) troca o segredo por um token (vale 15 minutos)
TOKEN=$(curl -fsS -X POST "$BASE/api/v1/token" \
  -d grant_type=client_credentials -d client_id="$CLIENT_ID" -d client_secret="$CLIENT_SECRET" | jq -r .access_token)

# 2) cria uma cidade; repetir o pedido com a mesma Idempotency-Key não duplica
curl -fsS -X POST "$BASE/api/v1/cidades" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -H "Idempotency-Key: exemplo-$(date +%Y%m%d)" \
  -d '{"nome":"Nova Mutum","uf":"MT","ibge":"5106240"}' | jq .

# 3) lista as cidades, 50 por vez, seguindo o next_cursor
CURSOR=""
while :; do
  PAGINA=$(curl -fsS "$BASE/api/v1/cidades?limit=50${CURSOR:+&cursor=$CURSOR}" -H "Authorization: Bearer $TOKEN")
  echo "$PAGINA" | jq -r '.data[] | "\(.id)\t\(.nome)/\(.uf)"'
  CURSOR=$(echo "$PAGINA" | jq -r '.next_cursor // empty')
  [ -n "$CURSOR" ] || break
done

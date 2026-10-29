"""Exemplo de cliente da API do UYSI ARBO, só com a biblioteca padrão do Python.

Uso:
    BASE=https://uysiarbonm.inovarsistemas.biz CLIENT_ID=uac_... CLIENT_SECRET=uas_... python exemplo.py
"""
import json
import os
import time
import urllib.error
import urllib.parse
import urllib.request


class ErroDaAPI(Exception):
    def __init__(self, status, corpo):
        super().__init__(f"HTTP {status}: {corpo}")
        self.status, self.corpo = status, corpo


class ClienteUysi:
    def __init__(self, base, client_id, client_secret):
        self.base, self.client_id, self.client_secret = base.rstrip("/"), client_id, client_secret
        self._token, self._vence = None, 0

    def _pedir(self, metodo, caminho, corpo=None, cabecalhos=None, form=False):
        dados = None
        cab = dict(cabecalhos or {})
        if corpo is not None:
            dados = urllib.parse.urlencode(corpo).encode() if form else json.dumps(corpo).encode()
            cab["Content-Type"] = "application/x-www-form-urlencoded" if form else "application/json"
        req = urllib.request.Request(self.base + caminho, data=dados, method=metodo, headers=cab)
        try:
            with urllib.request.urlopen(req, timeout=15) as r:
                return json.loads(r.read() or b"null")
        except urllib.error.HTTPError as e:
            raise ErroDaAPI(e.code, e.read().decode())

    def token(self):
        """Devolve um token válido, pedindo outro um minuto antes de vencer."""
        if not self._token or time.time() > self._vence - 60:
            r = self._pedir("POST", "/api/v1/token", {
                "grant_type": "client_credentials", "client_id": self.client_id, "client_secret": self.client_secret,
            }, form=True)
            self._token, self._vence = r["access_token"], time.time() + r["expires_in"]
        return self._token

    def chamar(self, metodo, caminho, corpo=None, chave_idempotencia=None):
        cab = {"Authorization": "Bearer " + self.token()}
        if chave_idempotencia:
            cab["Idempotency-Key"] = chave_idempotencia
        return self._pedir(metodo, caminho, corpo, cab)

    def cidades(self):
        """Percorre todas as cidades, página por página (cursor)."""
        cursor = None
        while True:
            q = "?limit=50" + (f"&cursor={urllib.parse.quote(cursor)}" if cursor else "")
            pagina = self.chamar("GET", "/api/v1/cidades" + q)
            yield from pagina["data"]
            cursor = pagina["next_cursor"]
            if not cursor:
                return


if __name__ == "__main__":
    api = ClienteUysi(os.environ["BASE"], os.environ["CLIENT_ID"], os.environ["CLIENT_SECRET"])
    try:
        print(api.chamar("POST", "/api/v1/cidades", {"nome": "Nova Mutum", "uf": "MT"}, chave_idempotencia="exemplo-py-1"))
    except ErroDaAPI as e:
        print("não criou:", e)
    for c in api.cidades():
        print(c["id"], c["nome"], c["uf"])

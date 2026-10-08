#!/usr/bin/env python3
# Mock Efí Pix (descartável): oauth + cob PUT/GET com status controlável.
import json
import re
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

STATE_FILE = "/tmp/mock-efi-status.txt"  # ATIVA ou CONCLUIDA
COBS = {}


def current_status():
    try:
        return open(STATE_FILE).read().strip() or "ATIVA"
    except OSError:
        return "ATIVA"


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def _json(self, obj, code=200):
        body = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):
        if self.path == "/oauth/token":
            return self._json({"access_token": "mock-tok", "expires_in": 3600})
        self.send_response(404)
        self.end_headers()

    def do_PUT(self):
        m = re.match(r"^/v2/cob/([A-Za-z0-9]+)$", self.path)
        if not m:
            self.send_response(404)
            self.end_headers()
            return
        length = int(self.headers.get("Content-Length", 0))
        payload = json.loads(self.rfile.read(length) or b"{}")
        txid = m.group(1)
        COBS[txid] = payload
        with open("/tmp/mock-efi-charges.log", "a") as f:
            f.write(json.dumps({"txid": txid, "valor": payload.get("valor"), "chave": payload.get("chave")}) + "\n")
        self._json({"txid": txid, "pixCopiaECola": f"000201MOCK{txid}",
                    "calendario": {"expiracao": 86400}, "status": "ATIVA"})

    def do_GET(self):
        m = re.match(r"^/v2/cob/([A-Za-z0-9]+)$", self.path)
        if not m or m.group(1) not in COBS:
            self.send_response(404)
            self.end_headers()
            return
        self._json({"txid": m.group(1), "status": current_status()})


HTTPServer(("127.0.0.1", 3591), H).serve_forever()

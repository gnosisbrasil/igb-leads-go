#!/usr/bin/env python3
# Mock meow + SMTP sink (descartável).
import json
import socketserver
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_POST(self):
        if self.path != "/api/messages/send-text":
            self.send_response(404)
            self.end_headers()
            return
        if self.headers.get("X-API-Key") != "test-key":
            self.send_response(401)
            self.end_headers()
            return
        length = int(self.headers.get("Content-Length", 0))
        payload = json.loads(self.rfile.read(length) or b"{}")
        with open("/tmp/mock-meow.log", "a") as f:
            f.write(json.dumps(payload, ensure_ascii=False) + "\n")
        body = json.dumps({"success": True}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


class SMTPHandler(socketserver.StreamRequestHandler):
    def handle(self):
        def w(s):
            self.wfile.write((s + "\r\n").encode())

        def rd():
            return self.rfile.readline().decode().strip()

        w("220 mock smtp")
        while True:
            line = rd()
            if not line:
                return
            u = line.upper()
            if u.startswith("EHLO") or u.startswith("HELO"):
                w("250-mock")
                w("250 AUTH PLAIN")
            elif u.startswith("AUTH"):
                w("235 ok")
            elif u.startswith("MAIL") or u.startswith("RCPT"):
                w("250 ok")
            elif u.startswith("DATA"):
                w("354 end with .")
                data = []
                while True:
                    dl = self.rfile.readline().decode()
                    if dl.strip() == ".":
                        break
                    data.append(dl)
                with open("/tmp/mock-smtp.log", "a") as f:
                    f.write("".join(data) + "\n====\n")
                w("250 ok")
            elif u.startswith("QUIT"):
                w("221 bye")
                return
            elif u.startswith("RSET") or u.startswith("NOOP"):
                w("250 ok")
            else:
                w("250 ok")


threading.Thread(target=lambda: HTTPServer(("127.0.0.1", 3592), H).serve_forever(), daemon=True).start()
socketserver.TCPServer.allow_reuse_address = True
socketserver.TCPServer(("127.0.0.1", 3525), SMTPHandler).serve_forever()

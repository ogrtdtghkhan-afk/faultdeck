"""Independent echo API used only by the Docker deployment acceptance test."""

import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit


class EchoHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def do_GET(self):
        self.respond()

    def do_POST(self):
        self.respond()

    def respond(self):
        target = urlsplit(self.path)
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        payload = {
            "service": "independent-deployment-upstream",
            "method": self.command,
            "path": target.path,
            "query": target.query,
            "body": body.decode("utf-8"),
            "authorization": self.headers.get("Authorization"),
            "proxyTokenPresent": "X-FaultDeck-Token" in self.headers,
            "contentType": self.headers.get("Content-Type"),
        }
        encoded = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, format, *args):
        pass


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8080), EchoHandler).serve_forever()

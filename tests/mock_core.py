#!/usr/bin/env python3
"""Minimal mock server for KWS_Core. Responds to /getStatusHost and other paths."""

import json
from http.server import HTTPServer, BaseHTTPRequestHandler

MEMORY_RESP = {
    "information": {
        "total_gb": 64,
        "used_gb": 16,
        "available_gb": 48,
        "used_percent": 25.0,
    },
    "message": "Host Status Return operation success",
}

DISK_RESP = {
    "information": {
        "total_gb": 500,
        "used_gb": 100,
        "free_gb": 400,
        "used_percent": 20.0,
    },
    "message": "Host Status Return operation success",
}

CPU_RESP = {
    "information": {
        "system_time": 100.0,
        "idle_time": 5000.0,
        "usage_percent": 5.0,
    },
    "message": "Host Status Return operation success",
}

GENERIC_RESP = {"message": "mock ok"}

# CMS mock response for POST /New/Instance
CMS_RESP = {
    "ip": "10.0.1.100",
    "macAddr": "52:54:00:aa:bb:cc",
    "sdnUUID": "mock-sdn-uuid-0001",
}

# VM status mock for GET /getStatusUUID
VM_STATUS_RESP = {
    "information": {
        "system_time": 50.0,
        "idle_time": 3000.0,
        "usage_percent": 3.5,
    },
    "message": "domain Status UUID operation success",
}


class Handler(BaseHTTPRequestHandler):
    def _read_body(self):
        length = int(self.headers.get("Content-Length", 0))
        if length:
            return json.loads(self.rfile.read(length))
        return {}

    def _respond(self, data, code=200):
        body = json.dumps(data).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/getStatusHost":
            body = self._read_body()
            dt = body.get("host_dataType", -1)
            if dt == 0:
                return self._respond(CPU_RESP)
            elif dt == 1:
                return self._respond(MEMORY_RESP)
            elif dt == 2:
                return self._respond(DISK_RESP)
            else:
                return self._respond(MEMORY_RESP)
        elif self.path == "/getStatusUUID":
            return self._respond(VM_STATUS_RESP)
        self._respond(GENERIC_RESP)

    def do_POST(self):
        self._read_body()
        if self.path == "/New/Instance":
            return self._respond(CMS_RESP)
        self._respond(GENERIC_RESP)

    def do_DELETE(self):
        self._read_body()
        self._respond(GENERIC_RESP)

    def log_message(self, fmt, *args):
        print(f"[mock-core] {fmt % args}")


if __name__ == "__main__":
    server = HTTPServer(("0.0.0.0", 8080), Handler)
    print("[mock-core] Listening on :8080")
    server.serve_forever()

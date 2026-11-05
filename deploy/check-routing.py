#!/usr/bin/env python3
"""Exercise the production Caddy matchers against two local fake upstreams.

Usage: python3 deploy/check-routing.py /path/to/caddy
Only loopback ephemeral ports are used; this never requests certificates.
"""

import http.server
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request


def upstream(name):
    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            size = int(self.headers.get("Content-Length", "0"))
            body = self.rfile.read(size).decode()
            result = json.dumps({"upstream": name, "method": self.command,
                                 "path": self.path, "body": body}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(result)))
            self.end_headers()
            self.wfile.write(result)

        do_POST = do_GET

        def log_message(self, *_):
            pass

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server


def replace_upstreams(value, replacements):
    if isinstance(value, dict):
        for key, item in value.items():
            if key == "dial" and item in replacements:
                value[key] = replacements[item]
            else:
                replace_upstreams(item, replacements)
    elif isinstance(value, list):
        for item in value:
            replace_upstreams(item, replacements)


def main():
    caddy = sys.argv[1] if len(sys.argv) > 1 else "caddy"
    config = Path(__file__).with_name("Caddyfile")
    environment = dict(os.environ, DOMAIN="routing.example", OLD_DOMAINS="")
    adapted = subprocess.run([caddy, "adapt", "--config", str(config),
                              "--adapter", "caddyfile"], check=True,
                             capture_output=True, text=True, env=environment)
    value = json.loads(adapted.stdout)
    main_app, lab_app = upstream("main"), upstream("lab")
    replace_upstreams(value, {"gscouter:8080": "127.0.0.1:%d" % main_app.server_port,
                              "rotation-lab:8080": "127.0.0.1:%d" % lab_app.server_port})
    with socket.socket() as reservation:
        reservation.bind(("127.0.0.1", 0))
        port = reservation.getsockname()[1]
    for server in value["apps"]["http"]["servers"].values():
        server["listen"] = ["127.0.0.1:%d" % port]
        server["automatic_https"] = {"disable": True}
        server.pop("tls_connection_policies", None)
    value["apps"].pop("tls", None)
    value["admin"] = {"disabled": True}
    try:
        with tempfile.TemporaryDirectory(prefix="rotation-lab-route-") as directory:
            local = Path(directory) / "caddy.json"
            local.write_text(json.dumps(value))
            with (Path(directory) / "caddy.log").open("w+") as logs:
                process = subprocess.Popen([caddy, "run", "--config", str(local)],
                                           stdout=logs, stderr=logs)
                try:
                    for _ in range(200):
                        if process.poll() is not None:
                            logs.seek(0)
                            raise RuntimeError(logs.read())
                        try:
                            with socket.create_connection(("127.0.0.1", port), .1):
                                break
                        except OSError:
                            time.sleep(.025)
                    else:
                        raise RuntimeError("Caddy did not listen")
                    cases = [
                        ("GET", "/lab", "lab"),
                        ("GET", "/lab/", "lab"),
                        ("GET", "/lab/assets/gso.wasm", "lab"),
                        ("GET", "/lab/assets/lab-worker.js", "lab"),
                        ("GET", "/lab/profile?uid=800000001", "lab"),
                        ("POST", "/lab/input", "lab"),
                        ("GET", "/laboratory", "main"),
                        ("GET", "/labyrinth", "main"),
                        ("GET", "/score?a=fixture", "main"),
                        ("GET", "/static/worker.js", "main"),
                        ("GET", "/favicon.ico", "main"),
                        ("GET", "/healthz", "main"),
                        ("GET", "/", "main"),
                    ]
                    for method, path, expected in cases:
                        body = b'{"builds":[]}' if method == "POST" else None
                        request = urllib.request.Request(
                            "http://127.0.0.1:%d%s" % (port, path), data=body,
                            headers={"Host": "routing.example"}, method=method)
                        with urllib.request.urlopen(request, timeout=3) as response:
                            result = json.load(response)
                        assert result["upstream"] == expected, (path, result)
                        assert result["path"] == path, (path, result)
                        assert result["method"] == method, (path, result)
                        if body is not None:
                            assert result["body"] == body.decode(), result
                    print("Caddy routing passed: %d exact, child, POST and main-fallback requests" % len(cases))
                finally:
                    process.terminate()
                    process.wait(timeout=5)
    finally:
        for server in (main_app, lab_app):
            server.shutdown()
            server.server_close()


if __name__ == "__main__":
    main()

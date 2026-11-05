#!/usr/bin/env python3
"""Check the running release container before publishing its public image."""
import hashlib
import json
from pathlib import Path
import sys
import time
import urllib.error
import urllib.request

base = sys.argv[1].rstrip("/")
assets = Path(sys.argv[2])
manifest = json.loads((assets / "manifest.json").read_text())


def get(route):
    with urllib.request.urlopen(base + route, timeout=10) as response:
        return response.read(), response.headers


for attempt in range(30):
    try:
        page, headers = get("/lab")
        break
    except (urllib.error.URLError, TimeoutError):
        if attempt == 29:
            raise
        time.sleep(1)

assert b'id="lab-start"' in page
assert headers.get("X-Robots-Tag") == "noindex, nofollow"
served, _ = get("/lab/assets/manifest.json")
assert json.loads(served) == manifest
for name, field in [("gso.wasm", "wasm_sha256"), ("dictionary.json", "dictionary_sha256")]:
    body, headers = get("/lab/assets/" + name)
    assert hashlib.sha256(body).hexdigest() == manifest[field], name
    if name.endswith(".wasm"):
        assert headers.get_content_type() == "application/wasm"

for name in ("lab.js", "lab-worker.js"):
    body, headers = get("/lab/assets/" + name)
    assert body and "javascript" in headers.get_content_type()

# TestRotationLabRealBuildInput separately checks successful four-build input
# preparation. Here ensure the packaged server registered the route and rejects
# malformed input without starting any simulation or making an Enka request.
request = urllib.request.Request(
    base + "/lab/input",
    data=b'{"builds":[]}',
    headers={"Content-Type": "application/json"},
    method="POST",
)
try:
    urllib.request.urlopen(request, timeout=10)
    raise AssertionError("malformed lab input was accepted")
except urllib.error.HTTPError as error:
    assert error.code == 400, error.code
    assert json.loads(error.read())["error"]

print(json.dumps({"http": "passed", "assets": "hashes match", "engine": manifest["engine"]["commit"]}))

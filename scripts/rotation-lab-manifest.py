#!/usr/bin/env python3
"""Build dictionary inputs and record the exact browser asset provenance."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tomllib


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args])


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def provenance(root, excluded):
    names = git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").split(b"\0")
    digest = hashlib.sha256()
    count = 0
    for raw in sorted(set(names)):
        if not raw:
            continue
        name = raw.decode()
        path = root / name
        if name.split("/")[0] in excluded or not path.is_file():
            continue
        digest.update(raw + b"\0" + bytes.fromhex(sha(path)))
        count += 1
    return {
        "commit": git(root, "rev-parse", "HEAD").decode().strip(),
        "dirty": bool(git(root, "status", "--porcelain")),
        "source_sha256": digest.hexdigest(),
        "source_files": count,
    }


def main():
    command = sys.argv[1]
    site = Path(sys.argv[2]).resolve()
    if command == "list":
        dest = Path(sys.argv[3])
        # Exactly one directory level: never alternative or held-out rotations.
        paths = sorted((site / "teams").glob("*/rotation.gcsl"))
        if not paths:
            raise SystemExit("No main-library rotations found")
        dest.write_text("".join(f"{p.parent.name}\t{p}\n" for p in paths))
    elif command == "manifest":
        optimizer = Path(sys.argv[3]).resolve()
        assets = Path(sys.argv[4])
        browser = tomllib.loads((optimizer / "browser/Cargo.toml").read_text())
        engine = browser["dependencies"]["gse"]["rev"]
        if len(engine) != 40 or any(c not in "0123456789abcdef" for c in engine):
            raise SystemExit("Browser engine must have an immutable 40-character Git revision")
        manifest = {
            "format": 1,
            "protocol": "gso-browser-v1",
            "engine": {"commit": engine},
            "optimizer": provenance(optimizer, {"target", "runs", "reports", ".git"}),
            "site": provenance(site, {"lab-static", ".git", ".lab-qa"}),
            "wasm": "gso.wasm",
            "dictionary": "dictionary.json",
            "wasm_sha256": sha(assets / "gso.wasm"),
            "dictionary_sha256": sha(assets / "dictionary.json"),
            "scoring": {
                "duration": 450, "warmup": 150, "swap_delay": 12,
                "enemy_level": 100, "resistance": 0.1, "radius": 2,
                "position": [0, 2.4], "periodic_energy": True,
            },
        }
        (assets / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    else:
        raise SystemExit("Expected list or manifest")


if __name__ == "__main__":
    main()

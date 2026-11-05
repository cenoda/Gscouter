#!/usr/bin/env bash
# Build browser inference assets without modifying either repository's sources.
set -euo pipefail

site_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
optimizer=${1:-"$site_root/../gscouter-optimizer-browser"}
output=${2:-"$site_root/lab-static"}
optimizer=$(cd "$optimizer" && pwd)
mkdir -p "$output"
output=$(cd "$output" && pwd)
build_dir=${GSCOUTER_LAB_BUILD_DIR:-"$optimizer/target/rotation-lab"}
mkdir -p "$build_dir"
build_dir=$(cd "$build_dir" && pwd)
stage=$(mktemp -d "$output/.build.XXXXXX")
trap 'rm -rf "$stage"' EXIT

for executable in cargo python3 git; do
  command -v "$executable" >/dev/null || { printf 'Missing dependency: %s\n' "$executable" >&2; exit 1; }
done
test -f "$optimizer/browser/Cargo.toml" || { printf 'Optimizer checkout needs browser/Cargo.toml: %s\n' "$optimizer" >&2; exit 1; }

# The historical native tool only extracts the block dictionary; it runs no
# simulations. The browser crate separately pins the new engine for all fights.
cargo build --release --locked --manifest-path "$optimizer/Cargo.toml" --target-dir "$build_dir/native"
python3 "$site_root/scripts/rotation-lab-manifest.py" list "$site_root" "$stage/rotations.tsv"
site_commit=$(git -C "$site_root" rev-parse HEAD)
"$build_dir/native/release/gso" dictionary build --list "$stage/rotations.tsv" \
  --source "Gscouter@$site_commit:teams/*/rotation.gcsl" --out "$stage/dictionary.json"
cargo build --release --locked --target wasm32-unknown-unknown \
  --manifest-path "$optimizer/browser/Cargo.toml" --target-dir "$build_dir/browser"
cp "$build_dir/browser/wasm32-unknown-unknown/release/gso_browser.wasm" "$stage/gso.wasm"
python3 "$site_root/scripts/rotation-lab-manifest.py" manifest "$site_root" "$optimizer" "$stage"

# Publish the manifest last. Stop the lab server before rebuilding an active
# assets directory; matching asset hashes are validated at server startup.
mv "$stage/gso.wasm" "$output/gso.wasm"
mv "$stage/dictionary.json" "$output/dictionary.json"
mv "$stage/manifest.json" "$output/manifest.json"
printf 'Rotation lab assets: %s\n' "$output"

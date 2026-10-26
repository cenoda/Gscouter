#!/bin/sh
# Pulls the latest deploy config and image, and restarts what changed.
# Run every few minutes by gscouter-update.timer.
set -eu
cd /opt/gscouter
base=https://raw.githubusercontent.com/cenoda/gscouter/main/deploy
before=$(cat Caddyfile 2>/dev/null | md5sum)
for f in docker-compose.yml Caddyfile update.sh; do
	curl -fsSL "$base/$f" -o "$f.new" && mv "$f.new" "$f"
done
chmod +x update.sh
docker compose pull -q
docker compose up -d --remove-orphans
# Caddy mounts the Caddyfile, and the download replaced the file, so a
# changed Caddyfile needs a restart to be seen.
if [ "$before" != "$(md5sum < Caddyfile)" ]; then
	docker compose restart caddy
fi
docker image prune -f >/dev/null

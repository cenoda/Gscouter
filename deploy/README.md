# Deployment

Gscouter runs on one small Linux VM with Docker. Pushing to `main` builds `ghcr.io/cenoda/gscouter:latest`; the VM picks it up within 5 minutes.

## How it works

- `cloud-init.yaml` sets up a fresh Ubuntu 24.04 VM: Docker, a 2 GB swap file, and a systemd timer.
- Every 5 minutes the timer runs `update.sh`, which downloads `docker-compose.yml`, `Caddyfile` and itself from `main`, pulls new images, and restarts what changed.
- Caddy serves HTTPS for the domain in `/opt/gscouter/.env` and renews the certificate automatically.
- Ubuntu installs security updates automatically (unattended-upgrades is on by default).

## First-time setup

1. Create the VM (Azure: Ubuntu Server 24.04 LTS, size B2ats v2).
   - Allow inbound ports 80 and 443 (and 22 if you want SSH).
   - Under Advanced, paste `cloud-init.yaml` into "Custom data", with the `DOMAIN=` line changed to your domain.
2. Point the domain to the VM: add a DNS `A` record for the domain with the VM's public IP.
3. Wait a few minutes, then open `https://<domain>`.

The VM pulls the image without credentials, so the `gscouter` package on GitHub must stay public. It inherited public visibility from the repository; if the repository is made private, pulls will fail.

## Operations

- Logs: `cd /opt/gscouter && docker compose logs -f`
- Force an update now: `sudo systemctl start gscouter-update`
- Change the domain: edit `/opt/gscouter/.env`, then force an update.
- Short links live in the `gscouter_links` volume (`/data/links.db` in the app). The `links-owner` service gives the volume to the app's user on every start. To back them up: `docker run --rm -v gscouter_links:/data -v /opt/gscouter:/backup busybox:1.37 cp /data/links.db /backup/links-backup.db` (copy the `-wal` file too if there is one). Losing them breaks only short links: the long result links carry everything.

Without SSH, use the Azure portal's "Run command" (RunShellScript) for these.

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
- Change the domain: point the new domain (and its `www` name) to the VM, then in `/opt/gscouter/.env` set `DOMAIN` to it and add the old one to `OLD_DOMAINS` (space-separated), and force an update. `www.<domain>` and the old domains redirect to the same page on the new one, so shared links keep working.
- Search engines: the app tells IndexNow engines (Bing, Naver, Yandex) about new and changed pages on its own, keeping its key and what it submitted in `/data/indexnow.json` in the `gscouter_links` volume. Google and Naver need the site registered once by hand: add it in [Google Search Console](https://search.google.com/search-console) and [Naver Search Advisor](https://searchadvisor.naver.com), verify ownership (Google accepts a DNS TXT record; Naver checks the `naver-site-verification` meta tag in `internal/web/templates/layout.html`, which is gscouter.net's), and submit `https://<domain>/sitemap.xml`. They then re-read the sitemap by themselves.
- Short links live in the `gscouter_links` volume (`/data/links.db` in the app). The `links-owner` service gives the volume to the app's user on every start. To back them up: `docker run --rm -v gscouter_links:/data -v /opt/gscouter:/backup busybox:1.37 cp /data/links.db /backup/links-backup.db` (copy the `-wal` file too if there is one). Losing them breaks only short links: the long result links carry everything.
- Remove a short link (the about page promises this on request): put its ID (the part after `/s/`) in the `DELETE`, and run
  ```
  docker run --rm -i -v gscouter_links:/data alpine:3.22 sh -c 'apk add -q sqlite && sqlite3 /data/links.db && chown 65532:65532 /data/*' <<'SQL'
  DELETE FROM links WHERE id = 'ID';
  SQL
  ```
  It works while the app runs, and the short link stops working at once. The `chown` keeps the database files writable by the app.
- Usage counts: `docker run --rm -v gscouter_links:/data busybox:1.37 cat /data/usage.json` shows, per UTC day, front page views (`pages`), result page views (`results`) and distinct UIDs looked up (`uids`). The file is saved every 10 minutes and on shutdown.
  - Without the VM: set `GSCOUTER_USAGE_TOKEN` in `/opt/gscouter/.env` to a long random string (`openssl rand -hex 32`) and force an update. Then `curl -H "Authorization: Bearer <token>" https://<domain>/healthz` returns the counts, today's included; without the token `/healthz` answers `ok` as before, so the counts have no address of their own. An agent session reads them the same way when the token is in its environment as `GSCOUTER_USAGE_TOKEN` and the domain is allowed in its network settings.

Without SSH, use the Azure portal's "Run command" (RunShellScript) for these.

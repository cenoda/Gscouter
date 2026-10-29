# Aria commands

The moderator commands `/warn`, `/warnings` and `/unwarn` for the Gscouter Discord server, as a Cloudflare Worker with a D1 database. What they do and why: `docs/discord.md` §3–4.

## Layout

| Path | What |
|---|---|
| `src/index.js` | Worker entry: checks Discord's signature, answers, runs the daily expiry |
| `src/commands.js` | The commands and the expiry job |
| `src/policy.js` | The warning rules (ladder, 90-day expiry) |
| `src/store.js` | D1 queries |
| `src/discord.js` | Discord REST calls |
| `migrations/` | D1 schema |
| `scripts/register.mjs` | Registers the commands with Discord |

## Test

```
npm ci
npm test
```

## First setup

Done once, in this order.

1. **D1 database.** Cloudflare dashboard → Storage & Databases → D1 → Create, named `aria`. Put its ID in `wrangler.jsonc` (`database_id`); the ID is not a secret.
2. **Worker from this repository.** Workers & Pages → Create → Import a repository → this repository, with:
   - Root directory: `aria`
   - Build command: none
   - Deploy command: `npm run deploy` (applies new migrations, then deploys)
   - Production branch: `main`
   - Build watch paths: include `aria/*`, so pushes elsewhere in the repository do not redeploy it

   If the deploy log shows the migration step failing for lack of permission, paste `migrations/0001_warnings.sql` into the D1 database's Console once and change the deploy command to `npx wrangler deploy`.
3. **Bot token.** The Worker → Settings → Variables and Secrets → add a **Secret** `DISCORD_TOKEN` with Aria's bot token.
4. **Endpoint.** Copy the Worker's URL (`https://aria.<account>.workers.dev`) into Discord Developer Portal → Aria → General Information → Interactions Endpoint URL, and save. Discord tests the URL with a signed request; saving fails if the Worker or its public key is wrong.
5. **Commands.** With the bot token in the environment: `npm run register`. Run it again whenever `COMMANDS` in `src/commands.js` changes.

After that, a push to `main` that changes `aria/` redeploys the Worker. A new migration file needs the migration step (or the Console) before the code that uses it.

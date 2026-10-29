# Discord server

> Status: in progress · Last updated: 2026-10-29

The Gscouter Discord server: its channels and roles, the warning system, and the moderator commands Aria serves. The server is for Genshin Impact in general, not only the calculator. It is run in English, with few posts from staff; routine moderation and sorting reports is Aria's job.

## 1. Channels

| Category | Channel | Who sees it | Purpose |
|---|---|---|---|
| INFO | #welcome | everyone, read-only | Welcome and rules; the server's rules channel |
| | #announcements | everyone, read-only | Announcement channel |
| | #mod-updates | private | Discord's notices for moderators (the community updates channel) |
| | #mod-log | Moderator, read-only; Aria writes | Copy of every warning, revocation and expiry (§3.3) |
| GSCOUTER | #score-reports | everyone | Forum, one post per report; share link required. Status tags (Needs info, Checking, Confirmed, Fixed) only moderators set |
| | #team-requests | everyone | Teams to add to the library, with a source |
| COMMUNITY | #genshin | everyone | Anything about Genshin Impact |
| OPT-IN | #off-topic | Off-topic role | Anything else |
| | #flex | Flex role | Showing off pulls and accounts |
| | #leaks | Leaks role | Leaks. Gscouter uses official data only; nothing here goes into the calculator |

Leaks are allowed only in #leaks, and flexing only in #flex and #off-topic; elsewhere either earns a warning (§3). The opt-in roles are free: members pick them in onboarding when they join, or later in Channels & Roles, with no request and no bot involved. Moderators and Aria see the opt-in channels so they can moderate them. The owner does not take part in #leaks.

## 2. Roles

A member holds at most one role that grants permissions; display and opt-in roles grant none server-wide and can be combined.

| Role | Kind | Grants |
|---|---|---|
| Aria | bot | Broad permissions, no Administrator. Sits at the top so it can assign every role below |
| Developer | display | None. The owner, shown at the top of the member list |
| Moderator | permission | On top of @everyone: manage messages and threads, pin, time out, kick, ban, manage nicknames, view the audit log. Not channels, roles or server settings |
| Contributor | display | None. A report that led to a fix |
| Team Author | display | None. A team that went into the library |
| Off-topic, Flex, Leaks | opt-in | View of their channel only |
| @everyone | base | Defaults, without creating private threads, which nobody could moderate |

The server owner holds every permission without a role. Discord shows the warning commands only to members with Moderate Members, so Moderator and above.

## 3. Warnings

### 3.1 Three warnings

Moderators warn with `/warn member reason`. What follows depends on the member's active warnings after the new one:

| Active | Result |
|---|---|
| 1 | Recorded |
| 2 | 24-hour timeout |
| 3 | Ban |

The member gets a DM with the reason and the count before any action, since a banned member can no longer be reached. If their DMs are closed, the warning still stands and the log says so.

### 3.2 Reducing warnings

- **Expiry**: after 90 days without a new warning, the oldest active warning expires. The clock restarts at each new warning and each expiry, so warnings leave one at a time, 90 days apart. A daily job does this.
- **Revocation**: a moderator can revoke one with `/unwarn member reason [id]` (default: the latest active one), for a mistake or an appeal. A timeout or ban already given is not lifted automatically.

### 3.3 The log

Every warning is a row in a D1 database that is never deleted: expiry and revocation only mark a row `expired` or `revoked`, with when, by whom and why. `/warnings member` shows a member's whole history. Each event is also posted to #mod-log for people to read; the database is the record, so a failed post does not fail the command.

### 3.4 Moderators

The warning commands refuse to target a moderator. Abuse of moderator power is reported to the owner, and a moderator found abusing it is banned at once, without warnings.

## 4. Aria's commands

`/warn`, `/warnings` and `/unwarn` run as a Cloudflare Worker (`aria/`), separate from the site's VM, so that moderation adds nothing to run and nothing to pay for.

- Discord posts each command to the Worker's URL (the app's Interactions Endpoint URL), so no process stays online. The Worker checks Discord's signature, answers "thinking" at once, and fills in the reply after the DM, the action and the log, which can take longer than the 3 seconds Discord allows.
- Warnings live in D1 (Cloudflare's SQLite); a Cron Trigger runs the expiry daily at 03:17 UTC.
- Cost: expected to stay within the free tiers of Workers, D1 and Cron Triggers; a few commands a day is far below their limits. To be confirmed against Cloudflare's current pricing at setup.
- Aria's other work (reading reports, tags, posts) is done by Claude through the Discord MCP server when a session is open (`.mcp.json`). The Worker only handles commands, so the two do not overlap.
- Tests: `cd aria && npm test` (Node 22; D1 is stood in for by `node:sqlite` with the real migration). Checked on 2026-10-29 under `wrangler dev` with a local D1: a signed PING, two warnings, and the daily job, with no runtime errors.

Setup and deployment: `aria/README.md`.

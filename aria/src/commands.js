// The moderator commands /warn, /warnings and /unwarn, and the daily expiry of
// old warnings (docs/discord.md §3).

import { TIMEOUT_MS, actionFor, decayDue, describeAction, isModerator } from "./policy.js";

const MODERATE_MEMBERS = String(1n << 40n);

// Registered as guild commands by scripts/register.mjs. Discord shows them only
// to members with Moderate Members, which the Moderator role has (docs/discord.md §2).
export const COMMANDS = [
  {
    name: "warn",
    description: "Give a member a warning (3 active warnings = ban)",
    default_member_permissions: MODERATE_MEMBERS,
    dm_permission: false,
    options: [
      { type: 6, name: "member", description: "Who to warn", required: true },
      { type: 3, name: "reason", description: "Why; the member sees this", required: true, max_length: 500 },
    ],
  },
  {
    name: "warnings",
    description: "Show a member's warning history",
    default_member_permissions: MODERATE_MEMBERS,
    dm_permission: false,
    options: [{ type: 6, name: "member", description: "Whose history", required: true }],
  },
  {
    name: "unwarn",
    description: "Revoke a member's warning (the record stays, marked revoked)",
    default_member_permissions: MODERATE_MEMBERS,
    dm_permission: false,
    options: [
      { type: 6, name: "member", description: "Whose warning", required: true },
      { type: 3, name: "reason", description: "Why it is revoked", required: true, max_length: 500 },
      { type: 4, name: "id", description: "Warning number (default: the latest active one)", required: false },
    ],
  },
];

function option(interaction, name) {
  return interaction.data.options?.find((o) => o.name === name)?.value;
}

function date(ms) {
  return new Date(ms).toISOString().slice(0, 10);
}

// Returns the text of the (private) reply to the moderator.
export async function handleCommand(interaction, { env, store, discord, now }) {
  const guildId = interaction.guild_id;
  if (guildId !== env.GUILD_ID) return "This command only works in the Gscouter server.";
  if (!isModerator(interaction.member?.permissions)) return "Only moderators can use this command.";

  const moderatorId = interaction.member.user.id;
  const userId = option(interaction, "member");
  const user = interaction.data.resolved?.users?.[userId];
  const member = interaction.data.resolved?.members?.[userId];

  switch (interaction.data.name) {
    case "warn":
      return warn({ env, store, discord, now, guildId, moderatorId, userId, user, member, reason: option(interaction, "reason") });
    case "warnings":
      return history({ store, guildId, userId });
    case "unwarn":
      return unwarn({ env, store, discord, now, guildId, moderatorId, userId, reason: option(interaction, "reason"), id: option(interaction, "id") });
    default:
      return "Unknown command.";
  }
}

async function warn({ env, store, discord, now, guildId, moderatorId, userId, user, member, reason }) {
  if (!member) return "That user is not a member of this server.";
  if (user?.bot) return "Bots can't be warned.";
  if (userId === moderatorId) return "You can't warn yourself.";
  // Moderators answer to the owner, not to each other (docs/discord.md §3.4).
  if (isModerator(member.permissions)) return "Moderators can't be warned with this command. Report it to the server owner.";

  const id = await store.addWarning({ guildId, userId, moderatorId, reason, now });
  const active = await store.activeCount(guildId, userId);
  const action = actionFor(active);

  // The DM goes first: after a ban the bot can no longer reach the member.
  let dm = "DM delivered";
  try {
    await discord.dm(
      userId,
      `You received a warning in the Gscouter server.\nReason: ${reason}\nActive warnings: ${active}/3. ` +
        `Result: ${describeAction(action)}.\n` +
        "Warnings expire one at a time after 90 days without a new one. Three active warnings mean a ban.",
    );
  } catch {
    dm = "DM failed (closed DMs)";
  }

  let result = describeAction(action);
  try {
    const audit = `Warning #${id}: ${reason}`;
    if (action === "timeout") await discord.timeout(guildId, userId, now + TIMEOUT_MS, audit);
    if (action === "ban") await discord.ban(guildId, userId, audit);
    if (action !== "none") await store.setAction(id, action);
  } catch (err) {
    result = `${action} FAILED (${err.message.slice(0, 120)}); apply it by hand`;
  }

  const line =
    `⚠️ Warning #${id} · <@${userId}> (\`${userId}\`) by <@${moderatorId}>\n` +
    `Reason: ${reason}\nActive warnings: ${active}/3 → ${result} · ${dm}`;
  await logSafely(discord, env, line);
  return line;
}

async function history({ store, guildId, userId }) {
  const rows = await store.list(guildId, userId);
  if (rows.length === 0) return `<@${userId}> has no warnings.`;
  const active = rows.filter((r) => r.status === "active").length;
  const lines = rows.slice(-15).map((r) => {
    const end = r.status === "active" ? "" : ` · ${r.status} ${date(r.ended_at)}${r.end_reason ? ` (${r.end_reason})` : ""}`;
    const act = r.action === "none" ? "" : ` · ${r.action}`;
    return `#${r.id} ${date(r.created_at)} by <@${r.moderator_id}>: ${r.reason}${act}${end}`;
  });
  const more = rows.length > 15 ? `\n(${rows.length - 15} older not shown)` : "";
  return `<@${userId}>: ${active}/3 active, ${rows.length} in total\n${lines.join("\n")}${more}`;
}

async function unwarn({ env, store, discord, now, guildId, moderatorId, userId, reason, id }) {
  const row = await store.revoke({ guildId, userId, id, by: moderatorId, reason, now });
  if (!row) return id ? `Warning #${id} is not an active warning of <@${userId}>.` : `<@${userId}> has no active warnings.`;
  const active = await store.activeCount(guildId, userId);
  const line =
    `↩️ Warning #${row.id} of <@${userId}> (\`${userId}\`) revoked by <@${moderatorId}>\n` +
    `Reason: ${reason}\nActive warnings: ${active}/3` +
    (row.action === "none" ? "" : `\nIts ${row.action} is not lifted automatically; lift it by hand if it should be.`);
  await logSafely(discord, env, line);
  return line;
}

// The database is the permanent record; the #mod-log post is a copy for
// people, so a failed post does not fail the command.
async function logSafely(discord, env, line) {
  try {
    await discord.send(env.MOD_LOG_CHANNEL_ID, line);
  } catch {
    // ignored: see above
  }
}

// Daily: expires one warning per member whose decay clock has run out
// (docs/discord.md §3.2). Returns the number expired.
export async function runDecay({ env, store, discord, now }) {
  const candidates = await store.decayCandidates(env.GUILD_ID);
  let expired = 0;
  for (const c of candidates) {
    if (!decayDue({ lastWarningAt: c.last_warning_at, lastDecayAt: c.last_decay_at }, now)) continue;
    await store.expire(c.oldest_active, now);
    expired++;
    const active = await store.activeCount(env.GUILD_ID, c.user_id);
    await logSafely(
      discord,
      env,
      `⏳ Warning #${c.oldest_active} of <@${c.user_id}> (\`${c.user_id}\`) expired: 90 days without a new warning. Active warnings: ${active}/3`,
    );
  }
  return expired;
}

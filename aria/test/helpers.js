// Test doubles: D1 on top of node:sqlite with the real migration, and a
// Discord client that records calls instead of making them.
import { readFileSync } from "node:fs";
import { DatabaseSync } from "node:sqlite";

export function fakeD1() {
  const db = new DatabaseSync(":memory:");
  db.exec(readFileSync(new URL("../migrations/0001_warnings.sql", import.meta.url), "utf8"));
  return {
    raw: db,
    prepare(sql) {
      let args = [];
      const stmt = {
        bind(...a) {
          args = a;
          return stmt;
        },
        first: async () => {
          const row = db.prepare(sql).get(...args);
          return row ? { ...row } : null;
        },
        all: async () => ({ results: db.prepare(sql).all(...args).map((r) => ({ ...r })) }),
        run: async () => db.prepare(sql).run(...args),
      };
      return stmt;
    },
  };
}

export function fakeDiscord({ failDm = false } = {}) {
  const calls = [];
  return {
    calls,
    async send(channelId, content) {
      calls.push({ op: "send", channelId, content });
    },
    async dm(userId, content) {
      if (failDm) throw new Error("HTTP 403");
      calls.push({ op: "dm", userId, content });
    },
    async timeout(guildId, userId, untilMs) {
      calls.push({ op: "timeout", userId, untilMs });
    },
    async ban(guildId, userId) {
      calls.push({ op: "ban", userId });
    },
  };
}

export const env = { GUILD_ID: "g1", MOD_LOG_CHANNEL_ID: "log1" };

const MOD_PERMS = String(1n << 40n);

// A slash command interaction as Discord sends it.
export function interaction(name, options, { moderatorId = "mod1", targetPerms = "0", targetBot = false, guildId = "g1", invokerPerms = MOD_PERMS } = {}) {
  const userId = options.member;
  return {
    guild_id: guildId,
    member: { user: { id: moderatorId }, permissions: invokerPerms },
    data: {
      name,
      options: Object.entries(options).map(([k, v]) => ({ name: k, value: v })),
      resolved: {
        users: { [userId]: { id: userId, bot: targetBot } },
        members: { [userId]: { permissions: targetPerms } },
      },
    },
  };
}

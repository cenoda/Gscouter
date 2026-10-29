import assert from "node:assert/strict";
import { test } from "node:test";
import { handleCommand, runDecay } from "../src/commands.js";
import { DAY_MS, TIMEOUT_MS, actionFor, decayDue } from "../src/policy.js";
import { Store } from "../src/store.js";
import { env, fakeD1, fakeDiscord, interaction } from "./helpers.js";

function setup(opts) {
  const db = fakeD1();
  return { db, store: new Store(db), discord: fakeDiscord(opts) };
}

const T0 = Date.UTC(2026, 10, 1);

test("ladder: record, then timeout, then ban", () => {
  assert.equal(actionFor(1), "none");
  assert.equal(actionFor(2), "timeout");
  assert.equal(actionFor(3), "ban");
  assert.equal(actionFor(4), "ban");
});

test("three warnings: DM each time, 24h timeout on the second, ban on the third", async () => {
  const { store, discord } = setup();
  const deps = { env, store, discord };
  for (let i = 0; i < 3; i++) {
    await handleCommand(interaction("warn", { member: "u1", reason: `r${i}` }), { ...deps, now: T0 + i });
  }
  const ops = discord.calls.map((c) => c.op);
  assert.deepEqual(ops, ["dm", "send", "dm", "timeout", "send", "dm", "ban", "send"]);
  assert.equal(discord.calls.find((c) => c.op === "timeout").untilMs, T0 + 1 + TIMEOUT_MS);
  assert.ok(discord.calls.every((c) => c.op !== "send" || c.channelId === "log1"));
  const rows = await store.list("g1", "u1");
  assert.deepEqual(rows.map((r) => r.action), ["none", "timeout", "ban"]);
});

test("a closed DM does not stop the warning", async () => {
  const { store, discord } = setup({ failDm: true });
  const reply = await handleCommand(interaction("warn", { member: "u1", reason: "spam" }), { env, store, discord, now: T0 });
  assert.match(reply, /DM failed/);
  assert.equal(await store.activeCount("g1", "u1"), 1);
});

test("refuses moderators, bots, oneself, other servers and non-moderators", async () => {
  const { store, discord } = setup();
  const deps = { env, store, discord, now: T0 };
  const mod = String(1n << 40n);
  assert.match(await handleCommand(interaction("warn", { member: "u2", reason: "x" }, { targetPerms: mod }), deps), /server owner/);
  assert.match(await handleCommand(interaction("warn", { member: "u2", reason: "x" }, { targetBot: true }), deps), /Bots/);
  assert.match(await handleCommand(interaction("warn", { member: "mod1", reason: "x" }), deps), /yourself/);
  assert.match(await handleCommand(interaction("warn", { member: "u2", reason: "x" }, { guildId: "other" }), deps), /only works/);
  assert.match(await handleCommand(interaction("warn", { member: "u2", reason: "x" }, { invokerPerms: "0" }), deps), /Only moderators/);
  assert.equal((await store.list("g1", "u2")).length, 0);
});

test("unwarn marks the latest active warning revoked and keeps the row", async () => {
  const { store, discord } = setup();
  const deps = { env, store, discord };
  await handleCommand(interaction("warn", { member: "u1", reason: "a" }), { ...deps, now: T0 });
  await handleCommand(interaction("warn", { member: "u1", reason: "b" }), { ...deps, now: T0 + 1 });
  const reply = await handleCommand(interaction("unwarn", { member: "u1", reason: "mistake" }), { ...deps, now: T0 + 2 });
  assert.match(reply, /#2 .*revoked/);
  assert.match(reply, /not lifted automatically/);
  const rows = await store.list("g1", "u1");
  assert.deepEqual(rows.map((r) => r.status), ["active", "revoked"]);
  assert.equal(await store.activeCount("g1", "u1"), 1);
  assert.match(await handleCommand(interaction("unwarn", { member: "u1", reason: "x", id: 2 }), { ...deps, now: T0 + 3 }), /not an active warning/);
});

test("warnings lists the whole history", async () => {
  const { store, discord } = setup();
  const deps = { env, store, discord };
  assert.match(await handleCommand(interaction("warnings", { member: "u1" }), { ...deps, now: T0 }), /no warnings/);
  await handleCommand(interaction("warn", { member: "u1", reason: "leak in #genshin" }), { ...deps, now: T0 });
  await handleCommand(interaction("unwarn", { member: "u1", reason: "appeal" }), { ...deps, now: T0 + 1 });
  const reply = await handleCommand(interaction("warnings", { member: "u1" }), { ...deps, now: T0 + 2 });
  assert.match(reply, /0\/3 active, 1 in total/);
  assert.match(reply, /leak in #genshin · revoked/);
});

test("decay clock runs from the latest warning or expiry", () => {
  assert.equal(decayDue({ lastWarningAt: 0, lastDecayAt: null }, 90 * DAY_MS - 1), false);
  assert.equal(decayDue({ lastWarningAt: 0, lastDecayAt: null }, 90 * DAY_MS), true);
  assert.equal(decayDue({ lastWarningAt: 0, lastDecayAt: 90 * DAY_MS }, 179 * DAY_MS), false);
});

test("daily run expires one warning per 90 quiet days, oldest first", async () => {
  const { store, discord } = setup();
  const deps = { env, store, discord };
  await handleCommand(interaction("warn", { member: "u1", reason: "a" }), { ...deps, now: T0 });
  await handleCommand(interaction("warn", { member: "u1", reason: "b" }), { ...deps, now: T0 + 10 * DAY_MS });

  assert.equal(await runDecay({ ...deps, now: T0 + 99 * DAY_MS }), 0);
  assert.equal(await runDecay({ ...deps, now: T0 + 100 * DAY_MS }), 1);
  assert.equal(await runDecay({ ...deps, now: T0 + 101 * DAY_MS }), 0);
  let rows = await store.list("g1", "u1");
  assert.deepEqual(rows.map((r) => r.status), ["expired", "active"]);

  assert.equal(await runDecay({ ...deps, now: T0 + 190 * DAY_MS }), 1);
  assert.equal(await runDecay({ ...deps, now: T0 + 400 * DAY_MS }), 0);
  rows = await store.list("g1", "u1");
  assert.deepEqual(rows.map((r) => r.status), ["expired", "expired"]);
  assert.ok(discord.calls.filter((c) => c.op === "send" && /expired/.test(c.content)).length === 2);
});

test("a new warning restarts the decay clock", async () => {
  const { store, discord } = setup();
  const deps = { env, store, discord };
  await handleCommand(interaction("warn", { member: "u1", reason: "a" }), { ...deps, now: T0 });
  await handleCommand(interaction("warn", { member: "u1", reason: "b" }), { ...deps, now: T0 + 80 * DAY_MS });
  assert.equal(await runDecay({ ...deps, now: T0 + 100 * DAY_MS }), 0);
  assert.equal(await runDecay({ ...deps, now: T0 + 170 * DAY_MS }), 1);
});

// Registers /warn, /warnings and /unwarn as guild commands of the Aria app.
// Run once, and again after changing COMMANDS:
//   DISCORD_TOKEN=... npm run register
import { readFileSync } from "node:fs";
import { COMMANDS } from "../src/commands.js";

const config = JSON.parse(
  readFileSync(new URL("../wrangler.jsonc", import.meta.url), "utf8").replace(/^\s*\/\/.*$/gm, ""),
);
const { APPLICATION_ID, GUILD_ID } = config.vars;
const token = process.env.DISCORD_TOKEN;
if (!token) throw new Error("DISCORD_TOKEN is not set");

const res = await fetch(`https://discord.com/api/v10/applications/${APPLICATION_ID}/guilds/${GUILD_ID}/commands`, {
  method: "PUT",
  headers: { Authorization: `Bot ${token}`, "Content-Type": "application/json" },
  body: JSON.stringify(COMMANDS),
});
if (!res.ok) throw new Error(`HTTP ${res.status}: ${await res.text()}`);
console.log("registered:", (await res.json()).map((c) => c.name).join(", "));

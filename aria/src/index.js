// Aria's slash commands, served as a Cloudflare Worker (docs/discord.md §4).
// Discord posts each interaction here; nothing has to stay online.

import { handleCommand, runDecay } from "./commands.js";
import { Discord } from "./discord.js";
import { Store } from "./store.js";
import { verifyRequest } from "./verify.js";

const PING = 1;
const APPLICATION_COMMAND = 2;
const PONG = 1;
const DEFERRED_REPLY = 5;
const EPHEMERAL = 64;

const json = (body, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

export default {
  async fetch(request, env, ctx) {
    if (request.method !== "POST") return new Response("Aria", { status: 200 });

    const body = await request.text();
    const ok = await verifyRequest(
      env.DISCORD_PUBLIC_KEY,
      request.headers.get("X-Signature-Ed25519"),
      request.headers.get("X-Signature-Timestamp"),
      body,
    );
    if (!ok) return new Response("invalid request signature", { status: 401 });

    const interaction = JSON.parse(body);
    if (interaction.type === PING) return json({ type: PONG });
    if (interaction.type !== APPLICATION_COMMAND) return json({ error: "unsupported" }, 400);

    // Discord wants an answer within 3 seconds; the DM, timeout and log can
    // take longer, so answer "thinking" now and fill the reply in after.
    const discord = new Discord(env.DISCORD_TOKEN);
    const work = handleCommand(interaction, { env, store: new Store(env.DB), discord, now: Date.now() })
      .catch((err) => `Something went wrong: ${err.message.slice(0, 300)}`)
      .then((text) => discord.editReply(env.APPLICATION_ID, interaction.token, text.slice(0, 2000)));
    ctx.waitUntil(work);
    return json({ type: DEFERRED_REPLY, data: { flags: EPHEMERAL } });
  },

  async scheduled(_event, env, ctx) {
    const discord = new Discord(env.DISCORD_TOKEN);
    ctx.waitUntil(runDecay({ env, store: new Store(env.DB), discord, now: Date.now() }));
  },
};

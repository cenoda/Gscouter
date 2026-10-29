import assert from "node:assert/strict";
import { test } from "node:test";
import { Discord } from "../src/discord.js";

// Workers throw "Illegal invocation" when fetch runs with another object as
// `this`; local Node does not, so check the binding directly.
test("calls fetch without binding it to the client", async () => {
  const seen = [];
  const fetchImpl = function (url, init) {
    seen.push({ self: this, url, init });
    return Promise.resolve(new Response(null, { status: 204 }));
  };
  const discord = new Discord("tok", fetchImpl);
  await discord.ban("g", "u", "Warning #3: spam");
  await discord.editReply("app", "itok", "done");
  assert.equal(seen.length, 2);
  assert.ok(seen.every((c) => c.self === undefined));
  assert.equal(seen[0].url, "https://discord.com/api/v10/guilds/g/bans/u");
  assert.equal(seen[0].init.headers.Authorization, "Bot tok");
  assert.equal(seen[1].init.headers.Authorization, undefined);
});

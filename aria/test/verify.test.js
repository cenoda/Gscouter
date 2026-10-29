import assert from "node:assert/strict";
import { test } from "node:test";
import worker from "../src/index.js";
import { verifyRequest } from "../src/verify.js";

const hex = (buf) => Buffer.from(buf).toString("hex");

async function keys() {
  const pair = await crypto.subtle.generateKey({ name: "Ed25519" }, true, ["sign", "verify"]);
  const pub = hex(await crypto.subtle.exportKey("raw", pair.publicKey));
  const sign = async (ts, body) => hex(await crypto.subtle.sign("Ed25519", pair.privateKey, new TextEncoder().encode(ts + body)));
  return { pub, sign };
}

test("accepts Discord's signature and rejects anything else", async () => {
  const { pub, sign } = await keys();
  const sig = await sign("123", "{}");
  assert.equal(await verifyRequest(pub, sig, "123", "{}"), true);
  assert.equal(await verifyRequest(pub, sig, "124", "{}"), false);
  assert.equal(await verifyRequest(pub, sig, "123", "{ }"), false);
  assert.equal(await verifyRequest(pub, "zz", "123", "{}"), false);
  assert.equal(await verifyRequest(pub, null, "123", "{}"), false);
});

test("worker answers a signed PING and refuses an unsigned one", async () => {
  const { pub, sign } = await keys();
  const env = { DISCORD_PUBLIC_KEY: pub };
  const body = JSON.stringify({ type: 1 });
  const req = async (sig) =>
    worker.fetch(
      new Request("https://aria.example/", {
        method: "POST",
        headers: { "X-Signature-Ed25519": sig, "X-Signature-Timestamp": "1" },
        body,
      }),
      env,
      { waitUntil() {} },
    );
  const good = await req(await sign("1", body));
  assert.equal(good.status, 200);
  assert.deepEqual(await good.json(), { type: 1 });
  assert.equal((await req("00".repeat(64))).status, 401);
});

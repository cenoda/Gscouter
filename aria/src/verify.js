// Discord signs every interaction it sends; requests that fail the check must
// be answered with 401, which Discord also tests when the endpoint URL is saved.

function hexToBytes(hex) {
  if (!/^[0-9a-f]*$/i.test(hex) || hex.length % 2 !== 0) return null;
  const out = new Uint8Array(hex.length / 2);
  for (let i = 0; i < out.length; i++) out[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  return out;
}

export async function verifyRequest(publicKeyHex, signatureHex, timestamp, body) {
  if (!signatureHex || !timestamp) return false;
  const key = hexToBytes(publicKeyHex);
  const signature = hexToBytes(signatureHex);
  if (!key || !signature) return false;
  try {
    const cryptoKey = await crypto.subtle.importKey("raw", key, { name: "Ed25519" }, false, ["verify"]);
    return await crypto.subtle.verify("Ed25519", cryptoKey, signature, new TextEncoder().encode(timestamp + body));
  } catch {
    return false;
  }
}

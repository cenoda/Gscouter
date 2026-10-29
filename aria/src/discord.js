// The few Discord REST calls the warning commands make, as the Aria bot.

const API = "https://discord.com/api/v10";

export class Discord {
  constructor(token, fetchImpl = fetch) {
    this.token = token;
    // Wrapped: the Workers runtime rejects fetch called as a method of another object.
    this.fetch = (...args) => fetchImpl(...args);
  }

  async call(method, path, body, reason) {
    const headers = { Authorization: `Bot ${this.token}` };
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (reason) headers["X-Audit-Log-Reason"] = encodeURIComponent(reason.slice(0, 400));
    const res = await this.fetch(API + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`${method} ${path}: HTTP ${res.status} ${text.slice(0, 200)}`);
    }
    return res.status === 204 ? null : res.json();
  }

  send(channelId, content) {
    return this.call("POST", `/channels/${channelId}/messages`, {
      content,
      allowed_mentions: { parse: [] },
    });
  }

  async dm(userId, content) {
    const channel = await this.call("POST", "/users/@me/channels", { recipient_id: userId });
    return this.send(channel.id, content);
  }

  timeout(guildId, userId, untilMs, reason) {
    return this.call(
      "PATCH",
      `/guilds/${guildId}/members/${userId}`,
      { communication_disabled_until: new Date(untilMs).toISOString() },
      reason,
    );
  }

  ban(guildId, userId, reason) {
    return this.call("PUT", `/guilds/${guildId}/bans/${userId}`, { delete_message_seconds: 0 }, reason);
  }

  // Replaces the deferred "Aria is thinking..." reply. Uses the interaction
  // token, not the bot token.
  editReply(applicationId, interactionToken, content) {
    return this.fetch(`${API}/webhooks/${applicationId}/${interactionToken}/messages/@original`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content, allowed_mentions: { parse: [] } }),
    });
  }
}

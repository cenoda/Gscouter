// Warning records in D1. Rows are never deleted: a warning that stops counting
// is marked expired or revoked, so the log stays complete (docs/discord.md §3.3).

export class Store {
  constructor(db) {
    this.db = db;
  }

  async addWarning({ guildId, userId, moderatorId, reason, now }) {
    const row = await this.db
      .prepare(
        "INSERT INTO warnings (guild_id, user_id, moderator_id, reason, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id",
      )
      .bind(guildId, userId, moderatorId, reason, now)
      .first();
    return row.id;
  }

  async setAction(id, action) {
    await this.db.prepare("UPDATE warnings SET action = ? WHERE id = ?").bind(action, id).run();
  }

  async activeCount(guildId, userId) {
    const row = await this.db
      .prepare("SELECT COUNT(*) AS n FROM warnings WHERE guild_id = ? AND user_id = ? AND status = 'active'")
      .bind(guildId, userId)
      .first();
    return row.n;
  }

  async list(guildId, userId) {
    const { results } = await this.db
      .prepare("SELECT * FROM warnings WHERE guild_id = ? AND user_id = ? ORDER BY id")
      .bind(guildId, userId)
      .all();
    return results;
  }

  // Revokes the given active warning, or the member's latest active one when
  // id is not given. Returns the revoked row, or null when there is none.
  async revoke({ guildId, userId, id, by, reason, now }) {
    const target = id
      ? await this.db
          .prepare("SELECT * FROM warnings WHERE id = ? AND guild_id = ? AND user_id = ? AND status = 'active'")
          .bind(id, guildId, userId)
          .first()
      : await this.db
          .prepare(
            "SELECT * FROM warnings WHERE guild_id = ? AND user_id = ? AND status = 'active' ORDER BY id DESC LIMIT 1",
          )
          .bind(guildId, userId)
          .first();
    if (!target) return null;
    await this.db
      .prepare("UPDATE warnings SET status = 'revoked', ended_at = ?, ended_by = ?, end_reason = ? WHERE id = ?")
      .bind(now, by, reason, target.id)
      .run();
    return target;
  }

  // Members with an active warning, with the times the decay clock runs from.
  async decayCandidates(guildId) {
    const { results } = await this.db
      .prepare(
        `SELECT user_id,
                MIN(CASE WHEN status = 'active' THEN id END) AS oldest_active,
                MAX(created_at) AS last_warning_at,
                MAX(CASE WHEN status = 'expired' THEN ended_at END) AS last_decay_at
           FROM warnings
          WHERE guild_id = ?
          GROUP BY user_id
         HAVING oldest_active IS NOT NULL`,
      )
      .bind(guildId)
      .all();
    return results;
  }

  async expire(id, now) {
    await this.db
      .prepare(
        "UPDATE warnings SET status = 'expired', ended_at = ?, ended_by = 'decay', end_reason = '90 days without a new warning' WHERE id = ? AND status = 'active'",
      )
      .bind(now, id)
      .run();
  }
}

-- Warning log (docs/discord.md §3.3). Rows are never deleted.
CREATE TABLE warnings (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  guild_id     TEXT    NOT NULL,
  user_id      TEXT    NOT NULL,
  moderator_id TEXT    NOT NULL,
  reason       TEXT    NOT NULL,
  created_at   INTEGER NOT NULL,                -- Unix milliseconds
  action       TEXT    NOT NULL DEFAULT 'none', -- none, timeout or ban, taken with this warning
  status       TEXT    NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'revoked')),
  ended_at     INTEGER,                         -- when it stopped counting
  ended_by     TEXT,                            -- moderator id, or 'decay'
  end_reason   TEXT
);

CREATE INDEX warnings_member ON warnings (guild_id, user_id, status);

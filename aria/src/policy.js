// Warning rules for the Discord server (docs/discord.md §3). Pure functions, so
// they can be tested without Discord or a database.

export const DAY_MS = 24 * 60 * 60 * 1000;

// One active warning expires after this long without a new warning or an
// earlier expiry (docs/discord.md §3.2).
export const DECAY_MS = 90 * DAY_MS;

// The timeout given on the second active warning (docs/discord.md §3.1).
export const TIMEOUT_MS = 1 * DAY_MS;

// Members with either permission are moderators: the warning commands refuse
// to target them, since moderator misconduct goes to the owner (docs/discord.md §3.4).
export const PERM_ADMINISTRATOR = 1n << 3n;
export const PERM_MODERATE_MEMBERS = 1n << 40n;

export function isModerator(permissions) {
  const p = BigInt(permissions ?? 0);
  return (p & (PERM_ADMINISTRATOR | PERM_MODERATE_MEMBERS)) !== 0n;
}

// What a member's active warning count leads to, right after a new warning:
// 1 is a record only, 2 a 24-hour timeout, 3 or more a ban (docs/discord.md §3.1).
export function actionFor(activeCount) {
  if (activeCount >= 3) return "ban";
  if (activeCount === 2) return "timeout";
  return "none";
}

// Whether one of a member's active warnings expires now. The clock restarts at
// the latest warning and at the latest expiry, so warnings leave one at a time,
// 90 days apart (docs/discord.md §3.2).
export function decayDue({ lastWarningAt, lastDecayAt }, now) {
  const since = Math.max(lastWarningAt ?? 0, lastDecayAt ?? 0);
  return now - since >= DECAY_MS;
}

export function describeAction(action) {
  switch (action) {
    case "ban":
      return "banned from the server";
    case "timeout":
      return "timed out for 24 hours";
    default:
      return "no further action";
  }
}

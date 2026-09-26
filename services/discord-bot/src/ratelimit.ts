/**
 * Per-user sliding-window rate limiter for slash commands.
 *
 * Discord allows a burst of interactions from one account; without a limiter
 * a single user could hammer the Sorolens API through the bot and burn the
 * shared upstream quota. Each Discord user id gets their own window.
 *
 * The clock is injectable so tests are deterministic.
 */

export interface RateLimitResult {
  allowed: boolean;
  /** Milliseconds until the caller's oldest hit leaves the window. */
  retryAfterMs: number;
  /** Hits remaining in the current window after this call. */
  remaining: number;
}

export class SlidingWindowRateLimiter {
  private readonly hits = new Map<string, number[]>();

  constructor(
    private readonly limit: number,
    private readonly windowMs = 60_000,
    private readonly now: () => number = Date.now,
  ) {
    if (limit < 1) throw new Error("rate limit must be at least 1");
  }

  /** Record an attempt for `key` and report whether it is allowed. */
  check(key: string): RateLimitResult {
    const now = this.now();
    const cutoff = now - this.windowMs;
    const recent = (this.hits.get(key) ?? []).filter((t) => t > cutoff);

    if (recent.length >= this.limit) {
      this.hits.set(key, recent);
      const oldest = recent[0];
      return {
        allowed: false,
        retryAfterMs: Math.max(1, oldest + this.windowMs - now),
        remaining: 0,
      };
    }

    recent.push(now);
    this.hits.set(key, recent);
    return { allowed: true, retryAfterMs: 0, remaining: this.limit - recent.length };
  }

  /** Drop the window for one key, or all keys when called with no argument. */
  reset(key?: string): void {
    if (key === undefined) this.hits.clear();
    else this.hits.delete(key);
  }
}

/** Human-friendly "try again in 42s" fragment for ephemeral replies. */
export function describeRetry(retryAfterMs: number): string {
  const seconds = Math.max(1, Math.ceil(retryAfterMs / 1000));
  return `${seconds} second${seconds === 1 ? "" : "s"}`;
}

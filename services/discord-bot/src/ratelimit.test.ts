import { describe, it, expect } from "vitest";
import { SlidingWindowRateLimiter, describeRetry } from "./ratelimit.js";

describe("SlidingWindowRateLimiter", () => {
  it("allows up to the limit then blocks", () => {
    const limiter = new SlidingWindowRateLimiter(3, 60_000, () => 1_000);
    expect(limiter.check("u1").allowed).toBe(true);
    expect(limiter.check("u1").allowed).toBe(true);
    const third = limiter.check("u1");
    expect(third.allowed).toBe(true);
    expect(third.remaining).toBe(0);

    const fourth = limiter.check("u1");
    expect(fourth.allowed).toBe(false);
    expect(fourth.retryAfterMs).toBe(60_000);
  });

  it("tracks users independently", () => {
    const limiter = new SlidingWindowRateLimiter(1, 60_000, () => 5_000);
    expect(limiter.check("u1").allowed).toBe(true);
    expect(limiter.check("u1").allowed).toBe(false);
    expect(limiter.check("u2").allowed).toBe(true);
  });

  it("frees the slot once the window slides past the oldest hit", () => {
    let now = 0;
    const limiter = new SlidingWindowRateLimiter(1, 1_000, () => now);
    expect(limiter.check("u1").allowed).toBe(true);
    now = 500;
    expect(limiter.check("u1").allowed).toBe(false);
    now = 1_001;
    expect(limiter.check("u1").allowed).toBe(true);
  });

  it("reports the shrinking retry delay", () => {
    let now = 0;
    const limiter = new SlidingWindowRateLimiter(1, 1_000, () => now);
    limiter.check("u1");
    now = 400;
    const blocked = limiter.check("u1");
    expect(blocked.retryAfterMs).toBe(600);
  });

  it("reset clears one key or all keys", () => {
    const limiter = new SlidingWindowRateLimiter(1, 60_000, () => 0);
    limiter.check("u1");
    limiter.check("u2");
    limiter.reset("u1");
    expect(limiter.check("u1").allowed).toBe(true);
    expect(limiter.check("u2").allowed).toBe(false);
    limiter.reset();
    expect(limiter.check("u2").allowed).toBe(true);
  });

  it("rejects a nonsensical limit", () => {
    expect(() => new SlidingWindowRateLimiter(0)).toThrow(/at least 1/);
  });
});

describe("describeRetry", () => {
  it("rounds up to whole seconds with correct pluralisation", () => {
    expect(describeRetry(1)).toBe("1 second");
    expect(describeRetry(1_400)).toBe("2 seconds");
    expect(describeRetry(60_000)).toBe("60 seconds");
  });
});

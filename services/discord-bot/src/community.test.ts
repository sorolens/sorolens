import { describe, expect, it, beforeEach } from "vitest";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  dueCotwWeek,
  formatCongrats,
  formatCotw,
  formatLeaderboard,
  milestonesReached,
  ordinal,
  previousWeek,
  rankContributors,
  weekStart,
} from "./community.js";
import * as db from "./db.js";

const pr = (author: string, mergedAt: string, number = 1) => ({ author, mergedAt, number });

describe("rankContributors", () => {
  it("ranks by count and breaks ties by who reached it first", () => {
    const ranked = rankContributors([
      pr("bob", "2026-11-02T10:00:00Z"),
      pr("alice", "2026-11-01T10:00:00Z"),
      pr("alice", "2026-11-03T10:00:00Z"),
      pr("carol", "2026-11-01T09:00:00Z"),
      pr("carol", "2026-11-02T09:00:00Z"),
      pr("dave", "2026-11-01T00:00:00Z"),
    ]);
    expect(ranked.map((r) => [r.login, r.count])).toEqual([
      ["carol", 2],
      ["alice", 2],
      ["dave", 1],
      ["bob", 1],
    ]);
  });

  it("keeps only linked logins, case-insensitively", () => {
    const ranked = rankContributors(
      [pr("Alice", "2026-11-01T00:00:00Z"), pr("stranger", "2026-11-01T00:00:00Z")],
      new Set(["alice"])
    );
    expect(ranked.map((r) => r.login)).toEqual(["Alice"]);
  });
});

describe("weeks (Sunday to Saturday, UTC)", () => {
  it("finds the week start and previous week", () => {
    // 2026-11-19 is a Thursday.
    expect(weekStart(new Date("2026-11-19T15:00:00Z")).toISOString()).toBe("2026-11-15T00:00:00.000Z");
    const prev = previousWeek(new Date("2026-11-22T08:00:00Z"));
    expect(prev.key).toBe("2026-11-15");
    expect(prev.end.toISOString()).toBe("2026-11-22T00:00:00.000Z");
  });

  it("is due from Sunday 08:00 UTC, once per week", () => {
    expect(dueCotwWeek(new Date("2026-11-22T07:59:00Z"), null)).toBeNull();
    expect(dueCotwWeek(new Date("2026-11-22T08:00:00Z"), null)?.key).toBe("2026-11-15");
    // A bot that slept through Sunday still posts on Monday.
    expect(dueCotwWeek(new Date("2026-11-23T12:00:00Z"), "2026-11-08")?.key).toBe("2026-11-15");
    expect(dueCotwWeek(new Date("2026-11-23T12:00:00Z"), "2026-11-15")).toBeNull();
  });
});

describe("milestones", () => {
  const ms = [
    { count: 10, roleId: "r10" },
    { count: 5, roleId: "r5" },
    { count: 20, roleId: "r20" },
  ];
  it("returns every milestone at or below the count", () => {
    expect(milestonesReached(4, ms)).toEqual([]);
    expect(milestonesReached(10, ms).map((m) => m.count)).toEqual([5, 10]);
  });
  it("formats ordinals", () => {
    expect([1, 2, 3, 4, 11, 12, 13, 21, 22, 101, 111].map(ordinal)).toEqual([
      "1st", "2nd", "3rd", "4th", "11th", "12th", "13th", "21st", "22nd", "101st", "111th",
    ]);
  });
});

describe("messages", () => {
  it("tags a linked member with their avatar and shows a milestone", () => {
    const msg = formatCongrats({
      discordId: "42",
      login: "alice",
      avatarUrl: "https://cdn.discordapp.com/avatars/42/x.png",
      prNumber: 900,
      prTitle: "fix: thing",
      prUrl: "https://github.com/x/y/pull/900",
      total: 5,
      milestone: { count: 5, roleId: "r5" },
    });
    expect(msg.content).toBe("🎉 <@42> just got PR #900 merged!");
    expect(msg.embed.title).toBe("#900 fix: thing");
    expect(msg.embed.thumbnail).toContain("cdn.discordapp.com");
    expect(msg.embed.description).toContain("**5th**");
    expect(msg.embed.description).toContain("<@&r5>");
    expect(msg.embed.description).not.toContain("/connect");
  });

  it("congratulates an unlinked author by GitHub name and nudges them to /connect", () => {
    const msg = formatCongrats({
      discordId: null,
      login: "bob",
      avatarUrl: "https://avatars.githubusercontent.com/u/1",
      prNumber: 7,
      prTitle: "docs",
      prUrl: "https://github.com/x/y/pull/7",
      total: 1,
    });
    expect(msg.content).toBe("🎉 **bob** just got PR #7 merged!");
    expect(msg.content).not.toContain("<@");
    expect(msg.embed.thumbnail).toContain("githubusercontent");
    expect(msg.embed.description).toContain("`bob`'s **1st**");
    expect(msg.embed.description).toContain("/connect");
  });

  it("renders the leaderboard with medals", () => {
    const text = formatLeaderboard("Top", [
      { login: "a", count: 3, discordId: "1" },
      { login: "b", count: 1, discordId: "2" },
    ]);
    expect(text).toContain("🥇 <@1>");
    expect(text).toContain("**1** merged PR\n");
    expect(formatLeaderboard("Top", [])).toContain("No merged PRs");
  });

  it("announces the contributor of the week and runners-up", () => {
    const week = previousWeek(new Date("2026-11-22T08:00:00Z"));
    const text = formatCotw({
      weekStart: week.start,
      weekEnd: week.end,
      roleId: "cotw",
      rows: [
        { login: "a", count: 4, discordId: "1" },
        { login: "b", count: 2, discordId: "2" },
      ],
    });
    expect(text).toContain("2026-11-15 to 2026-11-21");
    expect(text).toContain("<@1>");
    expect(text).toContain("<@&cotw>");
    expect(text).toContain("Runners-up: <@2> (2)");
  });
});

describe("community storage", () => {
  beforeEach(() => db.openDb(join(mkdtempSync(join(tmpdir(), "community-")), "t.db")));

  it("stores settings and milestones", () => {
    expect(db.getCommunitySettings()).toMatchObject({ congratsEnabled: true, cotwEnabled: true, channelId: null });
    db.updateCommunitySettings({ channelId: "activity", congratsChannelId: "live", cotwLastWeek: "2026-11-15" });
    expect(db.getCommunitySettings()).toMatchObject({ channelId: "activity", congratsChannelId: "live", cotwLastWeek: "2026-11-15" });
    db.setMilestone(5, "r5");
    db.setMilestone(5, "r5b");
    db.setMilestone(10, "r10");
    expect(db.listMilestones()).toEqual([{ count: 5, roleId: "r5b" }, { count: 10, roleId: "r10" }]);
    expect(db.removeMilestone(5)).toBe(true);
    expect(db.listMilestones()).toHaveLength(1);
  });
});

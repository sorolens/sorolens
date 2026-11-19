import { describe, expect, it, vi, beforeEach } from "vitest";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  formatIssueList,
  hasWaveApplication,
  meetsRoleRules,
  parseIssueList,
  parseIssueRefs,
  processApplication,
  type AssignDeps,
} from "./assign.js";
import * as db from "./db.js";
import type { IssueComment, IssueInfo } from "./github.js";

const WAVE = (login: string) =>
  `@${login} has applied to work on this issue as part of the **Stellar Wave Program**'s 11th wave.\n\n<!-- wave:application-id:232a555b-d5df-4535-be7e-fdd9471fb057 -->`;

describe("parsing", () => {
  it("reads unique #refs from a message", () => {
    expect(parseIssueRefs("please #812 and #840, #812 again")).toEqual([812, 840]);
    expect(parseIssueRefs("no refs, 812, a#9, &#39;")).toEqual([]);
  });

  it("parses owner lists with ranges", () => {
    expect(parseIssueList("836-838, 812 #900")).toEqual([812, 836, 837, 838, 900]);
    expect(parseIssueList("5-3")).toEqual([3, 4, 5]);
    expect(() => parseIssueList("abc")).toThrow();
    expect(() => parseIssueList("1-10000")).toThrow(/wider/);
  });

  it("formats lists compactly", () => {
    expect(formatIssueList([812, 836, 837, 838, 900])).toBe("#812, #836-#838, #900");
    expect(formatIssueList([])).toBe("(none)");
  });
});

describe("rules", () => {
  it("detects a Wave application by the linked author only", () => {
    const comments: IssueComment[] = [
      { author: "someone", authorType: "User", body: WAVE("alice") },
      { author: "Alice", authorType: "User", body: "I can do this" },
    ];
    expect(hasWaveApplication(comments, "alice")).toBe(false);
    comments.push({ author: "ALICE", authorType: "User", body: WAVE("alice") });
    expect(hasWaveApplication(comments, "alice")).toBe(true);
  });

  it("needs all required roles and one any-of role", () => {
    const s = { requiredRoles: ["real"], anyOfRoles: ["contrib", "core"] };
    expect(meetsRoleRules(new Set(["real", "core"]), s)).toBe(true);
    expect(meetsRoleRules(new Set(["real"]), s)).toBe(false);
    expect(meetsRoleRules(new Set(["core"]), s)).toBe(false);
    expect(meetsRoleRules(new Set(["real"]), { requiredRoles: ["real"], anyOfRoles: [] })).toBe(true);
  });
});

describe("processApplication", () => {
  let issues: Map<number, IssueInfo>;
  let comments: Map<number, IssueComment[]>;
  let deps: AssignDeps;

  const issue = (n: number, assignees: string[] = [], state: "open" | "closed" = "open"): IssueInfo => ({
    number: n, state, isPullRequest: false, assignees, title: `t${n}`, url: `https://gh/${n}`,
  });

  beforeEach(() => {
    db.openDb(join(mkdtempSync(join(tmpdir(), "assign-")), "t.db"));
    db.upsertLink("u1", "alice");
    db.upsertLink("u2", "bob");
    db.updateAssignSettings({
      enabled: true, channelId: "chan", requiredRoles: ["real"], anyOfRoles: ["contrib", "core"], maxPerUser: 2,
    });
    db.addToPool([10, 11, 12, 13]);
    issues = new Map([10, 11, 12, 13].map((n) => [n, issue(n)]));
    comments = new Map([10, 11, 12, 13].map((n) => [n, [{ author: "alice", authorType: "User", body: WAVE("alice") }, { author: "bob", authorType: "User", body: WAVE("bob") }]]));
    deps = {
      repo: "o/r",
      getSettings: db.getAssignSettings,
      getLink: db.getByDiscord,
      isInPool: db.isInPool,
      getGrant: db.getGrant,
      listGrantsFor: db.listGrantsFor,
      claimIssue: db.claimIssue,
      releaseIssue: db.releaseIssue,
      getIssue: vi.fn(async (_r, n) => issues.get(n) ?? null),
      listIssueComments: vi.fn(async (_r, n) => comments.get(n) ?? []),
      addAssignee: vi.fn(async (_r, n, login) => {
        const i = issues.get(n)!;
        i.assignees = [...i.assignees, login];
        return i.assignees;
      }),
    };
  });

  const msg = (content: string, discordId = "u1", roles = ["real", "core"], channelId = "chan") => ({
    discordId, channelId, memberRoles: new Set(roles), content,
  });

  it("ignores other channels and messages without refs", async () => {
    expect(await processApplication(msg("#10", "u1", ["real", "core"], "other"), deps)).toEqual({ kind: "ignore" });
    expect(await processApplication(msg("hello"), deps)).toEqual({ kind: "ignore" });
  });

  it("declines when off, wrong roles or not linked", async () => {
    expect((await processApplication(msg("#10", "u1", ["core"]), deps)).kind).toBe("declined");
    expect((await processApplication(msg("#10", "nobody"), deps)).kind).toBe("declined");
    db.updateAssignSettings({ enabled: false });
    expect(await processApplication(msg("#10"), deps)).toMatchObject({ kind: "declined", reason: expect.stringMatching(/closed/) });
  });

  it("assigns eligible issues", async () => {
    const r = await processApplication(msg("#10 #11"), deps);
    expect(r).toEqual({ kind: "processed", outcomes: [
      { issue: 10, ok: true, url: "https://gh/10" },
      { issue: 11, ok: true, url: "https://gh/11" },
    ] });
    expect(db.listGrantsFor("u1").map((g) => g.issueNumber)).toEqual([10, 11]);
  });

  it("declines the whole request when over the max", async () => {
    const r = await processApplication(msg("#10 #11 #12"), deps);
    expect(r.kind).toBe("declined");
    expect(deps.addAssignee).not.toHaveBeenCalled();

    await processApplication(msg("#10"), deps);
    const r2 = await processApplication(msg("#11 #12"), deps);
    expect(r2).toMatchObject({ kind: "declined", reason: expect.stringMatching(/1 more/) });
  });

  it("frees the cap once a bot-given issue is closed", async () => {
    await processApplication(msg("#10 #11"), deps);
    issues.get(10)!.state = "closed";
    const r = await processApplication(msg("#12"), deps);
    expect(r).toMatchObject({ kind: "processed", outcomes: [{ issue: 12, ok: true }] });
  });

  it("rejects issues outside the pool, taken, closed, or without a Wave application", async () => {
    issues.set(20, issue(20));
    issues.get(11)!.assignees = ["carol"];
    issues.get(12)!.state = "closed";
    comments.set(13, []);
    db.updateAssignSettings({ maxPerUser: 10 });
    const r = await processApplication(msg("#20 #11 #12 #13"), deps);
    expect(r.kind).toBe("processed");
    const outs = (r as { outcomes: { ok: boolean; reason?: string }[] }).outcomes;
    expect(outs.every((o) => !o.ok)).toBe(true);
    expect(outs.map((o) => o.reason)).toEqual([
      "isn't open for applications",
      "is already taken",
      "is closed",
      expect.stringMatching(/Drips Wave/),
    ]);
  });

  it("skips issues someone else already has and assigns the rest (first come, first served)", async () => {
    await processApplication(msg("#10", "u2"), deps);
    const r = await processApplication(msg("#10 #11"), deps);
    expect(r).toMatchObject({ kind: "processed", outcomes: [
      { issue: 10, ok: false, reason: "is already taken" },
      { issue: 11, ok: true },
    ] });
    expect(issues.get(10)!.assignees).toEqual(["bob"]);
  });

  it("does not hand the same issue to two people", async () => {
    await processApplication(msg("#10"), deps);
    const r = await processApplication(msg("#10", "u2"), deps);
    expect(r).toMatchObject({ outcomes: [{ ok: false, reason: "is already taken" }] });
  });

  it("releases the claim when GitHub drops the assignee", async () => {
    deps.addAssignee = vi.fn(async () => []);
    const r = await processApplication(msg("#10"), deps);
    expect(r).toMatchObject({ outcomes: [{ ok: false }] });
    expect(db.getGrant(10)).toBeNull();
  });
});

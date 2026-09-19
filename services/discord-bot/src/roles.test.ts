import { describe, it, expect, vi } from "vitest";
import type { Guild, GuildMember, GuildMemberRoleManager } from "discord.js";
import { syncRoles, type Tiers } from "./roles.js";

const tiers: Tiers = {
  contributor: "role_c",
  coreContributor: "role_core",
  contributorThreshold: 1,
  coreContributorThreshold: 3,
};

/** Build a mock guild + member with the given initial role ids. */
function mockGuild(memberId: string, initialRoles: string[]) {
  const add = vi.fn();
  const remove = vi.fn();
  const cache = { has: (id: string) => initialRoles.includes(id) };
  const member = {
    roles: { cache, add, remove } as unknown as GuildMemberRoleManager,
  } as unknown as GuildMember;
  const guild = {
    members: { fetch: vi.fn().mockResolvedValue(member) },
  } as unknown as Guild;
  return { guild, add, remove };
}

describe("syncRoles", () => {
  it("assigns Contributor at threshold", async () => {
    const { guild, add, remove } = mockGuild("u1", []);
    const r = await syncRoles(guild, "u1", 1, tiers);
    expect(r.targetTier).toBe("contributor");
    expect(add).toHaveBeenCalledWith("role_c", expect.any(String));
    expect(remove).not.toHaveBeenCalled();
  });

  it("promotes to Core Contributor at the higher threshold", async () => {
    const { guild, add, remove } = mockGuild("u1", ["role_c"]);
    const r = await syncRoles(guild, "u1", 3, tiers);
    expect(r.targetTier).toBe("core");
    expect(add).toHaveBeenCalledWith("role_core", expect.any(String));
    expect(remove).toHaveBeenCalledWith("role_c", expect.any(String));
  });

  it("no-op when already at correct tier", async () => {
    const { guild, add, remove } = mockGuild("u1", ["role_core"]);
    await syncRoles(guild, "u1", 5, tiers);
    expect(add).not.toHaveBeenCalled();
    expect(remove).not.toHaveBeenCalled();
  });

  it("revokes both roles when merged PRs drop below Contributor threshold", async () => {
    const { guild, remove } = mockGuild("u1", ["role_c", "role_core"]);
    const r = await syncRoles(guild, "u1", 0, tiers);
    expect(r.targetTier).toBe("none");
    expect(remove).toHaveBeenCalledWith("role_c", expect.any(String));
    expect(remove).toHaveBeenCalledWith("role_core", expect.any(String));
  });

  it("survives when Discord member cannot be fetched", async () => {
    const guild = {
      members: { fetch: vi.fn().mockRejectedValue(new Error("unknown")) },
    } as unknown as Guild;
    const r = await syncRoles(guild, "u1", 5, tiers);
    expect(r.granted).toEqual([]);
    expect(r.revoked).toEqual([]);
  });
});

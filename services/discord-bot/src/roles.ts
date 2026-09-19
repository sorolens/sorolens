/**
 * Role assignment logic.
 *
 * The bot idempotently converges a member's roles to the tier their
 * merged-PR count places them in. A member with 5 merged PRs holds
 * "Core Contributor" only, not both "Contributor" and "Core Contributor".
 * That matches the Discord convention of one role per level.
 */

import type { Guild } from "discord.js";

export interface Tiers {
  contributor: string;
  coreContributor: string;
  contributorThreshold: number;
  coreContributorThreshold: number;
}

export interface AssignmentResult {
  discordId: string;
  mergedPRs: number;
  targetTier: "core" | "contributor" | "none";
  granted: string[];
  revoked: string[];
}

/**
 * Given a member's Discord id and their merged-PR count, converge their
 * role membership to the correct tier.
 */
export async function syncRoles(
  guild: Guild,
  discordId: string,
  mergedPRs: number,
  tiers: Tiers,
): Promise<AssignmentResult> {
  const target: AssignmentResult["targetTier"] =
    mergedPRs >= tiers.coreContributorThreshold
      ? "core"
      : mergedPRs >= tiers.contributorThreshold
        ? "contributor"
        : "none";

  const member = await guild.members.fetch(discordId).catch(() => null);
  if (!member) {
    return { discordId, mergedPRs, targetTier: target, granted: [], revoked: [] };
  }

  const hasContrib = member.roles.cache.has(tiers.contributor);
  const hasCore = member.roles.cache.has(tiers.coreContributor);

  const grant: string[] = [];
  const revoke: string[] = [];

  if (target === "core") {
    if (!hasCore) grant.push(tiers.coreContributor);
    if (hasContrib) revoke.push(tiers.contributor);
  } else if (target === "contributor") {
    if (!hasContrib) grant.push(tiers.contributor);
    if (hasCore) revoke.push(tiers.coreContributor);
  } else {
    if (hasContrib) revoke.push(tiers.contributor);
    if (hasCore) revoke.push(tiers.coreContributor);
  }

  for (const r of grant) {
    await member.roles.add(r, `sorolens-bot: mergedPRs=${mergedPRs}`);
  }
  for (const r of revoke) {
    await member.roles.remove(r, `sorolens-bot: mergedPRs=${mergedPRs}`);
  }

  return {
    discordId,
    mergedPRs,
    targetTier: target,
    granted: grant,
    revoked: revoke,
  };
}

/**
 * GitHub API helpers.
 *
 * We use the search API to count merged PRs by author because it is one
 * network round trip instead of paginating through pulls?state=closed.
 * The search API is rate-limited more aggressively (30 req/min for
 * authenticated calls) which is comfortably more headroom than a
 * contributor bot needs.
 */

import { Octokit } from "@octokit/rest";

let client: Octokit | null = null;

export function initGithub(token: string): Octokit {
  client = new Octokit({ auth: token, userAgent: "sorolens-discord-bot" });
  return client;
}

function requireClient(): Octokit {
  if (!client) throw new Error("github client not initialised");
  return client;
}

/** Number of merged PRs authored by `login` against `repo` (e.g. "sorolens/sorolens"). */
export async function countMergedPRs(login: string, repo: string): Promise<number> {
  const gh = requireClient();
  const q = `repo:${repo} is:pr is:merged author:${login}`;
  const r = await gh.search.issuesAndPullRequests({
    q,
    per_page: 1,
  });
  return r.data.total_count;
}

/** Validate a GitHub login exists (returns null if 404). */
export async function getUser(login: string): Promise<{ login: string; id: number } | null> {
  const gh = requireClient();
  try {
    const r = await gh.users.getByUsername({ username: login });
    return { login: r.data.login, id: r.data.id };
  } catch (e: unknown) {
    if (e && typeof e === "object" && "status" in e && (e as { status: number }).status === 404) {
      return null;
    }
    throw e;
  }
}

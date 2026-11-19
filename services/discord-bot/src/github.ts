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
export async function countMergedPRs(
  login: string,
  repo: string
): Promise<number> {
  const gh = requireClient();
  const q = `repo:${repo} is:pr is:merged author:${login}`;
  const r = await gh.search.issuesAndPullRequests({
    q,
    per_page: 1,
  });
  return r.data.total_count;
}

/** Validate a GitHub login exists (returns null if 404). */
export async function getUser(
  login: string
): Promise<{ login: string; id: number } | null> {
  const gh = requireClient();
  try {
    const r = await gh.users.getByUsername({ username: login });
    return { login: r.data.login, id: r.data.id };
  } catch (e: unknown) {
    if (
      e &&
      typeof e === "object" &&
      "status" in e &&
      (e as { status: number }).status === 404
    ) {
      return null;
    }
    throw e;
  }
}

function splitRepo(repo: string): { owner: string; repo: string } {
  const [owner, name] = repo.split("/");
  if (!owner || !name) throw new Error(`invalid repo "${repo}"`);
  return { owner, repo: name };
}

export interface IssueInfo {
  number: number;
  state: "open" | "closed";
  isPullRequest: boolean;
  assignees: string[];
  title: string;
  url: string;
}

/** Fetch an issue, or null when it does not exist. */
export async function getIssue(
  repo: string,
  issueNumber: number
): Promise<IssueInfo | null> {
  const gh = requireClient();
  try {
    const r = await gh.issues.get({ ...splitRepo(repo), issue_number: issueNumber });
    return {
      number: r.data.number,
      state: r.data.state === "closed" ? "closed" : "open",
      isPullRequest: Boolean(r.data.pull_request),
      assignees: (r.data.assignees ?? []).map((a) => a.login),
      title: r.data.title,
      url: r.data.html_url,
    };
  } catch (e: unknown) {
    if (
      e &&
      typeof e === "object" &&
      "status" in e &&
      ((e as { status: number }).status === 404 ||
        (e as { status: number }).status === 410)
    ) {
      return null;
    }
    throw e;
  }
}

export interface IssueComment {
  author: string;
  authorType: string;
  body: string;
}

/** Every comment on an issue (paginated). */
export async function listIssueComments(
  repo: string,
  issueNumber: number
): Promise<IssueComment[]> {
  const gh = requireClient();
  const all = await gh.paginate(gh.issues.listComments, {
    ...splitRepo(repo),
    issue_number: issueNumber,
    per_page: 100,
  });
  return all.map((c) => ({
    author: c.user?.login ?? "",
    authorType: c.user?.type ?? "",
    body: c.body ?? "",
  }));
}

/**
 * Assign `login` to an issue. GitHub silently drops assignees it cannot
 * assign, so the caller gets back the resulting assignee list to verify.
 */
export async function addAssignee(
  repo: string,
  issueNumber: number,
  login: string
): Promise<string[]> {
  const gh = requireClient();
  const r = await gh.issues.addAssignees({
    ...splitRepo(repo),
    issue_number: issueNumber,
    assignees: [login],
  });
  return (r.data.assignees ?? []).map((a) => a.login);
}

export interface OpenIssue {
  number: number;
  title: string;
  comments: number;
}

/** Open issues with no assignee (pull requests excluded), optionally by label. */
export async function listOpenUnassignedIssues(
  repo: string,
  label?: string
): Promise<OpenIssue[]> {
  const gh = requireClient();
  const all = await gh.paginate(gh.issues.listForRepo, {
    ...splitRepo(repo),
    state: "open",
    assignee: "none",
    ...(label ? { labels: label } : {}),
    per_page: 100,
  });
  return all
    .filter((i) => !i.pull_request)
    .map((i) => ({ number: i.number, title: i.title, comments: i.comments }));
}

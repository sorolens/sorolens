// Pre-warms the Next.js dev server so the first (cold) compile of each route
// does not eat into per-test timeouts when tests run in parallel.
export default async function globalSetup(): Promise<void> {
  const base = "http://localhost:3000";
  const paths = [
    "/",
    "/contracts",
    "/contracts/new",
    "/contracts/CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C33",
    "/live",
    "/watchdog",
    "/watchdog/CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C33",
    "/playground",
    "/settings",
  ];

  for (const path of paths) {
    let warmed = false;
    let lastFailure = "unknown error";

    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        const res = await fetch(base + path, {
          signal: AbortSignal.timeout(90_000),
        });
        if (res.ok) {
          warmed = true;
          break;
        }
        lastFailure = `HTTP ${res.status}`;
        console.warn(`[e2e warmup] ${path} -> ${res.status}, retrying`);
      } catch (err) {
        lastFailure = String(err);
        console.warn(`[e2e warmup] ${path} failed: ${String(err)}, retrying`);
        await new Promise((r) => setTimeout(r, 2000));
      }
    }

    if (!warmed) {
      throw new Error(
        `[e2e warmup] failed for ${path} after 3 attempts: ${lastFailure}`
      );
    }
  }
}

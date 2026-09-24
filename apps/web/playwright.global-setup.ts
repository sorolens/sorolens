// Pre-warms the Next.js dev server so the first (cold) compile of each route
// does not eat into per-test timeouts when tests run in parallel.
export default async function globalSetup(): Promise<void> {
  const base = "http://localhost:3000";
  const paths = [
    "/",
    "/contracts",
    "/contracts/CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C33",
    "/watchdog",
    "/watchdog/CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C33",
    "/playground",
  ];

  for (const path of paths) {
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        const res = await fetch(base + path, {
          signal: AbortSignal.timeout(90_000),
        });
        if (res.ok) break;
        console.warn(`[e2e warmup] ${path} -> ${res.status}, retrying`);
      } catch (err) {
        console.warn(`[e2e warmup] ${path} failed: ${String(err)}, retrying`);
        await new Promise((r) => setTimeout(r, 2000));
      }
    }
  }
}

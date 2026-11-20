import { test, expect, type Page } from "@playwright/test";
import { CONTRACT_ID, mockApi } from "./helpers";

const PAGES = [
  "/",
  "/contracts",
  `/contracts/${CONTRACT_ID}`,
  "/events",
  "/invocations",
  "/watchdog",
  "/settings",
];

interface Violation {
  directive: string;
  blocked: string;
}

/** Records every violation the browser raises, from the first script on. */
async function trackViolations(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const store: { directive: string; blocked: string }[] = [];
    Object.defineProperty(window, "__cspViolations", { value: store });
    document.addEventListener("securitypolicyviolation", (event) => {
      store.push({
        directive: event.effectiveDirective,
        blocked: event.blockedURI,
      });
    });
  });
}

function readViolations(page: Page): Promise<Violation[]> {
  return page.evaluate(
    () =>
      (window as unknown as { __cspViolations: Violation[] }).__cspViolations
  );
}

for (const path of PAGES) {
  test(`${path} loads without CSP violations`, async ({ page }) => {
    const consoleViolations: string[] = [];
    page.on("console", (message) => {
      if (message.text().includes("Content Security Policy")) {
        consoleViolations.push(message.text());
      }
    });
    await trackViolations(page);
    await mockApi(page);

    await page.goto(path);
    await page.waitForLoadState("networkidle");

    expect(await readViolations(page)).toEqual([]);
    expect(consoleViolations).toEqual([]);
  });
}

test("the violation check fails when a page breaks the policy", async ({
  page,
}) => {
  await trackViolations(page);
  await mockApi(page);
  await page.goto("/contracts");

  // An injected inline event handler, the shape of a typical XSS payload.
  // script-src blocks it whatever 'strict-dynamic' trusts.
  await page.evaluate(() => {
    const img = document.createElement("img");
    img.setAttribute("onerror", "window.__cspBypassed = true;");
    img.src = "data:image/png;base64,invalid";
    document.body.appendChild(img);
  });
  await page.waitForTimeout(500);
  const executed = await page.evaluate(
    () => (window as unknown as { __cspBypassed?: boolean }).__cspBypassed
  );

  expect(executed).toBeUndefined();
  await expect
    .poll(() => readViolations(page))
    .toContainEqual(
      expect.objectContaining({
        directive: "script-src-attr",
        blocked: "inline",
      })
    );
});

test("every document gets its own nonce", async ({ request }) => {
  const nonceOf = async () => {
    const res = await request.get("/contracts");
    const policy = res.headers()["content-security-policy"];
    expect(policy).toContain("'strict-dynamic'");
    const html = await res.text();
    const nonce = /'nonce-([^']+)'/.exec(policy)?.[1];
    expect(nonce).toBeTruthy();
    expect(html).toContain(`nonce="${nonce}"`);
    return nonce;
  };

  expect(await nonceOf()).not.toBe(await nonceOf());
});

test("the report endpoint validates what it receives", async ({ request }) => {
  const valid = await request.post("/api/csp-report", {
    headers: { "content-type": "application/csp-report" },
    data: JSON.stringify({
      "csp-report": {
        "document-uri": "http://localhost:3000/contracts",
        "effective-directive": "script-src-elem",
        "blocked-uri": "inline",
      },
    }),
  });
  expect(valid.status()).toBe(204);

  const invalid = await request.post("/api/csp-report", {
    headers: { "content-type": "application/csp-report" },
    data: JSON.stringify({ nope: true }),
  });
  expect(invalid.status()).toBe(400);
});

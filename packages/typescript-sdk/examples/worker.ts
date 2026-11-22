// A Cloudflare Worker that alerts on watchdog webhooks and can list contracts.
// Deploy with `wrangler deploy` after setting SOROLENS_API_KEY and
// SOROLENS_WEBHOOK_SECRET (see README). No Node APIs are used.
import { SorolensClient, verifyWebhook } from "../dist/esm/index.js";

export interface Env {
  SOROLENS_API_KEY?: string;
  SOROLENS_WEBHOOK_SECRET: string;
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    if (url.pathname === "/webhooks/sorolens" && request.method === "POST") {
      const rawBody = await request.text();
      const valid = await verifyWebhook({
        secret: env.SOROLENS_WEBHOOK_SECRET,
        signature: request.headers.get("X-Sorolens-Signature"),
        timestamp: request.headers.get("X-Sorolens-Timestamp"),
        body: rawBody,
      });
      if (!valid) return new Response("invalid signature", { status: 401 });
      // Parse only after verification succeeded.
      const alert = JSON.parse(rawBody) as { severity?: string };
      return Response.json({ received: true, severity: alert.severity ?? "unknown" });
    }

    if (url.pathname === "/contracts") {
      const client = new SorolensClient({
        apiKey: env.SOROLENS_API_KEY,
        retry: { maxRetries: 3 },
      });
      const contracts: unknown[] = [];
      for await (const contract of client.contracts({ limit: 25 })) contracts.push(contract);
      return Response.json({ contracts });
    }

    return new Response("not found", { status: 404 });
  },
};

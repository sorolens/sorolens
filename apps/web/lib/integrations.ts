/**
 * Client for the integrations catalog (issue #612).
 *
 * `GET /api/v1/integrations` is public; the test-send call triggers an outbound
 * request and therefore needs a contributor identity, which the dashboard
 * provides with its workspace session.
 */

import { ApiError } from "./api";
import { API_URL } from "./config";
import { ensureSession } from "./user";

export interface Integration {
  id: "slack" | "discord" | "telegram";
  name: string;
  description: string;
  install_url: string;
  setup_steps: string[];
  configured: boolean;
}

export interface TestIntegrationResult {
  ok: boolean;
  detail?: string;
  error?: string;
}

async function errorFrom(res: Response): Promise<ApiError> {
  let body: { error?: string; code?: string } = {};
  try {
    body = await res.json();
  } catch {
    // ignore parse error
  }
  return new ApiError(res.status, body.error || res.statusText, body.code);
}

export async function listIntegrations(): Promise<Integration[]> {
  const res = await fetch(`${API_URL}/api/v1/integrations`);
  if (!res.ok) throw await errorFrom(res);
  const body = (await res.json()) as { integrations?: Integration[] };
  return body.integrations ?? [];
}

export async function testIntegration(
  id: Integration["id"]
): Promise<TestIntegrationResult> {
  const session = await ensureSession();
  const res = await fetch(
    `${API_URL}/api/v1/integrations/${encodeURIComponent(id)}/test`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${session.token}`,
      },
    }
  );

  // A 409/502 still carries an actionable error for the user.
  let body: Partial<TestIntegrationResult> = {};
  try {
    body = (await res.json()) as Partial<TestIntegrationResult>;
  } catch {
    // ignore parse error
  }
  if (!res.ok || body.ok === false) {
    throw new ApiError(
      res.status,
      body.error || `Test message failed (${res.status})`
    );
  }
  return { ok: true, detail: body.detail };
}

import { NextResponse } from "next/server";
import {
  CHANNELS,
  formatTestPayload,
  type SeverityFilter,
} from "@/lib/notifications";
import type { ChannelType } from "@/lib/types";

export const dynamic = "force-dynamic";

interface TestNotificationRequestBody {
  contract_id?: string;
  channel_type?: ChannelType;
  destination?: string;
  severity?: SeverityFilter;
}

export async function POST(req: Request) {
  let body: TestNotificationRequestBody;
  try {
    body = (await req.json()) as TestNotificationRequestBody;
  } catch {
    return NextResponse.json(
      { success: false, error: "Invalid JSON body" },
      { status: 400 }
    );
  }

  const { contract_id, channel_type, destination, severity } = body;

  if (!contract_id || !contract_id.trim()) {
    return NextResponse.json(
      { success: false, error: "contract_id is required" },
      { status: 400 }
    );
  }

  if (!channel_type || !CHANNELS[channel_type]) {
    return NextResponse.json(
      {
        success: false,
        error:
          "channel_type must be one of: slack, discord, pagerduty, webhook",
      },
      { status: 400 }
    );
  }

  if (!destination || !destination.trim()) {
    return NextResponse.json(
      {
        success: false,
        error: `${CHANNELS[channel_type].destinationLabel} is required`,
      },
      { status: 400 }
    );
  }

  const formatted = formatTestPayload({
    contractId: contract_id,
    channel: channel_type,
    destination,
    severity,
  });

  try {
    const res = await fetch(formatted.url, {
      method: "POST",
      headers: formatted.headers,
      body: JSON.stringify(formatted.body),
      signal: AbortSignal.timeout(10_000),
    });

    if (!res.ok) {
      let details = "";
      try {
        const text = await res.text();
        details = text.slice(0, 200).trim();
      } catch {
        // ignore body read error
      }
      return NextResponse.json(
        {
          success: false,
          error: `Upstream channel returned status ${res.status}${
            details ? `: ${details}` : ""
          }`,
        },
        { status: res.status >= 400 && res.status < 500 ? 400 : 502 }
      );
    }

    return NextResponse.json({
      success: true,
      message: `Test notification sent successfully to ${CHANNELS[channel_type].label}.`,
    });
  } catch (err: unknown) {
    const message =
      err instanceof Error ? err.message : "Failed to deliver notification";
    return NextResponse.json(
      { success: false, error: message },
      { status: 502 }
    );
  }
}

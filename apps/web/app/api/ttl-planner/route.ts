import { NextRequest, NextResponse } from "next/server";

const ALLOWED_NETWORKS = ["testnet", "mainnet", "futurenet"] as const;
const BASE64_RE = /^[A-Za-z0-9+/]+={0,2}$/;
const MAX_KEY_LENGTH = 200;
const MAX_KEYS = 1000;

interface PlanRequest {
  keys: string[]; // base64 XDR LedgerKeys
  newLiveUntilLedger: number;
  network: string;
}

interface PlanResponse {
  cli_command: string;
  entry_count: number;
  estimated_fee_stroops: number | null;
}

export async function POST(req: NextRequest): Promise<NextResponse> {
  let body: PlanRequest;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: "invalid JSON" }, { status: 400 });
  }

  if (!body || typeof body !== "object") {
    return NextResponse.json({ error: "invalid body" }, { status: 400 });
  }

  const { keys, newLiveUntilLedger } = body;
  if (!Array.isArray(keys) || keys.length === 0) {
    return NextResponse.json({ error: "keys required" }, { status: 400 });
  }
  if (keys.length > MAX_KEYS) {
    return NextResponse.json({ error: "too many keys" }, { status: 400 });
  }
  if (
    !keys.every(
      (k) =>
        typeof k === "string" &&
        k.length > 0 &&
        k.length <= MAX_KEY_LENGTH &&
        BASE64_RE.test(k)
    )
  ) {
    return NextResponse.json(
      { error: "keys must be base64 strings of at most 200 characters" },
      { status: 400 }
    );
  }
  if (
    typeof newLiveUntilLedger !== "number" ||
    !Number.isSafeInteger(newLiveUntilLedger) ||
    newLiveUntilLedger <= 0
  ) {
    return NextResponse.json(
      { error: "newLiveUntilLedger must be a positive integer" },
      { status: 400 }
    );
  }

  const network = body.network ?? "testnet";
  if (
    typeof network !== "string" ||
    !(ALLOWED_NETWORKS as readonly string[]).includes(network)
  ) {
    return NextResponse.json(
      { error: "network must be one of testnet, mainnet, futurenet" },
      { status: 400 }
    );
  }

  const keyArgs = keys.map((k) => `  --key "${k}"`).join(" \\\n");
  const cli_command =
    `stellar contract extend \\\n` +
    `  --network ${network} \\\n` +
    `  --ledgers-to-expire ${newLiveUntilLedger} \\\n` +
    keyArgs;

  return NextResponse.json({
    cli_command,
    entry_count: keys.length,
    estimated_fee_stroops: null,
  } satisfies PlanResponse);
}

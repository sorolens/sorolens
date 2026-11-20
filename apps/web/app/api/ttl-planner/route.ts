import { NextRequest, NextResponse } from "next/server";

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

  if (!body.keys || body.keys.length === 0) {
    return NextResponse.json({ error: "keys required" }, { status: 400 });
  }
  if (!body.newLiveUntilLedger || body.newLiveUntilLedger <= 0) {
    return NextResponse.json(
      { error: "newLiveUntilLedger must be positive" },
      { status: 400 },
    );
  }

  const network = body.network || "testnet";
  const keyArgs = body.keys
    .map((k) => `  --key "${k}"`)
    .join(" \\\n");
  const cli_command =
    `stellar contract extend \\\n` +
    `  --network ${network} \\\n` +
    `  --ledgers-to-expire ${body.newLiveUntilLedger} \\\n` +
    keyArgs;

  return NextResponse.json({
    cli_command,
    entry_count: body.keys.length,
    estimated_fee_stroops: null,
  } satisfies PlanResponse);
}

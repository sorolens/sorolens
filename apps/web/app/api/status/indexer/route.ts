import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

function parseIndexerMetrics(text: string) {
  const networks = new Map<
    string,
    {
      network: string;
      last_indexed_ledger: number | null;
      lag_ledgers: number | null;
    }
  >();

  for (const line of text.split("\n")) {
    const match = line.match(
      /^sorolens_indexer_(last_indexed_ledger|lag_ledgers)\{network="((?:\\.|[^"\\])*)"\}\s+([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)\s*$/
    );
    if (!match) continue;

    let network: string;
    try {
      network = JSON.parse(`"${match[2]}"`);
    } catch {
      continue;
    }
    const value = Number(match[3]);
    if (!Number.isFinite(value)) continue;

    const row = networks.get(network) ?? {
      network,
      last_indexed_ledger: null,
      lag_ledgers: null,
    };
    if (match[1] === "last_indexed_ledger") {
      row.last_indexed_ledger = value;
    } else {
      row.lag_ledgers = value;
    }
    networks.set(network, row);
  }

  return {
    networks: [...networks.values()].sort((a, b) =>
      a.network.localeCompare(b.network)
    ),
  };
}

export async function GET() {
  const metricsURL =
    process.env.INDEXER_METRICS_URL ??
    process.env.NEXT_PUBLIC_INDEXER_METRICS_URL ??
    "http://localhost:9100/metrics";

  try {
    const response = await fetch(metricsURL, {
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
    if (!response.ok) {
      return NextResponse.json(
        { error: "Indexer metrics unavailable" },
        { status: 502 }
      );
    }
    return NextResponse.json(parseIndexerMetrics(await response.text()));
  } catch {
    return NextResponse.json(
      { error: "Indexer metrics unavailable" },
      { status: 502 }
    );
  }
}

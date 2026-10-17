import { ImageResponse } from "next/og";
import type { CSSProperties } from "react";
import { contractOgCard } from "@/lib/og";
import type { StatsResponse } from "@/lib/types";
import {
  getContractForMetadata,
  getContractStatsForMetadata,
} from "@/lib/serverData";

export const size = { width: 1200, height: 630 };
export const contentType = "image/png";
export const alt = "Sorolens contract summary";

type Props = { params: Promise<{ id: string }> };

const bodyStyle: CSSProperties = {
  width: "100%",
  height: "100%",
  display: "flex",
  flexDirection: "column",
  justifyContent: "space-between",
  padding: "64px",
  background: "linear-gradient(135deg, #12101c 0%, #1c1735 55%, #2b1c53 100%)",
  color: "#f5f3ff",
  fontFamily: "sans-serif",
};

export default async function ContractOgImage({ params }: Props) {
  const { id } = await params;

  let label: string | null = null;
  let network: string | null = null;
  let stats: StatsResponse | null = null;
  try {
    const [contract, contractStats] = await Promise.all([
      getContractForMetadata(id),
      getContractStatsForMetadata(id),
    ]);
    label = contract?.label ?? null;
    network = contract?.network ?? null;
    stats = contractStats;
  } catch {
    // Fall through to the generic card below (issue #620: graceful fallback).
  }

  const card = contractOgCard({ id, label, network, stats });

  return new ImageResponse(
    <div style={bodyStyle}>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <div style={{ fontSize: 40, fontWeight: 700, letterSpacing: -1 }}>
          Sorolens
        </div>
        <div
          style={{
            display: "flex",
            padding: "10px 24px",
            borderRadius: 999,
            background: "rgba(124, 58, 237, 0.35)",
            border: "2px solid #7c3aed",
            fontSize: 28,
            textTransform: "capitalize",
          }}
        >
          {card.subtitle}
        </div>
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        <div style={{ fontSize: 76, fontWeight: 700, lineHeight: 1.05 }}>
          {card.title}
        </div>
        <div
          style={{
            display: "flex",
            fontSize: 30,
            color: "#c4b5fd",
            fontFamily: "monospace",
          }}
        >
          {card.shortId}
        </div>
      </div>

      <div style={{ display: "flex", gap: 48, fontSize: 30 }}>
        {card.stats.length > 0 ? (
          card.stats.map((stat) => (
            <div
              key={stat.label}
              style={{ display: "flex", flexDirection: "column", gap: 4 }}
            >
              <div style={{ fontSize: 44, fontWeight: 700 }}>{stat.value}</div>
              <div style={{ fontSize: 24, color: "#a5b4fc" }}>{stat.label}</div>
            </div>
          ))
        ) : (
          <div style={{ fontSize: 28, color: "#c4b5fd" }}>
            Events, invocations and storage for this contract
          </div>
        )}
      </div>
    </div>,
    { ...size }
  );
}

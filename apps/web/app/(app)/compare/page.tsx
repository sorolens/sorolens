import { CompareView } from "@/components/compare";
import type { TimeWindow } from "@/lib/types";

const SUPPORTED_WINDOWS: TimeWindow[] = ["24h", "7d", "30d"];
const DEFAULT_WINDOW: TimeWindow = "7d";
const MAX_IDS = 4;

interface ComparePageProps {
  searchParams: Promise<{ ids?: string; window?: string }>;
}

function parseIds(raw: string | undefined): string[] {
  if (!raw) return [];
  const seen = new Set<string>();
  const ids: string[] = [];
  for (const part of raw.split(",")) {
    const id = part.trim();
    if (!id || seen.has(id)) continue;
    seen.add(id);
    ids.push(id);
    if (ids.length === MAX_IDS) break;
  }
  return ids;
}

function parseWindow(raw: string | undefined): TimeWindow {
  return SUPPORTED_WINDOWS.includes(raw as TimeWindow)
    ? (raw as TimeWindow)
    : DEFAULT_WINDOW;
}

/**
 * Server component: reads ?ids=A,B&window=7d from the URL so a shared or
 * bookmarked comparison link renders its selection on first paint. All
 * interactive behaviour lives in the client CompareView.
 */
export default async function ComparePage({ searchParams }: ComparePageProps) {
  const params = await searchParams;
  return (
    <CompareView
      initialIds={parseIds(params.ids)}
      initialWindow={parseWindow(params.window)}
    />
  );
}

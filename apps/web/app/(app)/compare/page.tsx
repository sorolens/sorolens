import { CompareView } from "@/components/compare";
import { parseIds, parseWindow } from "@/lib/compareParams";

interface ComparePageProps {
  searchParams: Promise<{ ids?: string; window?: string }>;
}

/**
 * Server component: reads ?ids=A,B&window=7d from the URL so a shared or
 * bookmarked comparison link renders its selection on first paint. All
 * interactive behaviour lives in the client CompareView.
 */
export default async function ComparePage({ searchParams }: ComparePageProps) {
  const params = await searchParams;
  const parsed = parseIds(params.ids);
  return (
    <CompareView
      initialIds={parsed.ids}
      initialWindow={parseWindow(params.window)}
      droppedInvalid={parsed.dropped}
      truncatedExtra={parsed.truncated}
    />
  );
}

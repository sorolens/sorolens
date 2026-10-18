import { LiveStatsClient } from "@/components/LiveStatsClient";
import { getLandingStats } from "@/lib/serverData";

interface Props {
  className?: string;
}

export async function LiveStats({ className }: Props) {
  const initialData = await getLandingStats();
  return <LiveStatsClient className={className} initialData={initialData} />;
}

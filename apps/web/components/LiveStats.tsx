import { LiveStatsClient } from "@/components/LiveStatsClient";
import { getLandingStats } from "@/lib/serverData";

interface Props {
  className?: string;
}

/**
 * Server half of the landing-page stats. `generatedAt` is when these figures
 * were read here, which for an ISR-rendered page is the cache generation time —
 * the only thing that tells the client how old its own numbers are.
 */
export async function LiveStats({ className }: Props) {
  const stats = await getLandingStats();
  return (
    <LiveStatsClient
      className={className}
      initialData={{ ...stats, generatedAt: Date.now() }}
    />
  );
}

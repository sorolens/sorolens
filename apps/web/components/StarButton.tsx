import { useState, useTransition } from "react";
import { addToWatchlist, removeFromWatchlist } from "@/lib/api";

interface StarButtonProps {
  contractId: string;
  userId: string;
  initialInWatchlist?: boolean;
}

export function StarButton({ contractId, userId, initialInWatchlist = false }: StarButtonProps) {
  const [inWatchlist, setInWatchlist] = useState(initialInWatchlist);
  const [pending, startTransition] = useTransition();

  const toggle = () => {
    startTransition(async () => {
      try {
        if (inWatchlist) {
          await removeFromWatchlist(contractId, userId);
        } else {
          await addToWatchlist(contractId, userId);
        }
        setInWatchlist(!inWatchlist);
      } catch {
        // Silently handle - optimistic update will rollback on next fetch
      }
    });
  };

  return (
    <button
      type="button"
      onClick={toggle}
      disabled={pending}
      className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-sm transition-colors hover:bg-[var(--color-bg-card)]"
      aria-label={inWatchlist ? "Remove from watchlist" : "Add to watchlist"}
      title={inWatchlist ? "Remove from watchlist" : "Add to watchlist"}
    >
      <span
        className={
          inWatchlist
            ? "text-yellow-400"
            : "text-[var(--color-text-secondary)]"
        }
      >
        ★
      </span>
    </button>
  );
}

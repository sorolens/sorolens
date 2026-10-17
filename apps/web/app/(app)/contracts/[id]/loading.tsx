import {
  CardSkeleton,
  ChartSkeleton,
  TableSkeleton,
} from "@/components/Skeleton";

export default function ContractDetailLoading() {
  return (
    <div className="space-y-6" aria-label="Loading contract details">
      <CardSkeleton />
      <div className="grid gap-4 md:grid-cols-2">
        <ChartSkeleton />
        <ChartSkeleton />
      </div>
      <TableSkeleton rows={5} />
    </div>
  );
}

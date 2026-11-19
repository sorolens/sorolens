import { DashboardGrid } from "@/components/dashboard/DashboardGrid";

export const metadata = {
  title: "Dashboard",
  description:
    "A personal dashboard of Sorolens widgets: network stats, event volume, recent alerts and hot contracts.",
};

export default function DashboardPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
        <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
          Arrange the widgets that matter to you. Your layout is saved per
          browser and can be reset at any time.
        </p>
      </div>
      <DashboardGrid />
    </div>
  );
}

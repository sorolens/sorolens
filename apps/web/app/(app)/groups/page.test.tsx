/**
 * Tests for apps/web/app/(app)/groups/page.tsx
 *
 * vitest + @testing-library/react + jsdom. next/link is mocked to a plain <a>
 * and @/lib/api is mocked so the test controls the data returned.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock("@/components/Skeleton", () => ({
  CardSkeleton: () => <div data-testid="card-skeleton">skeleton</div>,
}));

const mockListGroups = vi.fn();
const mockCreateGroup = vi.fn();
const mockDeleteGroup = vi.fn();

vi.mock("@/lib/api", () => ({
  listGroups: (...args: unknown[]) => mockListGroups(...args),
  createGroup: (...args: unknown[]) => mockCreateGroup(...args),
  deleteGroup: (...args: unknown[]) => mockDeleteGroup(...args),
}));

expect.extend(matchers);

const GROUP_CORE = {
  id: "11111111-1111-4111-8111-111111111111",
  owner_id: "user_1",
  name: "Core protocol",
  created_at: "2026-01-01T00:00:00Z",
  stats: {
    group_id: "11111111-1111-4111-8111-111111111111",
    contract_count: 3,
    event_count: 120,
    invocation_count: 45,
    storage_entry_count: 9,
    average_health_score: 82.5,
  },
};

describe("GroupsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListGroups.mockResolvedValue({ groups: [GROUP_CORE] });
    mockCreateGroup.mockResolvedValue({ id: "new-group", name: "New" });
    mockDeleteGroup.mockResolvedValue({ deleted: true });
  });

  afterEach(() => {
    cleanup();
  });

  async function renderPage() {
    const { default: GroupsPage } = await import("@/app/(app)/groups/page");
    return render(<GroupsPage />);
  }

  it("renders the heading and each group's aggregate stat cards", async () => {
    await renderPage();

    expect(screen.getByRole("heading", { name: /groups/i })).toBeDefined();
    await waitFor(() =>
      expect(screen.getByText("Core protocol")).toBeDefined(),
    );

    expect(screen.getByText("120")).toBeDefined();
    expect(screen.getByText("45")).toBeDefined();
    expect(screen.getByText("9")).toBeDefined();
    expect(screen.getByText("82.5")).toBeDefined();
    expect(screen.getByText("3 contracts")).toBeDefined();
  });

  it("shows an empty state when the caller has no groups", async () => {
    mockListGroups.mockResolvedValue({ groups: [] });
    await renderPage();

    await waitFor(() =>
      expect(screen.getByText(/no groups yet/i)).toBeDefined(),
    );
  });

  it("creates a group with the trimmed name and reloads the list", async () => {
    await renderPage();
    await waitFor(() => expect(mockListGroups).toHaveBeenCalled());

    fireEvent.change(screen.getByLabelText(/new group name/i), {
      target: { value: "  Payments  " },
    });
    fireEvent.click(document.getElementById("create-group-btn")!);

    await waitFor(() => expect(mockCreateGroup).toHaveBeenCalledTimes(1));
    expect(mockCreateGroup.mock.calls[0][0]).toBe("Payments");
    await waitFor(() => expect(mockListGroups).toHaveBeenCalledTimes(2));
  });

  it("surfaces a confirmation error when creation fails", async () => {
    mockCreateGroup.mockRejectedValue(new Error("boom"));
    await renderPage();
    await waitFor(() => expect(mockListGroups).toHaveBeenCalled());

    fireEvent.change(screen.getByLabelText(/new group name/i), {
      target: { value: "Payments" },
    });
    fireEvent.click(document.getElementById("create-group-btn")!);

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toMatch(/could not create/i),
    );
  });
});

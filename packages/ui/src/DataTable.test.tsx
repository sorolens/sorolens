import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DataTable } from "./DataTable";
import type { Column } from "./DataTable";

expect.extend(matchers);

afterEach(() => {
  cleanup();
});

interface Item {
  id: string;
  name: string;
  count: number;
}

const COLUMNS: Column<Item>[] = [
  { key: "id", header: "ID", sortable: true },
  {
    key: "name",
    header: "Name",
    sortable: true,
    accessor: (item) => <strong>{item.name}</strong>,
  },
  { key: "count", header: "Count" },
];

const DATA: Item[] = [
  { id: "a", name: "Alpha", count: 3 },
  { id: "b", name: "Beta", count: 1 },
  { id: "c", name: "Gamma", count: 2 },
];

describe("DataTable", () => {
  it("renders column headers", () => {
    render(
      <DataTable columns={COLUMNS} data={DATA} rowKey={(i) => i.id} />,
    );
    expect(screen.getByText("ID")).toBeDefined();
    expect(screen.getByText("Name")).toBeDefined();
    expect(screen.getByText("Count")).toBeDefined();
  });

  it("renders all rows using accessors", () => {
    render(
      <DataTable columns={COLUMNS} data={DATA} rowKey={(i) => i.id} />,
    );
    // 'a', 'b', 'c' come from default string accessor for id column
    expect(screen.getByText("a")).toBeDefined();
    expect(screen.getByText("b")).toBeDefined();
    // Custom accessor wraps name in <strong>
    expect(screen.getByText("Alpha")).toBeDefined();
    expect(screen.getByText("Beta")).toBeDefined();
  });

  it("shows loading skeleton instead of rows when loading=true", () => {
    const { container } = render(
      <DataTable
        columns={COLUMNS}
        data={DATA}
        rowKey={(i) => i.id}
        loading={true}
      />,
    );
    // Should not render table rows
    const tableEl = container.querySelector("table");
    expect(tableEl).toBeNull();
    // Should render skeleton divs with animate-pulse class
    const pulses = container.querySelectorAll(".animate-pulse");
    expect(pulses.length).toBeGreaterThan(0);
  });

  it("shows emptyState when data is empty", () => {
    render(
      <DataTable
        columns={COLUMNS}
        data={[]}
        rowKey={(i) => i.id}
        emptyState={<p>Nothing here</p>}
      />,
    );
    expect(screen.getByText("Nothing here")).toBeDefined();
  });

  it("does not show emptyState when data is non-empty", () => {
    render(
      <DataTable
        columns={COLUMNS}
        data={DATA}
        rowKey={(i) => i.id}
        emptyState={<p>Nothing here</p>}
      />,
    );
    expect(screen.queryByText("Nothing here")).toBeNull();
  });

  it("calls onSort with the column key when a sortable header is clicked", () => {
    const onSort = vi.fn();
    render(
      <DataTable
        columns={COLUMNS}
        data={DATA}
        rowKey={(i) => i.id}
        onSort={onSort}
        sortColumn="id"
        sortDirection="asc"
      />,
    );
    fireEvent.click(screen.getByText("Name"));
    expect(onSort).toHaveBeenCalledWith("name");
  });

  it("does NOT call onSort when a non-sortable header is clicked", () => {
    const onSort = vi.fn();
    render(
      <DataTable
        columns={COLUMNS}
        data={DATA}
        rowKey={(i) => i.id}
        onSort={onSort}
      />,
    );
    fireEvent.click(screen.getByText("Count"));
    expect(onSort).not.toHaveBeenCalled();
  });

  it("shows ▲ indicator on sorted-asc column and ▼ on sorted-desc", () => {
    const { rerender } = render(
      <DataTable
        columns={COLUMNS}
        data={DATA}
        rowKey={(i) => i.id}
        sortColumn="id"
        sortDirection="asc"
      />,
    );
    expect(screen.getByText("▲")).toBeDefined();

    rerender(
      <DataTable
        columns={COLUMNS}
        data={DATA}
        rowKey={(i) => i.id}
        sortColumn="id"
        sortDirection="desc"
      />,
    );
    expect(screen.getByText("▼")).toBeDefined();
  });

  it("calls onRowClick with the row item when a row is clicked", () => {
    const onRowClick = vi.fn();
    render(
      <DataTable
        columns={COLUMNS}
        data={DATA}
        rowKey={(i) => i.id}
        onRowClick={onRowClick}
      />,
    );
    // Click row containing "Alpha"
    fireEvent.click(screen.getByText("Alpha"));
    expect(onRowClick).toHaveBeenCalledWith(DATA[0]);
  });

  // Negative: loading=true should suppress rows even if data is present
  it("NEGATIVE: does not render table when loading even with data present", () => {
    const { container } = render(
      <DataTable
        columns={COLUMNS}
        data={DATA}
        rowKey={(i) => i.id}
        loading={true}
      />,
    );
    expect(container.querySelector("tbody")).toBeNull();
    expect(screen.queryByText("Alpha")).toBeNull();
  });
});

import * as matchers from "@testing-library/jest-dom/matchers";
import type { ReactNode } from "react";
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
    render(<DataTable columns={COLUMNS} data={DATA} rowKey={(i) => i.id} />);
    expect(screen.getByText("ID")).toBeDefined();
    expect(screen.getByText("Name")).toBeDefined();
    expect(screen.getByText("Count")).toBeDefined();
  });

  it("renders all rows using accessors", () => {
    render(<DataTable columns={COLUMNS} data={DATA} rowKey={(i) => i.id} />);
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
      />
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
      />
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
      />
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
      />
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
      />
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
      />
    );
    expect(screen.getByText("▲")).toBeDefined();

    rerender(
      <DataTable
        columns={COLUMNS}
        data={DATA}
        rowKey={(i) => i.id}
        sortColumn="id"
        sortDirection="desc"
      />
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
      />
    );
    // Click row containing "Alpha"
    fireEvent.click(screen.getByText("Alpha"));
    expect(onRowClick).toHaveBeenCalledWith(DATA[0]);
  });

  describe("accessibility (issue #610)", () => {
    it("marks every header cell as a column header with scope=col", () => {
      const { container } = render(
        <DataTable columns={COLUMNS} data={DATA} rowKey={(i) => i.id} />
      );
      const headers = container.querySelectorAll("th");
      expect(headers.length).toBe(COLUMNS.length);
      headers.forEach((th) => expect(th).toHaveAttribute("scope", "col"));
    });

    it("exposes aria-sort on sortable headers (none/ascending/descending)", () => {
      const { container, rerender } = render(
        <DataTable
          columns={COLUMNS}
          data={DATA}
          rowKey={(i) => i.id}
          sortColumn="id"
          sortDirection="asc"
        />
      );
      const th = (key: string) =>
        container.querySelector(`th[data-key="${key}"]`)!;
      expect(th("id")).toHaveAttribute("aria-sort", "ascending");
      expect(th("name")).toHaveAttribute("aria-sort", "none");
      expect(th("count")).not.toHaveAttribute("aria-sort");

      rerender(
        <DataTable
          columns={COLUMNS}
          data={DATA}
          rowKey={(i) => i.id}
          sortColumn="id"
          sortDirection="desc"
        />
      );
      expect(th("id")).toHaveAttribute("aria-sort", "descending");
    });

    it("renders sortable headers as real buttons (keyboard operable)", () => {
      const onSort = vi.fn();
      render(
        <DataTable
          columns={COLUMNS}
          data={DATA}
          rowKey={(i) => i.id}
          onSort={onSort}
        />
      );
      // The accessible control for the sortable "Name" header is a button.
      fireEvent.click(screen.getByRole("button", { name: /name/i }));
      expect(onSort).toHaveBeenCalledWith("name");
    });

    it("renders a real link in the row-link column when rowHref is set", () => {
      render(
        <DataTable
          columns={COLUMNS}
          data={DATA}
          rowKey={(i) => i.id}
          rowHref={(i) => `/items/${i.id}`}
        />
      );
      const links = screen.getAllByRole("link");
      expect(links.length).toBe(DATA.length);
      expect(links[0]).toHaveAttribute("href", "/items/a");
      expect(links[0]).toHaveTextContent("a");
    });

    it("puts the link in linkColumnKey when the first column is decorative", () => {
      const { container } = render(
        <DataTable
          columns={COLUMNS}
          data={DATA}
          rowKey={(i) => i.id}
          linkColumnKey="name"
          rowHref={(i) => `/items/${i.id}`}
        />
      );
      // Link lives in the Name cell, not the first (ID) cell.
      const firstRow = container.querySelector("tbody tr")!;
      const [idCell, nameCell] = firstRow.querySelectorAll("td");
      expect(idCell!.querySelector("a")).toBeNull();
      expect(nameCell!.querySelector("a")).toHaveAttribute("href", "/items/a");
    });

    it("uses the custom link component for router navigation", () => {
      const SpyLink = ({ href, children }: { href: string; children: ReactNode }) => (
        <a href={href} data-router="true">
          {children}
        </a>
      );
      render(
        <DataTable
          columns={COLUMNS}
          data={DATA}
          rowKey={(i) => i.id}
          rowHref={(i) => `/items/${i.id}`}
          linkComponent={SpyLink}
        />
      );
      expect(document.querySelector("a[data-router=\"true\"]")).not.toBeNull();
    });

    it("does not render links when rowHref returns null for a row", () => {
      render(
        <DataTable
          columns={COLUMNS}
          data={DATA}
          rowKey={(i) => i.id}
          rowHref={(i) => (i.id === "b" ? null : `/items/${i.id}`)}
        />
      );
      const links = screen.getAllByRole("link");
      expect(links.length).toBe(DATA.length - 1);
      expect(screen.queryByText("b").closest("a")).toBeNull();
    });
  });

  // Negative: loading=true should suppress rows even if data is present
  it("NEGATIVE: does not render table when loading even with data present", () => {
    const { container } = render(
      <DataTable
        columns={COLUMNS}
        data={DATA}
        rowKey={(i) => i.id}
        loading={true}
      />
    );
    expect(container.querySelector("tbody")).toBeNull();
    expect(screen.queryByText("Alpha")).toBeNull();
  });
});

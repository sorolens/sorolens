import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ResourcePercentilesTable } from "./ResourcePercentilesTable";

expect.extend(matchers);
afterEach(cleanup);

const percentiles = {
  cpu_insn: { p50: 120000, p95: 340000, p99: 800000 },
  mem_byte: { p50: 4096, p95: 16384, p99: 65536 },
  fee: { p50: 150, p95: 420.4, p99: 1100 },
};

describe("ResourcePercentilesTable", () => {
  it("renders one row per metric with p50, p95 and p99", () => {
    render(
      <ResourcePercentilesTable
        percentiles={percentiles}
        invocationCount={12}
        windowLabel="7d"
      />
    );

    expect(
      screen.getByRole("heading", { name: "Resource percentiles" })
    ).toBeInTheDocument();
    const cpu = screen.getByRole("row", { name: /CPU/ });
    expect(within(cpu).getByText("120,000")).toBeInTheDocument();
    expect(within(cpu).getByText("340,000")).toBeInTheDocument();
    expect(within(cpu).getByText("800,000")).toBeInTheDocument();
    const fee = screen.getByRole("row", { name: /Fee/ });
    expect(within(fee).getByText("420")).toBeInTheDocument();
    expect(screen.getByRole("row", { name: /Memory/ })).toBeInTheDocument();
  });

  it("renders nothing when the API omits percentiles", () => {
    const { container } = render(
      <ResourcePercentilesTable invocationCount={12} windowLabel="7d" />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when the window has no invocations", () => {
    const { container } = render(
      <ResourcePercentilesTable
        percentiles={percentiles}
        invocationCount={0}
        windowLabel="24h"
      />
    );
    expect(container).toBeEmptyDOMElement();
  });
});

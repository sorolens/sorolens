import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ContractForecastChart } from "./ContractForecastChart";

const getContractForecast = vi.fn();

vi.mock("@/lib/api", () => ({
  getContractForecast: (...args: unknown[]) => getContractForecast(...args),
}));

vi.mock("recharts", () => ({
  Area: () => null,
  AreaChart: ({
    children,
    data,
  }: {
    children: React.ReactNode;
    data: unknown[];
  }) => (
    <div data-testid="area-chart" data-points={data.length}>
      {children}
    </div>
  ),
  CartesianGrid: () => null,
  Line: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  Tooltip: () => null,
  XAxis: () => null,
  YAxis: () => null,
}));

afterEach(() => {
  cleanup();
  getContractForecast.mockReset();
});

const forecast = {
  contract_id: "contract-1",
  lookback_days: 90,
  series: [
    {
      metric: "fees",
      horizon: 30,
      daily_count: 2,
      points: [
        { date: "2026-10-18", value: 100, lower: 80, upper: 120 },
        { date: "2026-10-19", value: 110, lower: 85, upper: 135 },
      ],
    },
    { metric: "invocations", horizon: 30, daily_count: 0, points: [] },
    { metric: "events", horizon: 30, daily_count: 0, points: [] },
  ],
};

describe("ContractForecastChart", () => {
  it("renders forecast range and fetches the selected horizon", async () => {
    getContractForecast.mockResolvedValue(forecast);
    render(<ContractForecastChart contractId="contract-1" />);

    expect(await screen.findByTestId("forecast-chart")).toHaveAttribute(
      "aria-label",
      "Fees forecast with 95% confidence range"
    );
    expect(screen.getByTestId("area-chart")).toHaveAttribute(
      "data-points",
      "2"
    );

    fireEvent.change(screen.getByLabelText("Forecast horizon"), {
      target: { value: "14" },
    });
    await waitFor(() =>
      expect(getContractForecast).toHaveBeenLastCalledWith("contract-1", 14)
    );
  });

  it("shows an insufficient-data state", async () => {
    getContractForecast.mockResolvedValue({
      ...forecast,
      series: forecast.series.map((series) => ({ ...series, points: [] })),
    });
    render(<ContractForecastChart contractId="contract-1" />);

    expect(
      await screen.findByText(/not enough indexed data to forecast fees/i)
    ).toBeInTheDocument();
  });
});

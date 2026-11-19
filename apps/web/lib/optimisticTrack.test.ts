import { afterEach, expect, it, vi } from "vitest";
import { trackContractOptimistically, type ContractRow } from "./optimisticTrack";

afterEach(() => {
  vi.unstubAllGlobals();
});

it("restores the original list when the API rejects a new contract", async () => {
  const originalList: ContractRow[] = [];
  const setList = vi.fn();
  const input = { id: "contract-123", network: "testnet" };
  const response = Response.json({ error: "Invalid contract ID" }, { status: 400 });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));

  const result = await trackContractOptimistically(originalList, setList, input, {
    userId: "test-user",
    network: "testnet",
  });

  expect(setList).toHaveBeenNthCalledWith(1, [
    expect.objectContaining({ id: "contract-123", status: "pending" }),
  ]);
  expect(setList).toHaveBeenNthCalledWith(2, []);
  expect(result).toEqual({
    ok: false,
    message: "Couldn't track contract: Invalid contract ID",
  });
});

it("skips rollback when a newer list has already loaded", async () => {
  const setList = vi.fn();
  const input = { id: "contract-123", network: "testnet" };
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));

  const result = await trackContractOptimistically([], setList, input, {
    userId: "test-user",
    network: "testnet",
    shouldRollback: () => false,
  });

  expect(setList).toHaveBeenCalledTimes(1);
  expect(setList).toHaveBeenCalledWith([
    expect.objectContaining({ id: input.id, status: "pending" }),
  ]);
  expect(result).toEqual({
    ok: false,
    message: "Couldn't track contract. Please try again.",
  });
});

it("sends the registration request and keeps the pending row on success", async () => {
  const input = { id: "contract-123", network: "testnet" };
  const setList = vi.fn();
  const fetchMock = vi.fn().mockResolvedValue(Response.json(input));
  vi.stubGlobal("fetch", fetchMock);

  const result = await trackContractOptimistically([], setList, input, {
    userId: "test-user",
    network: "testnet",
  });

  expect(fetchMock).toHaveBeenCalledWith(
    expect.stringMatching(/\/api\/v1\/contracts$/),
    expect.objectContaining({ method: "POST", body: JSON.stringify(input) }),
  );
  expect(result).toEqual({ ok: true });
  expect(setList).toHaveBeenCalledTimes(1);
  expect(setList).toHaveBeenCalledWith([
    expect.objectContaining({ id: input.id, status: "pending" }),
  ]);
});
